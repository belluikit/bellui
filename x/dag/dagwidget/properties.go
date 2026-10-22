package dagwidget

import (
	"encoding/json"
	"fmt"
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
type PropertiesPanel struct {
	widget.BaseWidget

	Registry *dag.Registry
	Commands *CommandStack
	// CanvasRef is needed for node renaming and group/annotation editing:
	// commands carry the canvas, and anchor pickers enumerate the graph. Set
	// by the app screen after construction; without it a node shows no Name
	// field.
	CanvasRef *DAGCanvas

	node       *dag.Node
	group      *dag.Group
	annotation *dag.Annotation
	linkEdge   *dag.LinkEdge
	component  *dag.Component
	header     *widget.Label
	typeLabel  *widget.Label
	streamInfo *widget.Label
	form       *widget.Form
	content    *bellui.Container
	empty      *widget.Label
	rebuilding bool // true while a Set* rebuild is running, suppresses callbacks
}

// NewPropertiesPanel creates a new properties panel.
func NewPropertiesPanel(registry *dag.Registry, cmds *CommandStack) *PropertiesPanel {
	p := &PropertiesPanel{
		Registry:   registry,
		Commands:   cmds,
		header:     widget.NewLabel("No selection"),
		typeLabel:  widget.NewLabel(""),
		streamInfo: widget.NewLabel(""),
		form:       widget.NewForm(),
		empty:      widget.NewLabel("Select a node to view properties"),
	}
	p.header.TextStyle.Bold = true
	p.content = container.NewVBox(p.empty)
	p.ExtendBaseWidget(p)
	return p
}

// SetNode updates the panel to show the properties of the given node.
// Pass nil to clear the panel entirely (any mode).
func (p *PropertiesPanel) SetNode(n *dag.Node) {
	p.rebuilding = true
	defer func() { p.rebuilding = false }()

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
	p.form.Items = nil
	if p.CanvasRef != nil {
		p.form.Items = append(p.form.Items, widget.NewFormItem("Name", p.nameEntry(n)))
	}

	if p.component != nil {
		// Load current values from node metadata
		props := p.loadProps()

		for _, pd := range p.component.Properties {
			pd := pd // capture
			item := p.createFormItem(pd, props)
			p.form.Items = append(p.form.Items, item)
		}
	}

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
			OnDone: func() {
				if p.node != nil {
					p.header.SetText(p.node.Label)
				}
			},
		})
	}
	return entry
}

// SetGroup updates the panel to edit a group's label and color.
func (p *PropertiesPanel) SetGroup(g *dag.Group) {
	p.rebuilding = true
	defer func() { p.rebuilding = false }()

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

	labelEntry := widget.NewEntry()
	labelEntry.SetText(g.Label)
	labelEntry.OnChanged = func(s string) {
		if p.rebuilding || p.group == nil {
			return
		}
		if p.CanvasRef != nil && !p.CanvasRef.Capabilities.EditProperties {
			return
		}
		p.doCommand(&SetGroupLabelCommand{
			Canvas:  p.CanvasRef,
			GroupID: p.group.ID,
			Old:     p.group.Label,
			New:     s,
			OnDone:  func() { p.header.SetText(p.group.Label) },
		})
	}

	colorEntry := widget.NewEntry()
	colorEntry.SetText(g.Color)
	colorEntry.SetPlaceHolder("#rrggbb")
	colorEntry.OnChanged = func(s string) {
		if p.rebuilding || p.group == nil {
			return
		}
		if p.CanvasRef != nil && !p.CanvasRef.Capabilities.EditProperties {
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
		})
	}

	form := widget.NewForm(
		widget.NewFormItem("Label", labelEntry),
		widget.NewFormItem("Color", colorEntry),
	)
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
		if p.CanvasRef != nil && !p.CanvasRef.Capabilities.Annotations {
			return
		}
		p.doCommand(&SetAnnotationTextCommand{
			Canvas:       p.CanvasRef,
			AnnotationID: p.annotation.ID,
			OldText:      p.annotation.Text,
			NewText:      s,
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
		if p.CanvasRef != nil && !p.CanvasRef.Capabilities.Annotations {
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
				// The anchor command re-creates the annotation; re-resolve it.
				if p.CanvasRef != nil {
					p.annotation = p.CanvasRef.Graph.Annotations[a.ID]
				}
			},
		})
	}

	kindSelect := widget.NewSelect(kindNames, func(name string) {
		if p.rebuilding {
			return
		}
		kind := kindByName[name]
		populateTargets(kind)
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
	populateTargets(a.AnchorKind)
	if a.AnchorID != "" {
		for _, opt := range targetSelect.Options {
			if anchorIDFromOption(opt) == a.AnchorID {
				targetSelect.SetSelected(opt)
				break
			}
		}
	}

	form := widget.NewForm(
		widget.NewFormItem("Text", textEntry),
		widget.NewFormItem("Anchor", kindSelect),
		widget.NewFormItem("Target", targetSelect),
	)
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
		if p.CanvasRef != nil && !p.CanvasRef.Capabilities.EditProperties {
			return
		}
		p.doCommand(&SetLinkEdgeLabelCommand{
			Canvas: p.CanvasRef,
			EdgeID: p.linkEdge.ID,
			Old:    p.linkEdge.Label,
			New:    s,
		})
	}

	form := widget.NewForm(
		widget.NewFormItem("Label", labelEntry),
	)
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
			OnDone:   func() { p.refreshForm() },
		}
		p.Commands.Do(cmd)
	} else {
		setMetadataKey(p.node, key, newValue)
	}
}

// refreshForm re-renders the form with current metadata values.
// Called after undo/redo to keep the UI in sync.
func (p *PropertiesPanel) refreshForm() {
	if p.node != nil {
		p.SetNode(p.node)
	}
}

func (p *PropertiesPanel) createFormItem(pd dag.PropertyDef, props map[string]any) *widget.FormItem {
	currentVal := props[pd.Key]
	if currentVal == nil {
		currentVal = pd.Default
	}

	switch pd.Kind {
	case dag.PropertyBool:
		check := widget.NewCheck("", func(v bool) {
			p.setProp(pd.Key, v)
		})
		if b, ok := currentVal.(bool); ok {
			check.Checked = b
		}
		return widget.NewFormItem(pd.Label, check)

	case dag.PropertyEnum:
		sel := widget.NewSelect(pd.EnumValues, func(v string) {
			p.setProp(pd.Key, v)
		})
		if s, ok := currentVal.(string); ok {
			sel.SetSelected(s)
		}
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
