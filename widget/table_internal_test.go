package widget

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
)

func TestTable_Empty(t *testing.T) {
	table := &Table{}
	table.Resize(geo.NewSize(120, 120))

	table.CreateRenderer()
	cellRenderer := test.TempWidgetRenderer(t, table.content.Content.(*tableCells))
	cellRenderer.Refresh() // let's not crash :)
}

func TestTable_Cache(t *testing.T) {
	c := test.NewCanvas()
	table := NewTable(
		func() (int, int) { return 3, 5 },
		func() core.CanvasObject {
			return NewLabel("placeholder")
		},
		func(id TableCellID, c core.CanvasObject) {
			text := fmt.Sprintf("Cell %d, %d", id.Row, id.Col)
			c.(*Label).SetText(text)
		},
	)
	c.SetContent(table)
	c.SetPadded(false)
	c.Resize(geo.NewSize(120, 148))

	cellRenderer := test.TempWidgetRenderer(t, table.content.Content.(*tableCells))
	cellRenderer.Refresh()
	cellLabel := func(i int) *Label {
		return unwrapTableCell(cellRenderer.Objects()[i]).(*Label)
	}
	assert.Greater(t, len(cellRenderer.(*tableCellsRenderer).visible), 0)
	assert.Equal(t, "Cell 0, 0", cellLabel(0).Text)
	objRef := cellLabel(0)

	test.Scroll(c, geo.NewPos(10, 10), -150, -150)
	assert.Equal(t, float32(0), table.content.Offset.Y) // we didn't scroll as data shorter
	assert.Equal(t, float32(150), table.content.Offset.X)
	assert.Greater(t, len(cellRenderer.(*tableCellsRenderer).visible), 0)
	assert.Equal(t, "Cell 0, 1", cellLabel(0).Text)
	assert.NotEqual(t, objRef, cellLabel(0)) // we want to re-use visible cells without rewriting them
}

func TestTable_ChangeTheme(t *testing.T) {
	test.NewTempApp(t)

	table := NewTable(
		func() (int, int) { return 3, 5 },
		func() core.CanvasObject {
			return NewLabel("placeholder")
		},
		func(id TableCellID, c core.CanvasObject) {
			text := fmt.Sprintf("Cell %d, %d", id.Row, id.Col)
			c.(*Label).SetText(text)
		},
	)
	table.CreateRenderer()

	table.Resize(geo.NewSize(50, 30))
	content := test.TempWidgetRenderer(t, table.content.Content.(*tableCells)).(*tableCellsRenderer)
	w := test.NewWindow(table)
	w.SetPadded(false)
	defer w.Close()
	w.Resize(geo.NewSize(180, 180))
	test.AssertImageMatches(t, "table/theme_initial.png", w.Canvas().Capture())
	assert.Equal(t, "Cell 0, 0", unwrapTableCell(content.Objects()[0]).(*Label).Text)
	assert.Equal(t, NewLabel("placeholder").MinSize(), unwrapTableCell(content.Objects()[0]).(*Label).Size())

	test.WithTestTheme(t, func() {
		test.TempWidgetRenderer(t, table).Refresh()
		test.AssertImageMatches(t, "table/theme_changed.png", w.Canvas().Capture())
	})
	assert.Equal(t, NewLabel("placeholder").MinSize(), unwrapTableCell(content.Objects()[0]).(*Label).Size())
}

func TestTable_Filled(t *testing.T) {
	test.NewTempApp(t)
	test.ApplyTheme(t, test.Theme())

	table := NewTable(
		func() (int, int) { return 5, 5 },
		func() core.CanvasObject {
			r := canvas.NewRectangle(color.Black)
			r.SetMinSize(geo.NewSize(30, 20))
			r.Resize(geo.NewSize(30, 20))
			return r
		},
		func(TableCellID, core.CanvasObject) {},
	)

	w := test.NewWindow(table)
	defer w.Close()
	w.Resize(geo.NewSize(180, 180))
	w.Content().Refresh()
	test.AssertImageMatches(t, "table/filled.png", w.Canvas().Capture())
}

func TestTable_Focus(t *testing.T) {
	test.NewTempApp(t)

	table := NewTable(
		func() (int, int) { return 5, 5 },
		func() core.CanvasObject {
			r := canvas.NewRectangle(color.Black)
			r.SetMinSize(geo.NewSize(30, 20))
			r.Resize(geo.NewSize(30, 20))
			return r
		},
		func(TableCellID, core.CanvasObject) {},
	)

	window := test.NewWindow(table)
	defer window.Close()
	window.Resize(table.MinSize().Max(geo.NewSize(300, 200)))

	canvas := window.Canvas().(test.WindowlessCanvas)
	assert.Nil(t, canvas.Focused())

	canvas.FocusNext()
	assert.NotNil(t, canvas.Focused())
	assert.Equal(t, table, canvas.Focused())
	assert.Equal(t, TableCellID{0, 0}, table.currentFocus)

	table.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	assert.Equal(t, TableCellID{1, 0}, table.currentFocus)

	table.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, TableCellID{1, 1}, table.currentFocus)

	table.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Equal(t, TableCellID{1, 0}, table.currentFocus)

	table.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	assert.Equal(t, TableCellID{0, 0}, table.currentFocus)

	canvas.Focused().TypedKey(&event.KeyEvent{Name: event.KeySpace})
	assert.Equal(t, &TableCellID{0, 0}, table.selectedCell)

	table.Select(TableCellID{Row: 1, Col: 1})
	assert.Equal(t, &TableCellID{1, 1}, table.selectedCell)
	assert.Equal(t, TableCellID{1, 1}, table.currentFocus)
}

func TestTable_Headers(t *testing.T) {
	table := NewTableWithHeaders(
		func() (int, int) { return 5, 5 },
		func() core.CanvasObject {
			return NewLabel("text")
		},
		func(_ TableCellID, _ core.CanvasObject) {
		},
	)
	table.Resize(geo.NewSize(120, 120))

	cellRenderer := test.TempWidgetRenderer(t, table.content.Content.(*tableCells))
	assert.Equal(t, "text", unwrapTableCell(cellRenderer.(*tableCellsRenderer).Objects()[2]).(*Label).Text)
	assert.Equal(t, "text", unwrapTableCell(cellRenderer.(*tableCellsRenderer).Objects()[5]).(*Label).Text)
	assert.True(t, areaContainsLabel(table.top.Content.(*bellui.Container).Objects, "A"))
	assert.True(t, areaContainsLabel(table.top.Content.(*bellui.Container).Objects, "B"))
	assert.True(t, areaContainsLabel(table.left.Content.(*bellui.Container).Objects, "1"))
	assert.True(t, areaContainsLabel(table.left.Content.(*bellui.Container).Objects, "2"))
}

func TestTable_JustHeaders(t *testing.T) {
	test.NewTempApp(t)

	table := NewTableWithHeaders(
		func() (int, int) { return 0, 9 },
		func() core.CanvasObject {
			return NewLabel("text")
		},
		func(_ TableCellID, _ core.CanvasObject) {
		},
	)

	w := test.NewWindow(table)
	defer w.Close()
	w.Resize(geo.NewSize(120, 120))

	test.AssertRendersToMarkup(t, "table/just_headers.xml", w.Canvas())
}

func TestTable_Sticky(t *testing.T) {
	table := NewTableWithHeaders(
		func() (int, int) { return 25, 25 },
		func() core.CanvasObject {
			return NewLabel("text")
		},
		func(i TableCellID, o core.CanvasObject) {
			o.(*Label).SetText(fmt.Sprintf("text %d,%d", i.Row, i.Col))
		},
	)
	table.Resize(geo.NewSize(120, 120))

	cellRenderer := test.TempWidgetRenderer(t, table.content.Content.(*tableCells)).(*tableCellsRenderer)
	assert.True(t, areaContainsLabel(cellRenderer.Objects(), "text 0,0"))
	assert.True(t, areaContainsLabel(cellRenderer.Objects(), "text 1,0"))
	assert.True(t, areaContainsLabel(cellRenderer.Objects(), "text 2,1"))
	assert.True(t, areaContainsLabel(table.top.Content.(*bellui.Container).Objects, "A"))
	assert.True(t, areaContainsLabel(table.top.Content.(*bellui.Container).Objects, "B"))
	assert.True(t, areaContainsLabel(table.left.Content.(*bellui.Container).Objects, "1"))
	assert.True(t, areaContainsLabel(table.left.Content.(*bellui.Container).Objects, "2"))

	table.ScrollTo(TableCellID{Row: 7, Col: 2})
	assert.True(t, areaContainsLabel(cellRenderer.Objects(), "text 6,1"))
	assert.True(t, areaContainsLabel(cellRenderer.Objects(), "text 6,2"))
	assert.True(t, areaContainsLabel(cellRenderer.Objects(), "text 9,3"))
	assert.True(t, areaContainsLabel(table.top.Content.(*bellui.Container).Objects, "C"))
	assert.True(t, areaContainsLabel(table.top.Content.(*bellui.Container).Objects, "D"))
	assert.True(t, areaContainsLabel(table.left.Content.(*bellui.Container).Objects, "7"))
	assert.True(t, areaContainsLabel(table.left.Content.(*bellui.Container).Objects, "8"))

	table.StickyRowCount = 1
	table.StickyColumnCount = 1
	table.Refresh()
	assert.True(t, areaContainsLabel(cellRenderer.Objects(), "text 7,2"))
	assert.True(t, areaContainsLabel(cellRenderer.Objects(), "text 7,4"))
	assert.True(t, areaContainsLabel(cellRenderer.Objects(), "text 9,3"))
	// stuck cells
	assert.True(t, areaContainsLabel(table.top.Content.(*bellui.Container).Objects, "text 0,3"))
	assert.True(t, areaContainsLabel(table.left.Content.(*bellui.Container).Objects, "text 7,0"))
	assert.True(t, areaContainsLabel(table.top.Content.(*bellui.Container).Objects, "C"))
	assert.True(t, areaContainsLabel(table.left.Content.(*bellui.Container).Objects, "8"))
	assert.True(t, areaContainsLabel(table.corner.Content.(*bellui.Container).Objects, "A"))
	assert.True(t, areaContainsLabel(table.corner.Content.(*bellui.Container).Objects, "1"))
	assert.True(t, areaContainsLabel(table.corner.Content.(*bellui.Container).Objects, "text 0,0"))
}

func TestTable_MinSize(t *testing.T) {
	for name, tt := range map[string]struct {
		cellSize             geo.Size
		expectedMinSize      geo.Size
		headRow, headCol     bool
		stickRows, stickCols int
	}{
		"small": {
			geo.NewSize(1, 1),
			geo.NewSize(float32(32), float32(32)),
			false, false,
			0, 0,
		},
		"large": {
			geo.NewSize(100, 100),
			geo.NewSize(100, 100),
			false, false,
			0, 0,
		},
		"sticky": {
			geo.NewSize(40, 40),
			geo.NewSize(84, 84),
			false, false,
			1, 1,
		},
		"headerrow": {
			geo.NewSize(1, 1),
			geo.NewSize(float32(32), float32(46)),
			true, false,
			0, 0,
		},
		"headercol": {
			geo.NewSize(1, 1),
			geo.NewSize(float32(46), float32(32)),
			false, true,
			0, 0,
		},
		"headers": {
			geo.NewSize(1, 1),
			geo.NewSize(float32(46), float32(46)),
			true, true,
			0, 0,
		},
		"stickyandheaders": {
			geo.NewSize(40, 40),
			geo.NewSize(98, 98),
			true, true,
			1, 1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			table := NewTable(
				func() (int, int) { return 5, 5 },
				func() core.CanvasObject {
					r := canvas.NewRectangle(color.Black)
					r.SetMinSize(tt.cellSize)
					r.Resize(tt.cellSize)
					return r
				},
				func(TableCellID, core.CanvasObject) {},
			)
			table.ShowHeaderRow = tt.headRow
			table.ShowHeaderColumn = tt.headCol
			table.CreateHeader = func() core.CanvasObject {
				r := canvas.NewRectangle(color.White)
				r.SetMinSize(geo.NewSize(10, 10))
				r.Resize(geo.NewSize(10, 10))
				return r
			}
			table.StickyRowCount = tt.stickRows
			table.StickyColumnCount = tt.stickCols

			assert.Equal(t, tt.expectedMinSize, table.MinSize())
		})
	}
}

func TestTable_Resize(t *testing.T) {
	table := NewTable(
		func() (int, int) { return 2, 2 },
		func() core.CanvasObject {
			return NewLabel("abc")
		},
		func(TableCellID, core.CanvasObject) {},
	)

	w := test.NewTempWindow(t, table)
	w.Resize(geo.NewSize(100, 100))
	test.AssertImageMatches(t, "table/resize.png", w.Canvas().Capture())
}

func TestTable_Unselect(t *testing.T) {
	test.NewTempApp(t)

	table := NewTable(
		func() (int, int) { return 3, 5 },
		func() core.CanvasObject {
			return NewLabel("placeholder")
		},
		func(id TableCellID, c core.CanvasObject) {
			text := fmt.Sprintf("Cell %d, %d", id.Row, id.Col)
			c.(*Label).SetText(text)
		},
	)
	unselectedRow, unselectedColumn := -1, -1
	table.OnUnselected = func(id TableCellID) {
		unselectedRow = id.Row
		unselectedColumn = id.Col
	}
	table.selectedCell = &TableCellID{1, 1}
	w := test.NewWindow(table)
	w.SetPadded(false)
	defer w.Close()
	w.Resize(geo.NewSize(180, 180))

	table.Unselect(*table.selectedCell)
	assert.Equal(t, 1, unselectedRow)
	assert.Equal(t, 1, unselectedColumn)
	test.AssertImageMatches(t, "table/theme_initial.png", w.Canvas().Capture())

	unselectedRow, unselectedColumn = -1, -1
	table.Select(TableCellID{2, 2})
	table.Unselect(TableCellID{1, 1})
	assert.Equal(t, -1, unselectedRow)
	assert.Equal(t, -1, unselectedColumn)

	table.UnselectAll()
	assert.Equal(t, 2, unselectedRow)
	assert.Equal(t, 2, unselectedColumn)
}

func TestTable_Refresh(t *testing.T) {
	displayText := "placeholder"
	table := NewTable(
		func() (int, int) { return 5, 5 },
		func() core.CanvasObject {
			return NewLabel("template")
		},
		func(_ TableCellID, obj core.CanvasObject) {
			obj.(*Label).SetText(displayText)
		},
	)
	table.Resize(geo.NewSize(120, 300))

	cellRenderer := test.TempWidgetRenderer(t, table.content.Content.(*tableCells))
	assert.Equal(t, "placeholder", unwrapTableCell(cellRenderer.(*tableCellsRenderer).Objects()[7]).(*Label).Text)

	displayText = "replaced"
	table.Refresh()
	assert.Equal(t, "replaced", unwrapTableCell(cellRenderer.(*tableCellsRenderer).Objects()[7]).(*Label).Text)

	displayText = "only"
	table.RefreshItem(TableCellID{2, 1})
	assert.Equal(t, "only", unwrapTableCell(cellRenderer.(*tableCellsRenderer).Objects()[5]).(*Label).Text)
	assert.Equal(t, "replaced", unwrapTableCell(cellRenderer.(*tableCellsRenderer).Objects()[6]).(*Label).Text)
}

func TestTable_ScrollTo(t *testing.T) {
	test.NewTempApp(t)

	// for this test the separator thickness is 0
	test.ApplyTheme(t, &paddingZeroTheme{test.Theme()})

	// we will test a 20 row x 5 column table where each cell is 50x50
	const (
		maxRows int     = 20
		maxCols int     = 5
		width   float32 = 50
		height  float32 = 50
	)

	templ := canvas.NewRectangle(color.Gray16{})
	templ.SetMinSize(geo.Size{Width: width, Height: height})

	table := NewTable(
		func() (int, int) { return maxRows, maxCols },
		func() core.CanvasObject { return templ },
		func(TableCellID, core.CanvasObject) {},
	)

	w := test.NewWindow(table)
	defer w.Close()

	// these position expectations have a built-in assumption that the window
	// is smaller than or equal to the size of a single table cell.
	expectedOffset := func(row, col float32) geo.Position {
		return geo.Position{
			X: col * width,
			Y: row * height,
		}
	}

	tt := []struct {
		name string
		in   TableCellID
		want geo.Position
	}{
		{
			"row 0, col 0",
			TableCellID{},
			expectedOffset(0, 0),
		},
		{
			"row 0, col 1",
			TableCellID{Row: 0, Col: 1},
			expectedOffset(0, 1),
		},
		{
			"row 1, col 0",
			TableCellID{Row: 1, Col: 0},
			expectedOffset(1, 0),
		},
		{
			"row 1, col 1",
			TableCellID{Row: 1, Col: 1},
			expectedOffset(1, 1),
		},
		{
			"second last element",
			TableCellID{Row: maxRows - 2, Col: maxCols - 2},
			expectedOffset(float32(maxRows)-2, float32(maxCols)-2),
		},
		{
			"last element",
			TableCellID{Row: maxRows - 1, Col: maxCols - 1},
			expectedOffset(float32(maxRows)-1, float32(maxCols)-1),
		},
		{
			"row 0, col 0 (scrolling backwards)",
			TableCellID{},
			expectedOffset(0, 0),
		},
		{
			"row 99, col 99 (scrolling beyond the end)",
			TableCellID{Row: 99, Col: 99},
			expectedOffset(float32(maxRows)-1, float32(maxCols)-1),
		},
		{
			"row -1, col -1 (scrolling before the start)",
			TableCellID{Row: -1, Col: -1},
			expectedOffset(0, 0),
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			table.ScrollTo(tc.in)
			assert.Equal(t, tc.want, table.offset)
			assert.Equal(t, tc.want, table.content.Offset)
		})
	}
}

func TestTable_ScrollToBottom(t *testing.T) {
	test.NewTempApp(t)
	test.ApplyTheme(t, test.NewTheme())

	const (
		maxRows int     = 20
		maxCols int     = 5
		width   float32 = 50
		height  float32 = 50
	)

	templ := canvas.NewRectangle(color.Gray16{})
	templ.SetMinSize(geo.NewSize(width, height))

	table := NewTable(
		func() (int, int) { return maxRows, maxCols },
		func() core.CanvasObject { return templ },
		func(TableCellID, core.CanvasObject) {},
	)

	w := test.NewWindow(table)
	defer w.Close()

	w.Resize(geo.NewSize(200, 200))

	table.ScrollTo(TableCellID{19, 2})
	want := table.offset

	table.ScrollTo(TableCellID{2, 2})
	table.ScrollToBottom()

	assert.Equal(t, want, table.offset)
	assert.Equal(t, want, table.content.Offset)
}

func TestTable_ScrollToLeading(t *testing.T) {
	test.NewTempApp(t)

	table := NewTable(
		func() (int, int) { return 3, 5 },
		func() core.CanvasObject {
			return NewLabel("placeholder")
		},
		func(id TableCellID, c core.CanvasObject) {
			text := fmt.Sprintf("Cell %d, %d", id.Row, id.Col)
			c.(*Label).SetText(text)
		},
	)

	w := test.NewWindow(table)
	defer w.Close()

	table.ScrollTo(TableCellID{Row: 8, Col: 4})
	prev := table.offset
	table.ScrollToLeading()

	want := geo.Position{X: 0, Y: prev.Y}
	assert.Equal(t, want, table.offset)
	assert.Equal(t, want, table.content.Offset)
}

func TestTable_ScrollToOffset(t *testing.T) {
	test.NewTempApp(t)

	const (
		maxRows int     = 6
		maxCols int     = 10
		width   float32 = 50
		height  float32 = 50
	)

	templ := canvas.NewRectangle(color.Gray16{})
	templ.SetMinSize(geo.Size{Width: width, Height: height})

	table := NewTable(
		func() (int, int) { return maxRows, maxCols },
		func() core.CanvasObject { return templ },
		func(TableCellID, core.CanvasObject) {},
	)

	w := test.NewWindow(table)
	defer w.Close()

	want := geo.NewPos(48, 25)
	table.ScrollToOffset(want)
	assert.Equal(t, want, table.offset)
	assert.Equal(t, want, table.content.Offset)
}

func TestTable_ScrollToTop(t *testing.T) {
	test.NewTempApp(t)

	const (
		maxRows int     = 6
		maxCols int     = 10
		width   float32 = 50
		height  float32 = 50
	)

	templ := canvas.NewRectangle(color.Gray16{})
	templ.SetMinSize(geo.Size{Width: width, Height: height})

	table := NewTable(
		func() (int, int) { return maxRows, maxCols },
		func() core.CanvasObject { return templ },
		func(TableCellID, core.CanvasObject) {},
	)

	w := test.NewWindow(table)
	defer w.Close()

	table.ScrollTo(TableCellID{12, 3})
	prev := table.offset

	table.ScrollToTop()

	want := geo.Position{X: prev.X, Y: 0}
	assert.Equal(t, want, table.offset)
	assert.Equal(t, want, table.content.Offset)
}

func TestTable_ScrollToTrailing(t *testing.T) {
	test.NewTempApp(t)

	table := NewTable(
		func() (int, int) { return 24, 24 },
		func() core.CanvasObject {
			return NewLabel("placeholder")
		},
		func(id TableCellID, c core.CanvasObject) {
			text := fmt.Sprintf("Cell %d, %d", id.Row, id.Col)
			c.(*Label).SetText(text)
		},
	)

	w := test.NewWindow(table)
	defer w.Close()

	w.Resize(geo.NewSize(200, 200))

	table.ScrollTo(TableCellID{Row: 7, Col: 23})
	want := table.offset

	table.ScrollTo(TableCellID{Row: 7})
	table.ScrollToTrailing()

	assert.Equal(t, want, table.offset)
	assert.Equal(t, want, table.content.Offset)
}

func TestTable_Selection(t *testing.T) {
	test.NewTempApp(t)

	table := NewTable(
		func() (int, int) { return 5, 5 },
		func() core.CanvasObject {
			return NewLabel("placeholder")
		},
		func(id TableCellID, c core.CanvasObject) {
			text := fmt.Sprintf("Cell %d, %d", id.Row, id.Col)
			c.(*Label).SetText(text)
		},
	)
	assert.Nil(t, table.selectedCell)

	w := test.NewWindow(table)
	defer w.Close()
	w.Resize(geo.NewSize(180, 180))

	selectedCol, selectedRow := 0, 0
	table.OnSelected = func(id TableCellID) {
		selectedCol = id.Col
		selectedRow = id.Row
	}
	test.TapCanvas(w.Canvas(), geo.NewPos(35, 58))
	w.Canvas().Unfocus() // don't include table focus in test
	assert.Equal(t, 0, table.selectedCell.Col)
	assert.Equal(t, 1, table.selectedCell.Row)
	assert.Equal(t, 0, selectedCol)
	assert.Equal(t, 1, selectedRow)

	test.AssertRendersToMarkup(t, "table/selected.xml", w.Canvas())

	// check out of bounds col
	w.Resize(geo.NewSize(680, 180))
	test.TapCanvas(w.Canvas(), geo.NewPos(575, 58))
	assert.Equal(t, 0, selectedCol)

	// check out of bounds row
	w.Resize(geo.NewSize(180, 480))
	test.TapCanvas(w.Canvas(), geo.NewPos(35, 428))
	assert.Equal(t, 1, selectedRow)
}

func TestTable_Selection_OnHeader(t *testing.T) {
	test.NewTempApp(t)

	table := NewTableWithHeaders(
		func() (int, int) { return 5, 5 },
		func() core.CanvasObject {
			return NewLabel("placeholder")
		},
		func(id TableCellID, c core.CanvasObject) {
			text := fmt.Sprintf("Cell %d, %d", id.Row, id.Col)
			c.(*Label).SetText(text)
		},
	)
	assert.Nil(t, table.selectedCell)

	w := test.NewWindow(table)
	defer w.Close()
	w.Resize(geo.NewSize(180, 180))

	selected := false
	table.OnSelected = func(TableCellID) {
		selected = true
	}
	test.TapCanvas(w.Canvas(), geo.NewPos(35, 5))
	assert.False(t, selected)

	test.TapCanvas(w.Canvas(), geo.NewPos(5, 58))
	assert.False(t, selected)
}

func TestTable_Select(t *testing.T) {
	test.NewTempApp(t)

	table := NewTable(
		func() (int, int) { return 5, 5 },
		func() core.CanvasObject {
			return NewLabel("placeholder")
		},
		func(id TableCellID, c core.CanvasObject) {
			text := fmt.Sprintf("Cell %d, %d", id.Row, id.Col)
			c.(*Label).SetText(text)
		},
	)

	w := test.NewWindow(table)
	defer w.Close()
	w.Resize(geo.NewSize(180, 180))

	selectedCol, selectedRow := 0, 0
	table.OnSelected = func(id TableCellID) {
		selectedCol = id.Col
		selectedRow = id.Row
	}
	table.Select(TableCellID{1, 0})
	assert.Equal(t, 0, table.selectedCell.Col)
	assert.Equal(t, 1, table.selectedCell.Row)
	assert.Equal(t, 0, selectedCol)
	assert.Equal(t, 1, selectedRow)
	test.AssertRendersToMarkup(t, "table/selected.xml", w.Canvas())

	table.Select(TableCellID{4, 3})
	assert.Equal(t, 3, table.selectedCell.Col)
	assert.Equal(t, 4, table.selectedCell.Row)
	assert.Equal(t, 3, selectedCol)
	assert.Equal(t, 4, selectedRow)
	test.AssertRendersToMarkup(t, "table/selected_scrolled.xml", w.Canvas())

	table.Select(TableCellID{1, -1})
	assert.Equal(t, 3, table.selectedCell.Col)
	assert.Equal(t, 4, table.selectedCell.Row)
	assert.Equal(t, 3, selectedCol)
	assert.Equal(t, 4, selectedRow)

	table.Select(TableCellID{-1, -1})
	assert.Equal(t, 3, table.selectedCell.Col)
	assert.Equal(t, 4, table.selectedCell.Row)
	assert.Equal(t, 3, selectedCol)
	assert.Equal(t, 4, selectedRow)
}

func TestTable_SetColumnWidth(t *testing.T) {
	test.NewTempApp(t)
	test.ApplyTheme(t, test.Theme())

	table := NewTable(
		func() (int, int) { return 5, 5 },
		func() core.CanvasObject {
			return NewLabel("placeholder")
		},
		func(id TableCellID, obj core.CanvasObject) {
			if id.Col == 0 {
				obj.(*Label).Text = "p"
			} else {
				obj.(*Label).Text = "placeholder"
			}
			obj.Refresh()
		},
	)
	table.SetColumnWidth(0, 32)
	table.Resize(geo.NewSize(120, 120))
	table.Select(TableCellID{1, 0})

	cellRenderer := test.TempWidgetRenderer(t, table.content.Content.(*tableCells))
	cellRenderer.Refresh()
	assert.Greater(t, len(cellRenderer.(*tableCellsRenderer).visible), 0)
	assert.Equal(t, float32(32), cellRenderer.(*tableCellsRenderer).Objects()[0].Size().Width)
	cell1Offset := theme.Padding()
	assert.Equal(t, float32(32)+cell1Offset, cellRenderer.(*tableCellsRenderer).Objects()[1].Position().X)

	table.SetColumnWidth(0, 24)
	assert.Equal(t, float32(24), cellRenderer.(*tableCellsRenderer).Objects()[0].Size().Width)
	assert.Equal(t, float32(24)+cell1Offset, cellRenderer.(*tableCellsRenderer).Objects()[1].Position().X)

	w := test.NewWindow(table)
	defer w.Close()
	w.Resize(geo.NewSize(120+(2*theme.Padding()), 120+(2*theme.Padding())))
	test.AssertImageMatches(t, "table/col_size.png", w.Canvas().Capture())
}

func TestTable_SetColumnWidth_Dragged(t *testing.T) {
	test.NewApp()

	table := NewTableWithHeaders(
		func() (int, int) { return 5, 5 },
		func() core.CanvasObject {
			return NewLabel("")
		},
		func(id TableCellID, obj core.CanvasObject) {
		},
	)
	table.ShowHeaderColumn = false
	table.StickyColumnCount = 0
	table.Refresh()

	c := test.NewCanvas()
	c.SetPadded(false)
	c.SetContent(table)
	c.Resize(geo.NewSize(120, 120))

	dragPos := geo.NewPos(table.cellSize.Width*2+theme.Padding()+2, 2) // gap between col 1 and 2
	table.HoverMoved(&event.HoverEvent{PointEvent: event.PointEvent{Position: dragPos}})
	table.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: dragPos}})
	test.Drag(c, dragPos.AddXY(5, 0), 5, 0) // expanded column 5.0

	assert.Equal(t, table.cellSize.Width+5, table.columnWidths[1])

	dragPos = dragPos.AddXY(5, 0)
	table.HoverMoved(&event.HoverEvent{PointEvent: event.PointEvent{Position: dragPos}})
	table.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: dragPos}})
	table.Dragged(&event.DragEvent{ // reduce less than min width
		PointEvent: event.PointEvent{Position: dragPos.SubtractXY(25, 0)},
		Dragged:    geo.Delta{DX: -25, DY: 0},
	})

	assert.Equal(t, table.cellSize.Width, table.columnWidths[1])

	dragPos = dragPos.SubtractXY(25, 0)
	test.Drag(c, dragPos.AddXY(25, 0), 25, 0) // return to before-min-drag

	assert.Equal(t, table.cellSize.Width+5, table.columnWidths[1])
}

func TestTable_SetRowHeight(t *testing.T) {
	test.NewTempApp(t)
	test.ApplyTheme(t, test.Theme())

	table := NewTable(
		func() (int, int) { return 5, 5 },
		func() core.CanvasObject {
			return NewLabel("place\nholder")
		},
		func(id TableCellID, obj core.CanvasObject) {
			if id.Row == 0 {
				obj.(*Label).Text = "p"
			} else {
				obj.(*Label).Text = "place\nholder"
			}
			obj.Refresh()
		},
	)
	table.SetRowHeight(0, 48)
	table.Resize(geo.NewSize(120, 120))
	table.Select(TableCellID{0, 1})

	cellRenderer := test.TempWidgetRenderer(t, table.content.Content.(*tableCells))
	cellRenderer.Refresh()
	assert.Greater(t, len(cellRenderer.(*tableCellsRenderer).visible), 0)
	assert.Equal(t, float32(48), cellRenderer.(*tableCellsRenderer).Objects()[0].Size().Height)
	cell1Offset := theme.Padding()
	assert.Equal(t, float32(48)+cell1Offset, cellRenderer.(*tableCellsRenderer).Objects()[3].Position().Y)

	table.SetRowHeight(0, 32)
	assert.Equal(t, float32(32), cellRenderer.(*tableCellsRenderer).Objects()[0].Size().Height)
	assert.Equal(t, float32(32)+cell1Offset, cellRenderer.(*tableCellsRenderer).Objects()[3].Position().Y)

	w := test.NewWindow(table)
	defer w.Close()
	w.Resize(geo.NewSize(120+(2*theme.Padding()), 120+(2*theme.Padding())))
	test.AssertImageMatches(t, "table/row_size.png", w.Canvas().Capture())
}

func TestTable_SetRowHeight_Dragged(t *testing.T) {
	test.NewApp()

	table := NewTableWithHeaders(
		func() (int, int) { return 5, 5 },
		func() core.CanvasObject {
			return NewLabel("")
		},
		func(id TableCellID, obj core.CanvasObject) {
		},
	)
	table.ShowHeaderRow = false
	table.StickyRowCount = 0
	table.Refresh()

	c := test.NewCanvas()
	c.SetPadded(false)
	c.SetContent(table)
	c.Resize(geo.NewSize(120, 150))

	dragPos := geo.NewPos(2, table.cellSize.Height*3+theme.Padding()*2+1) // gap between row 2 and 3
	table.HoverMoved(&event.HoverEvent{PointEvent: event.PointEvent{Position: dragPos}})
	table.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: dragPos}})
	test.Drag(c, dragPos.AddXY(0, 5), 0, 5) // expanded row 5.0

	assert.Equal(t, table.cellSize.Height+5, table.rowHeights[2])

	dragPos = dragPos.AddXY(0, 5)
	table.HoverMoved(&event.HoverEvent{PointEvent: event.PointEvent{Position: dragPos}})
	table.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: dragPos}})
	table.Dragged(&event.DragEvent{ // reduce less than min height
		PointEvent: event.PointEvent{Position: dragPos.SubtractXY(0, 25)},
		Dragged:    geo.Delta{DX: 0, DY: -25},
	})

	assert.Equal(t, table.cellSize.Height, table.rowHeights[2])

	dragPos = dragPos.SubtractXY(0, 25)
	test.Drag(c, dragPos.AddXY(0, 25), 0, 25) // return to before-min-drag

	assert.Equal(t, table.cellSize.Height+5, table.rowHeights[2])
}

func TestTable_ShowVisible(t *testing.T) {
	table := NewTable(
		func() (int, int) { return 50, 50 },
		func() core.CanvasObject {
			return NewLabel("placeholder")
		},
		func(TableCellID, core.CanvasObject) {},
	)
	table.Resize(geo.NewSize(120, 120))

	cellRenderer := test.TempWidgetRenderer(t, table.content.Content.(*tableCells))
	cellRenderer.Refresh()
	assert.Greater(t, len(cellRenderer.(*tableCellsRenderer).visible), 0)
}

func TestTable_SeparatorThicknessZero_NotPanics(t *testing.T) {
	test.NewTempApp(t)

	test.ApplyTheme(t, &paddingZeroTheme{test.Theme()})

	table := NewTable(
		func() (int, int) { return 500, 150 },
		func() core.CanvasObject {
			return NewLabel("placeholder")
		},
		func(TableCellID, core.CanvasObject) {},
	)

	assert.NotPanics(t, func() {
		table.Resize(geo.NewSize(400, 644))
	})
}

type paddingZeroTheme struct {
	bellui.Theme
}

func (t *paddingZeroTheme) Size(n bellui.ThemeSizeName) float32 {
	if n == theme.SizeNamePadding {
		return 0
	}
	return t.Theme.Size(n)
}

func areaContainsLabel(list []core.CanvasObject, text string) bool {
	for _, o := range list {
		l, ok := unwrapTableCell(o).(*Label)
		if !ok {
			continue
		}
		if l.Text == text {
			return true
		}
	}
	return false
}
