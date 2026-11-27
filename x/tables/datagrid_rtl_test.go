package tables

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/x/tables/pipeline"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rtlGridCols are columns for the right-to-left tests (#779): a pinned-left
// and a pinned-right column around main columns wider than the window, so
// the body scrolls sideways.
func rtlGridCols() []*ColumnDef {
	return []*ColumnDef{
		{Field: "id", Width: 60, Pinned: PinLeft},
		{Field: "name", Width: 150},
		{Field: "city", Width: 150},
		{Field: "amt", Width: 100},
		{Field: "note", Width: 120},
		{Field: "z", Width: 80, Pinned: PinRight},
	}
}

// rtlGridData is rows rows whose every cell text is unique within its row.
// The note column holds one of two values, to group by.
func rtlGridData(rows int) []map[string]any {
	data := make([]map[string]any, rows)
	for i := range data {
		data[i] = map[string]any{
			"id": fmt.Sprintf("i%d", i), "name": fmt.Sprintf("name %d", i), "city": fmt.Sprintf("city %d", i),
			"amt": i * 10, "note": fmt.Sprintf("note %d", i%2), "z": fmt.Sprintf("z%d", i),
			"n": i, "p": (i+1)/2 - 1,
		}
	}
	return data
}

// gridDirections runs f on a grid left-to-right, right-to-left app-wide, and
// inside a right-to-left DirectionScope in a left-to-right app. setup
// configures the grid before it is shown.
func gridDirections(t *testing.T, size geo.Size, setup func(*DataGrid), f func(t *testing.T, rtl bool, g *DataGrid, w bellui.Window)) {
	gridDirectionsRows(t, 5, size, setup, f)
}

// gridDirectionsRows is gridDirections with rows rows.
func gridDirectionsRows(t *testing.T, rows int, size geo.Size, setup func(*DataGrid), f func(t *testing.T, rtl bool, g *DataGrid, w bellui.Window)) {
	for _, c := range []struct {
		name   string
		global bellui.Direction
		scope  bool
	}{
		{"ltr", bellui.DirectionLTR, false},
		{"rtl", bellui.DirectionRTL, false},
		{"scope", bellui.DirectionLTR, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := newTestGrid(t, rtlGridData(rows), rtlGridCols())
			bellui.SetLayoutDirection(c.global)
			t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })
			if setup != nil {
				setup(g)
			}
			var content core.CanvasObject = g
			if c.scope {
				content = container.NewDirectionScope(g, bellui.DirectionRTL)
			}
			w := test.NewTempWindow(t, content)
			w.Resize(size)
			rtl := c.name != "ltr"
			require.Equal(t, rtl, g.rtl, "the grid was laid out in the direction under test")
			f(t, rtl, g, w)
		})
	}
}

// drawnRect is where an object is drawn on the grid: its left edge and width.
type drawnRect struct{ x, w float32 }

// leading is how far r is from the grid's leading edge: its left edge's
// distance from the left, or right to left its right edge's from the right.
func (r drawnRect) leading(rtl bool, width float32) float32 {
	if rtl {
		return width - r.x - r.w
	}
	return r.x
}

// drawnParts walks what the grid draws (its renderers' own objects, not fresh
// ones) and returns where each visible text and accessible node is, keyed by
// what it is and the row it is on, which do not change with direction.
func drawnParts(t *testing.T, g *DataGrid) map[string]drawnRect {
	t.Helper()
	d := bellui.CurrentApp().Driver()
	gridX := d.AbsolutePositionForObject(g).X
	gridY := d.AbsolutePositionForObject(g).Y
	parts := map[string]drawnRect{}
	add := func(o core.CanvasObject, key string) {
		p := d.AbsolutePositionForObject(o)
		key = fmt.Sprintf("%s@%.0f", key, p.Y-gridY)
		// Alike parts on one row (the filter entries) are told apart by
		// the order they are drawn in, which direction does not change.
		for n := 2; ; n++ {
			if _, dup := parts[key]; !dup {
				break
			}
			key = fmt.Sprintf("%s#%d", strings.TrimSuffix(key, fmt.Sprintf("#%d", n-1)), n)
		}
		parts[key] = drawnRect{p.X - gridX, o.Size().Width}
	}
	var walk func(o core.CanvasObject)
	walk = func(o core.CanvasObject) {
		if o == nil || !o.Visible() {
			return
		}
		switch v := o.(type) {
		case *canvas.Text:
			if v.Text != "" {
				add(o, "text "+v.Text)
			}
		case core.Accessible:
			key := fmt.Sprintf("%T %d %q", o, v.AccessibleRole(), v.AccessibleLabel())
			if c, ok := o.(interface{ AccessibleCellPosition() (int, int) }); ok {
				r, col := c.AccessibleCellPosition()
				key += fmt.Sprintf(" cell %d,%d", r, col)
			}
			add(o, key)
		}
		switch v := o.(type) {
		case *bellui.Container:
			for _, c := range v.Objects {
				walk(c)
			}
		case core.Widget:
			for _, c := range test.WidgetRenderer(v).Objects() {
				walk(c)
			}
		}
	}
	walk(g)
	return parts
}

// leadingParts is drawnParts as distances from the leading edge.
func leadingParts(t *testing.T, g *DataGrid, rtl bool) map[string]float32 {
	t.Helper()
	out := map[string]float32{}
	for k, r := range drawnParts(t, g) {
		out[k] = r.leading(rtl, g.Size().Width)
	}
	return out
}

// assertMirrorsLTR runs scenario on a grid in each direction and checks that
// right to left every text and accessible node is drawn as far from the
// leading edge as left to right: the grid is the mirror image of itself.
func assertMirrorsLTR(t *testing.T, size geo.Size, setup func(*DataGrid), scenario func(t *testing.T, rtl bool, g *DataGrid)) {
	t.Helper()
	var want map[string]float32
	gridDirections(t, size, setup, func(t *testing.T, rtl bool, g *DataGrid, _ bellui.Window) {
		if scenario != nil {
			scenario(t, rtl, g)
		}
		got := leadingParts(t, g, rtl)
		require.NotEmpty(t, got)
		if !rtl {
			want = got
			return
		}
		require.NotNil(t, want, "the left-to-right run comes first")
		keys := func(m map[string]float32) []string {
			var ks []string
			for k := range m {
				ks = append(ks, k)
			}
			sort.Strings(ks)
			return ks
		}
		require.Equal(t, keys(want), keys(got), "the same parts are drawn")
		for k, x := range want {
			assert.InDelta(t, x, got[k], 0.5, "%s is as far from the leading edge", k)
		}
	})
}

// The bands, their cells and headers, the floating filters, the checkboxes
// and the status bar are the mirror image of the left-to-right grid (#779).
func TestDataGrid_RTLMirrorsLayout(t *testing.T) {
	size := geo.NewSize(600, 300)
	t.Run("bands", func(t *testing.T) {
		assertMirrorsLTR(t, size, nil, nil)
	})
	t.Run("checkboxes and filters", func(t *testing.T) {
		assertMirrorsLTR(t, size, func(g *DataGrid) {
			g.Options.CheckboxSelection = true
			g.SelectionMode = SelectionMultiRow // for the select-all box
			g.Options.ShowStatusBar = true
			g.Options.FloatingFilters = true
		}, func(t *testing.T, _ bool, g *DataGrid) {
			require.NotNil(t, selectAllBox(g), "the header draws the select-all box")
		})
	})
	t.Run("scrolled to the last column", func(t *testing.T) {
		assertMirrorsLTR(t, size, nil, func(t *testing.T, _ bool, g *DataGrid) {
			g.scrollToCell(CellID{Row: 0, Col: 4})
			require.Positive(t, g.offset.X, "the body scrolled")
		})
	})
	t.Run("group rows with aggregates", func(t *testing.T) {
		assertMirrorsLTR(t, size, func(g *DataGrid) {
			g.visibleCol(4).def.AggFunc = bellui.Ptr(pipeline.AggSum)
			g.API().SetRowGroupColumn("note")
		}, func(t *testing.T, _ bool, g *DataGrid) {
			require.True(t, g.IsGroupRow(0), "the grid is grouped")
			g.ToggleGroup(g.groupKeyAt(t, 0))
		})
	})
	t.Run("master rows and an open detail", func(t *testing.T) {
		assertMirrorsLTR(t, size, func(g *DataGrid) {
			g.MasterDetail = MasterDetailConfig{
				Enabled: true, DetailHeight: 40,
				DetailRenderer: func(src int) core.CanvasObject { return canvas.NewText(fmt.Sprintf("detail %d", src), nil) },
			}
		}, func(t *testing.T, _ bool, g *DataGrid) {
			g.ToggleDetail(1)
			require.Positive(t, g.DetailBandHeight(1), "row 1's detail is open")
		})
	})
	t.Run("tree rows", func(t *testing.T) {
		assertMirrorsLTR(t, size, func(g *DataGrid) {
			g.SetColumnPinned("id", PinNone) // the disclosure is on the first main column
			g.API().SetTreeData(
				func(r any) any { return r.(map[string]any)["n"] },
				func(r any) int { return r.(map[string]any)["p"].(int) })
		}, func(t *testing.T, _ bool, g *DataGrid) {
			g.ToggleTreeNode(0)
			require.True(t, g.IsTreeRow(1), "the root opened")
		})
	})
	t.Run("an open editor", func(t *testing.T) {
		assertMirrorsLTR(t, size, func(g *DataGrid) {
			g.DefaultColDef = &DefaultColumnDef{Editable: true}
		}, func(t *testing.T, _ bool, g *DataGrid) {
			g.StartEditing(CellID{Row: 1, Col: 2})
			require.NotNil(t, g.editingCell, "the editor opened")
		})
	})
	t.Run("a swiped row", func(t *testing.T) {
		assertMirrorsLTR(t, size, func(g *DataGrid) {
			g.LeadingSwipeActions = func(int) []RowSwipeAction { return []RowSwipeAction{{Label: "Flag"}} }
		}, func(t *testing.T, rtl bool, g *DataGrid) {
			s := &g.swipe
			s.src = g.sourceRow(1)
			s.left, s.right = g.swipeActionsFor(s.src)
			offset := float32(60) // toward the trailing side, uncovering the leading action
			if rtl {
				offset = -offset
			}
			g.placeSwipe(offset)
			require.True(t, g.rowSwipeLayer.Visible())
		})
	})
	// Right to left the body grows at its left end, so the scroll must move
	// with it to keep its place (ScrollContainer.keepLeadingOffset).
	t.Run("a column widened while scrolled", func(t *testing.T) {
		assertMirrorsLTR(t, size, nil, func(t *testing.T, _ bool, g *DataGrid) {
			g.scrollToCell(CellID{Row: 0, Col: 4})
			before := g.offset.X
			g.visibleCol(1).width += 60
			g.refreshAfterColumnChange()
			require.Equal(t, before, g.offset.X, "the scroll kept its distance from the leading edge")
		})
	})
}

// dragColumnBorder drags column col's trailing border by dx toward the
// trailing side, as a mouse would on the header.
func dragColumnBorder(t *testing.T, g *DataGrid, rtl bool, col int, dx float32) {
	t.Helper()
	x := columnTrailingEdge(t, g, rtl, col)
	g.headerWidget.MouseDown(mouseAt(x, g.HeaderHeight()/2))
	if rtl {
		dx = -dx
	}
	g.headerWidget.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(x+dx, g.HeaderHeight()/2)}})
	g.headerWidget.DragEnd()
}

// columnTrailingEdge is the x of column col's divider as drawn: the header
// cell's right edge, or right to left its left edge.
func columnTrailingEdge(t *testing.T, g *DataGrid, rtl bool, col int) float32 {
	t.Helper()
	r := headerCellDrawn(t, g, col)
	if rtl {
		return r.x
	}
	return r.x + r.w
}

// headerCellDrawn is where column col's header cell is drawn.
func headerCellDrawn(t *testing.T, g *DataGrid, col int) drawnRect {
	t.Helper()
	title := g.ColumnHeader(col)
	for k, r := range drawnParts(t, g) {
		if k == fmt.Sprintf("*render.headerCellA11y %d %q@0", core.RoleHeading, title) {
			return r
		}
	}
	t.Fatalf("no header cell drawn for column %d (%q)", col, title)
	return drawnRect{}
}

// mouseAt is a primary-button press at (x, y).
func mouseAt(x, y float32) *desktop.MouseEvent {
	return &desktop.MouseEvent{PointEvent: event.PointEvent{Position: geo.NewPos(x, y)}, Button: desktop.MouseButtonPrimary}
}

// groupKeyAt is the key of the group header at display row row.
func (g *DataGrid) groupKeyAt(t *testing.T, row int) string {
	t.Helper()
	key, _, _, _ := g.GroupRowInfo(row)
	return key
}

// drawnCellRect is where the realized cell at display row row and absolute
// column col is drawn.
func drawnCellRect(t *testing.T, g *DataGrid, row, col int) drawnRect {
	t.Helper()
	for k, r := range drawnParts(t, g) {
		if strings.HasPrefix(k, "*render.cellHost ") && strings.Contains(k, fmt.Sprintf(" cell %d,%d@", row, col)) {
			return r
		}
	}
	t.Fatalf("no cell drawn at %d,%d", row, col)
	return drawnRect{}
}

// drawnNode is where the accessible node of type typ and the given label is
// drawn, and its y.
func drawnNode(t *testing.T, g *DataGrid, typ, label string) (drawnRect, float32) {
	t.Helper()
	prefix := fmt.Sprintf("%s ", typ)
	for k, r := range drawnParts(t, g) {
		if strings.HasPrefix(k, prefix) && strings.Contains(k, fmt.Sprintf("%q@", label)) {
			var y float32
			_, _ = fmt.Sscanf(k[strings.LastIndex(k, "@")+1:], "%g", &y)
			return r, y
		}
	}
	t.Fatalf("no %s %q drawn", typ, label)
	return drawnRect{}, 0
}

// A pointer resolves to the column drawn under it, header and body; a
// header border grabs the column it trails; arrow keys move toward the
// column drawn that way (#779).
func TestDataGrid_RTLInput(t *testing.T) {
	gridDirections(t, geo.NewSize(600, 300), nil, func(t *testing.T, rtl bool, g *DataGrid, _ bellui.Window) {
		headY := g.HeaderHeight() / 2
		for _, col := range []int{0, 1, 2, 3, 5} {
			h := headerCellDrawn(t, g, col)
			assert.Equal(t, CellID{Row: -1, Col: col}, g.cellAtPosition(geo.NewPos(h.x+h.w/2, headY)), "a tap on column %d's header", col)
			assert.Equal(t, col, g.ColumnAtX(h.x+h.w/2), "the header's own mapping for column %d", col)

			c := drawnCellRect(t, g, 1, col)
			y := g.bodyTop() + g.rowHeight()*1.5
			assert.Equal(t, CellID{Row: 1, Col: col}, g.cellAtPosition(geo.NewPos(c.x+c.w/2, y)), "a tap on column %d's cell", col)

			pos, size := g.cellWidgetRect(CellID{Row: 1, Col: col})
			assert.InDelta(t, c.x, pos.X, 0.5, "column %d's cell rectangle is where it is drawn", col)
			assert.InDelta(t, c.w, size.Width, 0.5)
		}
		for _, col := range []int{1, 2} {
			assert.Equal(t, col, g.headerBorderCol(columnTrailingEdge(t, g, rtl, col)), "column %d's trailing border grabs it", col)
		}

		g.currentFocus = CellID{Row: 1, Col: 2}
		left, right := 1, 3
		if rtl {
			left, right = 3, 1
		}
		g.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
		assert.Equal(t, left, g.currentFocus.Col, "Left moves to the column drawn on the left")
		g.currentFocus = CellID{Row: 1, Col: 2}
		g.TypedKey(&event.KeyEvent{Name: event.KeyRight})
		assert.Equal(t, right, g.currentFocus.Col, "Right moves to the column drawn on the right")
	})
}

// Dragging a column's border toward its trailing side widens it: right to
// left that is a drag to the left (#779). The columns overflow the window,
// so the drag also runs in a body that scrolls sideways (#1499).
func TestDataGrid_RTLResizeDrag(t *testing.T) {
	gridDirections(t, geo.NewSize(600, 300), nil, func(t *testing.T, rtl bool, g *DataGrid, _ bellui.Window) {
		dragColumnBorder(t, g, rtl, 2, 30)
		assert.Equal(t, float32(180), g.visibleCol(2).width, "the header's drag widened the column")

		// The grid's own press-and-drag path.
		x := columnTrailingEdge(t, g, rtl, 2)
		g.MouseDown(mouseAt(x, g.HeaderHeight()/2))
		require.Equal(t, 2, g.resizeCol)
		dx := float32(-20)
		if rtl {
			dx = -dx
		}
		g.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(x+dx, g.HeaderHeight()/2)}})
		g.DragEnd()
		assert.Equal(t, float32(160), g.visibleCol(2).width, "the grid's drag narrowed it")
	})
}

// A tap on a disclosure toggles its row: a tree node's, and a master row's
// detail (#779).
func TestDataGrid_RTLDisclosureTaps(t *testing.T) {
	t.Run("tree", func(t *testing.T) {
		gridDirections(t, geo.NewSize(600, 300), func(g *DataGrid) {
			g.SetColumnPinned("id", PinNone)
			g.API().SetTreeData(
				func(r any) any { return r.(map[string]any)["n"] },
				func(r any) int { return r.(map[string]any)["p"].(int) })
		}, func(t *testing.T, _ bool, g *DataGrid, _ bellui.Window) {
			r, y := drawnNode(t, g, "*render.treeRowA11y", "i0")
			assert.Equal(t, 0, g.treeToggleAt(geo.NewPos(r.x+r.w/2, y+g.rowHeight()/2)), "a tap on the root's chevron")
			assert.Equal(t, -1, g.treeToggleAt(geo.NewPos(g.Size().Width-r.x-r.w/2, y+g.rowHeight()/2)), "and not on its mirror image")
		})
	})
	t.Run("master detail", func(t *testing.T) {
		gridDirections(t, geo.NewSize(600, 300), func(g *DataGrid) {
			g.MasterDetail = MasterDetailConfig{
				Enabled: true, DetailHeight: 40,
				DetailRenderer: func(int) core.CanvasObject { return canvas.NewText("detail", nil) },
			}
		}, func(t *testing.T, _ bool, g *DataGrid, _ bellui.Window) {
			r, y := drawnNode(t, g, "*render.masterRowA11y", "i1 details")
			assert.Equal(t, 1, g.detailToggleAt(geo.NewPos(r.x+r.w/2, y+g.rowHeight()/2)), "a tap on row 1's chevron")
		})
	})
}

// A row's select checkbox toggles its row when clicked or touched, and a press
// elsewhere selects as usual: the grid finds the checkbox column where it is
// drawn, at column 0's side (#779, #1504).
func TestDataGrid_RTLSelectionColumn(t *testing.T) {
	var ltrLead float32
	gridDirections(t, geo.NewSize(600, 300), func(g *DataGrid) {
		g.Options.CheckboxSelection = true
		g.SelectionMode = SelectionMultiRow
	}, func(t *testing.T, rtl bool, g *DataGrid, w bellui.Window) {
		at := bellui.CurrentApp().Driver().AbsolutePositionForObject(g)
		press := func(pos geo.Position) {
			ev := mouseAt(pos.X, pos.Y)
			g.MouseDown(ev)
			g.MouseUp(ev)
		}
		click := func(row int) {
			_, pos := rowBox(t, g, row)
			press(pos)
			test.TapCanvas(w.Canvas(), at.Add(pos))
		}
		touch := func(row int) {
			_, pos := rowBox(t, g, row)
			test.TapCanvas(w.Canvas(), at.Add(pos))
		}

		box, pos := rowBox(t, g, 1)
		lead := drawnRect{pos.X - box.Size().Width/2, box.Size().Width}.leading(rtl, g.Size().Width)
		if rtl {
			assert.InDelta(t, ltrLead, lead, 0.5, "the box is as far from the leading edge as left to right")
		} else {
			ltrLead = lead
		}

		click(1)
		checkRows(t, g, "click row 1", 1)
		click(3)
		checkRows(t, g, "click row 3", 1, 3)
		click(1)
		checkRows(t, g, "click row 1 again", 3)
		touch(0)
		checkRows(t, g, "touch row 0", 0, 3)

		// The box's mirror image is another column: a press there selects
		// the row like any cell's.
		g.API().DeselectAll()
		mirror := geo.NewPos(g.Size().Width-pos.X, pos.Y)
		require.GreaterOrEqual(t, g.cellAtPosition(mirror).Col, 0, "a column is drawn at the box's mirror image")
		press(mirror)
		checkRows(t, g, "press the box's mirror image on row 1", 1)
	})
}

// The body scrolls from column 0's side: a wheel and the a11y scroll move it
// toward the last column, and a finger drags it the way it moves (#779).
func TestDataGrid_RTLScrolls(t *testing.T) {
	var wheel float32
	gridDirections(t, geo.NewSize(600, 300), nil, func(t *testing.T, rtl bool, g *DataGrid, _ bellui.Window) {
		s := g.scrollContent.Scroll
		far := g.cellsWidget.Size().Width - s.Size().Width
		require.Positive(t, far, "the body scrolls sideways")
		want := float32(0)
		if rtl {
			want = far
		}
		assert.Equal(t, want, s.Offset.X, "the Scroll opens at column 0's end")
		assert.Zero(t, g.offset.X)

		g.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(-30, 0)})
		require.Positive(t, g.offset.X, "a wheel scrolls toward the last column")
		if rtl {
			assert.Equal(t, wheel, g.offset.X, "as far as left to right")
		} else {
			wheel = g.offset.X
		}
		assert.Equal(t, g.offset.X, g.flipScrollX(s.Offset.X), "the grid's offset is the Scroll's, from the leading edge")

		g.offset = geo.Position{}
		g.scrollContent.ScrollTo(geo.NewPos(g.flipScrollX(0), 0))
		dx := float32(-50) // a finger moving toward the trailing side ...
		if rtl {
			dx = -dx
		}
		g.mousePressed = false
		g.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(300, 150)}, Dragged: geo.NewDelta(dx, 0)})
		assert.Equal(t, float32(50), g.offset.X, "... pulls the last columns in")
	})
}

// The accessible nodes of cells off the screen are where the cells would be
// drawn, as the realized ones are, and the a11y realize rectangle is on the
// leading side of the cell (#779).
func TestDataGrid_RTLAccessibleRects(t *testing.T) {
	// More rows than the body shows, so some are virtual.
	gridDirectionsRows(t, 60, geo.NewSize(600, 300), func(g *DataGrid) {
		g.SetColumnPinned("id", PinNone)
		g.SetColumnPinned("z", PinNone)
	}, func(t *testing.T, rtl bool, g *DataGrid, _ bellui.Window) {
		realized := map[int]float32{}
		for _, o := range test.WidgetRenderer(g.cellsWidget).Objects() {
			if c, ok := o.(interface{ AccessibleCellPosition() (int, int) }); ok && o.Visible() {
				_, col := c.AccessibleCellPosition()
				realized[col] = o.Position().X
			}
		}
		require.NotEmpty(t, realized)
		virtual := 0
		for _, n := range g.cellsWidget.AccessibleVirtualNodes() {
			if x, ok := realized[n.CellCol]; ok {
				assert.InDelta(t, x, n.Pos.X, 0.5, "column %d's virtual node is where its realized cells are", n.CellCol)
				virtual++
			}
		}
		require.Positive(t, virtual, "some cells are virtual")

		c := drawnCellRect(t, g, 1, 1)
		pos, size := g.cellRealizeRect(1, 1)
		assert.InDelta(t, drawnRect{c.x, c.w}.leading(rtl, g.Size().Width), drawnRect{pos.X, size.Width}.leading(rtl, g.Size().Width), 0.5,
			"the realize rectangle is as far from the leading edge as the cell")
	})
}

// A body taller than the grid shows the Scroll's vertical bar. Right to left
// the bar, its reserved strip, pinned-right's gutter beside it and the strip
// the group rows leave for it are all at the left, so the grid is still its
// own mirror image. This needs the Scroll to draw its bar on its left under
// right-to-left (#1496); before that, every main cell is one bar width off
// its header.
func TestDataGrid_RTLVerticalScrollBar(t *testing.T) {
	size := geo.NewSize(800, 300)
	for _, c := range []struct {
		name  string
		setup func(*DataGrid)
	}{
		{"bands", nil},
		{"group rows", func(g *DataGrid) { g.API().SetRowGroupColumn("note") }},
	} {
		t.Run(c.name, func(t *testing.T) {
			var want map[string]float32
			gridDirectionsRows(t, 40, size, c.setup, func(t *testing.T, rtl bool, g *DataGrid, _ bellui.Window) {
				if c.setup != nil {
					g.ToggleGroup(g.groupKeyAt(t, 0))
				}
				require.Positive(t, g.vScrollGutter(g.Size()), "the body overflows, so pinned-right keeps a gutter")
				got := leadingParts(t, g, rtl)
				if !rtl {
					want = got
					return
				}
				require.Len(t, got, len(want))
				for k, x := range want {
					assert.InDelta(t, x, got[k], 0.5, "%s is as far from the leading edge", k)
				}
			})
		})
	}
}
