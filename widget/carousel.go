package widget

import (
	"math"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

var _ event.Draggable = (*Carousel)(nil)

// Carousel is a pager: it shows one item at a time, and the user moves to the
// neighbouring item with its previous/next buttons or a horizontal swipe.
//
// During a horizontal drag the current item follows the finger and the
// neighbour on the side being revealed slides in beside it. On release the
// content animates, with the MD3 standard easing and medium duration, to the
// neighbouring item when the swipe was longer than a fifth of the width or was a
// flick of at least 400 points per second (and 24 points long), and back to the
// current item otherwise; a flick back towards the current item returns to it
// however far the content was moved. With animations turned off in the app settings
// it snaps instantly. Past the first or last item the drag meets resistance and
// always snaps back. Under a right-to-left layout the order, buttons and swipe
// direction are mirrored.
//
// It does not implement MD3's carousel layouts (multi-browse, hero,
// uncontained, keylines); see #744.
type Carousel struct {
	BaseWidget
	Items       []core.CanvasObject
	CurrentItem int
	OnChanged   func(int) `json:"-"`

	dragging bool
	axis     carouselAxis // the current gesture's locked axis
	dragBase float32      // finger travel, before edge resistance, the offset started from
	drag     geo.Delta    // accumulated over the current drag gesture
	samples  []carouselSample
	lastPos  geo.Position     // Position of the gesture's latest finger event
	lastEv   *event.DragEvent // the gesture's latest finger event
	lastStep geo.Delta        // the drag lastEv added
	coasting bool             // a driver is coasting a gesture this carousel already released
	coastPos geo.Position     // the coast's (release) position
	// coastEnd is set while the DragEnd of a coast this carousel already took
	// as the release is still to come, so that DragEnd does not end a later
	// gesture.
	coastEnd bool

	// offset is how far, in points and in screen terms (positive is to the
	// right), the content sits from its resting place on CurrentItem.
	offset   float32
	settling *bellui.Animation
	rtl      bool // see isRTL
}

type carouselAxis uint8

const (
	carouselAxisNone carouselAxis = iota
	carouselAxisHorizontal
	carouselAxisVertical
)

type carouselSample struct {
	at time.Time
	x  float32 // accumulated horizontal drag at that time
}

// NewCarousel creates a carousel with the given items.
func NewCarousel(items ...core.CanvasObject) *Carousel {
	c := &Carousel{Items: items}
	c.ExtendBaseWidget(c)
	return c
}

// AccessibleRole returns [core.RoleGroup]; the navigation buttons and item
// content inside provide their own semantics.
func (c *Carousel) AccessibleRole() core.AccessibleRole { return core.RoleGroup }

// AccessibleLabel returns an empty string; use SetAccessibilityLabel to
// name the pager.
func (c *Carousel) AccessibleLabel() string { return "" }

// Next advances to the next item.
func (c *Carousel) Next() { c.page(1) }

// Previous goes to the previous item.
func (c *Carousel) Previous() { c.page(-1) }

// page moves CurrentItem by one. While the content is in motion (a release or
// an earlier tap still settling) the offset is rebased onto the new item, so the
// settle animation retargets from where the content is rather than jumping.
func (c *Carousel) page(by int) {
	to := c.CurrentItem + by
	if to < 0 || to >= len(c.Items) {
		return
	}
	c.CurrentItem = to
	if shift := float32(by) * c.dirSign() * c.pitch(); c.dragging && c.axis == carouselAxisHorizontal {
		c.offset += shift // a tap during a drag: keep the finger's content in place
		c.dragBase = c.unresisted(c.offset) - c.drag.DX
	} else if c.offset != 0 {
		c.offset += shift
		c.settle()
	}
	if c.OnChanged != nil {
		c.OnChanged(c.CurrentItem)
	}
	c.Refresh()
}

const (
	// minSwipe is the shortest horizontal swipe, in points, that pages the
	// carousel however narrow it is, flick or not. It matches the 25dp
	// minimum fling distance of Android's ViewPager.
	minSwipe = 24

	// carouselFlickVelocity is the release speed, in points per second, from
	// which a swipe pages even when it is shorter than a fifth of the width.
	// 400 is the pager convention on Android: ViewPager's MIN_FLING_VELOCITY
	// and Compose's pager snap threshold (MinFlingVelocityDp) are both
	// 400dp/s, and a point here is a dp there.
	carouselFlickVelocity = 400

	// carouselVelocityWindow is how far back from the release the drag
	// samples that measure the flick speed reach, and carouselStoppedAfter is
	// how long without movement before the release counts as a stop (zero
	// speed). Both follow Android's VelocityTracker (100ms horizon,
	// 40ms ASSUME_POINTER_STOPPED_TIME).
	carouselVelocityWindow = 100 * time.Millisecond
	carouselStoppedAfter   = 40 * time.Millisecond

	// carouselEdgeResistance divides the drag past the first or last item.
	carouselEdgeResistance = 3
)

// carouselNow is the clock the flick speed is measured with; tests replace it.
// Events are stamped when handled, as event.DragEvent carries no time (#752).
var carouselNow = time.Now

// isRTL reports whether the carousel is laid out right to left. Every
// direction read in this file goes through it. It returns the direction cached
// by the last layout (which runs in the carousel's own direction, #696) or
// looked up at the start of the current gesture, so the per-frame paths — drag
// tracking and the settle animation — never walk the tree for it.
func (c *Carousel) isRTL() bool { return c.rtl }

// dirSign is +1 when later items sit to the right of the current one, -1 under RTL.
func (c *Carousel) dirSign() float32 {
	if c.isRTL() {
		return -1
	}
	return 1
}

// Dragged moves the content with the finger once the gesture has locked to the
// horizontal axis; a gesture that starts vertical is ignored.
func (c *Carousel) Dragged(e *event.DragEvent) {
	if c.coasting {
		if e.Position == c.coastPos {
			return // the driver's coast after a release this carousel already settled
		}
		c.coasting = false // a new gesture (a touch stops the coast first, #751)
	}
	if c.dragging && c.axis == carouselAxisHorizontal {
		// The finger has lifted and the mobile and embedded drivers are
		// coasting the gesture: common.DragCoast sends the release event again
		// with decaying deltas, at the release position, before DragEnd
		// (#751). Treat the coast as the release, so the flick speed is the
		// finger's and the content settles now instead of gliding by the
		// coast's distance. The coast shows as a drag without the pointer
		// moving. A driver that re-sends one event object (as the drivers did
		// before #756) also shows it as a repeated event: then the first step
		// was taken for a finger move, since the release position can differ
		// from the last move's, and is undone. DragCoast copies each step, so
		// with it that first step stays applied (one decayed delta).
		repeat := e == c.lastEv
		if repeat || (e.Position == c.lastPos && !e.Dragged.IsZero()) {
			if repeat {
				c.undoLastStep()
			}
			c.coasting = true
			c.coastPos = e.Position
			c.coastEnd = true
			c.release()
			return
		}
	}
	c.lastPos = e.Position
	c.lastEv = e
	c.lastStep = e.Dragged
	if !c.dragging {
		c.dragging = true
		c.rtl = isRTLAt(c.super()) // once per gesture: a DirectionScope may hold it (#696)
		c.axis = carouselAxisNone
		c.drag = geo.Delta{}
		c.samples = c.samples[:0]
	}
	c.drag.DX += e.Dragged.DX
	c.drag.DY += e.Dragged.DY
	if c.axis == carouselAxisNone {
		dx, dy := abs32(c.drag.DX), abs32(c.drag.DY)
		switch {
		case dx > dy:
			c.axis = carouselAxisHorizontal
			c.stopSettle()
			c.dragBase = c.unresisted(c.offset)
		case dy > dx:
			c.axis = carouselAxisVertical
		}
	}
	if c.axis != carouselAxisHorizontal {
		return
	}

	now := carouselNow()
	c.samples = append(c.samples, carouselSample{at: now, x: c.drag.DX})
	for len(c.samples) > 2 && now.Sub(c.samples[0].at) > carouselVelocityWindow {
		c.samples = c.samples[1:]
	}

	c.followFinger()
}

// followFinger places the content at the gesture's finger travel, with the
// edge resistance applied.
func (c *Carousel) followFinger() {
	pitch := c.pitch()
	c.offset = geo.Max(-pitch, geo.Min(pitch, c.resisted(c.dragBase+c.drag.DX)))
	c.relayoutTrack()
}

// undoLastStep takes back the latest finger event of a horizontal gesture: its
// drag, its speed sample and the content's move.
func (c *Carousel) undoLastStep() {
	c.drag.DX -= c.lastStep.DX
	c.drag.DY -= c.lastStep.DY
	if n := len(c.samples); n > 0 {
		c.samples = c.samples[:n-1]
	}
	c.followFinger()
}

// resisted is the offset a finger travel of raw moves the content to: past the
// first or last item it moves a carouselEdgeResistance-th as far.
func (c *Carousel) resisted(raw float32) float32 {
	if !c.hasItemRevealedBy(raw) {
		return raw / carouselEdgeResistance
	}
	return raw
}

// unresisted is the inverse of resisted: the finger travel that puts the
// content at offset off. A drag that catches content in motion starts from it,
// so a snap-back caught past the edge does not jump.
func (c *Carousel) unresisted(off float32) float32 {
	if !c.hasItemRevealedBy(off) {
		return off * carouselEdgeResistance
	}
	return off
}

// DragEnd settles a horizontal gesture. A flick (at least minSwipe long) pages
// towards the side the content was moved to reveal when it was flicked that
// way, and animates back to the current item when it was flicked back. A
// release that is not a flick pages towards that side when the content sits
// more than a fifth of the width from rest, and otherwise animates back.
func (c *Carousel) DragEnd() {
	if c.coastEnd {
		// The end of a coast whose release was already handled: its natural
		// end, or DragCoast.Stop when the next touch goes down (#756).
		c.coastEnd = false
		c.coasting = false
		return
	}
	if !c.dragging {
		return
	}
	c.release()
}

func (c *Carousel) release() {
	horizontal := c.axis == carouselAxisHorizontal
	v := c.releaseVelocity()
	c.dragging = false
	c.axis = carouselAxisNone
	c.samples = c.samples[:0]
	if !horizontal {
		return
	}

	off := c.offset
	var paging bool
	if abs32(c.drag.DX) >= minSwipe && abs32(v) >= carouselFlickVelocity {
		// A flick goes the way it was flicked: on to the side the content
		// was moved to reveal, or back to the current item. The offset can
		// point the other way from the finger's travel when the drag caught
		// content still settling.
		paging = off != 0 && (v < 0) == (off < 0)
	} else {
		dist := abs32(off)
		paging = dist >= minSwipe && dist > c.Size().Width/5
	}
	if !paging || !c.hasItemRevealedBy(off) {
		c.settle()
		return
	}
	forward := off < 0 // the content moves left to reveal the item on the right
	if c.isRTL() {
		forward = !forward
	}
	if forward {
		c.Next()
	} else {
		c.Previous()
	}
}

// releaseVelocity is the horizontal drag speed, in points per second, over the
// last carouselVelocityWindow before now; zero when the finger had stopped.
func (c *Carousel) releaseVelocity() float32 {
	n := len(c.samples)
	if n < 2 {
		return 0
	}
	now := carouselNow()
	last := c.samples[n-1]
	if now.Sub(last.at) > carouselStoppedAfter {
		return 0
	}
	first := c.samples[0]
	for _, s := range c.samples[:n-1] {
		if now.Sub(s.at) <= carouselVelocityWindow {
			first = s
			break
		}
	}
	dt := last.at.Sub(first.at).Seconds()
	if dt <= 0 {
		return 0
	}
	return (last.x - first.x) / float32(dt)
}

// hasItemRevealedBy reports whether an item exists on the side the content
// uncovers when it sits at offset off.
func (c *Carousel) hasItemRevealedBy(off float32) bool {
	if off == 0 {
		return true
	}
	forward := off < 0
	if c.isRTL() {
		forward = !forward
	}
	if forward {
		return c.CurrentItem < len(c.Items)-1
	}
	return c.CurrentItem > 0
}

// pitch is the distance between neighbouring items' origins.
func (c *Carousel) pitch() float32 {
	return geo.Max(0, c.Size().Width-carouselControlsWidth) + c.Theme().Size(theme.SizeNamePadding)
}

// settle animates the offset back to zero, the resting place on CurrentItem,
// or snaps there when animations are off.
func (c *Carousel) settle() {
	c.stopSettle()
	if c.offset == 0 || !bellui.CurrentApp().Settings().ShowAnimations() {
		c.offset = 0
		c.relayoutTrack()
		return
	}
	from := c.offset
	var a *bellui.Animation
	a = bellui.NewAnimation(canvas.DurationMedium2, func(done float32) {
		if c.settling != a {
			return // superseded by a drag or a retarget
		}
		if done >= 1 {
			c.offset = 0
			c.settling = nil
		} else {
			c.offset = from * (1 - done)
		}
		c.relayoutTrack()
	})
	a.Curve = bellui.AnimationEaseStandard
	c.settling = a
	a.Start()
}

func (c *Carousel) stopSettle() {
	if a := c.settling; a != nil {
		c.settling = nil
		a.Stop()
	}
}

// relayoutTrack repositions the items for the current offset without
// refreshing the controls.
func (c *Carousel) relayoutTrack() {
	// A settle can outlive the renderer (the carousel removed or its renderer
	// expired); do not build one for it.
	cr, ok := cache.CachedRenderer(c.super())
	if !ok {
		return
	}
	r, ok := cr.(*carouselRenderer)
	if !ok {
		return
	}
	r.layoutTrack()
	canvas.Refresh(r.track)
}

func abs32(f float32) float32 { return float32(math.Abs(float64(f))) }

func (c *Carousel) MinSize() geo.Size {
	c.ExtendBaseWidget(c)
	return c.BaseWidget.MinSize()
}

// carouselControlsWidth is the horizontal space the navigation buttons take,
// split evenly between the two sides of the item track.
const carouselControlsWidth = 80

func (c *Carousel) CreateRenderer() core.WidgetRenderer {
	c.ExtendBaseWidget(c)
	// The items sit on a clipped track so a neighbour sliding in during a
	// drag is cut at the track's edge rather than drawn over the buttons.
	track := &bellui.Container{Objects: c.Items, ClipToBounds: true}
	objects := []core.CanvasObject{track}

	dots := newCarouselDots(len(c.Items))
	for _, dot := range dots {
		objects = append(objects, dot)
	}

	// Navigation buttons.
	prevBtn := NewIconButton(theme.NavigateBackIcon(), func() { c.Previous() })
	prevBtn.Style = IconButtonStyleFilledTonal
	prevBtn.SetAccessibilityLabel(lang.L("Previous"))
	nextBtn := NewIconButton(theme.NavigateNextIcon(), func() { c.Next() })
	nextBtn.Style = IconButtonStyleFilledTonal
	nextBtn.SetAccessibilityLabel(lang.L("Next"))
	objects = append(objects, prevBtn, nextBtn)

	r := &carouselRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer(objects),
		track:        track,
		dots:         dots,
		prevBtn:      prevBtn,
		nextBtn:      nextBtn,
		carousel:     c,
	}
	r.Refresh()
	return r
}

type carouselRenderer struct {
	internalWidget.BaseRenderer
	track    *bellui.Container
	dots     []*canvas.Circle
	prevBtn  *IconButton
	nextBtn  *IconButton
	carousel *Carousel
}

func (r *carouselRenderer) MinSize() geo.Size {
	if len(r.carousel.Items) == 0 {
		return geo.NewSize(200, 150)
	}
	max := r.carousel.Items[0].MinSize()
	for _, item := range r.carousel.Items[1:] {
		min := item.MinSize()
		if min.Width > max.Width {
			max.Width = min.Width
		}
		if min.Height > max.Height {
			max.Height = min.Height
		}
	}
	return geo.NewSize(max.Width+carouselControlsWidth, max.Height+32) // nav buttons + dots
}

func (r *carouselRenderer) Layout(size geo.Size) {
	// Layout runs in the carousel's direction (a layout walk, or Refresh's
	// inDirectionOf), so the ambient direction is the right one to cache.
	r.carousel.rtl = lang.IsRTLDirection()
	contentH := size.Height - 32 // space for dots

	r.track.Move(geo.NewPos(carouselControlsWidth/2, 0))
	r.track.Resize(geo.NewSize(geo.Max(0, size.Width-carouselControlsWidth), contentH))
	r.layoutTrack()

	// Nav buttons.
	r.prevBtn.Resize(geo.NewSize(32, 32))
	r.prevBtn.Move(geo.NewPos(4, contentH/2-16))
	r.nextBtn.Resize(geo.NewSize(32, 32))
	r.nextBtn.Move(geo.NewPos(size.Width-36, contentH/2-16))

	// Dots.
	dotSize := float32(8)
	dotGap := float32(8)
	totalDotsW := float32(len(r.dots))*(dotSize+dotGap) - dotGap
	startX := (size.Width - totalDotsW) / 2
	for i, dot := range r.dots {
		dot.Resize(geo.NewSize(dotSize, dotSize))
		dot.Move(geo.NewPos(startX+float32(i)*(dotSize+dotGap), contentH+12))
	}

	// Under RTL reflect the controls: prev/next swap sides and the dots reverse.
	// The centred track maps onto itself; the items inside it are placed in
	// reading order by layoutTrack.
	mirrorIfRTL(r.Objects(), size.Width)
}

// layoutTrack places each item on the track, one pitch apart in reading order
// and shifted by the carousel's drag/settle offset, and shows only the items
// that overlap the track: at rest the current one, during a drag or settle
// also the neighbour sliding in.
func (r *carouselRenderer) layoutTrack() {
	c := r.carousel
	size := r.track.Size()
	pitch := c.pitch()
	sign := c.dirSign()
	for i, item := range c.Items {
		x := sign*float32(i-c.CurrentItem)*pitch + c.offset
		visible := i == c.CurrentItem // before the first layout the track has no width
		if size.Width > 0 {
			visible = x < size.Width && x+size.Width > 0
		}
		if !visible {
			item.Hide()
			continue
		}
		item.Move(geo.NewPos(x, 0))
		item.Resize(size)
		item.Show()
	}
}

// newCarouselDots makes the page indicator, one dot per item.
func newCarouselDots(n int) []*canvas.Circle {
	dots := make([]*canvas.Circle, n)
	for i := range dots {
		dots[i] = &canvas.Circle{}
	}
	return dots
}

// syncItems re-reads Carousel.Items, which the app may have replaced or
// resized since the renderer was built (#761, the #234 class): the track gets
// the live slice, the dots are rebuilt when the count changed, and CurrentItem
// is clamped into range when the slice shrank below it.
func (r *carouselRenderer) syncItems() {
	c := r.carousel
	if c.CurrentItem >= len(c.Items) {
		c.CurrentItem = len(c.Items) - 1
	}
	if c.CurrentItem < 0 {
		c.CurrentItem = 0
	}
	r.track.Objects = c.Items
	if len(r.dots) == len(c.Items) {
		return
	}
	r.dots = newCarouselDots(len(c.Items))
	objects := []core.CanvasObject{r.track}
	for _, dot := range r.dots {
		objects = append(objects, dot)
	}
	r.SetObjects(append(objects, r.prevBtn, r.nextBtn))
}

func (r *carouselRenderer) Refresh() {
	r.syncItems()
	th := r.carousel.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	for i, dot := range r.dots {
		if i == r.carousel.CurrentItem {
			dot.FillColor = th.Color(theme.ColorNamePrimary, v)
		} else {
			dot.FillColor = th.Color(theme.ColorNameOutlineVariant, v)
		}
		dot.Refresh()
	}

	// Hide prev/next at boundaries.
	if r.carousel.CurrentItem == 0 {
		r.prevBtn.Hide()
	} else {
		r.prevBtn.Show()
	}
	if r.carousel.CurrentItem >= len(r.carousel.Items)-1 {
		r.nextBtn.Hide()
	} else {
		r.nextBtn.Show()
	}

	// Outside a layout walk (Next, Previous, a swipe), so lay out in the
	// carousel's own direction, not the global one (#696).
	inDirectionOf(r.carousel.super(), func() { r.Layout(r.carousel.Size()) })
	canvas.Refresh(r.carousel.super())
}
