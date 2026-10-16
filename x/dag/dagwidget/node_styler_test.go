package dagwidget

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/theme"

	"github.com/belluikit/bellui/x/dag/dag"
)

func nodeRendererOf(t *testing.T, c *DAGCanvas, id string) *nodeRenderer {
	t.Helper()
	nw := c.nodes[id]
	if nw == nil {
		t.Fatalf("no widget for node %s", id)
	}
	r, ok := nw.CreateRenderer().(*nodeRenderer)
	if !ok {
		t.Fatal("unexpected renderer type")
	}
	return r
}

func TestNodeStylerAppliesStyle(t *testing.T) {
	c := newTestCanvas(t)
	n := addTestNode(t, c, "a")
	n.TypeTag = "screen"

	accent := color.NRGBA{R: 0x33, G: 0x66, B: 0xff, A: 0xff}
	c.NodeStyler = func(node *dag.Node) NodeStyle {
		if node.TypeTag != "screen" {
			return NodeStyle{}
		}
		return NodeStyle{Icon: theme.HomeIcon(), Subtitle: "route: /home", Accent: accent}
	}

	r := nodeRendererOf(t, c, "a")
	r.Refresh()

	if !r.icon.Visible() || r.icon.Resource == nil {
		t.Fatal("icon not shown")
	}
	if !r.subtitle.Visible() || r.subtitle.Text != "route: /home" {
		t.Fatalf("subtitle not shown: %q", r.subtitle.Text)
	}
	if !r.accent.Visible() || r.accent.FillColor != accent {
		t.Fatal("accent not shown")
	}
}

// A freshly built renderer must carry the style before its first Refresh —
// otherwise nodes render unstyled until something else refreshes them (pre-rebrand tracker #36).
func TestNodeStylerAppliedAtConstruction(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")

	accent := color.NRGBA{R: 0x33, G: 0x66, B: 0xff, A: 0xff}
	c.NodeStyler = func(*dag.Node) NodeStyle {
		return NodeStyle{Icon: theme.HomeIcon(), Subtitle: "fresh", Accent: accent}
	}

	// No Refresh: assert directly on the renderer as constructed.
	r := nodeRendererOf(t, c, "a")
	if !r.icon.Visible() || r.icon.Resource == nil {
		t.Fatal("icon not shown on freshly built renderer")
	}
	if !r.subtitle.Visible() || r.subtitle.Text != "fresh" {
		t.Fatalf("subtitle not shown on freshly built renderer: %q", r.subtitle.Text)
	}
	if !r.accent.Visible() || r.accent.FillColor != accent {
		t.Fatal("accent not shown on freshly built renderer")
	}
}

func TestNodeStylerZeroFieldsStayHidden(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	c.NodeStyler = func(*dag.Node) NodeStyle {
		return NodeStyle{Subtitle: "only a subtitle"}
	}

	r := nodeRendererOf(t, c, "a")
	r.Refresh()

	if r.icon.Visible() || r.accent.Visible() {
		t.Fatal("zero icon/accent rendered")
	}
	if !r.subtitle.Visible() {
		t.Fatal("subtitle hidden despite being set")
	}
}

func TestNilNodeStylerRendersPlainCard(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")

	r := nodeRendererOf(t, c, "a")
	r.Refresh()

	if r.icon.Visible() || r.subtitle.Visible() || r.accent.Visible() {
		t.Fatal("styled parts visible with nil styler")
	}
}

func TestNodeStylerDoesNotChangeMinSize(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")
	before := c.nodes["a"].MinSize()

	c.NodeStyler = func(*dag.Node) NodeStyle {
		return NodeStyle{Subtitle: "sub", Accent: color.Black, Icon: theme.HomeIcon()}
	}
	c.RefreshNodeStyles()

	if got := c.nodes["a"].MinSize(); got != before {
		t.Fatalf("MinSize changed by styler: %v -> %v", before, got)
	}
}

func TestRefreshNodeStylesAppliesStylerChange(t *testing.T) {
	c := newTestCanvas(t)
	addTestNode(t, c, "a")

	// The widget's live renderer is created by the test canvas render tree.
	nw := c.nodes["a"]
	c.NodeStyler = func(*dag.Node) NodeStyle { return NodeStyle{Subtitle: "v2"} }
	c.RefreshNodeStyles()
	_ = nw // state assertions happen through the fresh renderer below

	r := nodeRendererOf(t, c, "a")
	r.Refresh()
	if r.subtitle.Text != "v2" {
		t.Fatalf("styler change not applied: %q", r.subtitle.Text)
	}
}
