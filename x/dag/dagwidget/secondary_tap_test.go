package dagwidget

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/dag/dag"
)

func TestNodeSecondaryTapSelectsThenFiresHook(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	var order []string
	c.OnNodeSelected = func(id string) { order = append(order, "selected:"+id) }
	c.OnNodeActivated = func(string) { t.Fatal("secondary tap activated") }
	var gotEv *event.PointEvent
	c.OnNodeSecondaryTapped = func(id string, ev *event.PointEvent) {
		order = append(order, "secondary:"+id)
		gotEv = ev
	}

	nw := c.nodes["a"]
	ev := &event.PointEvent{Position: geo.NewPos(10, 10), AbsolutePosition: geo.NewPos(50, 50)}
	nw.TappedSecondary(ev)
	// A second secondary tap within the double-tap window must not activate.
	nw.TappedSecondary(ev)

	if !nw.Selected {
		t.Fatal("secondary tap did not select the node")
	}
	want := []string{"selected:a", "secondary:a", "selected:a", "secondary:a"}
	if len(order) != len(want) {
		t.Fatalf("callback order: %v", order)
	}
	for i, w := range want {
		if order[i] != w {
			t.Fatalf("callback order: %v", order)
		}
	}
	if gotEv != ev {
		t.Fatalf("hook did not forward the tap event: %v", gotEv)
	}
}

func TestGroupTitleSecondaryTapSelectsThenFiresHook(t *testing.T) {
	c := newTestCanvas(t)
	if err := c.AddGroup(&dag.Group{ID: "g1", Label: "G"}); err != nil {
		t.Fatal(err)
	}
	var order []string
	c.OnGroupSelected = func(id string) { order = append(order, "selected:"+id) }
	c.OnGroupActivated = func(string) { t.Fatal("secondary tap activated") }
	c.OnGroupSecondaryTapped = func(id string, ev *event.PointEvent) {
		order = append(order, "secondary:"+id)
	}

	tb := c.groups["g1"].titleBar
	ev := &event.PointEvent{AbsolutePosition: geo.NewPos(30, 10)}
	tb.TappedSecondary(ev)
	tb.TappedSecondary(ev)

	if !c.groups["g1"].Selected {
		t.Fatal("secondary tap did not select the group")
	}
	if len(order) != 4 || order[0] != "selected:g1" || order[1] != "secondary:g1" {
		t.Fatalf("callback order: %v", order)
	}
}

func TestCanvasSecondaryTapHitsGroupBody(t *testing.T) {
	c := newTestCanvas(t)
	g := &dag.Group{ID: "g1", Label: "G", Position: [2]float32{0, 0}, Size: [2]float32{200, 120}}
	if err := c.AddGroup(g); err != nil {
		t.Fatal(err)
	}
	var fired []string
	c.OnGroupSecondaryTapped = func(id string, ev *event.PointEvent) { fired = append(fired, id) }

	c.TappedSecondary(&event.PointEvent{Position: geo.NewPos(100, 80), AbsolutePosition: geo.NewPos(100, 80)})
	if len(fired) != 1 || fired[0] != "g1" {
		t.Fatalf("group body secondary tap: %v", fired)
	}
	if !c.groups["g1"].Selected {
		t.Fatal("secondary tap did not select the group")
	}
}

func TestCanvasSecondaryTapHitsLinkEdge(t *testing.T) {
	c := laneCanvas(t)
	if err := c.AddLinkEdge(&dag.LinkEdge{ID: "l1", Source: "a", Target: "b"}); err != nil {
		t.Fatal(err)
	}
	e := c.linkEdges["l1"]
	e.updateEndpoints()
	var order []string
	c.OnLinkEdgeSelected = func(id string) { order = append(order, "selected:"+id) }
	c.OnLinkEdgeSecondaryTapped = func(id string, ev *event.PointEvent) {
		order = append(order, "secondary:"+id)
	}

	mid := segMidpoint(e)
	c.TappedSecondary(&event.PointEvent{Position: mid, AbsolutePosition: mid})
	if len(order) != 2 || order[0] != "selected:l1" || order[1] != "secondary:l1" {
		t.Fatalf("callback order: %v", order)
	}
	if !e.Selected {
		t.Fatal("secondary tap did not select the link edge")
	}
}

func TestCanvasSecondaryTapHitsEdge(t *testing.T) {
	c := laneCanvas(t)
	edge := &dag.Edge{
		ID:     "e1",
		Source: dag.PortID{NodeID: "a", Name: "out"},
		Target: dag.PortID{NodeID: "b", Name: "in"},
	}
	c.Commands.Do(&AddEdgeCommand{Canvas: c, Edge: edge})
	if c.edges["e1"] == nil {
		t.Fatal("setup edge not added")
	}
	// Pin the bezier to a known straight segment; the renderer would compute
	// the real control points from port positions on layout.
	ew := c.edges["e1"]
	ew.p0, ew.p1 = geo.NewPos(160, 40), geo.NewPos(200, 40)
	ew.p2, ew.p3 = geo.NewPos(360, 40), geo.NewPos(400, 40)
	var order []string
	c.OnEdgeSelected = func(id string) { order = append(order, "selected:"+id) }
	c.OnEdgeSecondaryTapped = func(id string, ev *event.PointEvent) {
		order = append(order, "secondary:"+id)
	}

	c.TappedSecondary(&event.PointEvent{Position: geo.NewPos(280, 40), AbsolutePosition: geo.NewPos(280, 40)})
	if len(order) != 2 || order[0] != "selected:e1" || order[1] != "secondary:e1" {
		t.Fatalf("callback order: %v", order)
	}
	if !ew.Selected {
		t.Fatal("secondary tap did not select the edge")
	}
}

func TestAnnotationSecondaryTapSelectsThenFiresHook(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	if err := c.AddAnnotation(&dag.Annotation{ID: "an1", Text: "note", AnchorKind: dag.AnchorNode, AnchorID: "a"}); err != nil {
		t.Fatal(err)
	}
	var order []string
	c.OnAnnotationSelected = func(id string) { order = append(order, "selected:"+id) }
	c.OnAnnotationSecondaryTapped = func(id string, ev *event.PointEvent) {
		order = append(order, "secondary:"+id)
	}

	aw := c.annotations["an1"]
	aw.TappedSecondary(&event.PointEvent{AbsolutePosition: geo.NewPos(5, 5)})
	if len(order) != 2 || order[0] != "selected:an1" || order[1] != "secondary:an1" {
		t.Fatalf("callback order: %v", order)
	}
	if !aw.Selected {
		t.Fatal("secondary tap did not select the annotation")
	}
}

// A secondary tap on empty canvas is ignored: the selection survives and no
// callback fires — unlike a primary tap, which clears it.
func TestCanvasSecondaryTapOnEmptyCanvasIgnored(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	c.OnSelectionCleared = func() { t.Fatal("empty secondary tap cleared the selection") }

	nw := c.nodes["a"]
	nw.Tapped(&event.PointEvent{AbsolutePosition: geo.NewPos(10, 10)})
	if !nw.Selected {
		t.Fatal("primary tap did not select")
	}

	far := geo.NewPos(900, 900)
	c.TappedSecondary(&event.PointEvent{Position: far, AbsolutePosition: far})
	if !nw.Selected {
		t.Fatal("empty secondary tap deselected the node")
	}
}

func TestSecondaryTapNilCallbacksDoNotPanic(t *testing.T) {
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
	e := c.linkEdges["l1"]
	e.updateEndpoints()

	ev := &event.PointEvent{AbsolutePosition: geo.NewPos(5, 5)}
	c.nodes["a"].TappedSecondary(ev)
	c.groups["g1"].titleBar.TappedSecondary(ev)
	c.annotations["an1"].TappedSecondary(ev)
	mid := segMidpoint(e)
	c.TappedSecondary(&event.PointEvent{Position: mid, AbsolutePosition: mid})
	c.TappedSecondary(&event.PointEvent{Position: geo.NewPos(900, 900), AbsolutePosition: geo.NewPos(900, 900)})
}
