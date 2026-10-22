package dagwidget

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/dag/dag"
)

// PropertiesPanel displays and edits the properties of the selected element:
// a node's component properties, a group's label and color, an annotation's
// text and anchor, or a link edge's label. At most one of
// node/group/annotation/linkEdge is set.
//
// Every change is its own undo step, down to each keystroke in a text
// field: typing "Merge" into an empty Name field takes five Undos to
// reverse. That is deliberate (#640); edits are not merged per field or per
// focus.
type PropertiesPanel struct {
	widget.BaseWidget

	Registry *dag.Registry
	Commands *CommandStack
	// CanvasRef is needed for node renaming and group/annotation editing:
	// commands carry the canvas, and anchor pickers enumerate the graph. Set
	// by the app screen after construction; without it a node shows no Name
	// field, and edits to a group, annotation or link store nothing.
	CanvasRef *DAGCanvas

	node       *dag.Node
	group      *dag.Group
	annotation *dag.Annotation
	linkEdge   *dag.LinkEdge
	component  *dag.Component
	header     *widget.Label
	typeLabel  *widget.Label
	streamInfo *widget.Label
	form       *widget.Form // the form on show; nil while the panel is empty
	content    *bellui.Container
	empty      *widget.Label
	rebuilding bool // true while a Set* rebuild is running, suppresses callbacks

	// syncers write the shown element's current values into the fields the
	// last Set* call built, in place. An edit or an undo goes through them
	// rather than a rebuild, which would replace the entry being typed in and
	// drop its focus. Each gets the shown node's properties, loaded once per
	// sync (empty for the other views).
	syncers []func(props map[string]any)
}

// NewPropertiesPanel creates a new properties panel.
func NewPropertiesPanel(registry *dag.Registry, cmds *CommandStack) *PropertiesPanel {
	p := &PropertiesPanel{
		Registry:   registry,
		Commands:   cmds,
		header:     widget.NewLabel("No selection"),
		typeLabel:  widget.NewLabel(""),
		streamInfo: widget.NewLabel(""),
		empty:      widget.NewLabel("Select a node to view properties"),
	}
	p.header.TextStyle.Bold = true
	// Node labels and port summaries can be long; wrap them inside the
	// panel's column rather than widening it.
	for _, l := range []*widget.Label{p.header, p.typeLabel, p.streamInfo} {
		l.Wrapping = bellui.TextWrapWord
	}
	p.content = container.NewVBox(p.empty)
	p.ExtendBaseWidget(p)
	return p
}

// SetNode updates the panel to show the properties of the given node, with a
// new form: a widget.Form keeps the rows it has drawn, so its Items cannot be
// swapped for another node's. Pass nil to clear the panel entirely (any mode).
func (p *PropertiesPanel) SetNode(n *dag.Node) {
	p.rebuilding = true
	defer func() { p.rebuilding = false }()
	p.syncers = nil
	p.form = nil

	p.node = n
	p.group = nil
	p.annotation = nil
	p.linkEdge = nil
	if n == nil {
		p.component = nil
		p.content.Objects = []core.CanvasObject{p.empty}
		p.content.Refresh()
		return
	}

	p.component = nil
	if p.Registry != nil {
		p.component = p.Registry.Get(n.TypeTag)
	}

	p.header.SetText(n.Label)
	p.typeLabel.SetText("Type: " + n.TypeTag)
	p.streamInfo.SetText(p.buildStreamInfo())
	var items []*widget.FormItem
	if p.CanvasRef != nil {
		items = append(items, widget.NewFormItem("Name", p.nameEntry(n)))
	}

	if p.component != nil {
		// Load current values from node metadata
		props := p.loadProps()

		for _, pd := range p.component.Properties {
			items = append(items, p.createFormItem(pd, props))
		}
	}
	p.form = widget.NewForm(items...)

	p.content.Objects = []core.CanvasObject{
		p.header,
		p.typeLabel,
		p.streamInfo,
		widget.NewSeparator(),
		p.form,
	}
	p.content.Refresh()
}

// nameEntry edits the node's label through RenameNodeCommand, one undoable
// step per change like the group label.
func (p *PropertiesPanel) nameEntry(n *dag.Node) *widget.Entry {
	entry := widget.NewEntry()
	entry.SetText(n.Label)
	entry.OnChanged = func(s string) {
		if p.rebuilding || p.node == nil || !p.CanvasRef.Capabilities.EditProperties {
			return
		}
		p.doCommand(&RenameNodeCommand{
			Canvas: p.CanvasRef,
			NodeID: p.node.ID,
			Old:    p.node.Label,
			New:    s,
			OnDone: p.syncForm,
		})
	}
	p.syncers = append(p.syncers, func(map[string]any) {
		cur := liveOr(p.CanvasRef.Graph.Nodes, n.ID, n)
		p.header.SetText(cur.Label)
		syncText(entry, cur.Label)
	})
	return entry
}

// SetGroup updates the panel to edit a group's label and color.
func (p *PropertiesPanel) SetGroup(g *dag.Group) {
	p.rebuilding = true
	defer func() { p.rebuilding = false }()
	p.syncers = nil
	p.form = nil

	p.node = nil
	p.component = nil
	p.annotation = nil
	p.linkEdge = nil
	p.group = g
	if g == nil {
		p.content.Objects = []core.CanvasObject{p.empty}
		p.content.Refresh()
		return
	}

	p.header.SetText(g.Label)
	p.typeLabel.SetText(fmt.Sprintf("Group · %d nodes", len(g.Members)))
	live := func() *dag.Group {
		if p.CanvasRef == nil {
			return g
		}
		return liveOr(p.CanvasRef.Graph.Groups, g.ID, g)
	}

	labelEntry := widget.NewEntry()
	labelEntry.SetText(g.Label)
	labelEntry.OnChanged = func(s string) {
		if p.rebuilding || p.group == nil {
			return
		}
		if p.CanvasRef == nil || !p.CanvasRef.Capabilities.EditProperties {
			return
		}
		p.doCommand(&SetGroupLabelCommand{
			Canvas:  p.CanvasRef,
			GroupID: p.group.ID,
			Old:     p.group.Label,
			New:     s,
			OnDone:  p.syncForm,
		})
	}
	p.syncers = append(p.syncers, func(map[string]any) {
		cur := live()
		p.header.SetText(cur.Label)
		syncText(labelEntry, cur.Label)
	})

	colorEntry := widget.NewEntry()
	colorEntry.SetText(g.Color)
	colorEntry.SetPlaceHolder("#rrggbb")
	colorEntry.OnChanged = func(s string) {
		if p.rebuilding || p.group == nil {
			return
		}
		if p.CanvasRef == nil || !p.CanvasRef.Capabilities.EditProperties {
			return
		}
		if _, ok := parseHexColor(s); !ok && s != "" {
			return // incomplete/invalid — wait for a full color
		}
		p.doCommand(&SetGroupColorCommand{
			Canvas:  p.CanvasRef,
			GroupID: p.group.ID,
			Old:     p.group.Color,
			New:     s,
			OnDone:  p.syncForm,
		})
	}
	lastColor := g.Color
	p.syncers = append(p.syncers, func(map[string]any) {
		// A half-typed color is not stored yet; leave it to be finished
		// unless the stored color itself changed, by an undo or redo.
		cur := live().Color
		changed := cur != lastColor
		lastColor = cur
		if changed {
			syncText(colorEntry, cur)
		}
	})

	form := widget.NewForm(
		widget.NewFormItem("Label", labelEntry),
		widget.NewFormItem("Color", colorEntry),
	)
	p.form = form
	p.content.Objects = []core.CanvasObject{
		p.header,
		p.typeLabel,
		widget.NewSeparator(),
		form,
	}
	p.content.Refresh()
}

// SetAnnotation updates the panel to edit an annotation's text and anchor.
func (p *PropertiesPanel) SetAnnotation(a *dag.Annotation) {
	p.rebuilding = true
	defer func() { p.rebuilding = false }()
	p.syncers = nil
	p.form = nil

	p.node = nil
	p.component = nil
	p.group = nil
	p.linkEdge = nil
	p.annotation = a
	if a == nil {
		p.content.Objects = []core.CanvasObject{p.empty}
		p.content.Refresh()
		return
	}

	p.header.SetText("Annotation")
	p.typeLabel.SetText("")

	textEntry := widget.NewMultiLineEntry()
	textEntry.SetText(a.Text)
	textEntry.SetMinRowsVisible(4)
	textEntry.OnChanged = func(s string) {
		if p.rebuilding || p.annotation == nil {
			return
		}
		if p.CanvasRef == nil || !p.CanvasRef.Capabilities.Annotations {
			return
		}
		p.doCommand(&SetAnnotationTextCommand{
			Canvas:       p.CanvasRef,
			AnnotationID: p.annotation.ID,
			OldText:      p.annotation.Text,
			NewText:      s,
			OnDone:       p.syncForm,
		})
	}

	kindNames := []string{"None", "Node", "Edge", "Group", "Link"}
	kindByName := map[string]dag.AnchorKind{
		"None": dag.AnchorNone, "Node": dag.AnchorNode,
		"Edge": dag.AnchorEdge, "Group": dag.AnchorGroup,
		"Link": dag.AnchorLinkEdge,
	}

	targetSelect := widget.NewSelect(nil, nil)
	populateTargets := func(kind dag.AnchorKind) {
		targetSelect.Options = p.anchorCandidates(kind)
		targetSelect.Refresh()
	}
	applyAnchor := func(kind dag.AnchorKind, anchorID string) {
		if p.annotation == nil {
			return
		}
		if p.CanvasRef == nil || !p.CanvasRef.Capabilities.Annotations {
			return
		}
		p.doCommand(&SetAnnotationAnchorCommand{
			Canvas:       p.CanvasRef,
			AnnotationID: p.annotation.ID,
			OldKind:      p.annotation.AnchorKind,
			NewKind:      kind,
			OldAnchorID:  p.annotation.AnchorID,
			NewAnchorID:  anchorID,
			OnDone: func() {
				// The anchor command re-creates the annotation; re-resolve
				// it, but only while the panel still shows it: an undo can
				// run after the panel moved on to another element.
				if p.CanvasRef != nil && p.annotation != nil && p.annotation.ID == a.ID {
					if cur := p.CanvasRef.Graph.Annotations[a.ID]; cur != nil {
						p.annotation = cur
					}
				}
				p.syncForm()
			},
		})
	}

	kindSelect := widget.NewSelect(kindNames, func(name string) {
		if p.rebuilding {
			return
		}
		kind := kindByName[name]
		populateTargets(kind)
		// The old target belongs to the old kind; the user picks a new one.
		targetSelect.ClearSelected()
		if kind == dag.AnchorNone {
			applyAnchor(dag.AnchorNone, "")
		}
	})
	kindSelect.SetSelected(kindNames[a.AnchorKind])

	targetSelect.OnChanged = func(sel string) {
		if p.rebuilding || sel == "" {
			return
		}
		kind := kindByName[kindSelect.Selected]
		if kind == dag.AnchorNone {
			return
		}
		applyAnchor(kind, anchorIDFromOption(sel))
	}
	selectTarget := func(anchorID string) {
		for _, opt := range targetSelect.Options {
			if anchorIDFromOption(opt) == anchorID {
				targetSelect.SetSelected(opt)
				return
			}
		}
		targetSelect.ClearSelected()
	}
	populateTargets(a.AnchorKind)
	if a.AnchorID != "" {
		selectTarget(a.AnchorID)
	}
	lastKind, lastAnchorID := a.AnchorKind, a.AnchorID
	p.syncers = append(p.syncers, func(map[string]any) {
		// The anchor command re-creates the annotation, so read it afresh.
		cur := a
		if p.CanvasRef != nil {
			cur = liveOr(p.CanvasRef.Graph.Annotations, a.ID, a)
		}
		syncText(textEntry, cur.Text)
		changed := cur.AnchorKind != lastKind || cur.AnchorID != lastAnchorID
		lastKind, lastAnchorID = cur.AnchorKind, cur.AnchorID
		// A kind picked with no target yet stores nothing. Keep it until the
		// user picks a target or the stored anchor changes; a picked target
		// the model rejected does snap back.
		if !changed && targetSelect.Selected == "" {
			return
		}
		if kindSelect.Selected != kindNames[cur.AnchorKind] {
			kindSelect.SetSelected(kindNames[cur.AnchorKind])
			populateTargets(cur.AnchorKind)
		}
		if anchorIDFromOption(targetSelect.Selected) != cur.AnchorID {
			selectTarget(cur.AnchorID)
		}
	})

	form := widget.NewForm(
		widget.NewFormItem("Text", textEntry),
		widget.NewFormItem("Anchor", kindSelect),
		widget.NewFormItem("Target", targetSelect),
	)
	p.form = form
	p.content.Objects = []core.CanvasObject{
		p.header,
		widget.NewSeparator(),
		form,
	}
	p.content.Refresh()
}

// SetLinkEdge updates the panel to edit a link edge's label.
func (p *PropertiesPanel) SetLinkEdge(le *dag.LinkEdge) {
	p.rebuilding = true
	defer func() { p.rebuilding = false }()
	p.syncers = nil
	p.form = nil

	p.node = nil
	p.component = nil
	p.group = nil
	p.annotation = nil
	p.linkEdge = le
	if le == nil {
		p.content.Objects = []core.CanvasObject{p.empty}
		p.content.Refresh()
		return
	}

	p.header.SetText("Link")
	p.typeLabel.SetText(fmt.Sprintf("%s → %s", le.Source, le.Target))

	labelEntry := widget.NewEntry()
	labelEntry.SetText(le.Label)
	labelEntry.OnChanged = func(s string) {
		if p.rebuilding || p.linkEdge == nil {
			return
		}
		if p.CanvasRef == nil || !p.CanvasRef.Capabilities.EditProperties {
			return
		}
		p.doCommand(&SetLinkEdgeLabelCommand{
			Canvas: p.CanvasRef,
			EdgeID: p.linkEdge.ID,
			Old:    p.linkEdge.Label,
			New:    s,
			OnDone: p.syncForm,
		})
	}
	p.syncers = append(p.syncers, func(map[string]any) {
		cur := le
		if p.CanvasRef != nil {
			cur = liveOr(p.CanvasRef.Graph.LinkEdges, le.ID, le)
		}
		syncText(labelEntry, cur.Label)
	})

	form := widget.NewForm(
		widget.NewFormItem("Label", labelEntry),
	)
	p.form = form
	p.content.Objects = []core.CanvasObject{
		p.header,
		p.typeLabel,
		widget.NewSeparator(),
		form,
	}
	p.content.Refresh()
}

// anchorCandidates lists the selectable anchor targets for a kind as
// "label (id)" options.
func (p *PropertiesPanel) anchorCandidates(kind dag.AnchorKind) []string {
	if p.CanvasRef == nil {
		return nil
	}
	g := p.CanvasRef.Graph
	var opts []string
	switch kind {
	case dag.AnchorNode:
		for _, n := range g.Nodes {
			opts = append(opts, fmt.Sprintf("%s (%s)", n.Label, n.ID))
		}
	case dag.AnchorEdge:
		for _, e := range g.Edges {
			opts = append(opts, fmt.Sprintf("%s→%s (%s)", e.Source.NodeID, e.Target.NodeID, e.ID))
		}
	case dag.AnchorGroup:
		for _, grp := range g.Groups {
			opts = append(opts, fmt.Sprintf("%s (%s)", grp.Label, grp.ID))
		}
	case dag.AnchorLinkEdge:
		for _, le := range g.LinkEdges {
			opts = append(opts, fmt.Sprintf("%s→%s (%s)", le.Source, le.Target, le.ID))
		}
	}
	sort.Strings(opts)
	return opts
}

// anchorIDFromOption extracts the ID from a "label (id)" option string.
func anchorIDFromOption(opt string) string {
	open := strings.LastIndex(opt, "(")
	close := strings.LastIndex(opt, ")")
	if open < 0 || close <= open {
		return opt
	}
	return opt[open+1 : close]
}

// doCommand routes an edit through the command stack when available.
func (p *PropertiesPanel) doCommand(cmd Command) {
	if p.Commands != nil {
		p.Commands.Do(cmd)
	} else {
		cmd.Execute()
	}
}

func (p *PropertiesPanel) CreateRenderer() core.WidgetRenderer {
	scroll := container.NewScroll(p.content)
	scroll.SetMinSize(geo.NewSize(200, 0))
	return widget.NewSimpleRenderer(scroll)
}

func (p *PropertiesPanel) buildStreamInfo() string {
	if p.component == nil {
		return ""
	}
	var info string
	if len(p.component.Inputs) > 0 {
		info += "Inputs: "
		for i, pd := range p.component.Inputs {
			if i > 0 {
				info += ", "
			}
			info += pd.Name + " (" + pd.Stream.Name + ")"
		}
		info += "\n"
	}
	if len(p.component.Outputs) > 0 {
		info += "Outputs: "
		for i, pd := range p.component.Outputs {
			if i > 0 {
				info += ", "
			}
			info += pd.Name + " (" + pd.Stream.Name + ")"
		}
	}
	return info
}

func (p *PropertiesPanel) loadProps() map[string]any {
	props := make(map[string]any)
	if p.node != nil && p.node.Metadata != nil {
		json.Unmarshal(p.node.Metadata, &props)
	}
	return props
}

// setProp creates and executes a SetPropertyCommand through the CommandStack if available,
// otherwise falls back to direct mutation.
func (p *PropertiesPanel) setProp(key string, newValue any) {
	if p.rebuilding {
		return // suppress callbacks during form rebuild
	}
	if p.CanvasRef != nil && !p.CanvasRef.Capabilities.EditProperties {
		return
	}
	oldProps := p.loadProps()
	oldValue := oldProps[key]

	if p.Commands != nil {
		cmd := &SetPropertyCommand{
			Node:     p.node,
			Key:      key,
			OldValue: oldValue,
			NewValue: newValue,
			OnDone:   p.syncForm,
		}
		p.Commands.Do(cmd)
	} else {
		setMetadataKey(p.node, key, newValue)
	}
}

// syncForm writes the shown element's current values into the fields on
// show. Called after an edit, undo or redo; it leaves alone a field that
// already holds the value, so the entry being typed in keeps its text, caret
// and focus.
func (p *PropertiesPanel) syncForm() {
	// Restore rather than clear, so a sync nested in a Set* rebuild or in
	// another sync does not end the outer one's suppression early.
	was := p.rebuilding
	p.rebuilding = true
	defer func() { p.rebuilding = was }()
	props := p.loadProps()
	for _, sync := range p.syncers {
		sync(props)
	}
}

// liveOr returns the element with the given ID from a graph map, or shown
// when the map has none. The syncers read each element where its commands
// write it, by ID: the pointer a Set* call was given may be another copy.
func liveOr[T any](byID map[string]*T, id string, shown *T) *T {
	if cur := byID[id]; cur != nil {
		return cur
	}
	return shown
}

// syncText sets an entry's text only when it differs, so the caret of an
// entry being typed in stays where it is.
func syncText(e *widget.Entry, s string) {
	if e.Text != s {
		e.SetText(s)
	}
}

// propValue is a property's stored value, or its default when unset.
func propValue(pd dag.PropertyDef, props map[string]any) any {
	if v := props[pd.Key]; v != nil {
		return v
	}
	return pd.Default
}

// sameNumber reports whether an entry's text already denotes v, so "1." or
// "1.50" is not rewritten to "1" or "1.5" while being typed.
func sameNumber(text string, v any) bool {
	got, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return text == "" && v == nil
	}
	switch n := v.(type) {
	case int:
		return got == float64(n)
	case float64:
		return got == n
	case json.Number:
		want, err := n.Float64()
		return err == nil && got == want
	}
	return false
}

// syncNumber keeps a number entry showing the stored value. Text that does
// not denote it ("", "1e", "1.5" in an int field) was never stored: it is the
// user's unfinished edit, kept until the stored value itself changes, by an
// undo or redo.
func (p *PropertiesPanel) syncNumber(entry *widget.Entry, pd dag.PropertyDef, initial any) {
	last := formatNumber(initial)
	p.syncers = append(p.syncers, func(props map[string]any) {
		v := propValue(pd, props)
		changed := formatNumber(v) != last
		last = formatNumber(v)
		if changed && !sameNumber(entry.Text, v) {
			entry.SetText(formatNumber(v))
		}
	})
}

func (p *PropertiesPanel) createFormItem(pd dag.PropertyDef, props map[string]any) *widget.FormItem {
	currentVal := propValue(pd, props)

	switch pd.Kind {
	case dag.PropertyBool:
		check := widget.NewCheck("", func(v bool) {
			p.setProp(pd.Key, v)
		})
		if b, ok := currentVal.(bool); ok {
			check.Checked = b
		}
		p.syncers = append(p.syncers, func(props map[string]any) {
			b, _ := propValue(pd, props).(bool)
			if check.Checked != b {
				check.SetChecked(b)
			}
		})
		return widget.NewFormItem(pd.Label, check)

	case dag.PropertyEnum:
		sel := widget.NewSelect(pd.EnumValues, func(v string) {
			p.setProp(pd.Key, v)
		})
		if s, ok := currentVal.(string); ok {
			sel.SetSelected(s)
		}
		p.syncers = append(p.syncers, func(props map[string]any) {
			s, _ := propValue(pd, props).(string)
			switch {
			case s == sel.Selected:
			case slices.Contains(sel.Options, s):
				sel.SetSelected(s)
			default:
				// Unset, or a value the options lack (SetSelected would
				// ignore it and keep showing the old choice).
				sel.ClearSelected()
			}
		})
		return widget.NewFormItem(pd.Label, sel)

	case dag.PropertyInt:
		entry := widget.NewEntry()
		if currentVal != nil {
			entry.SetText(formatNumber(currentVal))
		}
		entry.OnChanged = func(s string) {
			if v, err := strconv.Atoi(s); err == nil {
				p.setProp(pd.Key, v)
			}
		}
		p.syncNumber(entry, pd, currentVal)
		return widget.NewFormItem(pd.Label, entry)

	case dag.PropertyFloat:
		entry := widget.NewEntry()
		if currentVal != nil {
			entry.SetText(formatNumber(currentVal))
		}
		entry.OnChanged = func(s string) {
			if v, err := strconv.ParseFloat(s, 64); err == nil {
				p.setProp(pd.Key, v)
			}
		}
		p.syncNumber(entry, pd, currentVal)
		return widget.NewFormItem(pd.Label, entry)

	default: // PropertyString
		entry := widget.NewEntry()
		if s, ok := currentVal.(string); ok {
			entry.SetText(s)
		}
		entry.OnChanged = func(s string) {
			p.setProp(pd.Key, s)
		}
		hint := ""
		if pd.Required {
			hint = "required"
		}
		entry.SetPlaceHolder(hint)
		p.syncers = append(p.syncers, func(props map[string]any) {
			s, _ := propValue(pd, props).(string)
			syncText(entry, s)
		})
		return widget.NewFormItem(pd.Label, entry)
	}
}

func formatNumber(v any) string {
	switch n := v.(type) {
	case int:
		return strconv.Itoa(n)
	case float64:
		return strconv.FormatFloat(n, 'f', -1, 64)
	case json.Number:
		return string(n)
	default:
		return ""
	}
}
