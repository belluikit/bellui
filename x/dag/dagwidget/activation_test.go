package dagwidget

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/dag/dag"
)

func TestNodeDoubleTapActivates(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	var activated []string
	c.OnNodeActivated = func(id string) { activated = append(activated, id) }

	nw := c.nodes["a"]
	ev := &event.PointEvent{AbsolutePosition: geo.NewPos(50, 50)}

	nw.Tapped(ev)
	if len(activated) != 0 {
		t.Fatal("single tap activated")
	}
	if !nw.Selected {
		t.Fatal("single tap did not select")
	}
	nw.Tapped(ev)
	if len(activated) != 1 || activated[0] != "a" {
		t.Fatalf("double tap activation: %v", activated)
	}
	if !nw.Selected {
		t.Fatal("node deselected by activation")
	}
	// A third tap starts a fresh cycle — no second activation.
	nw.Tapped(ev)
	if len(activated) != 1 {
		t.Fatalf("triple tap re-activated: %v", activated)
	}
}

func TestNodeDoubleTapFarApartDoesNotActivate(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	fired := false
	c.OnNodeActivated = func(string) { fired = true }

	nw := c.nodes["a"]
	nw.Tapped(&event.PointEvent{AbsolutePosition: geo.NewPos(10, 10)})
	nw.Tapped(&event.PointEvent{AbsolutePosition: geo.NewPos(100, 100)})
	if fired {
		t.Fatal("taps far apart activated")
	}
}

func TestGroupTitleDoubleTapActivates(t *testing.T) {
	c := newTestCanvas(t)
	if err := c.AddGroup(&dag.Group{ID: "g1", Label: "G"}); err != nil {
		t.Fatal(err)
	}
	var activated []string
	c.OnGroupActivated = func(id string) { activated = append(activated, id) }

	tb := c.groups["g1"].titleBar
	ev := &event.PointEvent{AbsolutePosition: geo.NewPos(30, 10)}
	tb.Tapped(ev)
	if len(activated) != 0 {
		t.Fatal("single tap activated")
	}
	if !c.groups["g1"].Selected {
		t.Fatal("single tap did not select the group")
	}
	tb.Tapped(ev)
	if len(activated) != 1 || activated[0] != "g1" {
		t.Fatalf("double tap activation: %v", activated)
	}
}

func TestEnterActivatesSelection(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	if err := c.AddGroup(&dag.Group{ID: "g1", Label: "G"}); err != nil {
		t.Fatal(err)
	}
	var nodes, groups []string
	c.OnNodeActivated = func(id string) { nodes = append(nodes, id) }
	c.OnGroupActivated = func(id string) { groups = append(groups, id) }

	enter := &event.KeyEvent{Name: event.KeyReturn}

	// Nothing selected: neither fires.
	c.TypedKey(enter)
	if len(nodes)+len(groups) != 0 {
		t.Fatal("Enter with no selection activated something")
	}

	c.nodes["a"].Selected = true
	c.TypedKey(enter)
	if len(nodes) != 1 || nodes[0] != "a" || len(groups) != 0 {
		t.Fatalf("Enter on selected node: nodes=%v groups=%v", nodes, groups)
	}

	c.deselectAll()
	c.groups["g1"].Selected = true
	c.TypedKey(&event.KeyEvent{Name: event.KeyEnter}) // keypad Enter too
	if len(groups) != 1 || groups[0] != "g1" {
		t.Fatalf("Enter on selected group: groups=%v", groups)
	}
}

func TestActivationNilCallbacksDoNotPanic(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")

	nw := c.nodes["a"]
	ev := &event.PointEvent{AbsolutePosition: geo.NewPos(5, 5)}
	nw.Tapped(ev)
	nw.Tapped(ev) // double tap, nil OnNodeActivated

	c.TypedKey(&event.KeyEvent{Name: event.KeyReturn}) // nil callbacks
}
