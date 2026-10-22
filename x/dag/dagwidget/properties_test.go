package dagwidget

import (
	"fmt"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/dag/dag"
)

// panelFixture is a canvas with two typed nodes and a properties panel shown
// in a test window, so tests can read what the panel actually draws.
type panelFixture struct {
	c     *DAGCanvas
	p     *PropertiesPanel
	w     bellui.Window
	first *dag.Node // "dash": Name + Schedule
	other *dag.Node // "orders": Name + Batch size + Ratio + Connection
}

func newPanelFixture(t *testing.T) *panelFixture {
	t.Helper()
	c := newTestCanvas(t)
	reg := dag.NewRegistry()
	reg.MustRegister(&dag.Component{
		ID: "dashboard", Name: "Dashboard",
		Properties: []dag.PropertyDef{{Key: "schedule", Label: "Schedule", Kind: dag.PropertyString}},
	})
	reg.MustRegister(&dag.Component{
		ID: "source", Name: "Source",
		Properties: []dag.PropertyDef{
			{Key: "batch", Label: "Batch size", Kind: dag.PropertyInt},
			{Key: "ratio", Label: "Ratio", Kind: dag.PropertyFloat},
			{Key: "conn", Label: "Connection", Kind: dag.PropertyString},
		},
	})
	first := &dag.Node{ID: "dash", Label: "Dashboard", TypeTag: "dashboard"}
	other := &dag.Node{ID: "orders", Label: "Orders", TypeTag: "source"}
	for _, n := range []*dag.Node{first, other} {
		if err := c.AddNode(n); err != nil {
			t.Fatal(err)
		}
	}
	p := NewPropertiesPanel(reg, c.Commands)
	p.CanvasRef = c
	w := test.NewTempWindow(t, p)
	w.Resize(geo.NewSize(320, 600))
	return &panelFixture{c: c, p: p, w: w, first: first, other: other}
}

// renderedEntries returns the visible entries the panel draws, in tree order.
// It walks the cached renderers, the ones a window paints, rather than reading
// form.Items. test.LaidOutObjects would not do: it builds a fresh renderer for
// every widget, so it shows what a new form would draw, not what this one does.
func renderedEntries(o core.CanvasObject) []*widget.Entry {
	return rendered[*widget.Entry](o)
}

// rendered returns the visible objects of type T the panel draws, in tree
// order, walking the cached renderers as renderedEntries does.
func rendered[T core.CanvasObject](o core.CanvasObject) []T {
	var out []T
	var walk func(core.CanvasObject)
	walk = func(o core.CanvasObject) {
		if !o.Visible() {
			return
		}
		if t, ok := o.(T); ok {
			out = append(out, t)
			return
		}
		switch obj := o.(type) {
		case core.Widget:
			for _, child := range test.WidgetRenderer(obj).Objects() {
				walk(child)
			}
		case *bellui.Container:
			for _, child := range obj.Objects {
				walk(child)
			}
		}
	}
	walk(o)
	return out
}

// formEntry returns the entry of the panel form item with the given label.
func formEntry(t *testing.T, p *PropertiesPanel, label string) *widget.Entry {
	t.Helper()
	for _, it := range p.form.Items {
		if it.Text == label {
			return it.Widget.(*widget.Entry)
		}
	}
	t.Fatalf("panel form has no %q item", label)
	return nil
}

func isRendered(p *PropertiesPanel, e *widget.Entry) bool {
	for _, r := range renderedEntries(p) {
		if r == e {
			return true
		}
	}
	return false
}

func TestPropertiesPanelDrawsTheSecondNodesFields(t *testing.T) {
	f := newPanelFixture(t)
	f.p.SetNode(f.first)
	// Draw the first node, as a window frame would between two selections:
	// the bug only shows once the form's renderer exists.
	if e := formEntry(t, f.p, "Schedule"); !isRendered(f.p, e) {
		t.Fatal("first node's Schedule entry is not drawn")
	}
	f.p.SetNode(f.other)

	for _, label := range []string{"Name", "Batch size", "Ratio", "Connection"} {
		if e := formEntry(t, f.p, label); !isRendered(f.p, e) {
			t.Errorf("%q entry of the second node is not drawn", label)
		}
	}
	if got := len(renderedEntries(f.p)); got != 4 {
		t.Errorf("panel draws %d entries, want the second node's 4", got)
	}
}

// typeKeepingFocus types s one character at a time into the panel entry with
// the given label, failing if the entry stops being focused or drawn. Each
// rune goes to the window's focused object, as the drivers deliver keys;
// test.Type would not do, because it re-focuses its target (selecting all)
// on every call.
func typeKeepingFocus(t *testing.T, f *panelFixture, label, s string) {
	t.Helper()
	e := formEntry(t, f.p, label)
	e.SetText("")
	f.w.Canvas().Focus(e)
	for i, r := range s {
		f.w.Canvas().Focused().TypedRune(r)
		if f.w.Canvas().Focused() != e {
			t.Fatalf("%q: focus left the entry after typing %q", label, s[:i+1])
		}
		if !isRendered(f.p, e) {
			t.Fatalf("%q: the entry was replaced after typing %q", label, s[:i+1])
		}
	}
	if e.Text != s {
		t.Fatalf("%q: entry reads %q, want %q", label, e.Text, s)
	}
}

func TestPropertiesPanelTypingKeepsFocus(t *testing.T) {
	f := newPanelFixture(t)
	f.p.SetNode(f.first)
	f.p.SetNode(f.other)

	typeKeepingFocus(t, f, "Connection", "pg://db")
	typeKeepingFocus(t, f, "Batch size", "250")
	// "1." parses as 1, which formats back as "1": a field rewritten from the
	// stored value would eat the dot.
	typeKeepingFocus(t, f, "Ratio", "1.25")
	typeKeepingFocus(t, f, "Name", "Merge")

	props := f.p.loadProps()
	if props["conn"] != "pg://db" || props["batch"] != float64(250) || props["ratio"] != 1.25 {
		t.Fatalf("stored properties %v", props)
	}
	if f.other.Label != "Merge" || f.p.header.Text != "Merge" {
		t.Fatalf("node label %q, header %q, want Merge", f.other.Label, f.p.header.Text)
	}
}

func TestPropertiesPanelUndoUpdatesTheFields(t *testing.T) {
	f := newPanelFixture(t)
	f.p.SetNode(f.other)

	formEntry(t, f.p, "Connection").SetText("a")
	formEntry(t, f.p, "Connection").SetText("ab")
	formEntry(t, f.p, "Name").SetText("Merge")

	f.c.Commands.Undo()
	if got := formEntry(t, f.p, "Name").Text; got != "Orders" {
		t.Errorf("Name after undo reads %q, want Orders", got)
	}
	if f.p.header.Text != "Orders" {
		t.Errorf("header after undo reads %q, want Orders", f.p.header.Text)
	}
	f.c.Commands.Undo()
	if got := formEntry(t, f.p, "Connection").Text; got != "a" {
		t.Errorf("Connection after undo reads %q, want a", got)
	}
	if !isRendered(f.p, formEntry(t, f.p, "Connection")) {
		t.Error("Connection entry no longer drawn after undo")
	}
}

func TestPropertiesPanelPortSummaryWraps(t *testing.T) {
	c := newTestCanvas(t)
	reg := dag.NewRegistry()
	var inputs []dag.PortDef
	for i := range 8 {
		inputs = append(inputs, dag.PortDef{
			Name: fmt.Sprintf("input_%d", i), Direction: dag.PortInput,
			Stream: dag.StreamType{Name: "rows"},
		})
	}
	reg.MustRegister(&dag.Component{ID: "join", Name: "Join", Inputs: inputs})
	n := &dag.Node{ID: "j", Label: "Join", TypeTag: "join"}
	if err := c.AddNode(n); err != nil {
		t.Fatal(err)
	}
	p := NewPropertiesPanel(reg, c.Commands)
	p.CanvasRef = c
	w := test.NewTempWindow(t, p)
	w.Resize(geo.NewSize(320, 600))
	p.SetNode(n)

	if got := p.content.MinSize().Width; got > 320 {
		t.Fatalf("panel content needs %.0f px, wider than its 320 px column", got)
	}
	if p.streamInfo.Size().Width > p.content.Size().Width {
		t.Fatalf("port summary is %.0f px wide in a %.0f px panel", p.streamInfo.Size().Width, p.content.Size().Width)
	}
}

// Undo of a group, link or annotation edit shows the restored value in the
// panel, as it does for a node (#638).
func TestPropertiesPanelUndoUpdatesOtherElements(t *testing.T) {
	c := fullCanvas(t)
	p := NewPropertiesPanel(nil, c.Commands)
	p.CanvasRef = c
	test.NewTempWindow(t, p).Resize(geo.NewSize(320, 600))

	p.SetGroup(c.Graph.Groups["g1"])
	entries := renderedEntries(p)
	if len(entries) != 2 {
		t.Fatalf("group view draws %d entries, want Label and Color", len(entries))
	}
	label, color := entries[0], entries[1]
	label.SetText("Stage")
	color.SetText("#ff0000")
	c.Commands.Undo()
	if color.Text != "" {
		t.Errorf("group color after undo reads %q, want empty", color.Text)
	}
	c.Commands.Undo()
	if label.Text != "g1" || p.header.Text != "g1" {
		t.Errorf("group label after undo: entry %q, header %q, want g1", label.Text, p.header.Text)
	}

	p.SetLinkEdge(c.Graph.LinkEdges["l1"])
	link := renderedEntries(p)[0]
	link.SetText("feeds")
	c.Commands.Undo()
	if link.Text != "" {
		t.Errorf("link label after undo reads %q, want empty", link.Text)
	}

	p.SetAnnotation(c.Graph.Annotations["an1"])
	text := renderedEntries(p)[0]
	text.SetText("note, edited")
	c.Commands.Undo()
	if text.Text != "note" {
		t.Errorf("annotation text after undo reads %q, want note", text.Text)
	}

	selects := rendered[*widget.Select](p)
	if len(selects) != 2 {
		t.Fatalf("annotation view draws %d selects, want Anchor and Target", len(selects))
	}
	kind, target := selects[0], selects[1]
	kind.SetSelected("None")
	if a := c.Graph.Annotations["an1"]; a.AnchorKind != dag.AnchorNone {
		t.Fatalf("anchor not cleared: kind %v", a.AnchorKind)
	}
	c.Commands.Undo()
	if kind.Selected != "Node" || anchorIDFromOption(target.Selected) != "a" {
		t.Errorf("anchor after undo shows %q / %q, want Node / a", kind.Selected, target.Selected)
	}
}

// Each keystroke is its own undo step, a decision recorded in #640. If edits
// ever start merging, this fails and the decision has to be revisited.
func TestPropertiesPanelUndoesOneKeystrokeAtATime(t *testing.T) {
	f := newPanelFixture(t)
	f.p.SetNode(f.other)

	typeKeepingFocus(t, f, "Connection", "abc")
	name := formEntry(t, f.p, "Name")
	f.w.Canvas().Focus(name)
	f.w.Canvas().Focused().TypedRune('!')

	for _, want := range []struct{ name, conn string }{
		{"Orders", "abc"},
		{"Orders", "ab"},
		{"Orders", "a"},
	} {
		f.c.Commands.Undo()
		if f.other.Label != want.name || f.p.loadProps()["conn"] != want.conn {
			t.Fatalf("after an undo: name %q, connection %v; want %q, %q",
				f.other.Label, f.p.loadProps()["conn"], want.name, want.conn)
		}
	}
}

// annotationPanel shows annotation id of a fullCanvas, plus a free annotation
// "an2", in a panel drawn by a test window.
func annotationPanel(t *testing.T, id string) (*DAGCanvas, *PropertiesPanel) {
	t.Helper()
	c := fullCanvas(t)
	if err := c.AddAnnotation(&dag.Annotation{ID: "an2", Text: "free"}); err != nil {
		t.Fatal(err)
	}
	p := NewPropertiesPanel(nil, c.Commands)
	p.CanvasRef = c
	test.NewTempWindow(t, p).Resize(geo.NewSize(320, 600))
	p.SetAnnotation(c.Graph.Annotations[id])
	return c, p
}

// Undoing an anchor change after the panel moved on must not drag the panel
// back to the annotation the change was made on: the shown fields would then
// edit that other annotation (#641 review).
func TestPropertiesPanelAnchorUndoKeepsTheShownElement(t *testing.T) {
	c, p := annotationPanel(t, "an2")
	rendered[*widget.Select](p)[0].SetSelected("Node")
	rendered[*widget.Select](p)[1].SetSelected("a (a)")
	if c.Graph.Annotations["an2"].AnchorID != "a" {
		t.Fatal("anchor not applied")
	}

	p.SetAnnotation(c.Graph.Annotations["an1"])
	c.Commands.Undo()
	if p.annotation.ID != "an1" {
		t.Fatalf("undo switched the panel to %q", p.annotation.ID)
	}
	if got := renderedEntries(p)[0].Text; got != "note" {
		t.Fatalf("text entry reads %q, want an1's note", got)
	}

	p.SetNode(nil)
	c.Commands.Redo()
	if p.annotation != nil {
		t.Fatalf("redo set annotation %q on an empty panel", p.annotation.ID)
	}
}

// Picking an anchor kind stores nothing until a target is picked; an edit to
// another field in between must not reset the pick, while a target the model
// rejects still snaps back (#641 review).
func TestPropertiesPanelKeepsAPickedAnchorKind(t *testing.T) {
	c, p := annotationPanel(t, "an2")
	selects := rendered[*widget.Select](p)
	kind, target := selects[0], selects[1]
	kind.SetSelected("Node")
	renderedEntries(p)[0].SetText("free!")
	if kind.Selected != "Node" || len(target.Options) == 0 {
		t.Fatalf("text edit reset the picked kind: %q with targets %v", kind.Selected, target.Options)
	}

	target.Options = append(target.Options, "ghost (ghost)")
	target.SetSelected("ghost (ghost)")
	if a := c.Graph.Annotations["an2"]; a.AnchorKind != dag.AnchorNone {
		t.Fatalf("ghost anchor applied: %+v", a)
	}
	if kind.Selected != "None" || target.Selected != "" {
		t.Fatalf("rejected anchor shows %q / %q, want None / nothing", kind.Selected, target.Selected)
	}
}

// The panel may be handed another copy of a node than the graph's; rename
// writes the graph's, so the sync must read that one or every keystroke snaps
// the entry back (#641 review).
func TestPropertiesPanelSyncsFromTheGraph(t *testing.T) {
	f := newPanelFixture(t)
	cp := *f.other
	f.p.SetNode(&cp)
	typeKeepingFocus(t, f, "Name", "Merge")
	if got := f.c.Graph.Nodes["orders"].Label; got != "Merge" || f.p.header.Text != "Merge" {
		t.Fatalf("graph label %q, header %q, want Merge", got, f.p.header.Text)
	}
}

// Number text that does not parse was never stored: an edit elsewhere leaves
// it, and only a change to the stored value rewrites it (#641 review).
func TestPropertiesPanelKeepsAnUnfinishedNumber(t *testing.T) {
	f := newPanelFixture(t)
	f.p.SetNode(f.other)
	batch := formEntry(t, f.p, "Batch size")
	batch.SetText("5")
	batch.SetText("")
	formEntry(t, f.p, "Connection").SetText("x")
	if batch.Text != "" {
		t.Fatalf("an edit elsewhere refilled the cleared number with %q", batch.Text)
	}
	batch.SetText("1e")
	f.c.Commands.Undo() // the Connection edit
	if batch.Text != "1e" {
		t.Fatalf("an undo elsewhere rewrote the unfinished number to %q", batch.Text)
	}
	f.c.Commands.Undo() // Batch size 5 -> unset
	if batch.Text != "" {
		t.Fatalf("undoing the number left %q, want empty", batch.Text)
	}
	f.c.Commands.Redo()
	if batch.Text != "5" {
		t.Fatalf("redoing the number shows %q, want 5", batch.Text)
	}
}

// An enum select shows the stored value only when it is one of the options;
// SetSelected ignores anything else and would keep the old pick (#641 review).
func TestPropertiesPanelEnumUndoToAnUnlistedValue(t *testing.T) {
	c := newTestCanvas(t)
	reg := dag.NewRegistry()
	reg.MustRegister(&dag.Component{ID: "s", Name: "S", Properties: []dag.PropertyDef{{
		Key: "mode", Label: "Mode", Kind: dag.PropertyEnum,
		EnumValues: []string{"fast", "slow"}, Default: "auto",
	}}})
	n := &dag.Node{ID: "n", Label: "N", TypeTag: "s"}
	if err := c.AddNode(n); err != nil {
		t.Fatal(err)
	}
	p := NewPropertiesPanel(reg, c.Commands)
	p.CanvasRef = c
	test.NewTempWindow(t, p)
	p.SetNode(n)
	sel := rendered[*widget.Select](p)[0]
	sel.SetSelected("fast")
	c.Commands.Undo()
	if sel.Selected != "" {
		t.Fatalf("enum after undo shows %q, want nothing for the unlisted default", sel.Selected)
	}
	c.Commands.Redo()
	if sel.Selected != "fast" {
		t.Fatalf("enum after redo shows %q, want fast", sel.Selected)
	}
}

// p.form is the form on show in every view, nil when the panel is empty, and
// a sync keeps an outer rebuild's callback suppression (#641 review).
func TestPropertiesPanelFormAndSuppression(t *testing.T) {
	c := fullCanvas(t)
	p := NewPropertiesPanel(nil, c.Commands)
	p.CanvasRef = c
	p.SetGroup(c.Graph.Groups["g1"])
	if p.form == nil || len(p.form.Items) != 2 || p.form.Items[0].Text != "Label" {
		t.Fatal("p.form is not the group view's form")
	}
	p.SetLinkEdge(c.Graph.LinkEdges["l1"])
	if p.form == nil || len(p.form.Items) != 1 {
		t.Fatal("p.form is not the link view's form")
	}
	p.SetNode(nil)
	if p.form != nil {
		t.Fatal("p.form survives an emptied panel")
	}

	p.rebuilding = true
	p.syncForm()
	if !p.rebuilding {
		t.Fatal("syncForm ended an outer rebuild's suppression")
	}
}

// Without a CanvasRef, the commands behind group, annotation and link edits
// have no canvas to write: the edits store nothing rather than panic.
func TestPropertiesPanelWithoutCanvasRefStoresNothing(t *testing.T) {
	c := fullCanvas(t)
	p := NewPropertiesPanel(nil, c.Commands)
	test.NewTempWindow(t, p).Resize(geo.NewSize(320, 600))

	p.SetGroup(c.Graph.Groups["g1"])
	for _, e := range renderedEntries(p) {
		e.SetText("#00ff00")
	}
	p.SetLinkEdge(c.Graph.LinkEdges["l1"])
	renderedEntries(p)[0].SetText("feeds")
	p.SetAnnotation(c.Graph.Annotations["an1"])
	renderedEntries(p)[0].SetText("edited")
	rendered[*widget.Select](p)[0].SetSelected("None")

	if g := c.Graph.Groups["g1"]; g.Label != "g1" || g.Color != "" {
		t.Errorf("group edited without a canvas: %+v", g)
	}
	if l := c.Graph.LinkEdges["l1"]; l.Label != "" {
		t.Errorf("link edited without a canvas: %q", l.Label)
	}
	if a := c.Graph.Annotations["an1"]; a.Text != "note" || a.AnchorKind != dag.AnchorNode {
		t.Errorf("annotation edited without a canvas: %+v", a)
	}
}
