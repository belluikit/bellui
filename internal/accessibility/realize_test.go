//go:build accessibility

package accessibility_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// gridTable builds a tree with a virtualized grid nested inside a moved
// container and returns the grid's table node, so its absolute Pos is non-zero
// and a realized cell's offset is observable.
func gridTable(t *testing.T) (*accessibility.Node, *virtualGrid) {
	t.Helper()
	test.NewTempApp(t)
	g := newVirtualGrid()
	g.Resize(geo.NewSize(300, 300))
	g.Refresh()

	inner := container.NewWithoutLayout(g)
	inner.Move(geo.NewPos(20, 40))
	inner.Resize(geo.NewSize(300, 300))

	forest := accessibility.BuildTree(inner)
	require.Len(t, forest, 1)
	table := forest[0]
	require.Equal(t, core.RoleTable, table.Role)
	require.Equal(t, geo.NewPos(20, 40), table.Pos, "grid sits at the moved container's origin")
	return table, g
}

// RealizeCell reaches a cell far outside the near-viewport window (row 500000
// with a window around row 5000) and returns it as a wire node in absolute
// canvas coordinates, without materializing the rows between and without
// scrolling.
func TestRealizeCell_OffViewportAbsolutePositionAndIdentity(t *testing.T) {
	table, g := gridTable(t)

	node, ok := accessibility.RealizeCell(table, 500_000, 2)
	require.True(t, ok)
	require.NotNil(t, node)

	assert.Equal(t, core.RoleTableCell, node.Role)
	assert.Equal(t, "r500000c2", node.Label)
	assert.True(t, node.HasCellPosition)
	assert.Equal(t, 500_000, node.CellRow)
	assert.Equal(t, 2, node.CellCol)
	assert.Equal(t, [2]int{500_000, 2}, node.NodeID, "stable identity rides NodeID so the element tracks the record")

	// Absolute canvas position = container origin + the cell's logical offset.
	wantX := table.Pos.X + float32(2*100)
	wantY := table.Pos.Y + float32(500_000*30)
	assert.Equal(t, geo.NewPos(wantX, wantY), node.Pos, "off-viewport cell positioned relative to the grid")

	// A realized off-viewport cell carries no clip: it is delivered on demand
	// outside the reconcile walk and must never be culled the way an off-view
	// window node would be.
	assert.False(t, node.HasClip, "realized cell must not carry a clip (never culled)")

	// Reading a cell has no visible side effect — in particular it does not scroll.
	assert.Nil(t, g.scrolledTo, "AccessibleRealizeCell must not scroll")
}

func TestRealizeCell_OutOfRange(t *testing.T) {
	table, _ := gridTable(t)

	for _, c := range [][2]int{{-1, 0}, {0, -1}, {1_000_000, 0}, {0, 3}} {
		node, ok := accessibility.RealizeCell(table, c[0], c[1])
		assert.False(t, ok, "out-of-range %v should not realize", c)
		assert.Nil(t, node)
	}
}

func TestRealizeCell_NonRealizerAndNil(t *testing.T) {
	test.NewTempApp(t)

	// A plain accessible object that does not implement the realizer.
	plain := newAccWidget("btn", core.RoleButton, widget.NewLabel("btn"))
	plain.Resize(geo.NewSize(100, 30))
	forest := accessibility.BuildTree(container.NewWithoutLayout(plain))
	require.Len(t, forest, 1)

	node, ok := accessibility.RealizeCell(forest[0], 0, 0)
	assert.False(t, ok, "non-realizer object yields no cell")
	assert.Nil(t, node)

	node, ok = accessibility.RealizeCell(nil, 0, 0)
	assert.False(t, ok, "nil container yields no cell")
	assert.Nil(t, node)
}

func TestScrollCellIntoView_RecordsRequest(t *testing.T) {
	table, g := gridTable(t)

	ok := accessibility.ScrollCellIntoView(table, 42, 1)
	assert.True(t, ok, "grid supports scroll-into-view")
	require.NotNil(t, g.scrolledTo)
	assert.Equal(t, [2]int{42, 1}, *g.scrolledTo)
}

func TestScrollCellIntoView_NonScrollerAndNil(t *testing.T) {
	test.NewTempApp(t)

	plain := newAccWidget("btn", core.RoleButton, widget.NewLabel("btn"))
	plain.Resize(geo.NewSize(100, 30))
	forest := accessibility.BuildTree(container.NewWithoutLayout(plain))
	require.Len(t, forest, 1)

	assert.False(t, accessibility.ScrollCellIntoView(forest[0], 0, 0), "non-scroller reports unsupported")
	assert.False(t, accessibility.ScrollCellIntoView(nil, 0, 0), "nil container reports unsupported")
}

// FindVirtualCellRealizer locates the virtualized container in a forest so a
// bridge holding only the canvas nodes can route a realize request to it.
func TestFindVirtualCellRealizer(t *testing.T) {
	test.NewTempApp(t)

	g := newVirtualGrid()
	g.Resize(geo.NewSize(300, 300))
	g.Refresh()

	// Nest the grid under a non-accessible container and beside a plain widget
	// so the finder has to descend and skip a non-realizer.
	plain := newAccWidget("btn", core.RoleButton, widget.NewLabel("btn"))
	plain.Resize(geo.NewSize(100, 30))
	root := container.NewWithoutLayout(plain, container.NewWithoutLayout(g))

	forest := accessibility.BuildTree(root)
	found := accessibility.FindVirtualCellRealizer(forest)
	require.NotNil(t, found)
	assert.Same(t, g, found.Object, "finds the grid node, not the plain button")

	// A forest with no realizer returns nil.
	plainOnly := accessibility.BuildTree(container.NewWithoutLayout(newAccWidget("x", core.RoleButton, widget.NewLabel("x"))))
	assert.Nil(t, accessibility.FindVirtualCellRealizer(plainOnly))
}
