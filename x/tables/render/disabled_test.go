package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// disabledGrid is a fakeGrid that reports Disabled, as a disabled DataGrid
// does.
type disabledGrid struct {
	*fakeGrid
	off bool
}

func (d *disabledGrid) Disabled() bool { return d.off }
func (d *disabledGrid) Disable()       { d.off = true }
func (d *disabledGrid) Enable()        { d.off = false }

// disabledHeader is a fakeHeaderAccess that reports Disabled and counts
// resize starts.
type disabledHeader struct {
	fakeHeaderAccess
	off     bool
	resizes int
}

func (d *disabledHeader) Disabled() bool                { return d.off }
func (d *disabledHeader) Disable()                      { d.off = true }
func (d *disabledHeader) Enable()                       { d.off = false }
func (d *disabledHeader) HeaderBorderCol(x float32) int { return 0 }
func (d *disabledHeader) OnResizeDragStart(int, float32) {
	d.resizes++
}

// The drivers hand an accessible action, a header press and a header resize
// drag to the part under them whatever its grid's state, so each part the
// grid builds asks the grid (#1217). Disabled, every one is refused; enabled
// again, each one routes as before.
func TestRenderParts_RefuseInputWhileGridDisabled(t *testing.T) {
	test.NewTempApp(t)
	fg := &fakeGrid{
		rows: 3, cols: 2,
		groupRows:      map[int]fakeGroupInfo{0: {key: "g"}},
		treeActive:     true,
		treeParent:     map[int]bool{1: true},
		masterActive:   true,
		masterRows:     map[int]bool{2: true},
		masterExpanded: map[int]bool{},
	}
	grid := &disabledGrid{fakeGrid: fg, off: true}

	host := newCellHost(grid, canvas.NewText("", nil))
	host.row, host.col = 1, 1
	group := newGroupRowA11y(grid)
	group.set("g", 1, false, nil)
	tree := newTreeRowA11y(grid)
	tree.set(1, "row", 1, false)
	master := newMasterRowA11y(grid)
	master.set(2, "row", false)

	hdr := &disabledHeader{off: true}
	headerNode := newHeaderCellA11y(hdr)
	headerNode.set(1, "H1", SortIndicatorNone, true)
	header := NewHeaderWidget(hdr, &Viewport{})

	act := func() {
		host.PerformAccessibleAction(core.ActionSelect)
		group.PerformAccessibleAction(core.ActionExpand)
		tree.PerformAccessibleAction(core.ActionExpand)
		master.PerformAccessibleAction(core.ActionExpand)
		headerNode.PerformAccessibleAction(core.ActionPress)
		header.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: geo.NewPos(80, 10)}})
		header.HoverMoved(&event.HoverEvent{PointEvent: event.PointEvent{Position: geo.NewPos(80, 10)}})
	}

	act()
	assert.Empty(t, fg.selectedCells, "a disabled grid's cell refuses select")
	assert.Empty(t, fg.toggled, "a disabled grid's group row refuses expand")
	assert.Empty(t, fg.toggledTree, "a disabled grid's tree row refuses expand")
	assert.Empty(t, fg.toggledDetails, "a disabled grid's master row refuses expand")
	assert.Empty(t, hdr.tapped, "a disabled grid's header refuses press")
	assert.Zero(t, hdr.resizes, "a disabled grid's header starts no resize")
	assert.Equal(t, desktop.DefaultCursor, header.Cursor(), "no resize cursor while disabled")

	grid.Enable()
	hdr.Enable()
	act()
	assert.Len(t, fg.selectedCells, 1, "enabled, the cell selects")
	assert.Len(t, fg.toggled, 1, "enabled, the group row expands")
	assert.Len(t, fg.toggledTree, 1, "enabled, the tree row expands")
	assert.Len(t, fg.toggledDetails, 1, "enabled, the master row expands")
	assert.Len(t, hdr.tapped, 1, "enabled, the header node sorts")
	assert.Equal(t, 1, hdr.resizes, "enabled, a press on a border starts a resize")
}
