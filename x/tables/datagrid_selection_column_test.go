package tables

import (
	"fmt"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
	"github.com/belluikit/bellui/x/tables/pipeline"
)

// placedObject is an object the grid draws and where, relative to the grid.
type placedObject struct {
	obj      core.CanvasObject
	pos      geo.Position
	inHeader bool
	inFilter bool
}

// drawnObjects walks what the grid draws, through widget renderers,
// containers, clips and scrolls, and returns every visible object with its
// position relative to the grid.
func drawnObjects(g *DataGrid) []placedObject {
	var out []placedObject
	var walk func(o core.CanvasObject, at geo.Position, inHeader, inFilter bool)
	walk = func(o core.CanvasObject, at geo.Position, inHeader, inFilter bool) {
		if !o.Visible() {
			return
		}
		inHeader = inHeader || o == core.CanvasObject(g.headerWidget)
		inFilter = inFilter || (g.floatingFilterRow != nil && o == core.CanvasObject(g.floatingFilterRow))
		out = append(out, placedObject{obj: o, pos: at, inHeader: inHeader, inFilter: inFilter})
		var children []core.CanvasObject
		switch v := o.(type) {
		case *bellui.Container:
			children = v.Objects
		case *container.Clip:
			children = []core.CanvasObject{v.Content}
		case core.Widget:
			children = test.WidgetRenderer(v).Objects()
		}
		for _, c := range children {
			walk(c, at.Add(c.Position()), inHeader, inFilter)
		}
	}
	for _, c := range test.WidgetRenderer(g).Objects() {
		walk(c, c.Position(), false, false)
	}
	return out
}

// drawnCell returns where the body draws the cell at the accessible position
// (row, col): the checkbox column is accessible column 0 here.
func drawnCell(t *testing.T, g *DataGrid, row, col int) (x, w float32) {
	t.Helper()
	for _, p := range drawnObjects(g) {
		item, ok := p.obj.(core.AccessibleGridItem)
		if !ok || p.inHeader {
			continue
		}
		if r, c := item.AccessibleCellPosition(); r == row && c == col {
			return p.pos.X, p.obj.Size().Width
		}
	}
	t.Fatalf("no drawn cell at row %d, column %d", row, col)
	return 0, 0
}

// drawnHeader returns where the header draws the column titled title.
func drawnHeader(t *testing.T, g *DataGrid, title string) (x, w float32) {
	t.Helper()
	for _, p := range drawnObjects(g) {
		a, ok := p.obj.(core.Accessible)
		if ok && p.inHeader && a.AccessibleRole() == core.RoleHeading && a.AccessibleLabel() == title {
			return p.pos.X, p.obj.Size().Width
		}
	}
	t.Fatalf("no drawn header titled %q", title)
	return 0, 0
}

// selectAllBox returns the header's select-all box, or nil.
func selectAllBox(g *DataGrid) core.CanvasObject {
	for _, p := range drawnObjects(g) {
		a, ok := p.obj.(core.Accessible)
		if ok && p.inHeader && a.AccessibleRole() == core.RoleCheckbox {
			return p.obj
		}
	}
	return nil
}

// tapCentre taps o at its centre, where a check's box is.
func tapCentre(o core.CanvasObject) {
	test.TapAt(o.(event.Tappable), geo.NewPos(o.Size().Width/2, o.Size().Height/2))
}

func selectionColumnGrid(t *testing.T, width float32, rows int, cols ...string) *DataGrid {
	t.Helper()
	data := make([]map[string]any, rows)
	for i := range data {
		data[i] = map[string]any{}
		for _, c := range cols {
			data[i][c] = i
		}
	}
	defs := make([]*ColumnDef, len(cols))
	for i, c := range cols {
		defs[i] = &ColumnDef{Field: c, HeaderName: c, Width: 100}
	}
	g := newTestGrid(t, data, defs)
	g.Options.CheckboxSelection = true
	g.Options.FloatingFilters = true
	w := test.NewWindow(g)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(width, 300))
	return g
}

// checkColumnsLineUp checks that each main column's header and floating filter
// sit over its cells, and that a pointer anywhere on a cell resolves to its
// column.
func checkColumnsLineUp(t *testing.T, g *DataGrid, cols []string) {
	t.Helper()
	for i, title := range cols {
		cellX, cellW := drawnCell(t, g, 0, i+1)
		if hx, hw := drawnHeader(t, g, title); !near(hx, cellX) || !near(hw, cellW) {
			t.Errorf("column %s: header at x %v width %v, cells at x %v width %v", title, hx, hw, cellX, cellW)
		}
		filtered := false
		for _, p := range drawnObjects(g) {
			if _, ok := p.obj.(*widget.Entry); ok && p.inFilter && near(p.pos.X, cellX) {
				filtered = true
			}
		}
		if !filtered {
			t.Errorf("column %s: no floating filter starts where its cells do, at x %v", title, cellX)
		}
		if right := cellX + cellW; right > g.bodyScrollLeft() {
			if got := g.headerBorderCol(right); got != i {
				t.Errorf("column %s: the header border at its cells' right edge x %v resizes column %d", title, right, got)
			}
		}
		for _, x := range []float32{cellX + 1, cellX + cellW/2, cellX + cellW - 1} {
			if x < g.bodyScrollLeft() {
				continue // scrolled out of view
			}
			if got := g.columnAtX(x); got != i {
				t.Errorf("column %s: a pointer at x %v (cell %v-%v) resolves to column %d", title, x, cellX, cellX+cellW, got)
			}
		}
	}
}

// near reports whether a and b are the same position, give or take float
// rounding: the bands reach a position by different sums.
func near(a, b float32) bool { return a-b < 0.01 && b-a < 0.01 }

// bodyScrollLeft is the grid x where the scrolling band starts to show.
func (g *DataGrid) bodyScrollLeft() float32 { return g.scrollContent.Scroll.Position().X }

// TestDataGrid_SelectionColumnLinesUp pins #1498: the checkbox column shifted
// every main cell 32 px right of its header, floating filter and hit-test, and
// the content width left it out.
func TestDataGrid_SelectionColumnLinesUp(t *testing.T) {
	cols := []string{"alpha", "beta"}
	g := selectionColumnGrid(t, 400, 3, cols...)

	checkColumnsLineUp(t, g, cols)

	// A pointer on a row's checkbox is in no data column.
	boxX, boxW := drawnCell(t, g, 0, 0)
	if got := g.columnAtX(boxX + boxW/2); got != -1 {
		t.Errorf("a pointer on a row's checkbox resolves to column %d, want none", got)
	}

	// The content ends where the last column is drawn.
	lastX, lastW := drawnCell(t, g, 0, len(cols))
	if got, want := g.viewport.ContentSize().Width, lastX+lastW-g.bodyScrollLeft(); got != want {
		t.Errorf("content width %v, want %v (where the last column ends)", got, want)
	}
}

// TestDataGrid_SelectionColumnScrollsLastColumnIntoView checks that a grid
// scrolled sideways to its end shows the last column whole, and that the
// columns still line up.
func TestDataGrid_SelectionColumnScrollsLastColumnIntoView(t *testing.T) {
	cols := []string{"alpha", "beta", "gamma"}
	g := selectionColumnGrid(t, 250, 3, cols...)

	scroll := g.scrollContent.Scroll
	maxX := g.cellsWidget.MinSize().Width - scroll.Size().Width
	if maxX <= 0 {
		t.Fatalf("the test needs a grid that scrolls sideways; content %v, scroll %v", g.cellsWidget.MinSize(), scroll.Size())
	}
	g.scrollContent.ScrollTo(geo.NewPos(maxX, 0))

	lastX, lastW := drawnCell(t, g, 0, len(cols))
	if right, visible := lastX+lastW, scroll.Position().X+scroll.Size().Width; right > visible {
		t.Errorf("scrolled to the end, the last column ends at x %v, past the visible edge at %v", right, visible)
	}
	checkColumnsLineUp(t, g, cols)
}

// TestDataGrid_SelectionColumnPress checks what a press in the checkbox column
// does: nothing, since the box's own tap toggles the row (#1504), while a press
// on the data cell beside it is that cell's.
func TestDataGrid_SelectionColumnPress(t *testing.T) {
	g := selectionColumnGrid(t, 400, 3, "alpha", "beta")
	g.SelectionMode = SelectionMultiRow
	g.API().SelectRow(0)

	boxX, boxW := drawnCell(t, g, 1, 0)
	_, rowH := g.viewport.RowY(1)
	ry, _ := g.viewport.RowY(1)
	at := geo.NewPos(boxX+boxW/2, g.bodyTop()+ry+rowH/2)
	g.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: at}, Button: desktop.MouseButtonPrimary})
	g.MouseUp(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: at}, Button: desktop.MouseButtonPrimary})
	if got := g.API().GetSelectedRows(); len(got) != 1 || got[0] != 0 {
		t.Errorf("a press on row 1's checkbox changed the selection to %v, want it left at [0]", got)
	}

	// A press on a data cell beside it is that cell's.
	betaX, betaW := drawnCell(t, g, 2, 2)
	ry, _ = g.viewport.RowY(2)
	at = geo.NewPos(betaX+betaW/2, g.bodyTop()+ry+rowH/2)
	g.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: at}, Button: desktop.MouseButtonPrimary})
	g.MouseUp(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: at}, Button: desktop.MouseButtonPrimary})
	if g.currentFocus != (CellID{Row: 2, Col: 1}) {
		t.Errorf("a press on row 2's beta cell put the cursor on %+v, want {Row:2 Col:1}", g.currentFocus)
	}
}

// TestDataGrid_SelectAllBox checks the select-all box in the checkbox column's
// header: it selects every row, then clears them all (#1498).
func TestDataGrid_SelectAllBox(t *testing.T) {
	g := selectionColumnGrid(t, 400, 5, "alpha", "beta")
	g.SelectionMode = SelectionMultiRow
	g.Refresh()
	fired := 0
	g.OnSelectionChanged = func([]int) { fired++ }

	box := selectAllBox(g)
	if box == nil {
		t.Fatal("no select-all box in the header")
	}
	// It sits in the checkbox column, above the rows' boxes.
	rowBoxX, rowBoxW := drawnCell(t, g, 0, 0)
	for _, p := range drawnObjects(g) {
		if p.obj == box && (p.pos.X < rowBoxX || p.pos.X+box.Size().Width > rowBoxX+rowBoxW) {
			t.Errorf("the select-all box is at x %v-%v, outside the checkbox column %v-%v",
				p.pos.X, p.pos.X+box.Size().Width, rowBoxX, rowBoxX+rowBoxW)
		}
	}

	tapCentre(box)
	if got := g.SelectedRowCount(); got != 5 {
		t.Errorf("after the select-all box was tapped, %d rows are selected, want 5", got)
	}
	if s := selectAllBox(g).(core.AccessibleStateful).AccessibleState(); s&core.StateChecked == 0 {
		t.Errorf("with every row selected the box's state is %v, want checked", s)
	}

	tapCentre(selectAllBox(g))
	if got := g.SelectedRowCount(); got != 0 {
		t.Errorf("after the select-all box was tapped again, %d rows are selected, want 0", got)
	}
	if fired != 2 {
		t.Errorf("OnSelectionChanged fired %d times for two taps, want 2", fired)
	}

	// Some rows selected: the box shows a partial state, and a tap selects all.
	g.API().SelectRow(2)
	if s := selectAllBox(g).(core.AccessibleStateful).AccessibleState(); s&core.StatePartial == 0 || s&core.StateChecked != 0 {
		t.Errorf("with one row selected the box's state is %v, want partial and not checked", s)
	}
	tapCentre(selectAllBox(g))
	if got := g.SelectedRowCount(); got != 5 {
		t.Errorf("after a tap on the partial box, %d rows are selected, want 5", got)
	}
}

// TestDataGrid_SelectAllBoxOnlyForMultiRow checks that the header holds no
// select-all box when the grid cannot select several rows.
func TestDataGrid_SelectAllBoxOnlyForMultiRow(t *testing.T) {
	g := selectionColumnGrid(t, 400, 5, "alpha", "beta")
	for _, mode := range []SelectionMode{SelectionNone, SelectionSingleRow, SelectionSingleCell, SelectionMultiCell} {
		g.SelectionMode = mode
		g.Refresh()
		if selectAllBox(g) != nil {
			t.Errorf("selection mode %d shows a select-all box", mode)
		}
	}
}

// TestDataGrid_SelectionColumnWithPinnedBands checks the column between pinned
// bands: the checkbox column starts the main band, after pinned-left, and
// every column's header, cells, hit-test and editor and context-menu anchors
// agree.
func TestDataGrid_SelectionColumnWithPinnedBands(t *testing.T) {
	data := []map[string]any{{"p": 1, "alpha": 2, "beta": 3, "q": 4}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "p", HeaderName: "p", Width: 80, Pinned: PinLeft},
		{Field: "alpha", HeaderName: "alpha", Width: 100},
		{Field: "beta", HeaderName: "beta", Width: 100},
		{Field: "q", HeaderName: "q", Width: 80, Pinned: PinRight},
	})
	g.Options.CheckboxSelection = true
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(700, 300))

	for col, title := range []string{"p", "alpha", "beta", "q"} {
		a11yCol := col
		if col >= 1 {
			a11yCol++ // the checkbox column comes right after pinned-left
		}
		cellX, cellW := drawnCell(t, g, 0, a11yCol)
		if hx, _ := drawnHeader(t, g, title); !near(hx, cellX) {
			t.Errorf("column %s: header at x %v, cells at x %v", title, hx, cellX)
		}
		if got := g.columnAtX(cellX + cellW/2); got != col {
			t.Errorf("column %s: a pointer on its cell resolves to column %d", title, got)
		}
		if pos, _, ok := g.editingCellRect(CellID{Row: 0, Col: col}, g.Size()); !ok || !near(pos.X, cellX) {
			t.Errorf("column %s: the editor would open at x %v (shown %v), the cell is at %v", title, pos.X, ok, cellX)
		}
		if pos, _ := g.cellWidgetRect(CellID{Row: 0, Col: col}); !near(pos.X, cellX) {
			t.Errorf("column %s: the cell's anchor is at x %v, the cell at %v", title, pos.X, cellX)
		}
	}
	// pinned-right starts after the last main column, not over it.
	betaX, betaW := drawnCell(t, g, 0, 3)
	if qX, _ := drawnCell(t, g, 0, 4); qX < betaX+betaW {
		t.Errorf("pinned-right starts at x %v, over the last main column, which ends at %v", qX, betaX+betaW)
	}
	// pinned-left ends before the checkbox column, which starts the main band.
	boxX, _ := drawnCell(t, g, 0, 1)
	if mainX, _, _ := g.hitRegions(); !near(boxX, mainX) {
		t.Errorf("the checkbox column is at x %v, want the main band's start %v", boxX, mainX)
	}
}

// TestDataGrid_SelectionColumnGroupAggregates checks that a group row's
// aggregate sits in its column with the checkbox column before it.
func TestDataGrid_SelectionColumnGroupAggregates(t *testing.T) {
	data := []map[string]any{
		{"dept": "A", "salary": 100},
		{"dept": "A", "salary": 300},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "dept", HeaderName: "dept", Width: 100},
		{
			Field: "salary", HeaderName: "salary", Width: 100, AggFunc: bellui.Ptr(pipeline.AggSum),
			ValueFormatter: func(v any) string { return "$" + fmt.Sprint(v) },
		},
	})
	g.Options.CheckboxSelection = true
	g.API().SetRowGroupColumn("dept")
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(500, 300))

	hx, hw := drawnHeader(t, g, "salary")
	found := false
	for _, p := range drawnObjects(g) {
		if txt, ok := p.obj.(*canvas.Text); ok && !p.inHeader && txt.Text == "$400" {
			found = true
			if left, right := p.pos.X, p.pos.X+txt.Size().Width; left < hx-0.01 || right > hx+hw+0.01 {
				t.Errorf("the group's salary sum is drawn at x %v-%v, outside its column %v-%v", left, right, hx, hx+hw)
			}
		}
	}
	if !found {
		t.Fatal("no group aggregate $400 drawn")
	}
}

// TestDataGrid_SelectionColumnScrollToFirstColumn checks that bringing the
// first main column into view brings the checkbox column before it too.
func TestDataGrid_SelectionColumnScrollToFirstColumn(t *testing.T) {
	g := selectionColumnGrid(t, 250, 3, "alpha", "beta", "gamma")
	g.scrollContent.ScrollTo(geo.NewPos(g.cellsWidget.MinSize().Width-g.scrollContent.Scroll.Size().Width, 0))
	if g.offset.X <= 0 {
		t.Fatalf("the test needs a grid scrolled sideways, offset %v", g.offset)
	}
	g.scrollToCell(CellID{Row: 0, Col: 0})
	if g.offset.X != 0 {
		t.Errorf("after scrolling to the first column the offset is %v, want 0 (the checkbox column shown)", g.offset.X)
	}
}
