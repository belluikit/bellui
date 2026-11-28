package tables

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/tables/render"
)

// Heights of the optional rows around the body.
const (
	floatingFilterHeight = 30
	statusBarHeight      = 24
)

// dataGridRenderer implements core.WidgetRenderer for the DataGrid.
type dataGridRenderer struct {
	grid *DataGrid

	bg       *canvas.Rectangle
	headerBg *canvas.Rectangle

	pinnedLeftCells  *render.PinnedCellsWidget
	pinnedRightCells *render.PinnedCellsWidget

	floatingFilterRow *render.FloatingFilterRow
	statusBar         *render.StatusBar
	headerSep         *canvas.Rectangle

	objects []core.CanvasObject
}

func newDataGridRenderer(g *DataGrid) core.WidgetRenderer {
	tc := render.ResolveThemeColors()
	bg := canvas.NewRectangle(tc.Background)
	headerBg := canvas.NewRectangle(tc.HeaderBg)
	headerSep := canvas.NewRectangle(tc.Separator)

	var floatingFilter *render.FloatingFilterRow
	if g.Options.FloatingFilters {
		floatingFilter = render.NewFloatingFilterRow(g, g.viewport)
		// Share the reference so onScrolled can refresh it in sync with the body.
		g.floatingFilterRow = floatingFilter
	}

	var statusBar *render.StatusBar
	if g.Options.ShowStatusBar {
		statusBar = render.NewStatusBar(g)
	}

	r := &dataGridRenderer{
		grid:              g,
		bg:                bg,
		headerBg:          headerBg,
		headerSep:         headerSep,
		pinnedLeftCells:   g.pinnedLeftCells,
		pinnedRightCells:  g.pinnedRightCells,
		floatingFilterRow: floatingFilter,
		statusBar:         statusBar,
	}
	r.rebuildObjects()
	return r
}

func (r *dataGridRenderer) Layout(size geo.Size) {
	r.doLayout(size)
}

// doLayout positions all children for the given size. Called from both
// Layout() (normal Bellui lifecycle) and relayoutChildren() (during resize drags).
func (r *dataGridRenderer) doLayout(size geo.Size) {
	if size.Width <= 0 || size.Height <= 0 {
		return
	}
	g := r.grid
	// The layout direction is known here, inside a layout or a widget's
	// refresh; the swipe handlers read it later (#1483).
	g.rtl = lang.IsRTLDirection()
	headerH := g.HeaderHeight()
	layout := g.visibleLayout()
	padding := g.cellPadding()
	pinnedLeftW := layout.pinnedLeftWidth(padding)

	// Band anchors come from the single shared source (also used by hitRegions /
	// the header + floating-filter rows) so the body is drawn exactly where
	// hit-testing and the region rows place the columns.
	mainX, pinnedRightX, pinnedRightW := g.bandAnchors(size)

	// The viewport's column positions start past the checkbox-selection
	// column. Options.CheckboxSelection can change after the geometry was
	// built, and a layout, unlike Refresh, does not rebuild it (#1498).
	if g.viewport.Leading != g.selectionColumnWidth() {
		g.updateViewportGeometry()
	}

	// Background
	r.bg.Resize(size)

	// Header
	r.headerBg.Move(geo.NewPos(0, 0))
	r.headerBg.Resize(geo.NewSize(size.Width, headerH))
	g.headerWidget.Move(geo.NewPos(0, 0))
	g.headerWidget.Resize(geo.NewSize(size.Width, headerH))

	// Header separator
	r.headerSep.Move(geo.NewPos(0, headerH-1))
	r.headerSep.Resize(geo.NewSize(size.Width, 1))

	yOffset := headerH

	// Floating filter row
	if r.floatingFilterRow != nil {
		r.floatingFilterRow.Move(geo.NewPos(0, yOffset))
		r.floatingFilterRow.Resize(geo.NewSize(size.Width, floatingFilterHeight))
		yOffset += floatingFilterHeight
	}

	// Status bar height
	statusBarH := float32(0)
	if r.statusBar != nil {
		statusBarH = statusBarHeight
	}

	// The same body height bandAnchors judges vertical overflow by.
	contentH := g.bodyHeight(size.Height)

	// Layout: [pinned-left | main-scroll [pinned-right-overlay] gutter |]
	// The scroll area extends to the full remaining width so the scrollbar
	// is at the far right edge. The pinned-right is overlaid on top of the
	// scroll, after the main columns, and keeps clear of the vertical bar's
	// gutter when that bar shows (#1257). The viewport's effective width ends
	// where pinned-right begins, so cells don't render behind the overlay.
	scrollW := size.Width - mainX
	if scrollW < 0 {
		scrollW = 0
	}
	// The scroll content gets trailing room as wide as the overlaid zone, so
	// the last main column can scroll fully clear of pinned-right rather than
	// ending under it.
	trailing := float32(0)
	if pinnedRightW > 0 {
		trailing = pinnedRightW + padding + g.vScrollGutter(size)
	}
	trailingChanged := g.cellsWidget.SetTrailingWidth(trailing)
	scrollSize := geo.NewSize(scrollW, contentH)
	g.scrollContent.Scroll.Move(geo.NewPos(mainX, yOffset))
	if trailingChanged && g.scrollContent.Scroll.Size() == scrollSize {
		g.scrollContent.Scroll.Refresh() // a same-size Resize would not relayout
	}
	g.scrollContent.Scroll.Resize(scrollSize)

	effectiveViewW := scrollW
	if pinnedRightW > 0 {
		effectiveViewW = pinnedRightX - mainX - padding
	}
	if effectiveViewW < 0 {
		effectiveViewW = 0
	}
	g.viewport.Update(g.offset, geo.NewSize(effectiveViewW, contentH))
	g.noteVisibleRows()

	// Pinned left cells
	if pinnedLeftW > 0 {
		r.pinnedLeftCells.Move(geo.NewPos(0, yOffset))
		r.pinnedLeftCells.Resize(geo.NewSize(pinnedLeftW, contentH))
		r.pinnedLeftCells.Show()
	} else {
		r.pinnedLeftCells.Hide()
	}

	// Pinned right cells — overlaid on the scroll area at the shared anchor,
	// after the main columns, and ending above the horizontal bar when it
	// shows so the bar is not drawn under them (#1257).
	if pinnedRightW > 0 {
		_, barBottom := g.scrollBarInsets(scrollSize)
		r.pinnedRightCells.Move(geo.NewPos(pinnedRightX, yOffset))
		r.pinnedRightCells.Resize(geo.NewSize(pinnedRightW, geo.Max(contentH-barBottom, 0)))
		r.pinnedRightCells.Show()
	} else {
		r.pinnedRightCells.Hide()
	}

	// Right to left, pinned-left is at the right edge, the main scroll to its
	// left and pinned-right at the left end (#779). The rest spans the width.
	g.mirrorPlaced(size.Width, g.scrollContent.Scroll, r.pinnedLeftCells, r.pinnedRightCells)

	// Group-header overlay — spans the full body width, above every cell band,
	// so each group row's label and accessible node cover pinned-left through
	// pinned-right in one place (#110, #111).
	g.groupRowOverlay.Move(geo.NewPos(0, yOffset))
	g.groupRowOverlay.Resize(geo.NewSize(size.Width, contentH))
	g.groupRowOverlay.SetScrollBarInsets(g.scrollBarInsets(scrollSize))

	// The swiped row, over every band (#1483).
	g.rowSwipeLayer.Move(geo.NewPos(0, yOffset))
	g.rowSwipeLayer.Resize(geo.NewSize(size.Width, contentH))

	// Status bar
	if r.statusBar != nil {
		r.statusBar.Move(geo.NewPos(0, size.Height-statusBarH))
		r.statusBar.Resize(geo.NewSize(size.Width, statusBarH))
	}

	// Inline cell editor — overlaid on the editing cell.
	g.layoutEditOverlay(size)

	// Loading overlay
	if g.loadingOverlay != nil && g.loadingOverlay.Visible() {
		g.loadingOverlay.Move(geo.NewPos(0, 0))
		g.loadingOverlay.Resize(size)
	}

	// Viewport was already updated above with the effective width.
	// Refresh all children.
	g.cellsWidget.Refresh()
	g.headerWidget.Refresh()
	g.pinnedLeftCells.Refresh()
	g.pinnedRightCells.Refresh()
	g.groupRowOverlay.Refresh()
	if g.rowSwipeLayer.Visible() {
		g.rowSwipeLayer.Refresh()
	}
}

// scrollBarInsets returns the width of the vertical and the height of the
// horizontal scroll bar the main scroll of size scrollSize draws, each 0 when
// that bar is not shown. It follows the scroll's own rule: a bar shows when its
// direction scrolls and the content is larger than the scroll that way.
func (g *DataGrid) scrollBarInsets(scrollSize geo.Size) (right, bottom float32) {
	dir := g.scrollContent.Scroll.Direction
	content := g.cellsWidget.MinSize()
	barW := g.Theme().Size(bellui.ThemeSizeName("scrollBar"))
	if (dir == container.ScrollBoth || dir == container.ScrollVerticalOnly) && content.Height > scrollSize.Height {
		right = barW
	}
	if (dir == container.ScrollBoth || dir == container.ScrollHorizontalOnly) && content.Width > scrollSize.Width {
		bottom = barW
	}
	return right, bottom
}

func (r *dataGridRenderer) MinSize() geo.Size {
	g := r.grid
	headerH := g.HeaderHeight()
	rowH := g.rowHeight()
	minH := headerH + rowH*3

	layout := g.visibleLayout()
	minW := float32(0)
	for _, col := range layout.all {
		w := col.def.MinWidth
		if w <= 0 {
			w = 40
		}
		minW += w
	}

	if r.statusBar != nil {
		minH += statusBarHeight
	}

	return geo.NewSize(minW, minH)
}

func (r *dataGridRenderer) Refresh() {
	g := r.grid

	tc := render.ResolveThemeColors()
	r.bg.FillColor = tc.Background
	r.headerBg.FillColor = tc.HeaderBg
	r.headerSep.FillColor = tc.Separator

	g.updateViewportGeometry()

	// Re-run the full layout with the current size so that pinned regions,
	// scroll position, and all children are repositioned correctly.
	r.doLayout(g.Size())

	if r.statusBar != nil {
		r.statusBar.Refresh()
	}

	r.rebuildObjects()
}

func (r *dataGridRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *dataGridRenderer) Destroy() {}

func (r *dataGridRenderer) rebuildObjects() {
	g := r.grid
	r.objects = []core.CanvasObject{
		r.bg,
		r.headerBg,
		g.headerWidget,
		r.headerSep,
	}

	if r.floatingFilterRow != nil {
		r.objects = append(r.objects, r.floatingFilterRow)
	}

	r.objects = append(
		r.objects,
		r.pinnedLeftCells,
		g.scrollContent.Scroll,
		r.pinnedRightCells,
		// Above every cell band: group-header rows render once here, spanning
		// pinned-left through pinned-right.
		g.groupRowOverlay,
		// The swiped row and its actions, drawn whole across the bands.
		g.rowSwipeLayer,
	)

	if r.statusBar != nil {
		r.objects = append(r.objects, r.statusBar)
	}

	// The inline editor draws above all cell bands (it overlays the editing
	// cell, which may sit in a pinned band).
	if g.editWidget != nil {
		r.objects = append(r.objects, g.editOverlay())
	}

	if g.loadingOverlay != nil {
		r.objects = append(r.objects, g.loadingOverlay)
	}
}

// ShowTooltip shows a tooltip popup near the given position.
func ShowTooltip(text string, pos geo.Position, c core.Canvas) *widget.PopUp {
	if text == "" || c == nil {
		return nil
	}
	label := widget.NewLabel(text)
	popup := widget.NewPopUp(label, c)
	popup.ShowAtPosition(pos)
	return popup
}
