package tables

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/tables/render"
)

// rowSwipeLayer draws the row a finger is swiping, and the actions it
// reveals, over the body (#1483). The bands under it each clip their own
// columns, so a row slid across them by moving its cells in each band would
// be cut at every band's edge. The layer draws the whole row again instead,
// pinned cells included, with the same cell widgets the bands use, each
// narrowed to the one row, and slides it as one piece over an opaque
// background that hides the row beneath.
//
// The layer is hidden while no row is swiped. While one is, a cover over the
// row (swipeCover) takes taps and sideways drags on it, so a tap closes the
// row rather than reaching a cell, and a sideways drag moves it even where
// the body could scroll that way.
type rowSwipeLayer struct {
	widget.BaseWidget

	grid *DataGrid

	// vp is a copy of the grid's viewport with only the swiped row visible,
	// which the cell widgets below draw from.
	vp                      *render.Viewport
	cells                   *render.CellsWidget
	pinnedLeft, pinnedRight *render.PinnedCellsWidget
	cover                   *swipeCover
}

func newRowSwipeLayer(g *DataGrid) *rowSwipeLayer {
	l := &rowSwipeLayer{grid: g, vp: &render.Viewport{}}
	pool := render.NewCellPool()
	l.cells = render.NewCellsWidget(g, l.vp, pool)
	l.cells.OmitDetail()
	l.pinnedLeft = render.NewPinnedCellsWidget(g, render.PinnedLeft, pool, l.vp)
	l.pinnedRight = render.NewPinnedCellsWidget(g, render.PinnedRight, pool, l.vp)
	l.cover = &swipeCover{grid: g}
	l.cover.ExtendBaseWidget(l.cover)
	l.ExtendBaseWidget(l)
	l.Hide()
	return l
}

func (l *rowSwipeLayer) CreateRenderer() core.WidgetRenderer {
	l.ExtendBaseWidget(l)
	r := &rowSwipeLayerRenderer{layer: l, bg: canvas.NewRectangle(color.Transparent)}
	r.mainBand = container.NewWithoutLayout(l.cells)
	r.mainClip = container.NewClip(r.mainBand)
	r.row = container.NewWithoutLayout(r.bg, l.pinnedLeft, r.mainClip, l.pinnedRight, l.cover)
	r.band = container.NewWithoutLayout(r.row)
	r.clip = container.NewClip(r.band)
	return r
}

type rowSwipeLayerRenderer struct {
	layer *rowSwipeLayer

	// clip is the part of the row's rectangle inside the body; band, in it,
	// holds the buttons and then the row, which slides by the swipe's offset.
	clip *container.Clip
	band *bellui.Container
	row  *bellui.Container
	bg   *canvas.Rectangle

	// The main columns scroll sideways under the pinned ones: in the row they
	// sit in their own clip, at the main band's place, moved by the body's
	// horizontal scroll.
	mainClip *container.Clip
	mainBand *bellui.Container

	buttons []*swipeActionButton
	objects []core.CanvasObject
}

func (r *rowSwipeLayerRenderer) Layout(size geo.Size) { r.refresh(size) }

func (r *rowSwipeLayerRenderer) MinSize() geo.Size { return geo.NewSize(0, 0) }

func (r *rowSwipeLayerRenderer) Refresh() { r.refresh(r.layer.Size()) }

func (r *rowSwipeLayerRenderer) Objects() []core.CanvasObject { return r.objects }

func (r *rowSwipeLayerRenderer) Destroy() {}

func (r *rowSwipeLayerRenderer) refresh(size geo.Size) {
	l := r.layer
	g := l.grid
	s := &g.swipe
	row := -1
	if s.src >= 0 {
		row = g.displayRowForSource(s.src)
	}
	if row < 0 || row >= g.RowCount() || size.Width <= 0 || size.Height <= 0 {
		r.objects = nil
		return
	}

	// The row's cell band, in the layer's coordinates (the layer covers the
	// body), cut to the body.
	rowY, rowH := g.viewport.RowY(row)
	cellH := rowH - g.DetailBandHeight(row)
	y := rowY - g.offset.Y
	top, bottom := geo.Max(y, 0), geo.Min(y+cellH, size.Height)
	if bottom <= top {
		r.objects = nil
		return
	}
	width := size.Width
	r.clip.Move(geo.NewPos(0, top))
	r.clip.Resize(geo.NewSize(width, bottom-top))
	r.band.Move(geo.NewPos(0, y-top))
	r.band.Resize(geo.NewSize(width, cellH))

	r.layoutButtons(width, cellH)

	// The row, slid by the offset over a background that hides the one the
	// bands draw beneath it.
	r.row.Move(geo.NewPos(s.offset, 0))
	r.row.Resize(geo.NewSize(width, cellH))
	r.bg.FillColor = render.ResolveThemeColors().Background
	r.bg.Move(geo.NewPos(0, 0))
	r.bg.Resize(geo.NewSize(width, cellH))
	r.bg.Refresh()

	*l.vp = *g.viewport
	l.vp.VisibleRows = render.Range{Min: row, Max: row}

	layout := g.visibleLayout()
	pad := g.cellPadding()
	mainX, pinnedRightX, pinnedRightW := g.bandAnchors(g.Size())
	pinnedLeftW := layout.pinnedLeftWidth(pad)
	mainW := width - mainX
	if pinnedRightW > 0 {
		mainW = pinnedRightX - mainX - pad
	}

	if pinnedLeftW > 0 {
		l.pinnedLeft.Move(geo.NewPos(0, 0))
		l.pinnedLeft.Resize(geo.NewSize(pinnedLeftW, cellH))
		l.pinnedLeft.Show()
		l.pinnedLeft.SetOffsetY(rowY)
	} else {
		l.pinnedLeft.Hide()
	}

	r.mainClip.Move(geo.NewPos(mainX, 0))
	r.mainClip.Resize(geo.NewSize(geo.Max(mainW, 0), cellH))
	r.mainBand.Move(geo.NewPos(0, 0))
	r.mainBand.Resize(geo.NewSize(geo.Max(mainW, 0), cellH))
	l.cells.Move(geo.NewPos(-g.offset.X, -rowY))
	l.cells.Resize(l.cells.MinSize())
	l.cells.Refresh()

	if pinnedRightW > 0 {
		l.pinnedRight.Move(geo.NewPos(pinnedRightX, 0))
		l.pinnedRight.Resize(geo.NewSize(pinnedRightW, cellH))
		l.pinnedRight.Show()
		l.pinnedRight.SetOffsetY(rowY)
	} else {
		l.pinnedRight.Hide()
	}

	l.cover.Move(geo.NewPos(0, 0))
	l.cover.Resize(geo.NewSize(width, cellH))

	r.objects = []core.CanvasObject{r.clip}
	// Made after any theme override around the grid, so they take its scope
	// here, as the bands' rows do (#758).
	theme.MatchScope(l, r.clip)
}

// layoutButtons places the buttons of the side the row has slid off, from
// the row's outer edge inward, the first outermost. They sit where an open
// row leaves them, and the row covers what it has not uncovered yet. Past
// that they stretch to fill what it uncovered, and a row past a full swipe's
// threshold leaves only the first, filling it all.
func (r *rowSwipeLayerRenderer) layoutButtons(width, height float32) {
	g := r.layer.grid
	s := &g.swipe
	acts := s.actionsAt(s.offset)
	if s.offset == 0 {
		acts = nil
	}
	for len(r.buttons) < len(acts) {
		b := &swipeActionButton{grid: g}
		b.ExtendBaseWidget(b)
		r.buttons = append(r.buttons, b)
	}

	revealed := abs32(s.offset)
	full := g.fullSwipeAt(s.offset)
	total := g.swipeActionsWidth(acts)
	scale := float32(1)
	if total > 0 && revealed > total {
		scale = revealed / total
	}

	objs := make([]core.CanvasObject, 0, len(acts)+1)
	var at float32 // from the outer edge
	for i, b := range r.buttons {
		if i >= len(acts) {
			b.Hide()
			continue
		}
		w := g.swipeActionWidth(acts[i]) * scale
		if full {
			w = 0
			if i == 0 {
				w = revealed
			}
		}
		if w <= 0 {
			b.Hide()
			continue
		}
		x := at
		if s.offset < 0 {
			x = width - at - w
		}
		at += w
		b.set(acts[i], s.src)
		b.Move(geo.NewPos(x, 0))
		b.Resize(geo.NewSize(w, height))
		b.Show()
		objs = append(objs, b)
	}
	r.band.Objects = append(objs, r.row)
}

// swipeCover lies over the swiped row. A tap on it closes the row, and a
// sideways drag on it moves the row, so neither reaches what lies beneath: a
// cell, or the body, which would scroll. A drag up or down is left to the
// body, whose scrolling closes the row.
type swipeCover struct {
	widget.BaseWidget
	grid *DataGrid
}

var (
	_ event.Tappable    = (*swipeCover)(nil)
	_ event.Draggable   = (*swipeCover)(nil)
	_ event.DragClaimer = (*swipeCover)(nil)
)

func (c *swipeCover) CreateRenderer() core.WidgetRenderer {
	c.ExtendBaseWidget(c)
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (c *swipeCover) Tapped(*event.PointEvent) { c.grid.closeSwipe(true) }

// ClaimsDrag takes a drag that runs mostly sideways.
func (c *swipeCover) ClaimsDrag(d geo.Delta) bool { return sideways(d) }

// Dragged moves the swiped row, which the drag started on.
func (c *swipeCover) Dragged(e *event.DragEvent) {
	g := c.grid
	if g.disabled || g.swipe.src < 0 {
		return
	}
	if g.swipe.gesture == gestureUndecided {
		g.swipe.gesture = gestureSwipe
		g.continueSwipe()
	}
	if g.swipe.gesture == gestureSwipe {
		g.moveSwipe(e.Dragged.DX)
	}
}

func (c *swipeCover) DragEnd() { c.grid.endSwipeGesture() }

// swipeActionButton is the button of one RowSwipeAction, under the swiped
// row.
type swipeActionButton struct {
	widget.BaseWidget
	grid   *DataGrid
	action RowSwipeAction
	src    int
}

var (
	_ event.Tappable  = (*swipeActionButton)(nil)
	_ core.Accessible = (*swipeActionButton)(nil)
)

func (b *swipeActionButton) set(a RowSwipeAction, src int) {
	b.action, b.src = a, src
	b.Refresh()
}

func (b *swipeActionButton) Tapped(*event.PointEvent) { b.grid.chooseSwipeAction(b.action, b.src) }

func (b *swipeActionButton) AccessibleRole() core.AccessibleRole { return core.RoleButton }

func (b *swipeActionButton) AccessibleLabel() string { return b.action.Label }

func (b *swipeActionButton) CreateRenderer() core.WidgetRenderer {
	b.ExtendBaseWidget(b)
	r := &swipeActionButtonRenderer{
		button: b,
		bg:     canvas.NewRectangle(color.Transparent),
		icon:   &canvas.Image{FillMode: canvas.ImageFillContain},
		label:  canvas.NewText("", color.Black),
	}
	r.label.Alignment = bellui.TextAlignCenter
	r.objects = []core.CanvasObject{r.bg, r.icon, r.label}
	r.Refresh()
	return r
}

type swipeActionButtonRenderer struct {
	button  *swipeActionButton
	bg      *canvas.Rectangle
	icon    *canvas.Image
	label   *canvas.Text
	objects []core.CanvasObject
}

// swipeActionColors returns the colour of an action's button and of what is
// drawn on it, by its importance.
func swipeActionColors(i widget.Importance) (bg, fg bellui.ThemeColorName) {
	switch i {
	case widget.DangerImportance:
		return theme.ColorNameError, theme.ColorNameForegroundOnError
	case widget.WarningImportance:
		return theme.ColorNameWarning, theme.ColorNameForegroundOnWarning
	case widget.SuccessImportance:
		return theme.ColorNameSuccess, theme.ColorNameForegroundOnSuccess
	case widget.HighImportance:
		return theme.ColorNamePrimary, theme.ColorNameForegroundOnPrimary
	}
	return theme.ColorNameButton, theme.ColorNameForeground
}

func (r *swipeActionButtonRenderer) Refresh() {
	b := r.button
	bgName, fgName := swipeActionColors(b.action.Importance)
	r.bg.FillColor = theme.ColorForWidget(bgName, b)
	r.label.Text = b.action.Label
	r.label.Color = theme.ColorForWidget(fgName, b)
	r.label.TextSize = b.grid.swipeActionTextSize()
	if b.action.Icon != nil {
		r.icon.Resource = theme.NewColoredResource(b.action.Icon, fgName)
		r.icon.Show()
	} else {
		r.icon.Resource = nil
		r.icon.Hide()
	}
	r.Layout(b.Size())
	r.bg.Refresh()
	r.icon.Refresh()
	r.label.Refresh()
}

// Layout centres the icon above the label in the button.
func (r *swipeActionButtonRenderer) Layout(size geo.Size) {
	r.bg.Move(geo.NewPos(0, 0))
	r.bg.Resize(size)
	textH := r.label.MinSize().Height
	iconS := float32(0)
	if r.icon.Visible() {
		iconS = theme.IconInlineSize()
	}
	gap := float32(0)
	if iconS > 0 && r.label.Text != "" {
		gap = theme.InnerPadding() / 2
	}
	top := (size.Height - iconS - gap - textH) / 2
	r.icon.Move(geo.NewPos((size.Width-iconS)/2, top))
	r.icon.Resize(geo.NewSize(iconS, iconS))
	r.label.Move(geo.NewPos(0, top+iconS+gap))
	r.label.Resize(geo.NewSize(size.Width, textH))
}

func (r *swipeActionButtonRenderer) MinSize() geo.Size { return geo.NewSize(0, 0) }

func (r *swipeActionButtonRenderer) Objects() []core.CanvasObject { return r.objects }

func (r *swipeActionButtonRenderer) Destroy() {}
