package dagwidget

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/dag/dag"
)

func tabKey() *event.KeyEvent { return &event.KeyEvent{Name: event.KeyTab} }

func shiftTabKey() *event.KeyEvent {
	return &event.KeyEvent{Name: event.KeyTab, Modifier: event.KeyModifierShift}
}

// fullCanvas builds a canvas holding one element of every selectable kind:
// nodes a and b, edge e1, groups g1 and g2, group edge ge1, link edge l1,
// annotation an1. Traversal order over it is exactly that sequence.
func fullCanvas(t *testing.T) *DAGCanvas {
	t.Helper()
	c := laneCanvas(t)
	c.Commands.Do(&AddEdgeCommand{Canvas: c, Edge: &dag.Edge{
		ID:     "e1",
		Source: dag.PortID{NodeID: "a", Name: "out"},
		Target: dag.PortID{NodeID: "b", Name: "in"},
	}})
	if c.edges["e1"] == nil {
		t.Fatal("edge e1 not added")
	}
	for _, id := range []string{"g1", "g2"} {
		if err := c.AddGroup(&dag.Group{ID: id, Label: id, Position: [2]float32{0, 200}, Size: [2]float32{200, 100}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.AddGroupEdge(&dag.GroupEdge{ID: "ge1", Source: "g1", Target: "g2"}); err != nil {
		t.Fatal(err)
	}
	if err := c.AddLinkEdge(&dag.LinkEdge{ID: "l1", Source: "a", Target: "b"}); err != nil {
		t.Fatal(err)
	}
	if err := c.AddAnnotation(&dag.Annotation{ID: "an1", Text: "note", AnchorKind: dag.AnchorNode, AnchorID: "a"}); err != nil {
		t.Fatal(err)
	}
	return c
}

var fullOrder = []selectableRef{
	{kindNode, "a"},
	{kindNode, "b"},
	{kindEdge, "e1"},
	{kindGroup, "g1"},
	{kindGroup, "g2"},
	{kindGroupEdge, "ge1"},
	{kindLinkEdge, "l1"},
	{kindAnnotation, "an1"},
}

func TestSelectMethodsDriveTapPath(t *testing.T) {
	c := fullCanvas(t)
	var got []string
	c.OnNodeSelected = func(id string) { got = append(got, "node:"+id) }
	c.OnEdgeSelected = func(id string) { got = append(got, "edge:"+id) }
	c.OnGroupSelected = func(id string) { got = append(got, "group:"+id) }
	c.OnLinkEdgeSelected = func(id string) { got = append(got, "link:"+id) }
	c.OnAnnotationSelected = func(id string) { got = append(got, "ann:"+id) }

	steps := []struct {
		call     func() bool
		want     string
		selected func() bool
	}{
		{func() bool { return c.SelectNode("a") }, "node:a", func() bool { return c.nodes["a"].Selected }},
		{func() bool { return c.SelectEdge("e1") }, "edge:e1", func() bool { return c.edges["e1"].Selected }},
		{func() bool { return c.SelectGroup("g1") }, "group:g1", func() bool { return c.groups["g1"].Selected }},
		{func() bool { return c.SelectLinkEdge("l1") }, "link:l1", func() bool { return c.linkEdges["l1"].Selected }},
		{func() bool { return c.SelectAnnotation("an1") }, "ann:an1", func() bool { return c.annotations["an1"].Selected }},
	}
	for i, s := range steps {
		got = nil
		if !s.call() {
			t.Fatalf("step %d: select returned false", i)
		}
		if len(got) != 1 || got[0] != s.want {
			t.Fatalf("step %d: callbacks %v, want [%s]", i, got, s.want)
		}
		if !s.selected() {
			t.Fatalf("step %d: Selected flag not set", i)
		}
		if ref, ok := c.currentSelection(); !ok || !s.selected() {
			t.Fatalf("step %d: currentSelection %v %v", i, ref, ok)
		}
	}
	// The last selection is the sole one: the first step's target was cleared.
	if c.nodes["a"].Selected {
		t.Fatal("earlier selection not cleared")
	}
}

// A group edge selects silently — it has no selection callback, matching the
// pointer path.
func TestSelectGroupEdgeSilently(t *testing.T) {
	c := fullCanvas(t)
	if !c.SelectGroupEdge("ge1") {
		t.Fatal("SelectGroupEdge returned false")
	}
	if !c.groupEdges["ge1"].Selected {
		t.Fatal("group edge not selected")
	}
}

func TestSelectUnknownIDLeavesSelection(t *testing.T) {
	c := fullCanvas(t)
	c.SelectNode("a")
	c.OnNodeSelected = func(string) { t.Fatal("unknown-ID select fired a callback") }
	for i, call := range []func() bool{
		func() bool { return c.SelectNode("zzz") },
		func() bool { return c.SelectEdge("zzz") },
		func() bool { return c.SelectGroup("zzz") },
		func() bool { return c.SelectGroupEdge("zzz") },
		func() bool { return c.SelectLinkEdge("zzz") },
		func() bool { return c.SelectAnnotation("zzz") },
	} {
		if call() {
			t.Fatalf("call %d: unknown ID selected", i)
		}
	}
	if !c.nodes["a"].Selected {
		t.Fatal("existing selection lost")
	}
}

func TestClearSelectionFiresCallback(t *testing.T) {
	c := fullCanvas(t)
	c.SelectNode("a")
	cleared := 0
	c.OnSelectionCleared = func() { cleared++ }

	c.ClearSelection()
	if cleared != 1 {
		t.Fatalf("OnSelectionCleared fired %d times", cleared)
	}
	if _, ok := c.currentSelection(); ok {
		t.Fatal("selection not cleared")
	}
}

func TestEscapeClearsSelection(t *testing.T) {
	c := fullCanvas(t)
	c.SelectNode("a")
	cleared := 0
	c.OnSelectionCleared = func() { cleared++ }

	c.TypedKey(&event.KeyEvent{Name: event.KeyEscape})
	if cleared != 1 {
		t.Fatalf("OnSelectionCleared fired %d times", cleared)
	}
	if _, ok := c.currentSelection(); ok {
		t.Fatal("selection survived Escape")
	}
}

func TestAcceptsTab(t *testing.T) {
	if !newTestCanvas(t).AcceptsTab() {
		t.Fatal("canvas must capture Tab for element traversal")
	}
}

func TestTabCyclesElementsInStableOrder(t *testing.T) {
	c := fullCanvas(t)
	// One full cycle plus one: the wrap lands back on the first element.
	for i := 0; i <= len(fullOrder); i++ {
		c.TypedKey(tabKey())
		want := fullOrder[i%len(fullOrder)]
		got, ok := c.currentSelection()
		if !ok || got != want {
			t.Fatalf("Tab %d: selection %v %v, want %v", i+1, got, ok, want)
		}
	}
}

func TestShiftTabTraversesBackward(t *testing.T) {
	c := fullCanvas(t)
	// Nothing selected: Shift+Tab starts at the last element, then walks
	// backward through the whole order.
	for i := 0; i < len(fullOrder); i++ {
		c.TypedKey(shiftTabKey())
		want := fullOrder[len(fullOrder)-1-i]
		got, ok := c.currentSelection()
		if !ok || got != want {
			t.Fatalf("Shift+Tab %d: selection %v %v, want %v", i+1, got, ok, want)
		}
	}
}

// Traversal fires the same callbacks as a tap.
func TestTraversalFiresSelectionCallbacks(t *testing.T) {
	c := fullCanvas(t)
	var got []string
	c.OnNodeSelected = func(id string) { got = append(got, id) }

	c.TypedKey(tabKey())
	c.TypedKey(tabKey())
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("OnNodeSelected calls: %v", got)
	}
}

func TestTraversalSkipsElementsHiddenByCollapse(t *testing.T) {
	c := fullCanvas(t)
	for _, n := range []string{"a", "b"} {
		if err := c.Graph.AssignNodeToGroup(n, "g1"); err != nil {
			t.Fatal(err)
		}
	}
	c.Graph.Groups["g1"].Collapsed = true

	// Member nodes, the intra-group edge, and the intra-group link edge are
	// all hidden by the collapse; the annotation and both groups remain.
	want := []selectableRef{
		{kindGroup, "g1"},
		{kindGroup, "g2"},
		{kindGroupEdge, "ge1"},
		{kindAnnotation, "an1"},
	}
	for i, w := range want {
		c.TypedKey(tabKey())
		got, ok := c.currentSelection()
		if !ok || got != w {
			t.Fatalf("Tab %d: selection %v %v, want %v", i+1, got, ok, w)
		}
	}
}

// A selection that traversal skips (hidden mid-session by a collapse)
// restarts traversal at the first reachable element instead of stalling.
func TestTraversalRestartsFromHiddenSelection(t *testing.T) {
	c := fullCanvas(t)
	c.SelectNode("b")
	if err := c.Graph.AssignNodeToGroup("b", "g1"); err != nil {
		t.Fatal(err)
	}
	c.Graph.Groups["g1"].Collapsed = true

	c.TypedKey(tabKey())
	got, ok := c.currentSelection()
	if want := (selectableRef{kindNode, "a"}); !ok || got != want {
		t.Fatalf("selection %v %v, want %v", got, ok, want)
	}
}

// The a11y story end to end: Tab establishes a selection, the Menu key opens
// its context menu.
func TestTabThenMenuKeyFiresSecondaryHook(t *testing.T) {
	c := fullCanvas(t)
	var fired []string
	c.OnNodeSecondaryTapped = func(id string, ev *event.PointEvent) { fired = append(fired, id) }

	c.TypedKey(tabKey())
	c.TypedKey(menuKey())
	if len(fired) != 1 || fired[0] != "a" {
		t.Fatalf("Menu after Tab: %v", fired)
	}
}

func TestTraversalPansOffscreenElementIntoView(t *testing.T) {
	c := laneCanvas(t)
	c.Graph.Nodes["b"].Position = [2]float32{5000, 5000}
	c.Resize(geo.NewSize(300, 200))

	c.TypedKey(tabKey()) // node a: center (80,40) already on screen
	if c.Offset != geo.NewPos(0, 0) {
		t.Fatalf("Offset moved for on-screen element: %v", c.Offset)
	}

	c.TypedKey(tabKey()) // node b: far off screen, viewport pans to center it
	rect, ok := c.nodeVisibleScreenRect("b")
	if !ok {
		t.Fatal("node b unresolved")
	}
	if center, want := rectCenter(rect), geo.NewPos(150, 100); center != want {
		t.Fatalf("panned center %v, want viewport center %v", center, want)
	}
}
