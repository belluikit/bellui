package tables

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/tables/cell"
	"github.com/belluikit/bellui/x/tables/render"
)

// An edit ends, keeping what was typed, when the user presses elsewhere: on
// another cell, on the header, or outside the grid (#1522). Escape still
// cancels, and a window losing focus keeps the edit.

func editRows(n int) []map[string]any {
	rows := make([]map[string]any, n)
	for i := range rows {
		rows[i] = map[string]any{"id": i, "a": fmt.Sprintf("a%02d", i), "b": fmt.Sprintf("b%02d", i)}
	}
	return rows
}

// cellPoint is the grid-local centre of a display cell in the first band.
func cellPoint(g *DataGrid, row int, x float32) geo.Position {
	ry, rh := g.viewport.RowY(row)
	return geo.NewPos(x, g.bodyTop()+ry-g.offset.Y+rh/2)
}

// A press on another cell ends the edit: the desktop driver sends the grid a
// MouseDown and leaves the focus in the editor, and a touch screen sends a
// tap.
func TestDataGrid_EditEndsOnPressInGrid(t *testing.T) {
	data := editRows(3)
	g := newTestGrid(t, data, []*ColumnDef{{Field: "a", Width: 120}})
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	g.StartEditing(CellID{Row: 0, Col: 0})
	g.editWidget.(*cell.Entry).SetText("pressed")
	g.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: cellPoint(g, 2, 60)}, Button: desktop.MouseButtonPrimary})
	assert.False(t, g.IsEditing(), "a press on another cell ends the edit")
	assert.Equal(t, "pressed", data[0]["a"], "and keeps what was typed")

	g.StartEditing(CellID{Row: 1, Col: 0})
	g.editWidget.(*cell.Entry).SetText("tapped")
	g.Tapped(&event.PointEvent{Position: cellPoint(g, 2, 60)})
	assert.False(t, g.IsEditing(), "a tap on another cell ends the edit")
	assert.Equal(t, "tapped", data[1]["a"])
}

// A tap on the header ends the edit before it sorts, so what was typed lands
// in the row it was typed in.
func TestDataGrid_EditEndsOnHeaderTap(t *testing.T) {
	data := []map[string]any{{"a": "b"}, {"a": "a"}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "a", Width: 120}})
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	g.StartEditing(CellID{Row: 0, Col: 0})
	g.editWidget.(*cell.Entry).SetText("z")
	g.headerWidget.Tapped(&event.PointEvent{Position: geo.NewPos(60, g.HeaderHeight()/2)})

	assert.False(t, g.IsEditing(), "a header tap ends the edit")
	assert.Equal(t, "z", data[0]["a"], "the typed value is in the row it was typed in")
	assert.Equal(t, render.SortIndicatorAsc, g.SortState(0), "and the tap still sorts")
}

// When the focus moves outside the grid, to another widget or to nothing,
// the edit ends and the focus stays where it went.
func TestDataGrid_EditEndsWhenFocusLeaves(t *testing.T) {
	data := editRows(3)
	g := newTestGrid(t, data, []*ColumnDef{{Field: "a", Width: 120}})
	other := widget.NewEntry()
	w := test.NewWindow(container.NewBorder(other, nil, nil, nil, g))
	defer w.Close()
	w.Resize(geo.NewSize(400, 340))
	c := w.Canvas()

	g.StartEditing(CellID{Row: 0, Col: 0})
	editor := g.editWidget.(*cell.Entry)
	c.Focus(editor)
	require.True(t, c.Focused() == editor)
	editor.SetText("to other")
	c.Focus(other)
	assert.False(t, g.IsEditing(), "focusing another widget ends the edit")
	assert.Equal(t, "to other", data[0]["a"])
	assert.True(t, c.Focused() == other, "the focus stays where it went")

	g.StartEditing(CellID{Row: 1, Col: 0})
	editor = g.editWidget.(*cell.Entry)
	c.Focus(editor)
	editor.SetText("to nothing")
	c.Unfocus() // a click on nothing focusable
	assert.False(t, g.IsEditing(), "a click on nothing focusable ends the edit")
	assert.Equal(t, "to nothing", data[1]["a"])
	assert.Nil(t, c.Focused())
}

// A window losing focus tells the focused editor without moving the focus.
// The edit stays open, so switching apps loses nothing.
func TestDataGrid_EditSurvivesWindowBlur(t *testing.T) {
	data := editRows(3)
	g := newTestGrid(t, data, []*ColumnDef{{Field: "a", Width: 120}})
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	c := w.Canvas()

	g.StartEditing(CellID{Row: 0, Col: 0})
	editor := g.editWidget.(*cell.Entry)
	c.Focus(editor)
	editor.SetText("half typed")
	editor.FocusLost() // what the focus manager does when the window blurs
	assert.True(t, g.IsEditing(), "the edit survives the window losing focus")
	assert.Equal(t, "a00", data[0]["a"], "nothing is written")
}

// Escape cancels and Enter commits once, with the editor focused as it is in
// a running app. Handing the focus back to the grid makes the editor lose
// it, and that must not end the edit a second time.
func TestDataGrid_EditKeysWithFocusedEditor(t *testing.T) {
	data := editRows(3)
	g := newTestGrid(t, data, []*ColumnDef{{Field: "a", Width: 120}})
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	c := w.Canvas()
	changes := 0
	g.OnCellValueChanged = func(CellID, any, any) { changes++ }

	g.StartEditing(CellID{Row: 0, Col: 0})
	editor := g.editWidget.(*cell.Entry)
	c.Focus(editor)
	editor.SetText("discarded")
	editor.TypedKey(&event.KeyEvent{Name: event.KeyEscape})
	assert.False(t, g.IsEditing())
	assert.Equal(t, "a00", data[0]["a"], "Escape writes nothing")
	assert.Zero(t, changes)
	assert.False(t, g.CanUndo())
	assert.True(t, c.Focused() == g, "the focus is back on the grid")

	g.StartEditing(CellID{Row: 0, Col: 0})
	editor = g.editWidget.(*cell.Entry)
	c.Focus(editor)
	editor.SetText("kept")
	editor.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	assert.False(t, g.IsEditing())
	assert.Equal(t, "kept", data[0]["a"])
	assert.Equal(t, 1, changes, "Enter commits once")
	assert.True(t, c.Focused() == g)
}

// An editor scrolled partly under the header is clipped to the body: a tap on
// the header where the editor's top half would be reaches the header.
func TestDataGrid_EditorClippedBelowHeader(t *testing.T) {
	data := editRows(30)
	g := newTestGrid(t, data, []*ColumnDef{{Field: "a", Width: 120}})
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	g.StartEditing(CellID{Row: 1, Col: 0})
	ry, rh := g.viewport.RowY(1)
	g.onScrolled(geo.NewPos(0, ry+rh/2)) // row 1's top half is under the header
	_, _, visible := g.editingCellRect(CellID{Row: 1, Col: 0}, g.Size())
	require.True(t, visible, "the editor's lower half is in the body")

	test.TapCanvas(w.Canvas(), geo.NewPos(60, g.bodyTop()-4))
	assert.Equal(t, render.SortIndicatorAsc, g.SortState(0), "the tap reached the header, not the hidden part of the editor")
}

// A main-column editor scrolled partly under the pinned-left band is clipped
// to the main band: a tap on the pinned cell beside it reaches that cell.
func TestDataGrid_EditorClippedBesidePinnedBand(t *testing.T) {
	data := editRows(3)
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "id", Width: 60, Pinned: PinLeft},
		{Field: "a", Width: 160},
		{Field: "b", Width: 160},
		{Field: "c", Width: 160},
	})
	var clicked []CellID
	g.OnCellClicked = func(id CellID) { clicked = append(clicked, id) }
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	g.StartEditing(CellID{Row: 0, Col: 1})
	g.onScrolled(geo.NewPos(80, 0)) // column a's left half is under the pinned band
	_, _, visible := g.editingCellRect(CellID{Row: 0, Col: 1}, g.Size())
	require.True(t, visible, "the editor's right half is in the main band")

	mainX, _, _ := g.bandAnchors(g.Size())
	test.TapCanvas(w.Canvas(), cellPoint(g, 0, mainX-20))
	require.Len(t, clicked, 1, "the tap reached the grid, not the hidden part of the editor")
	assert.Equal(t, CellID{Row: 0, Col: 0}, clicked[0])
}

// A press on a column border, which starts a resize, ends the edit.
func TestDataGrid_EditEndsOnColumnBorderPress(t *testing.T) {
	data := editRows(3)
	g := newTestGrid(t, data, []*ColumnDef{{Field: "a", Width: 120}, {Field: "b", Width: 120}})
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	border := float32(-1)
	for x := float32(100); x < 140; x++ {
		if g.HeaderBorderCol(x) == 0 {
			border = x
			break
		}
	}
	require.GreaterOrEqual(t, border, float32(0), "found column a's border")

	g.StartEditing(CellID{Row: 0, Col: 1})
	g.editWidget.(*cell.Entry).SetText("resized")
	press := &desktop.MouseEvent{PointEvent: event.PointEvent{Position: geo.NewPos(border, g.HeaderHeight()/2)}, Button: desktop.MouseButtonPrimary}
	g.headerWidget.MouseDown(press)
	g.headerWidget.MouseUp(press)
	assert.False(t, g.IsEditing(), "a press on a column border ends the edit")
	assert.Equal(t, "resized", data[0]["b"])
}

// An editor scrolled partly under the status bar is clipped above it: a tap
// on the status bar where the editor's lower half would be reaches the grid.
func TestDataGrid_EditorClippedAboveStatusBar(t *testing.T) {
	data := editRows(30)
	g := newTestGrid(t, data, []*ColumnDef{{Field: "a", Width: 120}})
	g.Options.ShowStatusBar = true
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	const row = 10
	g.StartEditing(CellID{Row: row, Col: 0})
	bottom := g.Size().Height - statusBarHeight
	ry, rh := g.viewport.RowY(row)
	g.onScrolled(geo.NewPos(0, g.bodyTop()+ry-bottom+rh/2)) // the row's lower half is under the status bar
	_, _, visible := g.editingCellRect(CellID{Row: row, Col: 0}, g.Size())
	require.True(t, visible, "the editor's upper half is in the body")

	test.TapCanvas(w.Canvas(), geo.NewPos(60, bottom+4))
	assert.False(t, g.IsEditing(), "the tap reached the grid, which ends the edit, not the hidden part of the editor")
}

// Right to left the pinned band is on the right, and the clip follows it.
func TestDataGrid_EditorClippedBesidePinnedBandRTL(t *testing.T) {
	data := editRows(3)
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "id", Width: 60, Pinned: PinLeft},
		{Field: "a", Width: 160},
		{Field: "b", Width: 160},
		{Field: "c", Width: 160},
	})
	var clicked []CellID
	g.OnCellClicked = func(id CellID) { clicked = append(clicked, id) }
	w := test.NewWindow(container.NewDirectionScope(g, bellui.DirectionRTL))
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	require.True(t, g.rtl, "the grid was laid out right to left")

	g.StartEditing(CellID{Row: 0, Col: 1})
	g.onScrolled(geo.NewPos(g.flipScrollX(80), 0)) // column a's leading half is under the pinned band
	require.Equal(t, float32(80), g.offset.X)
	_, _, visible := g.editingCellRect(CellID{Row: 0, Col: 1}, g.Size())
	require.True(t, visible)

	mainX, _, _ := g.bandAnchors(g.Size())
	at := cellPoint(g, 0, g.Size().Width-(mainX-20)) // mirrored: inside the pinned band, on the right
	test.TapCanvas(w.Canvas(), at)
	require.Len(t, clicked, 1, "the tap reached the grid, not the hidden part of the editor")
	assert.Equal(t, CellID{Row: 0, Col: 0}, clicked[0])
}

// An old editor losing the focus does not end the edit that replaced it. A
// commit can stop short and leave the old editor focused (here its column was
// hidden mid-edit), so the next edit's focus move is the first to reach it.
func TestDataGrid_OldEditorFocusLossLeavesNewEdit(t *testing.T) {
	data := editRows(3)
	g := newTestGrid(t, data, []*ColumnDef{{ColID: "a", Field: "a", Width: 120}, {ColID: "b", Field: "b", Width: 120}})
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	c := w.Canvas()

	g.StartEditing(CellID{Row: 0, Col: 1})
	old := g.editWidget.(*cell.Entry)
	c.Focus(old)
	g.SetColumnVisible("b", false)
	g.StartEditing(CellID{Row: 0, Col: 0})

	require.False(t, g.editWidget == old, "a new editor replaced the old one")
	assert.True(t, g.IsEditing(), "the old editor losing the focus left the new edit open")
	assert.Equal(t, &CellID{Row: 0, Col: 0}, g.EditingCell())
}

// Right to left the editor sits over its mirrored cell, as drawn: the clip it
// draws in must not mirror it a second time.
func TestDataGrid_EditorOverItsCellRTL(t *testing.T) {
	data := editRows(3)
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "id", Width: 60, Pinned: PinLeft},
		{Field: "a", Width: 160},
		{Field: "b", Width: 160},
	})
	w := test.NewWindow(container.NewDirectionScope(g, bellui.DirectionRTL))
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	require.True(t, g.rtl)

	for _, col := range []int{0, 1} {
		g.StartEditing(CellID{Row: 0, Col: col})
		want, _, visible := g.editingCellRect(CellID{Row: 0, Col: col}, g.Size())
		require.True(t, visible)
		d := bellui.CurrentApp().Driver()
		got := d.AbsolutePositionForObject(g.editWidget).Subtract(d.AbsolutePositionForObject(g))
		assert.Equal(t, want, got, "column %d's editor is over its cell", col)
	}
}
