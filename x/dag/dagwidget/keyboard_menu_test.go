package dagwidget

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/dag/dag"
)

func menuKey() *event.KeyEvent { return &event.KeyEvent{Name: desktop.KeyMenu} }

func shiftF10() *event.KeyEvent {
	return &event.KeyEvent{Name: event.KeyF10, Modifier: event.KeyModifierShift}
}

// canvasAbs returns the canvas's window-absolute position — the offset
// keyboardPointEvent adds to canvas-local anchors.
func canvasAbs(c *DAGCanvas) geo.Position {
	return bellui.CurrentApp().Driver().AbsolutePositionForObject(c)
}

func TestMenuKeyFiresNodeSecondaryHook(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	c.nodes["a"].Selected = true
	// The selection is already the menu's target: unlike the pointer path,
	// the keyboard path must not re-select, so On*Selected must not re-fire.
	c.OnNodeSelected = func(string) { t.Fatal("keyboard menu re-fired OnNodeSelected") }
	c.OnNodeActivated = func(string) { t.Fatal("keyboard menu activated") }
	var fired []string
	var gotEv *event.PointEvent
	c.OnNodeSecondaryTapped = func(id string, ev *event.PointEvent) {
		fired = append(fired, id)
		gotEv = ev
	}

	c.TypedKey(menuKey())

	if len(fired) != 1 || fired[0] != "a" {
		t.Fatalf("Menu on selected node: %v", fired)
	}
	// Node "a" defaults to (0,0) 160x80 at zoom 1, so the anchor is its
	// center (80,40) — node-local Position, like the pointer path's widget
	// dispatch, and window-absolute AbsolutePosition.
	if gotEv.Position != geo.NewPos(80, 40) {
		t.Fatalf("Position: %v", gotEv.Position)
	}
	if gotEv.AbsolutePosition != canvasAbs(c).Add(geo.NewPos(80, 40)) {
		t.Fatalf("AbsolutePosition: %v", gotEv.AbsolutePosition)
	}
}

func TestShiftF10FiresNodeSecondaryHook(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	c.nodes["a"].Selected = true
	var fired []string
	c.OnNodeSecondaryTapped = func(id string, ev *event.PointEvent) { fired = append(fired, id) }

	c.TypedKey(shiftF10())
	if len(fired) != 1 || fired[0] != "a" {
		t.Fatalf("Shift+F10 on selected node: %v", fired)
	}
}

func TestPlainF10DoesNotFireSecondaryHook(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	c.nodes["a"].Selected = true
	c.OnNodeSecondaryTapped = func(string, *event.PointEvent) { t.Fatal("plain F10 fired the hook") }

	c.TypedKey(&event.KeyEvent{Name: event.KeyF10})
}

func TestMenuKeyFiresEdgeSecondaryHook(t *testing.T) {
	c := laneCanvas(t)
	edge := &dag.Edge{
		ID:     "e1",
		Source: dag.PortID{NodeID: "a", Name: "out"},
		Target: dag.PortID{NodeID: "b", Name: "in"},
	}
	c.Commands.Do(&AddEdgeCommand{Canvas: c, Edge: edge})
	// Pin the bezier to a known straight segment, as in the pointer test.
	ew := c.edges["e1"]
	ew.p0, ew.p1 = geo.NewPos(160, 40), geo.NewPos(200, 40)
	ew.p2, ew.p3 = geo.NewPos(360, 40), geo.NewPos(400, 40)
	c.deselectAll()
	ew.Selected = true
	c.OnEdgeSelected = func(string) { t.Fatal("keyboard menu re-fired OnEdgeSelected") }
	var gotEv *event.PointEvent
	var fired []string
	c.OnEdgeSecondaryTapped = func(id string, ev *event.PointEvent) {
		fired = append(fired, id)
		gotEv = ev
	}

	c.TypedKey(menuKey())

	if len(fired) != 1 || fired[0] != "e1" {
		t.Fatalf("Menu on selected edge: %v", fired)
	}
	// Bezier midpoint at t=0.5 of the pinned segment: (280,40), canvas-local
	// like the pointer path's canvas dispatch.
	if gotEv.Position != geo.NewPos(280, 40) {
		t.Fatalf("Position: %v", gotEv.Position)
	}
	if gotEv.AbsolutePosition != canvasAbs(c).Add(geo.NewPos(280, 40)) {
		t.Fatalf("AbsolutePosition: %v", gotEv.AbsolutePosition)
	}
}

func TestMenuKeyFiresGroupSecondaryHook(t *testing.T) {
	c := newTestCanvas(t)
	g := &dag.Group{ID: "g1", Label: "G", Position: [2]float32{0, 0}, Size: [2]float32{200, 120}}
	if err := c.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	c.groups["g1"].Selected = true
	c.OnGroupSelected = func(string) { t.Fatal("keyboard menu re-fired OnGroupSelected") }
	c.OnGroupActivated = func(string) { t.Fatal("keyboard menu activated") }
	var gotEv *event.PointEvent
	var fired []string
	c.OnGroupSecondaryTapped = func(id string, ev *event.PointEvent) {
		fired = append(fired, id)
		gotEv = ev
	}

	c.TypedKey(menuKey())

	if len(fired) != 1 || fired[0] != "g1" {
		t.Fatalf("Menu on selected group: %v", fired)
	}
	if gotEv.Position != geo.NewPos(100, 60) { // center of the 200x120 rect
		t.Fatalf("Position: %v", gotEv.Position)
	}
	if gotEv.AbsolutePosition != canvasAbs(c).Add(geo.NewPos(100, 60)) {
		t.Fatalf("AbsolutePosition: %v", gotEv.AbsolutePosition)
	}
}

func TestMenuKeyFiresLinkEdgeSecondaryHook(t *testing.T) {
	c := laneCanvas(t)
	if err := c.AddLinkEdge(&dag.LinkEdge{ID: "l1", Source: "a", Target: "b"}); err != nil {
		t.Fatal(err)
	}
	e := c.linkEdges["l1"]
	e.updateEndpoints()
	e.Selected = true
	c.OnLinkEdgeSelected = func(string) { t.Fatal("keyboard menu re-fired OnLinkEdgeSelected") }
	var gotEv *event.PointEvent
	var fired []string
	c.OnLinkEdgeSecondaryTapped = func(id string, ev *event.PointEvent) {
		fired = append(fired, id)
		gotEv = ev
	}

	c.TypedKey(menuKey())

	if len(fired) != 1 || fired[0] != "l1" {
		t.Fatalf("Menu on selected link edge: %v", fired)
	}
	mid := segMidpoint(e)
	if gotEv.Position != mid {
		t.Fatalf("Position: %v, want segment midpoint %v", gotEv.Position, mid)
	}
	if gotEv.AbsolutePosition != canvasAbs(c).Add(mid) {
		t.Fatalf("AbsolutePosition: %v", gotEv.AbsolutePosition)
	}
}

func TestMenuKeyFiresAnnotationSecondaryHook(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	a := &dag.Annotation{ID: "an1", Text: "note", AnchorKind: dag.AnchorNode, AnchorID: "a", Position: [2]float32{300, 200}}
	if err := c.AddAnnotation(a); err != nil {
		t.Fatal(err)
	}
	aw := c.annotations["an1"]
	aw.Selected = true
	c.OnAnnotationSelected = func(string) { t.Fatal("keyboard menu re-fired OnAnnotationSelected") }
	var gotEv *event.PointEvent
	var fired []string
	c.OnAnnotationSecondaryTapped = func(id string, ev *event.PointEvent) {
		fired = append(fired, id)
		gotEv = ev
	}

	c.TypedKey(menuKey())

	if len(fired) != 1 || fired[0] != "an1" {
		t.Fatalf("Menu on selected annotation: %v", fired)
	}
	// Box-local center, like the pointer path's widget dispatch.
	eff := aw.effectiveSize()
	center := geo.NewPos(eff.Width/2, eff.Height/2)
	if gotEv.Position != center {
		t.Fatalf("Position: %v, want %v", gotEv.Position, center)
	}
	want := canvasAbs(c).Add(geo.NewPos(300+eff.Width/2, 200+eff.Height/2))
	if gotEv.AbsolutePosition != want {
		t.Fatalf("AbsolutePosition: %v, want %v", gotEv.AbsolutePosition, want)
	}
}

func TestMenuKeyNoSelectionNoOps(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	c.OnNodeSecondaryTapped = func(string, *event.PointEvent) { t.Fatal("no selection fired node hook") }
	c.OnGroupSecondaryTapped = func(string, *event.PointEvent) { t.Fatal("no selection fired group hook") }

	c.TypedKey(menuKey())
	c.TypedKey(shiftF10())
}

// A selected group edge has no secondary hook: the keyboard path stays
// silent, matching the pointer path.
func TestMenuKeyGroupEdgeSelectionNoOps(t *testing.T) {
	c := newTestCanvas(t)
	for _, id := range []string{"g1", "g2"} {
		if err := c.AddGroup(&dag.Group{ID: id, Label: id}); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.AddGroupEdge(&dag.GroupEdge{ID: "ge1", Source: "g1", Target: "g2"}); err != nil {
		t.Fatal(err)
	}
	c.groupEdges["ge1"].Selected = true
	c.OnGroupSecondaryTapped = func(string, *event.PointEvent) { t.Fatal("group edge fired group hook") }

	c.TypedKey(menuKey())
}

func TestMenuKeyNilCallbacksDoNotPanic(t *testing.T) {
	c := laneCanvas(t)
	if err := c.AddGroup(&dag.Group{ID: "g1", Label: "G"}); err != nil {
		t.Fatal(err)
	}
	if err := c.AddLinkEdge(&dag.LinkEdge{ID: "l1", Source: "a", Target: "b"}); err != nil {
		t.Fatal(err)
	}
	if err := c.AddAnnotation(&dag.Annotation{ID: "an1", Text: "n", AnchorKind: dag.AnchorNode, AnchorID: "a"}); err != nil {
		t.Fatal(err)
	}
	c.linkEdges["l1"].updateEndpoints()

	for _, sel := range []func(){
		func() { c.nodes["a"].Selected = true },
		func() { c.groups["g1"].Selected = true },
		func() { c.linkEdges["l1"].Selected = true },
		func() { c.annotations["an1"].Selected = true },
	} {
		c.deselectAll()
		sel()
		c.TypedKey(menuKey())
	}
}
