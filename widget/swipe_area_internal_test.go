package widget

import (
	"image/color"
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type swipeProgress struct {
	d SwipeDirection
	p float32
}

// swipeCardBench is a 300x300 card SwipeArea alone in a window, with the
// held-animation app of the Carousel tests and a clock that moves only when
// the test says so. It records what the area reports.
type swipeCardBench struct {
	area     *SwipeArea
	win      bellui.Window
	drv      *carouselAnimDriver
	clock    *time.Duration
	swiped   []SwipeDirection
	progress []swipeProgress
}

func newSwipeCardBench(t *testing.T, content core.CanvasObject, noAnim bool) *swipeCardBench {
	t.Helper()
	a := test.NewTempApp(t)
	b := &swipeCardBench{drv: &carouselAnimDriver{Driver: a.Driver()}, clock: new(time.Duration)}
	bellui.SetCurrentApp(&carouselAnimApp{App: a, drv: b.drv, noAnim: noAnim})
	t.Cleanup(func() { bellui.SetCurrentApp(a) })

	base := time.Unix(1000, 0)
	swipeNow = func() time.Time { return base.Add(*b.clock) }
	t.Cleanup(func() { swipeNow = time.Now })

	b.area = NewSwipeArea(content, func(d SwipeDirection) { b.swiped = append(b.swiped, d) })
	b.area.Follow = true
	b.area.OnSwipeProgress = func(d SwipeDirection, p float32) { b.progress = append(b.progress, swipeProgress{d, p}) }
	b.win = test.NewTempWindow(t, b.area)
	b.win.SetPadded(false)
	b.win.Resize(geo.NewSize(300, 300))
	return b
}

// drag moves the finger dx, dy from the middle in 10 steps, slowly enough
// that the release is no flick, and lifts it.
func (b *swipeCardBench) drag(dx, dy float32) {
	test.DragSteps(b.win.Canvas(), geo.NewPos(150, 150), dx, dy, 10, func(int) { *b.clock += 50 * time.Millisecond })
}

func (b *swipeCardBench) transform() geo.Matrix { return b.area.card.Transform() }

// at is where the point x, y of the card is drawn, in the area.
func (b *swipeCardBench) at(x, y float32) (float32, float32) {
	tx, ty := b.transform().TransformPoint(x, y)
	p := b.area.card.Position()
	return p.X + tx, p.Y + ty
}

// atRest reports whether the card sits where it started, untilted.
func (b *swipeCardBench) atRest() bool {
	return b.area.card.Position().IsZero() && b.transform().IsIdentity()
}

func (b *swipeCardBench) lastProgress(t *testing.T) swipeProgress {
	t.Helper()
	require.NotEmpty(t, b.progress)
	return b.progress[len(b.progress)-1]
}

// While dragged, a card moves with the finger along the axis the drag started
// on, and a sideways drag tilts it the way it goes (#989).
func TestSwipeArea_CardFollowsTheFinger(t *testing.T) {
	b := newSwipeCardBench(t, canvas.NewRectangle(nil), false)
	area := b.area

	area.Dragged(dragBy(40, 4))
	m := b.transform()
	x, y := b.at(150, 150) // the card's centre, which it tilts about
	assert.InDelta(t, 190, x, 0.01, "the centre moves with the finger")
	assert.InDelta(t, 150, y, 0.01, "and only along the axis")
	assert.Greater(t, m.B, float32(0), "dragged right, it tilts clockwise")
	assert.Less(t, m.B, float32(0.09), "by a few degrees at most")
	assert.Equal(t, swipeProgress{SwipeRight, 40.0 / 60}, b.lastProgress(t), "a fifth of 300 swipes")

	area.Dragged(dragBy(-120, 0))
	m = b.transform()
	x, _ = b.at(150, 150)
	assert.InDelta(t, 70, x, 0.01)
	assert.Less(t, m.B, float32(0), "back past rest, it tilts the other way")
	assert.Equal(t, swipeProgress{SwipeLeft, 1}, b.lastProgress(t))
	area.DragEnd()
	b.drv.started[len(b.drv.started)-1].Tick(1)
	b.swiped = nil

	// A drag that starts up or down moves the card up or down, untilted.
	area.Dragged(dragBy(2, -30))
	assert.Equal(t, geo.NewPos(0, -30), area.card.Position())
	assert.True(t, b.transform().IsIdentity())
	assert.Equal(t, SwipeUp, b.lastProgress(t).d)
}

// A card released short of a swipe springs back to rest, and nothing is
// reported; one released at a swipe flies off the way it was thrown, OnSwiped
// runs once it is off, and it is back at rest (#989).
func TestSwipeArea_CardSpringsBackOrFliesOff(t *testing.T) {
	b := newSwipeCardBench(t, canvas.NewRectangle(nil), false)

	b.drag(50, 0) // under a fifth of the width, and slow
	require.Len(t, b.drv.started, 1, "the spring back animates")
	spring := b.drv.started[0]
	spring.Tick(0.5)
	x, _ := b.at(150, 150)
	assert.Greater(t, x, float32(150), "half way back")
	assert.Less(t, x, float32(200))
	spring.Tick(1)
	assert.True(t, b.atRest(), "at rest, untilted")
	assert.Empty(t, b.swiped)
	assert.Equal(t, float32(0), b.lastProgress(t).p)

	b.drag(-90, 10)
	require.Len(t, b.drv.started, 2)
	fly := b.drv.started[1]
	fly.Tick(0.5)
	assert.Empty(t, b.swiped, "reported once it is off, not as it goes")
	x, _ = b.at(150, 150)
	assert.Less(t, x, float32(150-90), "on its way out to the left")
	fly.Tick(1)
	assert.Equal(t, []SwipeDirection{SwipeLeft}, b.swiped)
	assert.True(t, b.atRest(), "back at rest for the next content")
	assert.Equal(t, float32(0), b.lastProgress(t).p)

	// A diagonal drag is no swipe however far it goes.
	b.swiped = nil
	b.drag(100, 80)
	b.drv.started[len(b.drv.started)-1].Tick(1)
	assert.Empty(t, b.swiped)
	assert.True(t, b.atRest())
}

// A flick throws the card off even when it has not gone a fifth of the way;
// a flick back towards rest does not (#989).
func TestSwipeArea_CardFlick(t *testing.T) {
	b := newSwipeCardBench(t, canvas.NewRectangle(nil), true)
	// Three 10pt steps 10ms apart: 20pt over 20ms between the first and last
	// sample, 1000 points per second.
	test.DragSteps(b.win.Canvas(), geo.NewPos(150, 150), 30, 0, 3, func(int) { *b.clock += 10 * time.Millisecond })
	assert.Equal(t, []SwipeDirection{SwipeRight}, b.swiped, "a 30pt flick swipes")

	b.swiped = nil
	test.DragSteps(b.win.Canvas(), geo.NewPos(150, 150), 30, 0, 3, func(int) { *b.clock += 100 * time.Millisecond })
	assert.Empty(t, b.swiped, "the same drag, slow, does not")

	// Out 50 slowly, then flicked 30 back towards rest: it springs back.
	area := b.area
	for range 5 {
		*b.clock += 50 * time.Millisecond
		area.Dragged(dragBy(10, 0))
	}
	for range 3 {
		*b.clock += 10 * time.Millisecond
		area.Dragged(dragBy(-10, 0))
	}
	area.DragEnd()
	assert.Empty(t, b.swiped)
	assert.True(t, b.atRest())
}

// With animations off a card swipe reports at once, and a card goes straight
// back to rest.
func TestSwipeArea_CardWithoutAnimations(t *testing.T) {
	b := newSwipeCardBench(t, canvas.NewRectangle(nil), true)
	b.drag(0, 100)
	assert.Equal(t, []SwipeDirection{SwipeDown}, b.swiped)
	assert.True(t, b.atRest())
	assert.Empty(t, b.drv.started)
}

// A drag that catches a card on its way off lands that swipe first; one that
// catches it springing back takes it from where it is (#989).
func TestSwipeArea_CardCaughtMidAnimation(t *testing.T) {
	b := newSwipeCardBench(t, canvas.NewRectangle(nil), false)
	area := b.area

	b.drag(120, 0)
	b.drv.started[0].Tick(0.3)
	area.Dragged(dragBy(5, 0))
	assert.Equal(t, []SwipeDirection{SwipeRight}, b.swiped)
	assert.Contains(t, b.drv.stopped, b.drv.started[0])
	x, _ := b.at(150, 150)
	assert.InDelta(t, 155, x, 0.01, "the new drag starts from rest")
	area.DragEnd()

	b.swiped = nil
	b.drag(-50, 0)
	spring := b.drv.started[len(b.drv.started)-1]
	spring.Tick(0.5)
	from, _ := b.at(150, 150)
	area.Dragged(dragBy(-5, 0))
	x, _ = b.at(150, 150)
	assert.InDelta(t, from-5, x, 0.01, "caught where it was")
	spring.Tick(1) // a stale tick of the superseded animation is ignored
	x, _ = b.at(150, 150)
	assert.InDelta(t, from-5, x, 0.01)
	area.DragEnd()
}

// A tap on a card still reaches its content.
func TestSwipeArea_CardTapsReachTheContent(t *testing.T) {
	taps := 0
	b := newSwipeCardBench(t, NewButton("x", func() { taps++ }), false)
	test.TapCanvas(b.win.Canvas(), geo.NewPos(150, 150))
	assert.Equal(t, 1, taps)
}

// Without Follow the content does not move, whatever is dragged.
func TestSwipeArea_ContentStaysWithoutFollow(t *testing.T) {
	b := newSwipeCardBench(t, canvas.NewRectangle(nil), false)
	b.area.Follow = false
	b.drag(120, 0)
	assert.Equal(t, []SwipeDirection{SwipeRight}, b.swiped)
	assert.True(t, b.atRest())
	assert.Empty(t, b.progress)
}

// The card's transform reaches the painter: a drawn card sits where it was
// dragged to, not where the area is.
func TestSwipeArea_CardIsDrawnWhereItWasDragged(t *testing.T) {
	red := color.NRGBA{R: 0xff, A: 0xff}
	rect := canvas.NewRectangle(red)
	b := newSwipeCardBench(t, rect, false)
	b.area.Dragged(dragBy(0, 100)) // straight down, so no tilt blurs the edges

	img := b.win.Canvas().Capture()
	scale := float32(img.Bounds().Dx()) / 300
	at := func(x, y float32) color.Color { return img.At(int(x*scale), int(y*scale)) }
	assert.NotEqual(t, red, color.NRGBAModel.Convert(at(150, 50)), "the top of the area is uncovered")
	assert.Equal(t, red, color.NRGBAModel.Convert(at(150, 150)))
}

func dragBy(dx, dy float32) *event.DragEvent {
	return &event.DragEvent{Dragged: geo.Delta{DX: dx, DY: dy}}
}

// A tilted card is drawn turned about its centre: dragged its own width, its
// middle is where the finger took it, and it leans the way it went.
func TestSwipeArea_CardTiltsAboutItsCentre(t *testing.T) {
	test.NewTempApp(t)
	red := color.NRGBA{R: 0xff, A: 0xff}
	area := NewSwipeArea(canvas.NewRectangle(red), nil)
	area.Follow = true
	w := test.NewTempWindow(t, &bellui.Container{Objects: []core.CanvasObject{area}})
	w.SetPadded(false)
	w.Resize(geo.NewSize(1000, 500)) // room for the tilted corners
	area.Resize(geo.NewSize(300, 300))
	area.Move(geo.NewPos(300, 100))
	area.Dragged(dragBy(300, 0)) // a full tilt, the card clear of the area

	img := w.Canvas().Capture()
	scale := float32(img.Bounds().Dx()) / 1000
	isRed := func(px, py int) bool { return color.NRGBAModel.Convert(img.At(px, py)) == red }
	var n, sx, sy float32
	for py := img.Bounds().Min.Y; py < img.Bounds().Max.Y; py++ {
		for px := img.Bounds().Min.X; px < img.Bounds().Max.X; px++ {
			if isRed(px, py) {
				n++
				sx += float32(px) / scale
				sy += float32(py) / scale
			}
		}
	}
	require.NotZero(t, n, "the card is drawn")
	assert.InDelta(t, 750, sx/n, 1, "its middle is where the finger took it")
	assert.InDelta(t, 250, sy/n, 1, "and has not dropped, as a turn about a corner would")

	// leftEdge is where the card starts along the row at y.
	leftEdge := func(y float32) float32 {
		py := int(y * scale)
		for px := img.Bounds().Min.X; px < img.Bounds().Max.X; px++ {
			if isRed(px, py) {
				return float32(px) / scale
			}
		}
		return -1
	}
	top, bottom := leftEdge(130), leftEdge(370)
	// 240 points apart at 5 degrees: tan(5°)·240 ≈ 21.
	assert.InDelta(t, 21, top-bottom, 2, "its top leans right")
}

// A stay direction lets the card follow and report progress, but a swipe
// that way springs it back, with OnSwiped run at the release; the other
// directions still throw (#1103).
func TestSwipeArea_CardStayDirection(t *testing.T) {
	b := newSwipeCardBench(t, canvas.NewRectangle(nil), false)
	b.area.Directions = map[SwipeDirection]SwipeCardMode{SwipeUp: SwipeStay, SwipeDown: SwipeStay}

	b.area.Dragged(dragBy(0, -40))
	assert.Equal(t, geo.NewPos(0, -40), b.area.card.Position(), "it follows the finger")
	assert.Equal(t, swipeProgress{SwipeUp, 40.0 / 60}, b.lastProgress(t))
	for range 6 {
		*b.clock += 50 * time.Millisecond
		b.area.Dragged(dragBy(0, -10))
	}
	assert.Equal(t, swipeProgress{SwipeUp, 1}, b.lastProgress(t))
	b.area.DragEnd()
	assert.Equal(t, []SwipeDirection{SwipeUp}, b.swiped, "reported at the release")
	require.Len(t, b.drv.started, 1)
	spring := b.drv.started[0]
	spring.Tick(0.5)
	_, y := b.at(150, 150)
	assert.Greater(t, y, float32(150-100), "on its way back, not off")
	assert.Less(t, y, float32(150))
	spring.Tick(1)
	assert.True(t, b.atRest())
	assert.Equal(t, []SwipeDirection{SwipeUp}, b.swiped, "reported once")

	b.swiped = nil
	b.drag(-90, 0)
	b.drv.started[len(b.drv.started)-1].Tick(0.5)
	assert.Empty(t, b.swiped, "left still throws, reported once off")
	b.drv.started[len(b.drv.started)-1].Tick(1)
	assert.Equal(t, []SwipeDirection{SwipeLeft}, b.swiped)

	// Short of a swipe, a stay direction reports nothing, as a throw would.
	b.swiped = nil
	b.drag(0, 30)
	b.drv.started[len(b.drv.started)-1].Tick(1)
	assert.Empty(t, b.swiped)
	assert.True(t, b.atRest())
}

// With animations off a stay swipe reports and the card is at rest at once.
func TestSwipeArea_CardStayWithoutAnimations(t *testing.T) {
	b := newSwipeCardBench(t, canvas.NewRectangle(nil), true)
	b.area.Directions = map[SwipeDirection]SwipeCardMode{SwipeDown: SwipeStay}
	b.drag(0, 100)
	assert.Equal(t, []SwipeDirection{SwipeDown}, b.swiped)
	assert.True(t, b.atRest())
	assert.Empty(t, b.drv.started)
}

// An ignored direction keeps the card at rest: it does not move that way, no
// progress above 0 is reported, and a swipe that way runs nothing. Dragged
// back across rest from a direction it may go, it stops at rest (#1103).
func TestSwipeArea_CardIgnoreDirection(t *testing.T) {
	b := newSwipeCardBench(t, canvas.NewRectangle(nil), false)
	b.area.Directions = map[SwipeDirection]SwipeCardMode{SwipeUp: SwipeIgnore, SwipeDown: SwipeIgnore, SwipeLeft: SwipeIgnore}

	b.drag(0, -120)
	if n := len(b.drv.started); n > 0 {
		b.drv.started[n-1].Tick(1)
	}
	assert.Empty(t, b.swiped)
	assert.True(t, b.atRest())
	for _, p := range b.progress {
		assert.Equal(t, float32(0), p.p, "never off rest")
	}

	b.area.Dragged(dragBy(40, 0))
	assert.Equal(t, geo.NewPos(40, 0), b.area.card.Position(), "right is not ignored")
	b.area.Dragged(dragBy(-100, 0))
	assert.True(t, b.atRest(), "it stops at rest rather than go left")
	assert.Equal(t, float32(0), b.lastProgress(t).p)
	b.area.DragEnd()
	assert.Empty(t, b.swiped, "a release at rest is no swipe")
}

// A direction a non-nil map leaves out still throws.
func TestSwipeArea_CardDirectionsDefaultToThrow(t *testing.T) {
	b := newSwipeCardBench(t, canvas.NewRectangle(nil), true)
	b.area.Directions = map[SwipeDirection]SwipeCardMode{SwipeUp: SwipeIgnore}
	b.drag(100, 0)
	assert.Equal(t, []SwipeDirection{SwipeRight}, b.swiped)
	assert.Equal(t, SwipeThrow, SwipeCardMode(0), "the zero value throws")
}
