package widget

import (
	"math"
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/paintgen"
	internalWidget "github.com/belluikit/bellui/internal/widget"
)

// SwipeDirection is the way a swipe across a SwipeArea went: the way the
// finger moved, on screen, whatever the reading direction.
//
// Since: 1.3
type SwipeDirection string

// The four directions a SwipeArea reports.
//
// Since: 1.3
const (
	SwipeLeft  SwipeDirection = "left"
	SwipeRight SwipeDirection = "right"
	SwipeUp    SwipeDirection = "up"
	SwipeDown  SwipeDirection = "down"
)

// SwipeCardMode is what a card SwipeArea does with a swipe one way (#1103).
// The zero value, SwipeThrow, is what it does with a direction it is not
// told about.
//
// Since: 1.3
type SwipeCardMode uint8

// The ways a card SwipeArea can treat a direction.
//
// Since: 1.3
const (
	// SwipeThrow throws the card off that way on a swipe, and OnSwiped runs
	// once it is off.
	SwipeThrow SwipeCardMode = iota

	// SwipeStay lets the card follow the finger that way, with progress
	// reported as for a throw, but a swipe springs it back instead: OnSwiped
	// runs at the release, as it starts back.
	SwipeStay

	// SwipeIgnore keeps the card at rest that way: it does not move, its
	// progress stays 0, and OnSwiped does not run.
	SwipeIgnore
)

// swipeDominance is how much longer a swipe must be along its axis than
// across it, so a diagonal drag reports nothing rather than a guess.
const swipeDominance = 2

const (
	// swipeCardTilt is how far, in radians, a card dragged its own width
	// sideways tilts: 5 degrees. It tilts in proportion on the way there,
	// and no further past it.
	swipeCardTilt = 5 * math.Pi / 180

	// swipeCardShare is the share of its own width (or height) a card must
	// be dragged to count as a swipe when it is not flicked: a fifth, as on
	// a Carousel.
	swipeCardShare = 5

	// swipeCardFlyOff is how much further than its own extent a swiped card
	// travels, so that a tilted corner is clear of the area too.
	swipeCardFlyOff = 1.25
)

// swipeNow is the clock the flick speed of a card is measured with; tests
// replace it.
var swipeNow = time.Now

var (
	_ core.Widget        = (*SwipeArea)(nil)
	_ event.Draggable    = (*SwipeArea)(nil)
	_ event.Tappable     = (*SwipeArea)(nil)
	_ core.Transformable = (*swipeCard)(nil)
)

// SwipeArea holds one object and reports a swipe across it (#970): a drag
// that goes at least minSwipe, Carousel's distance for a swipe, and mostly
// along one axis. Taps go to the content.
//
// With Follow set it is a card, swiped away as a photo is in a dating app
// (#989). The content moves with the finger along the axis the drag started
// on, and a sideways drag tilts it a few degrees. On release a swipe carries
// it off the side it was thrown to, OnSwiped runs, and the content is back at
// rest, ready to show the next thing; anything else springs it back. A card
// swipe must go a fifth of the card's width (or height), or be a flick of at
// least 400 points per second, as on a Carousel, so a short drag can be
// taken back. With animations off in the app settings it moves at once.
// Directions can make a swipe some way spring the card back and only report
// it, or keep the card from moving that way at all (#1103).
//
// Since: 1.3
type SwipeArea struct {
	BaseWidget

	Content  core.CanvasObject
	OnSwiped func(SwipeDirection) `json:"-"`

	// Follow makes the content a card that follows the finger.
	//
	// Since: 1.3
	Follow bool

	// OnSwipeProgress, when Follow is set, reports how far towards a swipe
	// the card sits, from 0 at rest to 1 where a release swipes, and which
	// way. It is called as the card moves, while dragged and while it springs
	// back, and with 0 once it rests again, so an app can show a hint (such
	// as a tick on the right) as the card leaves.
	//
	// Since: 1.3
	OnSwipeProgress func(SwipeDirection, float32) `json:"-"`

	// Directions, when Follow is set, says what a swipe each way does: a
	// direction missing from it, and every direction of a nil map, throws
	// the card (SwipeThrow). Without Follow every swipe is reported.
	//
	// Since: 1.3
	Directions map[SwipeDirection]SwipeCardMode

	dx, dy float32

	// The card, when Follow is set.
	card     *swipeCard
	dragging bool
	axis     swipeAxis
	base     float32 // the offset the gesture started from
	offset   float32 // how far the card sits from rest along the axis
	speed    dragSpeed
	moving   *bellui.Animation
	flying   SwipeDirection // the swipe a fly-off in progress reports
}

type swipeAxis uint8

const (
	swipeAxisNone swipeAxis = iota
	swipeAxisHorizontal
	swipeAxisVertical
)

// NewSwipeArea returns a SwipeArea around content that calls onSwiped.
//
// Since: 1.3
func NewSwipeArea(content core.CanvasObject, onSwiped func(SwipeDirection)) *SwipeArea {
	s := &SwipeArea{Content: content, OnSwiped: onSwiped}
	s.ExtendBaseWidget(s)
	return s
}

// CreateRenderer draws the content, filling the area, on a card that can be
// moved and tilted.
func (s *SwipeArea) CreateRenderer() core.WidgetRenderer {
	s.ExtendBaseWidget(s)
	s.card = newSwipeCard(s.Content)
	return &swipeAreaRenderer{area: s}
}

// Dragged adds the drag's movement to the swipe in progress, and moves the
// card with it when Follow is set.
func (s *SwipeArea) Dragged(e *event.DragEvent) {
	if !s.Follow {
		s.dx += e.Dragged.DX
		s.dy += e.Dragged.DY
		return
	}
	if !s.dragging {
		s.startDrag()
	}
	s.dx += e.Dragged.DX
	s.dy += e.Dragged.DY
	if s.axis == swipeAxisNone {
		switch ax, ay := abs32(s.dx), abs32(s.dy); {
		case ax > ay:
			s.axis = swipeAxisHorizontal
		case ay > ax:
			s.axis = swipeAxisVertical
		default:
			return
		}
	}
	along := s.along(s.dx, s.dy)
	s.speed.add(swipeNow(), along)
	to := s.base + along
	if s.Directions[s.directionAt(to)] == SwipeIgnore {
		// Held at rest: once there, nothing more to place or report.
		if s.offset == 0 {
			return
		}
		to = 0
	}
	s.place(to)
}

// startDrag begins a gesture on the card. One caught springing back goes on
// from where it is; one caught flying off finishes its swipe first.
func (s *SwipeArea) startDrag() {
	s.dragging = true
	s.dx, s.dy = 0, 0
	s.speed.reset()
	if s.flying != "" {
		s.land()
	}
	s.stopMoving()
	s.base = s.offset
	if s.offset == 0 {
		s.axis = swipeAxisNone
	}
}

// DragEnd reports the swipe, if the drag was one, and starts afresh. A card
// flies off first, or springs back when the drag was not a swipe.
func (s *SwipeArea) DragEnd() {
	dx, dy := s.dx, s.dy
	s.dx, s.dy = 0, 0
	if !s.Follow {
		if d, ok := swipeOf(dx, dy); ok && s.OnSwiped != nil {
			s.OnSwiped(d)
		}
		return
	}
	if !s.dragging {
		return
	}
	s.dragging = false
	v := s.speed.at(swipeNow())
	s.speed.reset()
	d, ok := s.cardSwipe(dx, dy, v)
	if ok && s.Directions[d] != SwipeStay {
		s.flyOff(d)
		return
	}
	s.springBack()
	if ok && s.OnSwiped != nil {
		s.OnSwiped(d)
	}
}

// cardSwipe is the direction a card released after a gesture that moved
// dx, dy at speed v along its axis is thrown, if it is a swipe: mostly along
// the axis, and either a flick the way the card sits or a fifth of its size
// from rest.
func (s *SwipeArea) cardSwipe(dx, dy, v float32) (SwipeDirection, bool) {
	along, across := abs32(s.along(dx, dy)), abs32(s.along(dy, dx))
	if s.offset == 0 || along < swipeDominance*across {
		return "", false
	}
	flick := along >= minSwipe && abs32(v) >= carouselFlickVelocity && (v < 0) == (s.offset < 0)
	if !flick && abs32(s.offset) < s.threshold() {
		return "", false
	}
	return s.direction(), true
}

// threshold is how far from rest a card released without a flick must sit
// to be swiped.
func (s *SwipeArea) threshold() float32 {
	return geo.Max(minSwipe, s.extent()/swipeCardShare)
}

// extent is the card's size along the axis.
func (s *SwipeArea) extent() float32 {
	if s.axis == swipeAxisVertical {
		return s.Size().Height
	}
	return s.Size().Width
}

// along is the part of dx, dy on the card's axis.
func (s *SwipeArea) along(dx, dy float32) float32 {
	if s.axis == swipeAxisVertical {
		return dy
	}
	return dx
}

// direction is the way the card sits from rest.
func (s *SwipeArea) direction() SwipeDirection {
	return s.directionAt(s.offset)
}

// directionAt is the way a card offset along the axis sits from rest.
func (s *SwipeArea) directionAt(offset float32) SwipeDirection {
	switch {
	case s.axis == swipeAxisVertical && offset < 0:
		return SwipeUp
	case s.axis == swipeAxisVertical:
		return SwipeDown
	case offset < 0:
		return SwipeLeft
	}
	return SwipeRight
}

// place moves the card to offset along the axis and reports the progress.
func (s *SwipeArea) place(offset float32) {
	s.offset = offset
	s.layoutCard()
	if s.OnSwipeProgress != nil {
		s.OnSwipeProgress(s.direction(), geo.Min(1, abs32(offset)/s.threshold()))
	}
}

// layoutCard puts the card at the offset, tilted about its centre by how far
// it went sideways. The card moves by its position and tilts by a rotation
// about its centre.
func (s *SwipeArea) layoutCard() {
	if s.card == nil {
		return
	}
	size := s.card.Size()
	if s.axis == swipeAxisVertical {
		s.card.SetTransform(geo.Identity())
		s.card.Move(geo.NewPos(0, s.offset))
		return
	}
	share := float32(0)
	if size.Width > 0 {
		share = geo.Max(-1, geo.Min(1, s.offset/size.Width))
	}
	s.card.SetTransform(geo.NewRotationAround(share*swipeCardTilt, size.Width/2, size.Height/2))
	s.card.Move(geo.NewPos(s.offset, 0))
}

// flyOff carries the card off the side it was thrown to, then lands it.
func (s *SwipeArea) flyOff(d SwipeDirection) {
	s.flying = d
	if !bellui.CurrentApp().Settings().ShowAnimations() {
		s.land()
		return
	}
	from := s.offset
	to := s.extent() * swipeCardFlyOff
	if from < 0 {
		to = -to
	}
	// Linear: the card leaves at speed, and an easing that starts slow would
	// stop it at the release.
	s.animate(canvas.DurationShort4, bellui.AnimationLinear, func(done float32) {
		s.place(from + (to-from)*done)
	}, s.land)
}

// land reports the swipe a fly-off was for and puts the card back at rest.
func (s *SwipeArea) land() {
	d := s.flying
	s.flying = ""
	s.stopMoving()
	if s.OnSwiped != nil {
		s.OnSwiped(d)
	}
	s.place(0)
	s.axis = swipeAxisNone
}

// springBack animates the card back to rest.
func (s *SwipeArea) springBack() {
	if s.offset == 0 || !bellui.CurrentApp().Settings().ShowAnimations() {
		s.place(0)
		s.axis = swipeAxisNone
		return
	}
	from := s.offset
	s.animate(canvas.DurationMedium2, bellui.AnimationEaseStandard, func(done float32) {
		s.place(from * (1 - done))
	}, func() { s.axis = swipeAxisNone })
}

// animate runs step through an animation, then done, unless a drag or another
// animation takes over first.
func (s *SwipeArea) animate(d time.Duration, curve bellui.AnimationCurve, step func(float32), done func()) {
	s.stopMoving()
	var a *bellui.Animation
	a = bellui.NewAnimation(d, func(f float32) {
		if s.moving != a {
			return
		}
		if f < 1 {
			step(f)
			return
		}
		step(1)
		s.moving = nil
		done()
	})
	a.Curve = curve
	s.moving = a
	a.Start()
}

func (s *SwipeArea) stopMoving() {
	if a := s.moving; a != nil {
		s.moving = nil
		a.Stop()
	}
}

// Tapped passes a tap on to the content.
func (s *SwipeArea) Tapped(e *event.PointEvent) {
	if t, ok := s.Content.(event.Tappable); ok {
		t.Tapped(e)
	}
}

// swipeOf is the direction of a drag that moved dx, dy, if it was a swipe.
func swipeOf(dx, dy float32) (SwipeDirection, bool) {
	ax, ay := abs32(dx), abs32(dy)
	switch {
	case ax >= minSwipe && ax >= swipeDominance*ay:
		if dx < 0 {
			return SwipeLeft, true
		}
		return SwipeRight, true
	case ay >= minSwipe && ay >= swipeDominance*ax:
		if dy < 0 {
			return SwipeUp, true
		}
		return SwipeDown, true
	}
	return "", false
}

type swipeAreaRenderer struct {
	area *SwipeArea
}

func (r *swipeAreaRenderer) Destroy() {}

func (r *swipeAreaRenderer) Layout(size geo.Size) {
	r.area.card.Resize(size)
	r.area.layoutCard()
}

func (r *swipeAreaRenderer) MinSize() geo.Size {
	return r.area.card.MinSize()
}

func (r *swipeAreaRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.area.card}
}

func (r *swipeAreaRenderer) Refresh() {
	r.area.card.setContent(r.area.Content)
	r.area.card.Refresh()
}

// swipeCard is what a SwipeArea's content is drawn on, so that it can move
// and tilt with the finger while the area itself, which the drag's events
// are measured against, stays put. Its transform only ever rotates (see
// layoutCard).
type swipeCard struct {
	BaseWidget
	content core.CanvasObject

	mu        sync.RWMutex // the transform is read by the painter
	transform geo.Matrix
}

func newSwipeCard(content core.CanvasObject) *swipeCard {
	c := &swipeCard{content: content, transform: geo.Identity()}
	c.ExtendBaseWidget(c)
	return c
}

func (c *swipeCard) CreateRenderer() core.WidgetRenderer {
	return &swipeCardRenderer{card: c, objects: []core.CanvasObject{c.content}}
}

func (c *swipeCard) setContent(content core.CanvasObject) {
	if content == c.content {
		return
	}
	c.content = content
	content.Resize(c.Size())
}

// Transform is how the card tilts.
func (c *swipeCard) Transform() geo.Matrix {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.transform
}

// SetTransform tilts the card. Like a canvas object's, it widens the next
// partial-repaint region to everything, as no canvas can place a transformed
// subtree. It repaints rather than refreshes: a refresh would free the
// content's textures, an image's among them, on every frame of a drag.
func (c *swipeCard) SetTransform(m geo.Matrix) {
	c.mu.Lock()
	changed := c.transform != m
	c.transform = m
	c.mu.Unlock()
	if changed {
		paintgen.Changed()
		internalWidget.Repaint(c)
	}
}

type swipeCardRenderer struct {
	card    *swipeCard
	objects []core.CanvasObject
}

func (r *swipeCardRenderer) Destroy() {}

func (r *swipeCardRenderer) Layout(size geo.Size) {
	r.objects[0].Resize(size)
}

func (r *swipeCardRenderer) MinSize() geo.Size {
	return r.objects[0].MinSize()
}

func (r *swipeCardRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *swipeCardRenderer) Refresh() {
	r.objects[0] = r.card.content
	r.objects[0].Refresh()
}
