package widget

import (
	"image/color"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

var _ core.Widget = (*Activity)(nil)

// Activity is used to indicate that something is happening that should be waited for,
// or is in the background (depending on usage).
//
// Since: 1.0
type Activity struct {
	BaseWidget

	started bool
}

// NewActivity returns a widget for indicating activity
//
// Since: 1.0
func NewActivity() *Activity {
	a := &Activity{}
	a.ExtendBaseWidget(a)
	return a
}

func (a *Activity) MinSize() geo.Size {
	a.ExtendBaseWidget(a)
	return a.BaseWidget.MinSize()
}

// Start the activity indicator animation
func (a *Activity) Start() {
	if a.started {
		return // already started
	}

	a.started = true

	a.Refresh()
}

// Stop the activity indicator animation
func (a *Activity) Stop() {
	if !a.started {
		return // already stopped
	}

	a.started = false

	a.Refresh()
}

func (a *Activity) CreateRenderer() core.WidgetRenderer {
	dots := make([]core.CanvasObject, 3)
	v := bellui.CurrentApp().Settings().ThemeVariant()
	for i := range dots {
		dots[i] = canvas.NewCircle(a.Theme().Color(theme.ColorNamePrimary, v))
	}
	r := &activityRenderer{dots: dots, parent: a}
	r.anim = &bellui.Animation{
		Duration:    time.Second * 2,
		RepeatCount: bellui.AnimationRepeatForever,
		Tick:        r.animate,
	}
	r.updateColor()

	if a.started {
		r.start()
	}

	return r
}

var _ core.WidgetRenderer = (*activityRenderer)(nil)

type activityRenderer struct {
	anim   *bellui.Animation
	dots   []core.CanvasObject
	parent *Activity

	bound      geo.Size
	maxCol     color.NRGBA
	maxRad     float32
	wasStarted bool
}

func (a *activityRenderer) Destroy() {
	a.parent.started = false
	a.stop()
}

func (a *activityRenderer) Layout(size geo.Size) {
	a.maxRad = geo.Min(size.Width, size.Height) / 2
	a.bound = size
}

func (a *activityRenderer) MinSize() geo.Size {
	return geo.NewSquareSize(a.parent.Theme().Size(theme.SizeNameInlineIcon))
}

func (a *activityRenderer) Objects() []core.CanvasObject {
	return a.dots
}

func (a *activityRenderer) Refresh() {
	if a.parent.started {
		if !a.wasStarted {
			a.start()
		}
	} else if a.wasStarted {
		a.stop()
	}

	a.updateColor()
}

func (a *activityRenderer) animate(done float32) {
	off := done * 2
	if off > 1 {
		off = 2 - off
	}

	off1 := (done + 0.25) * 2
	if done >= 0.75 {
		off1 = (done - 0.75) * 2
	}
	if off1 > 1 {
		off1 = 2 - off1
	}

	off2 := (done + 0.75) * 2
	if done >= 0.25 {
		off2 = (done - 0.25) * 2
	}
	if off2 > 1 {
		off2 = 2 - off2
	}

	a.scaleDot(a.dots[0].(*canvas.Circle), off)
	a.scaleDot(a.dots[1].(*canvas.Circle), off1)
	a.scaleDot(a.dots[2].(*canvas.Circle), off2)
}

func (a *activityRenderer) scaleDot(dot *canvas.Circle, off float32) {
	rad := a.maxRad - a.maxRad*off/1.2
	mid := geo.NewPos(a.bound.Width/2, a.bound.Height/2)

	dot.Move(mid.Subtract(geo.NewSquareOffsetPos(rad)))
	dot.Resize(geo.NewSquareSize(rad * 2))

	alpha := uint8(0 + int(float32(a.maxCol.A)*off))
	dot.FillColor = color.NRGBA{R: a.maxCol.R, G: a.maxCol.G, B: a.maxCol.B, A: alpha}
	dot.Refresh()
}

func (a *activityRenderer) start() {
	a.wasStarted = true
	a.anim.Start()
}

func (a *activityRenderer) stop() {
	a.wasStarted = false
	a.anim.Stop()
}

func (a *activityRenderer) updateColor() {
	v := bellui.CurrentApp().Settings().ThemeVariant()
	rr, gg, bb, aa := a.parent.Theme().Color(theme.ColorNamePrimary, v).RGBA()
	a.maxCol = color.NRGBA{R: uint8(rr >> 8), G: uint8(gg >> 8), B: uint8(bb >> 8), A: uint8(aa >> 8)}
}
