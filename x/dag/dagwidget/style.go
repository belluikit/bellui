package dagwidget

import (
	"image/color"

	"github.com/belluikit/bellui"

	"github.com/belluikit/bellui/x/dag/dag"
)

// NodeStyle is per-node presentation computed by a NodeStyler. Every field
// is optional; the zero value renders the plain default card.
type NodeStyle struct {
	// Icon is drawn in the title row, left of the title.
	Icon bellui.Resource
	// Subtitle is a smaller, dimmed line under the title.
	Subtitle string
	// Accent tints a vertical bar on the card's left edge.
	Accent color.Color
}

// NodeStyler derives a NodeStyle from a node — typically keyed on
// Node.TypeTag or Metadata. Styles are presentation only and deliberately
// not persisted: deriving callers restyle on load. Consulted by the node
// renderer at construction and on every Refresh; keep it cheap.
type NodeStyler func(*dag.Node) NodeStyle

// RefreshNodeStyles re-renders every node so a changed NodeStyler (or
// changed node metadata a styler keys on) takes effect.
func (c *DAGCanvas) RefreshNodeStyles() {
	for _, nw := range c.nodes {
		nw.Refresh()
	}
}
