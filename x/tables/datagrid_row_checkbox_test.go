package tables

import (
	"reflect"
	"sort"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// rowBox returns row's select checkbox as the grid draws it, and its centre in
// grid coordinates. It finds the box by its row alone, so it holds beside
// pinned bands and right to left.
func rowBox(t *testing.T, g *DataGrid, row int) (*widget.Check, geo.Position) {
	t.Helper()
	ry, rh := g.viewport.RowY(row)
	centreY := g.bodyTop() + ry - g.offset.Y + rh/2
	for _, p := range drawnObjects(g) {
		if c, ok := p.obj.(*widget.Check); ok && !p.inHeader && p.pos.Y <= centreY && centreY < p.pos.Y+c.Size().Height {
			return c, geo.NewPos(p.pos.X+c.Size().Width/2, centreY)
		}
	}
	t.Fatalf("no checkbox drawn for row %d", row)
	return nil, geo.Position{}
}

// clickBox clicks row's checkbox with the mouse: a press and release on the
// grid, then the tap, which goes to the deepest tappable object under the
// pointer, as a driver sends them.
func clickBox(t *testing.T, g *DataGrid, row int) {
	t.Helper()
	_, at := rowBox(t, g, row)
	ev := &desktop.MouseEvent{PointEvent: event.PointEvent{Position: at}, Button: desktop.MouseButtonPrimary}
	g.MouseDown(ev)
	g.MouseUp(ev)
	touchBox(t, g, row)
}

// touchBox taps row's checkbox as a touch screen does: a tap with no press.
func touchBox(t *testing.T, g *DataGrid, row int) {
	t.Helper()
	_, at := rowBox(t, g, row)
	c := bellui.CurrentApp().Driver().CanvasForObject(g)
	test.TapCanvas(c, g.Position().Add(at))
}

// checkRows checks the selected source rows and that every row's checkbox
// shows its row's state.
func checkRows(t *testing.T, g *DataGrid, step string, want ...int) {
	t.Helper()
	got := g.API().GetSelectedRows()
	sort.Ints(got)
	if want == nil {
		want = []int{}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: selected rows %v, want %v", step, got, want)
	}
	for row := 0; row < g.RowCount(); row++ {
		if c, _ := rowBox(t, g, row); c.Checked != g.IsSelected(row, 0) {
			t.Errorf("%s: row %d's checkbox shows checked=%v, the row's selected=%v", step, row, c.Checked, g.IsSelected(row, 0))
		}
	}
}

// TestDataGrid_RowCheckboxTogglesMultiRow pins #1504: a click on a row's
// checkbox toggles that row and keeps the others, and the box shows the row's
// state. The box used to show unchecked right after a click selected the row,
// because it toggled itself after the grid's press had selected it.
func TestDataGrid_RowCheckboxTogglesMultiRow(t *testing.T) {
	g := selectionColumnGrid(t, 400, 4, "alpha", "beta")
	g.SelectionMode = SelectionMultiRow
	g.Refresh()

	clickBox(t, g, 1)
	checkRows(t, g, "click row 1", 1)
	clickBox(t, g, 3)
	checkRows(t, g, "click row 3", 1, 3)
	clickBox(t, g, 1)
	checkRows(t, g, "click row 1 again", 3)

	// After select-all, a click clears just that row.
	g.SelectAll()
	clickBox(t, g, 2)
	checkRows(t, g, "select all, then click row 2", 0, 1, 3)
}

// TestDataGrid_RowCheckboxTogglesSingleRow checks single-row mode: a click
// selects the row in place of the selected one, and a click on the selected
// row clears it.
func TestDataGrid_RowCheckboxTogglesSingleRow(t *testing.T) {
	g := selectionColumnGrid(t, 400, 4, "alpha", "beta")
	g.SelectionMode = SelectionSingleRow
	g.Refresh()
	fired := 0
	g.OnSelectionChanged = func([]int) { fired++ }

	clickBox(t, g, 1)
	checkRows(t, g, "click row 1", 1)
	clickBox(t, g, 2)
	checkRows(t, g, "click row 2", 2)
	clickBox(t, g, 2)
	checkRows(t, g, "click row 2 again")
	if fired != 3 {
		t.Errorf("OnSelectionChanged fired %d times for three clicks, want 3", fired)
	}
}

// TestDataGrid_RowCheckboxTouch checks a touch screen, which sends the tap and
// no press: the box alone toggles the row.
func TestDataGrid_RowCheckboxTouch(t *testing.T) {
	g := selectionColumnGrid(t, 400, 4, "alpha", "beta")
	g.SelectionMode = SelectionMultiRow
	g.Refresh()

	touchBox(t, g, 0)
	touchBox(t, g, 2)
	checkRows(t, g, "touch rows 0 and 2", 0, 2)
	touchBox(t, g, 0)
	checkRows(t, g, "touch row 0 again", 2)
}

// TestDataGrid_RowCheckboxFollowsTheAPI checks that a selection made through
// the API shows in the boxes and is not toggled back by them: the grid sets a
// box's state without reporting it as a click.
func TestDataGrid_RowCheckboxFollowsTheAPI(t *testing.T) {
	g := selectionColumnGrid(t, 400, 4, "alpha", "beta")
	g.SelectionMode = SelectionMultiRow
	g.Refresh()
	fired := 0
	g.OnSelectionChanged = func([]int) { fired++ }

	g.API().SelectRow(2)
	checkRows(t, g, "SelectRow(2)", 2)
	g.API().DeselectAll()
	checkRows(t, g, "DeselectAll")
	if fired != 0 {
		t.Errorf("OnSelectionChanged fired %d times for API calls that don't report, want 0", fired)
	}
}

// TestDataGrid_RowCheckboxAccessibleToggle checks the box's accessible toggle
// action, which a screen reader uses: it toggles the row like a click.
func TestDataGrid_RowCheckboxAccessibleToggle(t *testing.T) {
	g := selectionColumnGrid(t, 400, 4, "alpha", "beta")
	g.SelectionMode = SelectionMultiRow
	g.Refresh()

	box, _ := rowBox(t, g, 1)
	box.PerformAccessibleAction(core.ActionToggle)
	checkRows(t, g, "toggle row 1", 1)
	box, _ = rowBox(t, g, 1)
	box.PerformAccessibleAction(core.ActionToggle)
	checkRows(t, g, "toggle row 1 again")
}

// TestDataGrid_RowCheckboxCellMode checks a cell selection mode, where a
// row's box stands for the row's first cell: a click selects that cell, as a
// press on it does, and the box shows it.
func TestDataGrid_RowCheckboxCellMode(t *testing.T) {
	g := selectionColumnGrid(t, 400, 4, "alpha", "beta")
	g.SelectionMode = SelectionSingleCell
	g.Refresh()

	clickBox(t, g, 1)
	if !g.IsSelected(1, 0) {
		t.Error("a click on row 1's checkbox did not select its first cell")
	}
	if c, _ := rowBox(t, g, 1); !c.Checked {
		t.Error("row 1's checkbox shows unchecked with its first cell selected")
	}
}
