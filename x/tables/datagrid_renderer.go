package tables

import (
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/tables/render"
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
	headerH := g.HeaderHeight()
	layout := g.visibleLayout()
	padding := g.cellPadding()
	pinnedLeftW := layout.pinnedLeftWidth(padding)

	// Band anchors come from the single shared source (also used by hitRegions /
	// the header + floating-filter rows) so the body is drawn exactly where
	// hit-testing and the region rows place the columns.
	mainX, pinnedRightX, pinnedRightW := g.bandAnchors(size.Width)

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
		filterH := float32(30)
		r.floatingFilterRow.Move(geo.NewPos(0, yOffset))
		r.floatingFilterRow.Resize(geo.NewSize(size.Width, filterH))
		yOffset += filterH
	}

	// Status bar height
	statusBarH := float32(0)
	if r.statusBar != nil {
		statusBarH = 24
	}

	contentH := size.Height - yOffset - statusBarH

	// Layout: [pinned-left | main-scroll [pinned-right-overlay] scrollbar |]
	// The scroll area extends to the full remaining width so the scrollbar
	// is at the far right edge. The pinned-right is overlaid on top of the
	// scroll, after the main columns. The viewport's effective width is
	// reduced so cells don't render behind the overlay.
	scrollW := size.Width - mainX
	if scrollW < 0 {
		scrollW = 0
	}
	g.scrollContent.Scroll.Move(geo.NewPos(mainX, yOffset))
	g.scrollContent.Scroll.Resize(geo.NewSize(scrollW, contentH))

	// Reduce the viewport's effective visible width so the CellsRenderer
	// doesn't render cells that would be hidden behind the pinned-right overlay.
	effectiveViewW := scrollW - pinnedRightW
	if pinnedRightW > 0 {
		effectiveViewW -= padding
	}
	if effectiveViewW < 0 {
		effectiveViewW = 0
	}
	g.viewport.Update(g.offset, geo.NewSize(effectiveViewW, contentH))

	// Pinned left cells
	if pinnedLeftW > 0 {
		r.pinnedLeftCells.Move(geo.NewPos(0, yOffset))
		r.pinnedLeftCells.Resize(geo.NewSize(pinnedLeftW, contentH))
		r.pinnedLeftCells.Show()
	} else {
		r.pinnedLeftCells.Hide()
	}

	// Pinned right cells — overlaid on the scroll area at the shared anchor,
	// after the main columns.
	if pinnedRightW > 0 {
		r.pinnedRightCells.Move(geo.NewPos(pinnedRightX, yOffset))
		r.pinnedRightCells.Resize(geo.NewSize(pinnedRightW, contentH))
		r.pinnedRightCells.Show()
	} else {
		r.pinnedRightCells.Hide()
	}

	// Group-header overlay — spans the full body width, above every cell band,
	// so each group row's label and accessible node cover pinned-left through
	// pinned-right in one place (#110, #111).
	g.groupRowOverlay.Move(geo.NewPos(0, yOffset))
	g.groupRowOverlay.Resize(geo.NewSize(size.Width, contentH))

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
		minH += 24
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
	)

	if r.statusBar != nil {
		r.objects = append(r.objects, r.statusBar)
	}

	// The inline editor draws above all cell bands (it overlays the editing
	// cell, which may sit in a pinned band).
	if g.editWidget != nil {
		r.objects = append(r.objects, g.editWidget)
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
