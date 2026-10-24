package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/layoutgen"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

// ScrollDirection represents the directions in which a Scroll can scroll its child content.
type ScrollDirection = bellui.ScrollDirection

// Constants for valid values of ScrollDirection.
const (
	// ScrollBoth supports horizontal and vertical scrolling.
	ScrollBoth ScrollDirection = iota
	// ScrollHorizontalOnly specifies the scrolling should only happen left to right.
	ScrollHorizontalOnly
	// ScrollVerticalOnly specifies the scrolling should only happen top to bottom.
	ScrollVerticalOnly
	// ScrollNone turns off scrolling for this container.
	//
	// Since: 1.0
	ScrollNone
)

type scrollBarOrientation int

// We default to vertical as 0 due to that being the original orientation offered
const (
	scrollBarOrientationVertical   scrollBarOrientation = 0
	scrollBarOrientationHorizontal scrollBarOrientation = 1
	scrollContainerMinSize                              = float32(32) // TODO consider the smallest useful scroll view?
)

type scrollBarRenderer struct {
	BaseRenderer
	scrollBar  *scrollBar
	background *canvas.Rectangle
	minSize    geo.Size
}

func (r *scrollBarRenderer) Layout(size geo.Size) {
	if !r.scrollBar.area.expanded() {
		// The bar widget keeps the full (wide) size so a finger or pointer
		// can grab anywhere across it, but until hovered (never, on touch)
		// only a thin visual strip is drawn flush to the trailing edge. This
		// decouples the grabbable hit area from the drawn bar, so the wide
		// target neither obscures the content's edge nor needs a pixel-exact
		// approach flight to hit (pre-rebrand tracker #17, pre-rebrand tracker #449).
		thin := theme.CurrentForWidget(r.scrollBar).Size(theme.SizeNameScrollBarSmall)
		switch r.scrollBar.orientation {
		case scrollBarOrientationHorizontal:
			r.background.Move(geo.NewPos(0, size.Height-thin))
			r.background.Resize(geo.NewSize(size.Width, thin))
		default:
			r.background.Move(geo.NewPos(size.Width-thin, 0))
			r.background.Resize(geo.NewSize(thin, size.Height))
		}
		return
	}
	r.background.Move(geo.NewPos(0, 0))
	r.background.Resize(size)
}

func (r *scrollBarRenderer) MinSize() geo.Size {
	return r.minSize
}

func (r *scrollBarRenderer) Refresh() {
	th := theme.CurrentForWidget(r.scrollBar)
	v := bellui.CurrentApp().Settings().ThemeVariant()

	r.background.FillColor = th.Color(theme.ColorNameScrollBar, v)
	r.background.CornerRadius = th.Size(theme.SizeNameScrollBarRadius)
	// The bar widget size no longer changes on hover (the hit area is
	// permanently wide), so re-run the layout here to toggle between the
	// thin idle visual and the full-width hovered one (pre-rebrand tracker #449).
	r.Layout(r.scrollBar.Size())
	r.background.Refresh()
}

var (
	_ event.Hoverable = (*scrollBar)(nil)
	_ event.Draggable = (*scrollBar)(nil)
)

type scrollBar struct {
	Base
	area            *scrollBarArea
	draggedDistance float32
	dragStart       float32
	orientation     scrollBarOrientation
}

func (b *scrollBar) CreateRenderer() core.WidgetRenderer {
	th := theme.CurrentForWidget(b)
	v := bellui.CurrentApp().Settings().ThemeVariant()

	background := canvas.NewRectangle(th.Color(theme.ColorNameScrollBar, v))
	background.CornerRadius = th.Size(theme.SizeNameScrollBarRadius)
	r := &scrollBarRenderer{
		scrollBar:  b,
		background: background,
	}
	r.SetObjects([]core.CanvasObject{background})
	return r
}

func (b *scrollBar) Cursor() desktop.Cursor {
	return desktop.DefaultCursor
}

func (b *scrollBar) DragEnd() {
	b.area.isDragging = false

	if bellui.CurrentDevice().IsMobile() {
		b.area.HoverOut()
		return
	}
	b.area.Refresh()
}

func (b *scrollBar) Dragged(e *event.DragEvent) {
	if !b.area.isDragging {
		b.area.isDragging = true
		b.area.HoverIn(nil)

		switch b.orientation {
		case scrollBarOrientationHorizontal:
			b.dragStart = b.Position().X
		case scrollBarOrientationVertical:
			b.dragStart = b.Position().Y
		}
		b.draggedDistance = 0
	}

	switch b.orientation {
	case scrollBarOrientationHorizontal:
		b.draggedDistance += e.Dragged.DX
	case scrollBarOrientationVertical:
		b.draggedDistance += e.Dragged.DY
	}
	b.area.moveBar(b.draggedDistance+b.dragStart, b.Size())
}

func (b *scrollBar) HoverIn(e *event.HoverEvent) {
	b.area.HoverIn(e)
}

func (b *scrollBar) HoverMoved(*event.HoverEvent) {
}

func (b *scrollBar) HoverOut() {
	b.area.HoverOut()
}

func newScrollBar(area *scrollBarArea) *scrollBar {
	b := &scrollBar{area: area, orientation: area.orientation}
	b.ExtendBaseWidget(b)
	return b
}

// expanded reports whether the bar should draw at its full width: a pointer
// hover or an active drag. Touch devices have no hover, so their bar visual
// stays thin permanently.
//
// The *hit area* is a separate concern and is always full width on every
// device: a small (~3px) target is nearly impossible to hit with a finger
// (pre-rebrand tracker #17) and needs a pixel-exact approach with a mouse — wasm-in-browser
// especially, where the idle grab strip was ~6px before hover could grow it
// (pre-rebrand tracker #449). The renderers draw the thin idle visual inside the wide widget, so
// the wide target no longer obscures the content's edge.
func (a *scrollBarArea) expanded() bool {
	if bellui.CurrentDevice().IsMobile() {
		return false
	}
	return a.isMouseIn || a.isDragging
}

type scrollBarAreaRenderer struct {
	BaseRenderer
	area       *scrollBarArea
	bar        *scrollBar
	background *canvas.Rectangle
}

func (r *scrollBarAreaRenderer) Layout(size geo.Size) {
	r.layoutWithTheme(theme.CurrentForWidget(r.area), size)
}

func (r *scrollBarAreaRenderer) layoutWithTheme(th bellui.Theme, size geo.Size) {
	var barHeight, barWidth, barX, barY float32
	var bkgHeight, bkgWidth, bkgX, bkgY float32
	switch r.area.orientation {
	case scrollBarOrientationHorizontal:
		barWidth, barHeight, barX, barY = r.barSizeAndOffset(th, r.area.scroll.Offset.X, r.area.scroll.Content.Size().Width, r.area.scroll.Size().Width)
		r.area.barLeadingEdge = barX
		r.area.barTrailingEdge = barX + barWidth
		bkgWidth, bkgHeight, bkgX, bkgY = size.Width, barHeight, 0, barY
	default:
		barHeight, barWidth, barY, barX = r.barSizeAndOffset(th, r.area.scroll.Offset.Y, r.area.scroll.Content.Size().Height, r.area.scroll.Size().Height)
		r.area.barLeadingEdge = barY
		r.area.barTrailingEdge = barY + barHeight
		bkgWidth, bkgHeight, bkgX, bkgY = barWidth, size.Height, barX, 0
	}
	r.bar.Move(geo.NewPos(barX, barY))
	r.bar.Resize(geo.NewSize(barWidth, barHeight))
	r.background.Move(geo.NewPos(bkgX, bkgY))
	r.background.Resize(geo.NewSize(bkgWidth, bkgHeight))
}

func (r *scrollBarAreaRenderer) MinSize() geo.Size {
	// The area is permanently the full bar width: the hit target stays
	// grabbable without a hover dance, and layouts that reserve bar space
	// (Scroll.ReserveBarSpace) no longer re-wrap when a hover used to grow
	// the area (pre-rebrand tracker #449).
	barSize := theme.CurrentForWidget(r.area).Size(theme.SizeNameScrollBar)
	return geo.NewSquareSize(barSize)
}

func (r *scrollBarAreaRenderer) Refresh() {
	th := theme.CurrentForWidget(r.area)
	r.bar.Refresh()
	r.background.FillColor = th.Color(theme.ColorNameScrollBarBackground, bellui.CurrentApp().Settings().ThemeVariant())
	// The full-width track shows only while the bar draws expanded; over the
	// thin idle (and touch) visual it would re-obscure the content edge the
	// wide invisible hit area just freed (pre-rebrand tracker #17).
	r.background.Hidden = !r.area.expanded()
	r.layoutWithTheme(th, r.area.Size())
	canvas.Refresh(r.bar)
	canvas.Refresh(r.background)
}

func (r *scrollBarAreaRenderer) barSizeAndOffset(th bellui.Theme, contentOffset, contentLength, scrollLength float32) (length, width, lengthOffset, widthOffset float32) {
	scrollBarSize := th.Size(theme.SizeNameScrollBar)
	if scrollLength < contentLength {
		portion := scrollLength / contentLength
		length = float32(int(scrollLength)) * portion
		// Keep the thumb grabbable however long the content is: on a 50k-line
		// document the proportional length is sub-pixel, and platform
		// scrollbars all apply a floor (pre-rebrand tracker #449). Themes without the token fall
		// back to the historical floor of one bar width. The floor is capped
		// at half the track so a short scroll area always keeps room to drag.
		minThumb := th.Size(theme.SizeNameScrollBarMinThumb)
		if minThumb <= 0 {
			minThumb = scrollBarSize
		}
		length = geo.Max(length, geo.Min(minThumb, scrollLength/2))
	} else {
		length = scrollLength
	}
	if contentOffset != 0 {
		lengthOffset = (scrollLength - length) * (contentOffset / (contentLength - scrollLength))
	}
	// The bar widget is always full width — the wide hit target — while the
	// thin idle visual is drawn inside it by the bar's renderer (pre-rebrand tracker #449).
	width = scrollBarSize
	return length, width, lengthOffset, widthOffset
}

var (
	_ event.Hoverable = (*scrollBarArea)(nil)
	_ event.Tappable  = (*scrollBarArea)(nil)
	_ event.Draggable = (*scrollBarArea)(nil)
)

type scrollBarArea struct {
	Base

	isDragging  bool
	isMouseIn   bool
	scroll      *Scroll
	bar         *scrollBar
	orientation scrollBarOrientation

	// updated from renderer Layout
	// coordinates Y in vertical orientation, X in horizontal
	barLeadingEdge  float32
	barTrailingEdge float32
}

func (a *scrollBarArea) CreateRenderer() core.WidgetRenderer {
	th := theme.CurrentForWidget(a)
	v := bellui.CurrentApp().Settings().ThemeVariant()
	a.bar = newScrollBar(a)
	background := canvas.NewRectangle(th.Color(theme.ColorNameScrollBarBackground, v))
	background.Hidden = !a.expanded()
	return &scrollBarAreaRenderer{BaseRenderer: NewBaseRenderer([]core.CanvasObject{background, a.bar}), area: a, bar: a.bar, background: background}
}

// Tapped scrolls the content so the bar centers on the tap point ("jump to
// the spot that's clicked"). Earlier versions paged by nearly a viewport per
// click when the macOS "jump to the next page" preference was active; the
// jump is now unconditional on every platform — in large documents a page is
// a barely visible thumb movement, and paging to a distant position takes
// dozens of clicks.
func (a *scrollBarArea) Tapped(e *event.PointEvent) {
	barSize := a.bar.Size()
	switch a.orientation {
	case scrollBarOrientationHorizontal:
		if e.Position.X < a.barLeadingEdge || e.Position.X > a.barTrailingEdge {
			a.moveBar(geo.Max(0, e.Position.X-barSize.Width/2), barSize)
		}
	case scrollBarOrientationVertical:
		if e.Position.Y < a.barLeadingEdge || e.Position.Y > a.barTrailingEdge {
			a.moveBar(geo.Max(0, e.Position.Y-barSize.Height/2), barSize)
		}
	}
}

// Dragged makes a drag that starts on the track (not on the bar itself) warp
// the bar to be centered at the pointer and keep it tracking the pointer, the
// drag-through companion of the jump in Tapped. Drags that start on the bar
// are delivered to the bar widget instead and move it relatively.
func (a *scrollBarArea) Dragged(e *event.DragEvent) {
	if !a.isDragging {
		a.isDragging = true
		a.scroll.refreshBars()
	}

	barSize := a.bar.Size()
	switch a.orientation {
	case scrollBarOrientationHorizontal:
		a.moveBar(e.Position.X-barSize.Width/2, barSize)
	default:
		a.moveBar(e.Position.Y-barSize.Height/2, barSize)
	}
}

func (a *scrollBarArea) DragEnd() {
	a.isDragging = false

	if bellui.CurrentDevice().IsMobile() {
		a.HoverOut()
		return
	}
	a.Refresh()
}

func (a *scrollBarArea) HoverIn(*event.HoverEvent) {
	a.isMouseIn = true
	a.scroll.refreshBars()
}

func (a *scrollBarArea) HoverMoved(*event.HoverEvent) {
}

func (a *scrollBarArea) HoverOut() {
	a.isMouseIn = false
	if a.isDragging {
		return
	}

	a.scroll.refreshBars()
}

func (a *scrollBarArea) moveBar(offset float32, barSize geo.Size) {
	oldX := a.scroll.Offset.X
	oldY := a.scroll.Offset.Y
	switch a.orientation {
	case scrollBarOrientationHorizontal:
		a.scroll.Offset.X = a.computeScrollOffset(barSize.Width, offset, a.scroll.Size().Width, a.scroll.Content.Size().Width)
	default:
		a.scroll.Offset.Y = a.computeScrollOffset(barSize.Height, offset, a.scroll.Size().Height, a.scroll.Content.Size().Height)
	}
	if a.scroll.Offset.X != oldX || a.scroll.Offset.Y != oldY {
		a.scroll.notifyScrolled(a.scroll.Offset)
	}
	a.scroll.refreshWithoutOffsetUpdate()
}

func (a *scrollBarArea) computeScrollOffset(length, offset, scrollLength, contentLength float32) float32 {
	maxOffset := scrollLength - length
	if offset < 0 {
		offset = 0
	} else if offset > maxOffset {
		offset = maxOffset
	}
	ratio := offset / maxOffset
	scrollOffset := ratio * (contentLength - scrollLength)
	return scrollOffset
}

func newScrollBarArea(scroll *Scroll, orientation scrollBarOrientation) *scrollBarArea {
	a := &scrollBarArea{scroll: scroll, orientation: orientation}
	a.ExtendBaseWidget(a)
	return a
}

type scrollContainerRenderer struct {
	BaseRenderer
	scroll                  *Scroll
	vertArea                *scrollBarArea
	horizArea               *scrollBarArea
	leftShadow, rightShadow *Shadow
	topShadow, bottomShadow *Shadow
	oldMinSize              geo.Size
}

func (r *scrollContainerRenderer) layoutBars(size geo.Size) {
	scrollerSize := r.scroll.Size()
	if r.scroll.Direction == ScrollVerticalOnly || r.scroll.Direction == ScrollBoth {
		r.vertArea.Resize(geo.NewSize(r.vertArea.MinSize().Width, size.Height))
		r.vertArea.Move(geo.NewPos(scrollerSize.Width-r.vertArea.Size().Width, 0))
		r.topShadow.Resize(geo.NewSize(size.Width, 0))
		r.bottomShadow.Resize(geo.NewSize(size.Width, 0))
		r.bottomShadow.Move(geo.NewPos(0, scrollerSize.Height))
	}

	if r.scroll.Direction == ScrollHorizontalOnly || r.scroll.Direction == ScrollBoth {
		r.horizArea.Resize(geo.NewSize(size.Width, r.horizArea.MinSize().Height))
		r.horizArea.Move(geo.NewPos(0, scrollerSize.Height-r.horizArea.Size().Height))
		r.leftShadow.Resize(geo.NewSize(0, size.Height))
		r.rightShadow.Resize(geo.NewSize(0, size.Height))
		r.rightShadow.Move(geo.NewPos(scrollerSize.Width, 0))
	}

	r.scroll.pinToRTLStart()    // start RTL horizontal content at its right edge
	r.scroll.updateOffset(0, 0) // clamp offset to valid range after content resize
	r.updatePosition()
}

func (r *scrollContainerRenderer) Layout(size geo.Size) {
	c := r.scroll.Content
	contentMin := c.MinSize()
	contentSize := contentMin.Max(size)

	// On touch devices the scrollbar overlays the content's right edge (drawn
	// on top) instead of reserving layout space, so content keeps its full
	// width and text does not reflow narrower just because the bar is wide
	// enough to grab. On desktop the bar reserves space so it never overlaps
	// content.
	if r.scroll.ReserveBarSpace && !bellui.CurrentDevice().IsMobile() {
		// Reserve space for the vertical scrollbar when content overflows
		// vertically but adapts to available width (e.g., wrapped text).
		// Without this, the scrollbar renders on top of content, clipping
		// the right edge and causing text to wrap too late.
		dir := r.scroll.Direction
		if (dir == ScrollVerticalOnly || dir == ScrollBoth) &&
			contentMin.Height > size.Height && contentMin.Width <= size.Width {
			contentSize.Width = geo.Max(contentMin.Width, size.Width-r.vertArea.MinSize().Width)
		}
	}

	c.Resize(contentSize)
	r.layoutBars(size)
}

func (r *scrollContainerRenderer) MinSize() geo.Size {
	return r.scroll.MinSize()
}

func (r *scrollContainerRenderer) Refresh() {
	r.horizArea.Refresh()
	r.vertArea.Refresh()
	r.leftShadow.Refresh()
	r.topShadow.Refresh()
	r.rightShadow.Refresh()
	r.bottomShadow.Refresh()

	if len(r.BaseRenderer.Objects()) == 0 || r.BaseRenderer.Objects()[0] != r.scroll.Content {
		// push updated content object to baseRenderer
		r.BaseRenderer.Objects()[0] = r.scroll.Content
	}
	size := r.scroll.Size()
	newMin := r.scroll.Content.MinSize()
	if r.oldMinSize == newMin && r.oldMinSize == r.scroll.Content.Size() &&
		(size.Width <= r.oldMinSize.Width && size.Height <= r.oldMinSize.Height) {
		r.layoutBars(size)
		return
	}

	r.oldMinSize = newMin
	r.Layout(size)
}

func (r *scrollContainerRenderer) handleAreaVisibility(contentSize, scrollSize float32, area *scrollBarArea) {
	if contentSize <= scrollSize {
		area.Hide()
	} else if r.scroll.Visible() {
		area.Show()
	}
}

func (r *scrollContainerRenderer) handleShadowVisibility(offset, contentSize, scrollSize float32, shadowStart core.CanvasObject, shadowEnd core.CanvasObject) {
	if !r.scroll.Visible() {
		return
	}
	if r.scroll.HideShadows {
		shadowStart.Hide()
		shadowEnd.Hide()
		return
	}
	if offset > 0 {
		shadowStart.Show()
	} else {
		shadowStart.Hide()
	}
	if offset < contentSize-scrollSize {
		shadowEnd.Show()
	} else {
		shadowEnd.Hide()
	}
}

func (r *scrollContainerRenderer) updatePosition() {
	if r.scroll.Content == nil {
		return
	}
	scrollSize := r.scroll.Size()
	contentSize := r.scroll.Content.Size()

	r.scroll.Content.Move(geo.NewPos(-r.scroll.Offset.X, -r.scroll.Offset.Y))

	if r.scroll.Direction == ScrollVerticalOnly || r.scroll.Direction == ScrollBoth {
		r.handleAreaVisibility(contentSize.Height, scrollSize.Height, r.vertArea)
		r.handleShadowVisibility(r.scroll.Offset.Y, contentSize.Height, scrollSize.Height, r.topShadow, r.bottomShadow)
		cache.Renderer(r.vertArea).Layout(scrollSize)
	} else {
		r.vertArea.Hide()
		r.topShadow.Hide()
		r.bottomShadow.Hide()
	}
	if r.scroll.Direction == ScrollHorizontalOnly || r.scroll.Direction == ScrollBoth {
		r.handleAreaVisibility(contentSize.Width, scrollSize.Width, r.horizArea)
		r.handleShadowVisibility(r.scroll.Offset.X, contentSize.Width, scrollSize.Width, r.leftShadow, r.rightShadow)
		cache.Renderer(r.horizArea).Layout(scrollSize)
	} else {
		r.horizArea.Hide()
		r.leftShadow.Hide()
		r.rightShadow.Hide()
	}

	if r.scroll.Direction != ScrollHorizontalOnly {
		canvas.Refresh(r.vertArea) // this is required to force the canvas to update, we have no "Redraw()"
	} else {
		canvas.Refresh(r.horizArea) // this is required like above but if we are horizontal
	}
}

// Scroll defines a container that is smaller than the Content.
// The Offset is used to determine the position of the child widgets within the container.
type Scroll struct {
	Base
	minSize   geo.Size
	Direction ScrollDirection
	Content   core.CanvasObject
	Offset    geo.Position
	// OnScrolled can be set to be notified when the Scroll has changed position.
	// You should not update the Scroll.Offset from this method.
	//
	// Since: 1.0
	OnScrolled func(geo.Position) `json:"-"`

	// ReserveBarSpace, when true, reduces the content width (or height)
	// to prevent the scrollbar from overlapping content. This ensures that
	// text wraps correctly and visual elements are not clipped by the scrollbar.
	ReserveBarSpace bool

	// HideShadows, when true, suppresses the edge fade-shadows that are
	// otherwise shown when the content overflows. Text inputs (widget.Entry)
	// set this: a text field clips its overflowing text at the edge rather
	// than fading it, and the fade-shadow gradient otherwise dims content
	// (and, when the viewport overlaps it, the surrounding border) at the
	// scrolled edge — see pre-rebrand tracker #353 / pre-rebrand tracker #357.
	HideShadows bool

	// rtlPinned records that the initial right-edge position has been applied (or
	// superseded by an explicit scroll) under a right-to-left layout, so it is
	// done at most once and never fights a later user scroll.
	rtlPinned bool

	// scrollListeners holds the registry behind AddScrollListener, keyed by an
	// incrementing id so registrations can be removed independently of order.
	scrollListeners map[int]func(geo.Position)
	nextListenerID  int
}

// AddScrollListener registers fn to be called whenever the scroll offset
// changes, returning a function that removes the registration. Unlike the
// single-slot [Scroll.OnScrolled] field, any number of listeners can attach and
// detach independently and in any order, so overlays such as ScrollIndicator can
// observe a Scroll without clobbering each other's callbacks.
//
// Listeners run on the main goroutine, after OnScrolled. As with OnScrolled, do
// not change Scroll.Offset from a listener.
func (s *Scroll) AddScrollListener(fn func(geo.Position)) (remove func()) {
	if fn == nil {
		return func() {}
	}
	if s.scrollListeners == nil {
		s.scrollListeners = make(map[int]func(geo.Position))
	}
	id := s.nextListenerID
	s.nextListenerID++
	s.scrollListeners[id] = fn
	return func() { delete(s.scrollListeners, id) }
}

// notifyScrolled dispatches an offset change to OnScrolled and every registered
// scroll listener. Callers invoke it only when the offset has actually moved.
func (s *Scroll) notifyScrolled(offset geo.Position) {
	if s.OnScrolled != nil {
		s.OnScrolled(offset)
	}
	for _, fn := range s.scrollListeners {
		fn(offset)
	}
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer
func (s *Scroll) CreateRenderer() core.WidgetRenderer {
	scr := &scrollContainerRenderer{
		BaseRenderer: NewBaseRenderer([]core.CanvasObject{s.Content}),
		scroll:       s,
	}
	scr.vertArea = newScrollBarArea(s, scrollBarOrientationVertical)
	scr.topShadow = NewShadow(ShadowBottom, SubmergedContentLevel)
	scr.bottomShadow = NewShadow(ShadowTop, SubmergedContentLevel)
	scr.horizArea = newScrollBarArea(s, scrollBarOrientationHorizontal)
	scr.leftShadow = NewShadow(ShadowRight, SubmergedContentLevel)
	scr.rightShadow = NewShadow(ShadowLeft, SubmergedContentLevel)
	scr.SetObjects(append(scr.Objects(), scr.topShadow, scr.bottomShadow, scr.leftShadow, scr.rightShadow,
		scr.vertArea, scr.horizArea))
	scr.updatePosition()

	return scr
}

// ScrollToBottom will scroll content to container bottom - to show latest info which end user just added.
// Like every programmatic scroll it reports the move through OnScrolled.
func (s *Scroll) ScrollToBottom() {
	// Past the end; moveTo clamps it to the true bottom. Only a real move
	// supersedes the RTL start pin: before the first layout this clamps to
	// nothing, and the horizontal offset is not the caller's to choose here.
	if s.moveTo(geo.NewPos(s.Offset.X, s.Content.MinSize().Height)) {
		s.rtlPinned = true
	}
}

// ScrollToTop will scroll content to container top.
// Like every programmatic scroll it reports the move through OnScrolled.
func (s *Scroll) ScrollToTop() {
	s.ScrollToOffset(geo.Position{})
}

// MinSize returns the smallest size this widget can shrink to
func (s *Scroll) MinSize() geo.Size {
	min := geo.NewSize(scrollContainerMinSize, scrollContainerMinSize).Max(s.minSize)
	switch s.Direction {
	case ScrollHorizontalOnly:
		min.Height = geo.Max(min.Height, s.Content.MinSize().Height)
	case ScrollVerticalOnly:
		min.Width = geo.Max(min.Width, s.Content.MinSize().Width)
	case ScrollNone:
		return s.Content.MinSize()
	}
	return min
}

// SetMinSize specifies a minimum size for this scroll container.
// If the specified size is larger than the content size then scrolling will not be enabled
// This can be helpful to appear larger than default if the layout is collapsing this widget.
func (s *Scroll) SetMinSize(size geo.Size) {
	if s.minSize != size {
		layoutgen.Changed()
	}
	s.minSize = size
}

// Refresh causes this widget to be redrawn in it's current state
func (s *Scroll) Refresh() {
	s.refreshBars()

	if s.Content != nil {
		s.Content.Refresh()
	}
}

// Resize is called when this scroller should change size. We refresh to ensure the scroll bars are updated.
func (s *Scroll) Resize(sz geo.Size) {
	if sz == s.Size() {
		return
	}

	s.Base.Resize(sz)
	s.refreshBars()
}

// ScrollToOffset will update the location of the content of this scroll container.
// The offset is clamped to the content, and a resulting move is reported
// through OnScrolled and the scroll listeners, as a user scroll is.
//
// Since: 1.0
func (s *Scroll) ScrollToOffset(p geo.Position) {
	if s.Offset == p {
		return
	}

	s.rtlPinned = true // explicit positioning supersedes the initial right-edge pin
	s.moveTo(p)
}

// moveTo sets the offset to p, clamped to the content, refreshes, and reports
// a resulting move. It returns whether the offset changed.
func (s *Scroll) moveTo(p geo.Position) bool {
	old := s.Offset
	s.Offset = p
	s.clampOffset(0, 0)
	s.refreshWithoutOffsetUpdate()
	if s.Offset == old {
		return false
	}
	s.notifyScrolled(s.Offset)
	return true
}

func (s *Scroll) refreshWithoutOffsetUpdate() {
	s.Base.Refresh()
}

// Scrolled is called when an input device triggers a scroll event
func (s *Scroll) Scrolled(ev *event.ScrollEvent) {
	if s.Direction != ScrollNone {
		s.applyScroll(s.eventScrollDelta(ev))
	}
}

// eventScrollDelta resolves a ScrollEvent's delta for this scroller. Pointer
// scrolls (wheel, touchpad) get the pointer conventions — RTL flips horizontal
// input, and vertical input redirects to horizontal-only content — while a
// direct-touch scroll (#291) is direct manipulation: the content tracks the
// finger, so the raw deltas apply in both cases, exactly as on the mobile
// driver's drag path.
func (s *Scroll) eventScrollDelta(ev *event.ScrollEvent) (float32, float32) {
	if ev.Source == event.ScrollSourceTouch {
		return ev.Scrolled.DX, ev.Scrolled.DY
	}
	return s.resolveScrollDelta(ev.Scrolled.DX, ev.Scrolled.DY)
}

// TryScrolled handles a scroll event and returns true if the event was consumed
// (i.e., the content actually scrolled). Returns false if the scroll container
// is at its boundary in the scroll direction or if the content fits entirely,
// allowing the event to propagate to a parent scrollable.
func (s *Scroll) TryScrolled(ev *event.ScrollEvent) bool {
	if s.Direction == ScrollNone {
		return false
	}
	if s.Content == nil {
		return false
	}

	size := s.Size()
	contentSize := s.Content.Size()
	if contentSize.Width <= size.Width && contentSize.Height <= size.Height {
		return false
	}

	dx, dy := s.eventScrollDelta(ev)
	min := s.Content.MinSize()

	newX := computeOffset(s.Offset.X, -dx, size.Width, min.Width)
	newY := computeOffset(s.Offset.Y, -dy, size.Height, min.Height)

	if newX == s.Offset.X && newY == s.Offset.Y {
		return false // at boundary, allow propagation
	}

	s.applyScroll(dx, dy)
	return true
}

func (s *Scroll) refreshBars() {
	s.updateOffset(0, 0)
	s.refreshWithoutOffsetUpdate()
}

// pinToRTLStart positions horizontal content at its right edge the first time it
// overflows under a right-to-left layout, so a freshly shown scroller opens at
// the content's beginning (the right) rather than its left. It runs at most
// once; afterwards the user's (or an explicit) scroll position is preserved.
// Under the default left-to-right layout it is a no-op.
func (s *Scroll) pinToRTLStart() {
	if s.rtlPinned || s.Content == nil {
		return
	}
	if s.Direction != ScrollHorizontalOnly && s.Direction != ScrollBoth {
		return
	}
	// The ambient direction, not a lookup: this runs on every layout of an
	// unpinned horizontal scroller (#696). A layout walk pushes it, and a
	// Refresh or Resize outside one runs in the scroller's own direction
	// (#749).
	if !lang.IsRTLDirection() {
		return
	}
	if s.Content.MinSize().Width <= s.Size().Width {
		return // not yet overflowing; revisit on a later layout
	}
	// A value past the maximum; updateOffset clamps it to the true right edge.
	s.Offset.X = s.Content.MinSize().Width
	s.rtlPinned = true
}

// resolveScrollDelta maps a raw input delta to a content-space delta. It
// redirects vertical wheel input to the horizontal axis for narrow content
// (swapAxesIfNeeded) and, under a right-to-left layout, reverses the horizontal
// component so that advancing through the content — the natural forward scroll —
// moves toward the content's left edge.
func (s *Scroll) resolveScrollDelta(dx, dy float32) (float32, float32) {
	dx, dy = s.swapAxesIfNeeded(dx, dy)
	// An input event, so no layout walk is running: look the direction up
	// (#696), but only when there is a horizontal delta to flip — vertical
	// wheel scrolling is the hot path and must not walk the tree.
	if dx != 0 && driver.IsRTLAt(s.super()) {
		dx = -dx
	}
	return dx, dy
}

// swapAxesIfNeeded swaps dx/dy when content is wider than the container but
// fits vertically and the user scrolls vertically (dx==0). This redirects
// vertical scroll input to horizontal scrolling for narrow horizontal content.
func (s *Scroll) swapAxesIfNeeded(dx, dy float32) (float32, float32) {
	min := s.Content.MinSize()
	size := s.Size()
	if size.Width < min.Width && size.Height >= min.Height && dx == 0 {
		return dy, dx
	}
	return dx, dy
}

// applyScroll moves the offset by an already-resolved delta.
func (s *Scroll) applyScroll(dx, dy float32) {
	if s.updateOffset(dx, dy) {
		s.rtlPinned = true // a real scroll supersedes the initial right-edge pin
		s.refreshWithoutOffsetUpdate()
	}
}

// updateOffset moves the offset by a delta, clamped to the content, and
// reports a resulting move.
func (s *Scroll) updateOffset(deltaX, deltaY float32) bool {
	moved := s.clampOffset(deltaX, deltaY)
	if moved {
		s.notifyScrolled(s.Offset)
	}
	return moved
}

// clampOffset moves the offset by a delta, clamped to the content, without
// reporting it. It returns whether the offset changed.
func (s *Scroll) clampOffset(deltaX, deltaY float32) bool {
	size := s.Size()
	contentSize := s.Content.Size()
	if contentSize.Width <= size.Width && contentSize.Height <= size.Height {
		if s.Offset.X != 0 || s.Offset.Y != 0 {
			s.Offset.X = 0
			s.Offset.Y = 0
			return true
		}
		return false
	}
	oldX := s.Offset.X
	oldY := s.Offset.Y
	min := s.Content.MinSize()
	s.Offset.X = computeOffset(s.Offset.X, -deltaX, size.Width, min.Width)
	s.Offset.Y = computeOffset(s.Offset.Y, -deltaY, size.Height, min.Height)

	return s.Offset.X != oldX || s.Offset.Y != oldY
}

func computeOffset(start, delta, outerWidth, innerWidth float32) float32 {
	offset := start + delta
	if offset+outerWidth >= innerWidth {
		offset = innerWidth - outerWidth
	}

	return geo.Max(offset, 0)
}

// NewScroll creates a scrollable parent wrapping the specified content.
// Note that this may cause the MinSize to be smaller than that of the passed object.
func NewScroll(content core.CanvasObject) *Scroll {
	s := newScrollContainerWithDirection(ScrollBoth, content)
	s.ExtendBaseWidget(s)
	return s
}

// NewHScroll create a scrollable parent wrapping the specified content.
// Note that this may cause the MinSize.Width to be smaller than that of the passed object.
func NewHScroll(content core.CanvasObject) *Scroll {
	s := newScrollContainerWithDirection(ScrollHorizontalOnly, content)
	s.ExtendBaseWidget(s)
	return s
}

// NewVScroll create a scrollable parent wrapping the specified content.
// Note that this may cause the MinSize.Height to be smaller than that of the passed object.
func NewVScroll(content core.CanvasObject) *Scroll {
	s := newScrollContainerWithDirection(ScrollVerticalOnly, content)
	s.ExtendBaseWidget(s)
	return s
}

func newScrollContainerWithDirection(direction ScrollDirection, content core.CanvasObject) *Scroll {
	s := &Scroll{
		Direction:       direction,
		Content:         content,
		ReserveBarSpace: true,
	}
	s.ExtendBaseWidget(s)
	return s
}
