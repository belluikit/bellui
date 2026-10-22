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
