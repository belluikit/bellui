package dagwidget

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/dag/dag"
)

// testPaletteItem returns the palette item for a one-component registry,
// wired to c, with the canvas sized like a page.
func testPaletteItem(t *testing.T, c *DAGCanvas) *paletteItem {
	t.Helper()
	reg := dag.NewRegistry()
	reg.MustRegister(&dag.Component{
		ID: "step", Name: "Step", Category: "Steps",
		Inputs:  []dag.PortDef{{Name: "in", Direction: dag.PortInput}},
		Outputs: []dag.PortDef{{Name: "out", Direction: dag.PortOutput}},
	})
	c.Resize(geo.NewSize(800, 600))
	p := NewPalette(reg, c)
	for _, obj := range p.content.Objects {
		if item, ok := obj.(*paletteItem); ok {
			return item
		}
	}
	t.Fatal("palette built no item")
	return nil
}

func nodeGraphRect(c *DAGCanvas, id string) (pos [2]float32, size geo.Size) {
	return c.Graph.Nodes[id].Position, c.nodes[id].effectiveSize()
}

func TestPaletteTapFansOutNodes(t *testing.T) {
	c := newTestCanvas(t)
	item := testPaletteItem(t, c)

	const taps = 5
	for range taps {
		item.Tapped(&event.PointEvent{})
	}
	if len(c.Graph.Nodes) != taps {
		t.Fatalf("got %d nodes after %d taps, want %d", len(c.Graph.Nodes), taps, taps)
	}
	var ids []string
	for id := range c.Graph.Nodes {
		ids = append(ids, id)
	}
	for i, a := range ids {
		pa, sa := nodeGraphRect(c, a)
		for _, b := range ids[i+1:] {
			pb, sb := nodeGraphRect(c, b)
			if pa[0] < pb[0]+sb.Width && pb[0] < pa[0]+sa.Width &&
				pa[1] < pb[1]+sb.Height && pb[1] < pa[1]+sa.Height {
				t.Errorf("nodes %s at %v and %s at %v overlap", a, pa, b, pb)
			}
		}
	}

	// The first tap still lands on the viewport center.
	center := c.screenToGraph(geo.NewPos(400, 300))
	found := false
	for _, n := range c.Graph.Nodes {
		if n.Position == [2]float32{center.X, center.Y} {
			found = true
		}
	}
	if !found {
		t.Errorf("no node at the viewport center %v", center)
	}
}

func TestPaletteTapAvoidsGroups(t *testing.T) {
	c := newTestCanvas(t)
	item := testPaletteItem(t, c)
	center := c.screenToGraph(geo.NewPos(400, 300))
	g := &dag.Group{
		ID: "g", Label: "G",
		Position: [2]float32{center.X - 100, center.Y - 100}, Size: [2]float32{300, 300},
	}
	if err := c.AddGroup(g); err != nil {
		t.Fatal(err)
	}

	item.Tapped(&event.PointEvent{})
	for id := range c.Graph.Nodes {
		pos, size := nodeGraphRect(c, id)
		if pos[0] < g.Position[0]+g.Size[0] && g.Position[0] < pos[0]+size.Width &&
			pos[1] < g.Position[1]+g.Size[1] && g.Position[1] < pos[1]+size.Height {
			t.Errorf("node at %v lands inside the group", pos)
		}
	}
}

func TestPaletteTapIsUndoable(t *testing.T) {
	c := newTestCanvas(t)
	item := testPaletteItem(t, c)
	item.Tapped(&event.PointEvent{})
	item.Tapped(&event.PointEvent{})

	c.Commands.Undo()
	if len(c.Graph.Nodes) != 1 || len(c.nodes) != 1 {
		t.Fatalf("undo left %d nodes (%d widgets), want 1", len(c.Graph.Nodes), len(c.nodes))
	}
	c.Commands.Undo()
	if len(c.Graph.Nodes) != 0 || len(c.nodes) != 0 {
		t.Fatalf("second undo left %d nodes, want 0", len(c.Graph.Nodes))
	}
	c.Commands.Redo()
	if len(c.Graph.Nodes) != 1 || len(c.nodes) != 1 {
		t.Fatalf("redo restored %d nodes, want 1", len(c.Graph.Nodes))
	}
}

func TestRenameNodeCommand(t *testing.T) {
	c := newTestCanvas(t)
	n := addTestNode(t, c, "a")

	c.Commands.Do(&RenameNodeCommand{Canvas: c, NodeID: "a", Old: "a", New: "Load"})
	if n.Label != "Load" || c.nodes["a"].Label != "Load" {
		t.Fatalf("rename not applied: graph %q, card %q", n.Label, c.nodes["a"].Label)
	}
	c.Commands.Undo()
	if n.Label != "a" || c.nodes["a"].Label != "a" {
		t.Fatalf("undo did not restore: graph %q, card %q", n.Label, c.nodes["a"].Label)
	}
	c.Commands.Redo()
	if n.Label != "Load" || c.nodes["a"].Label != "Load" {
		t.Fatalf("redo did not re-apply: graph %q, card %q", n.Label, c.nodes["a"].Label)
	}
}

// panelNameEntry returns the panel form's Name entry, or nil.
func panelNameEntry(p *PropertiesPanel) *widget.Entry {
	for _, it := range p.form.Items {
		if it.Text == "Name" {
			return it.Widget.(*widget.Entry)
		}
	}
	return nil
}

func TestPropertiesPanelRenamesNode(t *testing.T) {
	c := newTestCanvas(t)
	n := addTestNode(t, c, "a")
	p := NewPropertiesPanel(nil, c.Commands)
	p.CanvasRef = c
	p.SetNode(n)

	name := panelNameEntry(p)
	if name == nil {
		t.Fatal("panel shows no Name field")
	}
	name.SetText("Load")
	if n.Label != "Load" || c.nodes["a"].Label != "Load" || p.header.Text != "Load" {
		t.Fatalf("typing did not rename: graph %q, card %q, header %q", n.Label, c.nodes["a"].Label, p.header.Text)
	}
	c.Commands.Undo()
	if n.Label != "a" {
		t.Fatalf("undo did not restore the label: %q", n.Label)
	}

	c.Capabilities.EditProperties = false
	p.SetNode(n)
	panelNameEntry(p).SetText("Blocked")
	if n.Label != "a" {
		t.Fatalf("rename ran with EditProperties off: %q", n.Label)
	}
}

func TestPropertiesPanelNoNameWithoutCanvas(t *testing.T) {
	c := newTestCanvas(t)
	n := addTestNode(t, c, "a")
	p := NewPropertiesPanel(nil, c.Commands)
	p.SetNode(n)
	if panelNameEntry(p) != nil {
		t.Fatal("Name field shown without CanvasRef, where it could not update the card")
	}
}
