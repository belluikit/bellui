package tables

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
	"github.com/belluikit/bellui/x/tables/cell"
)

// gridCallbacks counts every callback a DataGrid fires.
type gridCallbacks struct {
	clicked, doubleClicked, secondary, selection, sorted, filtered int
	valueChanged, activated, contextMenu, rowMoved                 int
}

func (c *gridCallbacks) total() int {
	return c.clicked + c.doubleClicked + c.secondary + c.selection + c.sorted + c.filtered +
		c.valueChanged + c.activated + c.contextMenu + c.rowMoved
}

func (c *gridCallbacks) wire(g *DataGrid) {
	g.OnCellClicked = func(CellID) { c.clicked++ }
	g.OnCellDoubleClicked = func(CellID) { c.doubleClicked++ }
	g.OnCellSecondaryTapped = func(CellID, *event.PointEvent) { c.secondary++ }
	g.OnSelectionChanged = func([]int) { c.selection++ }
	g.OnSortChanged = func([]SortItem) { c.sorted++ }
	g.OnFilterChanged = func() { c.filtered++ }
	g.OnCellValueChanged = func(CellID, any, any) { c.valueChanged++ }
	g.OnRowActivated = func(int) { c.activated++ }
	g.OnCellContextMenu = func(CellID, geo.Position) { c.contextMenu++ }
	g.OnRowMoved = func(int, int) { c.rowMoved++ }
}

// walkGridTree calls fn on o and on every object under it, through the
// grid's cached renderers (test.TempWidgetRenderer returns the cached one,
// where test.LaidOutObjects would build fresh ones and hide what is drawn).
func walkGridTree(t *testing.T, o core.CanvasObject, fn func(core.CanvasObject)) {
	t.Helper()
	fn(o)
	switch x := o.(type) {
	case *bellui.Container:
		for _, c := range x.Objects {
			walkGridTree(t, c, fn)
		}
	case core.Widget:
		for _, c := range test.TempWidgetRenderer(t, x).Objects() {
			walkGridTree(t, c, fn)
		}
	}
}

// gridLook collects what the disabled look changes: the header title's and
// a built-in cell's text colour, and the disabled state of the filter
// entries and select checkboxes the grid builds.
type gridLook struct {
	headerColor, cellColor any
	entries, checks        []core.Disableable
}

func lookOf(t *testing.T, g *DataGrid) gridLook {
	t.Helper()
	var l gridLook
	walkGridTree(t, g.headerWidget, func(o core.CanvasObject) {
		if tx, ok := o.(*canvas.Text); ok && tx.Text == "Name" {
			l.headerColor = tx.Color
		}
	})
	walkGridTree(t, g.cellsWidget, func(o core.CanvasObject) {
		switch x := o.(type) {
		case *canvas.Text:
			if x.Text == "row3" {
				l.cellColor = x.Color
			}
		case *widget.Check:
			l.checks = append(l.checks, x)
		}
	})
	walkGridTree(t, g.floatingFilterRow, func(o core.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok {
			l.entries = append(l.entries, e)
		}
	})
	return l
}

func assertAllDisabled(t *testing.T, parts []core.Disableable, want bool, what string) {
	t.Helper()
	require.NotEmpty(t, parts, what)
	for _, p := range parts {
		assert.Equal(t, want, p.Disabled(), what)
	}
}

// A disabled DataGrid is visible but inert (#1217, belluikit/player#641).
// Disable works before the grid is drawn; while disabled no tap, drag,
// scroll, key, shortcut or editor/popup request changes anything or fires a
// callback; Tab skips the grid and its parts, and a focus on either is
// dropped; the grid's own text takes the disabled colour. Enable restores
// all of it.
func TestDataGrid_Disable(t *testing.T) {
	editable := true
	g := newTestGrid(t, manyRows(200), []*ColumnDef{
		{ColID: "n", Field: "n", HeaderName: "Name", Editable: &editable},
		{ColID: "m", Field: "n", HeaderName: "Copy"},
	})
	g.Options.FloatingFilters = true
	g.Options.CheckboxSelection = true
	g.SelectionMode = SelectionMultiRow
	var cb gridCallbacks
	cb.wire(g)

	// Before the first draw: no window, no renderer.
	g.Disable()
	require.True(t, g.Disabled())

	w := test.NewTempWindow(t, g)
	w.Resize(geo.NewSize(400, 400))
	c := w.Canvas()

	disabledFg := theme.Color(theme.ColorNameDisabled)
	look := lookOf(t, g)
	assert.Equal(t, disabledFg, look.headerColor, "header title in the disabled colour")
	assert.Equal(t, disabledFg, look.cellColor, "built-in cell text in the disabled colour")
	assertAllDisabled(t, look.entries, true, "filter entries disabled")
	assertAllDisabled(t, look.checks, true, "select checkboxes disabled")
	assert.Equal(t, container.ScrollNone, g.scrollContent.Scroll.Direction, "the body scroller is off")

	headerH := g.HeaderHeight()
	bodyY := headerH + 30 + g.rowHeight()*3 + 2 // below the floating filter row
	body := geo.NewPos(120, bodyY)
	headerCell := geo.NewPos(40, headerH/2)
	widthBefore := g.ColumnWidths()[0]

	refuse := func() {
		t.Helper()
		test.TapCanvas(c, body)
		g.DoubleTapped(&event.PointEvent{Position: body})
		g.TappedSecondary(&event.PointEvent{Position: body})
		g.TappedSecondary(&event.PointEvent{Position: headerCell})
		g.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: body}, Button: desktop.MouseButtonPrimary})
		g.MouseUp(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: body}, Button: desktop.MouseButtonPrimary})
		g.headerWidget.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: headerCell}})
		test.Drag(c, body, 0, -120) // a touch drag over the body
		g.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: geo.NewPos(g.ColumnWidths()[0]+g.cellPadding()+30, headerH/2)}})
		g.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(300, headerH/2)}, Dragged: geo.NewDelta(150, 0)})
		g.DragEnd()
		test.Scroll(c, body, 0, -200)                                   // the wheel over the body
		g.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -200)}) // an AT scroll
		g.AccessibleScrollCellIntoView(150, 0)
		for _, k := range []event.KeyName{event.KeyDown, event.KeyRight, event.KeyF2, event.KeyReturn, event.KeySpace, desktop.KeyMenu} {
			g.TypedKey(&event.KeyEvent{Name: k})
		}
		g.TypedShortcut(&bellui.ShortcutSelectAll{})
		g.StartEditing(CellID{Row: 1, Col: 0})
		g.ShowFilterPopup(0, geo.NewPos(10, 10))
		for _, e := range look.entries {
			test.Type(e.(*widget.Entry), "row1")
		}
		c.Unfocus()
	}

	refuse()
	assert.Zero(t, cb.total(), "no callback fires while disabled: %+v", cb)
	assert.Empty(t, g.API().GetSelectedRows(), "nothing selected")
	assert.Equal(t, CellID{}, g.currentFocus, "the keyboard cursor did not move")
	assert.Equal(t, float32(0), g.scrollContent.Offset().Y, "the body did not scroll")
	assert.Equal(t, widthBefore, g.ColumnWidths()[0], "no column resized")
	assert.Empty(t, g.sortModel, "no column sorted")
	assert.False(t, g.IsEditing(), "no editor opened")
	assert.Empty(t, c.Overlays().List(), "no popup or menu opened")
	assert.Equal(t, 200, g.RowCount(), "no filter applied")

	// Tab skips the grid and the parts it builds.
	test.FocusNext(c)
	assert.Nil(t, c.Focused(), "Tab skips a disabled grid, its filter entries and checkboxes")

	// Enable restores the look and the input.
	g.Enable()
	look = lookOf(t, g)
	assert.NotEqual(t, disabledFg, look.headerColor, "header title back to its colour")
	assert.NotEqual(t, disabledFg, look.cellColor, "cell text back to its colour")
	assertAllDisabled(t, look.entries, false, "filter entries enabled")
	assertAllDisabled(t, look.checks, false, "select checkboxes enabled")
	assert.Equal(t, container.ScrollBoth, g.scrollContent.Scroll.Direction, "the body scroller is back")

	test.TapCanvas(c, body)
	assert.Equal(t, 1, cb.clicked, "enabled, a tap clicks a cell")
	test.Scroll(c, body, 0, -200)
	assert.Greater(t, g.scrollContent.Offset().Y, float32(0), "enabled, the wheel scrolls")
	test.FocusNext(c)
	assert.NotNil(t, c.Focused(), "enabled, Tab reaches the grid")

	// Disable after the first draw, with the focus on the grid itself...
	c.Focus(g)
	require.Equal(t, g, c.Focused())
	g.Disable()
	assert.Nil(t, c.Focused(), "Disable drops the focus on the grid")
	assert.Equal(t, disabledFg, lookOf(t, g).headerColor, "the look applies after the first draw")

	// ...on one of its filter entries...
	g.Enable()
	entry := lookOf(t, g).entries[0].(*widget.Entry)
	c.Focus(entry)
	require.Equal(t, entry, c.Focused())
	g.Disable()
	assert.Nil(t, c.Focused(), "Disable drops the focus on a filter entry")

	// ...and in an open editor, which closes without committing.
	g.Enable()
	g.ScrollToRow(0) // the editor shows only over a cell in view
	g.StartEditing(CellID{Row: 0, Col: 0})
	require.True(t, g.IsEditing())
	editor, ok := g.editWidget.(*cell.Entry)
	require.True(t, ok)
	c.Focus(editor) // as a tap into it would; the test canvas did not take StartEditing's focus
	require.True(t, c.Focused() == editor, "the editor holds the focus")
	editor.SetText("changed")
	before := cb.valueChanged
	g.Disable()
	assert.False(t, g.IsEditing(), "Disable closes the editor")
	assert.Nil(t, c.Focused(), "Disable drops the focus in the editor")
	assert.Equal(t, before, cb.valueChanged, "the edit is not committed")
	assert.Equal(t, "row0", g.DisplayRowData(0).(map[string]any)["n"], "the row keeps its value")

	// Repeated calls are no-ops.
	g.Disable()
	assert.True(t, g.Disabled())
	g.Enable()
	g.Enable()
	assert.False(t, g.Disabled())
}
