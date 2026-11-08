package widget

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// Press feedback timing (#1085). The ripple covers the shape within
// pressSpread of the press, and after the release finishes spreading and
// fades within pressRelease, so even a quick tap shows it once the finger
// lifts.
const (
	pressSpread  = canvas.DurationMedium1
	pressRelease = canvas.DurationStandard
)

// showAnimations and startAnimation reach the app. A test of this package
// replaces them, as it cannot make an app.
var (
	showAnimations = func() bool { return bellui.CurrentApp().Settings().ShowAnimations() }
	startAnimation = (*bellui.Animation).Start
)

// PressFeedback draws the MD3 pressed state of a tappable widget on a
// canvas.Ripple the widget's renderer holds: a state layer in the widget's
// content colour, at the theme's pressed opacity, that spreads from the press
// point while the pointer is down and fades after it lifts (#1085). It used
// to be a ripple started at the release, fading as it grew, which a fingertip
// hid until it was a few percent.
//
// The widget calls Press from TouchDown and a primary MouseDown, Release from
// TouchUp and MouseUp, and Cancel from TouchCancel and HoverOut (a mouse
// released elsewhere sends the widget no MouseUp). Tap is for an activation
// with no press before it, such as the keyboard's. All run on the main
// goroutine.
//
// With animations off, the state layer covers the shape from the press to the
// release, with no ripple.
type PressFeedback struct {
	// Ripple is what the feedback draws on. The renderer places it above the
	// background and below the content, and keeps it the widget's size.
	Ripple *canvas.Ripple

	pressed bool
	anim    *bellui.Animation
	full    float32 // the radius that covers the shape from the press point
}

// NewPressFeedback returns feedback that draws on a new, hidden ripple.
func NewPressFeedback() *PressFeedback {
	r := canvas.NewRipple(color.Transparent)
	r.Hide()
	return &PressFeedback{Ripple: r}
}

// Pressed reports whether the pointer is down on the widget: between a Press
// and its Release or Cancel.
func (f *PressFeedback) Pressed() bool { return f.pressed }

// Press shows the pressed state from at, in the widget's coordinates, over a
// shape of size with corners of cornerRadius, in content, the colour of the
// widget's text or icon.
func (f *PressFeedback) Press(at geo.Position, size geo.Size, cornerRadius float32, content color.Color) {
	f.stop()
	f.pressed = true
	r := f.Ripple
	r.Center = at
	r.CornerRadius = cornerRadius
	r.Color = opaque(content)
	r.Opacity = pressedOpacity(content)
	r.Resize(size)
	f.full = coverRadius(at, size)
	if !showAnimations() {
		r.Radius = f.full
		r.Show()
		r.Refresh()
		return
	}
	r.Radius = 0
	r.Show()
	f.anim = bellui.NewAnimation(pressSpread, func(done float32) {
		r.Radius = f.full * done
		r.Refresh()
	})
	f.anim.Curve = bellui.AnimationEaseOut
	startAnimation(f.anim)
}

// Release ends the press: the ripple finishes covering the shape, then fades.
// It does nothing without a press.
func (f *PressFeedback) Release() {
	if !f.pressed {
		return
	}
	f.pressed = false
	f.stop()
	r := f.Ripple
	if !showAnimations() {
		r.Hide()
		return
	}
	from, opacity := r.Radius, r.Opacity
	f.anim = bellui.NewAnimation(pressRelease, func(done float32) {
		spread := min(1, done*2)
		r.Radius = from + (f.full-from)*spread
		if done > 0.5 {
			r.Opacity = opacity * (1 - (done-0.5)*2)
		}
		if done >= 1 {
			r.Hide()
			return
		}
		r.Refresh()
	})
	startAnimation(f.anim)
}

// Cancel ends the press with no feedback left: the pointer left the widget,
// or the touch turned into a scroll or a drag, so it did not tap it.
func (f *PressFeedback) Cancel() {
	f.pressed = false
	f.stop()
	f.Ripple.Hide()
}

// Tap shows a press and its release at once, for an activation that had no
// press before it: a key, an accessibility action, or a tap whose press the
// widget did not see. It does nothing while a press or its release shows.
func (f *PressFeedback) Tap(at geo.Position, size geo.Size, cornerRadius float32, content color.Color) {
	if f.pressed || f.Ripple.Visible() {
		return
	}
	f.Press(at, size, cornerRadius, content)
	f.Release()
}

// Resize keeps the ripple the widget's size. A renderer calls it from Layout.
func (f *PressFeedback) Resize(size geo.Size) {
	f.Ripple.Resize(size)
}

func (f *PressFeedback) stop() {
	if f.anim != nil {
		f.anim.Stop()
		f.anim = nil
	}
}

// coverRadius is the distance from at to the farthest corner of size.
func coverRadius(at geo.Position, size geo.Size) float32 {
	dx := geo.Max(at.X, size.Width-at.X)
	dy := geo.Max(at.Y, size.Height-at.Y)
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

// pressedOpacity is the theme's pressed state layer opacity. The painters
// take a ripple's alpha from its Opacity alone.
func pressedOpacity(content color.Color) float32 {
	_, _, _, a := theme.StateLayerPressed(content).RGBA()
	return float32(a) / 0xffff
}

func opaque(c color.Color) color.Color {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return color.Black
	}
	// Un-premultiply.
	return color.NRGBA64{R: uint16(r * 0xffff / a), G: uint16(g * 0xffff / a), B: uint16(b * 0xffff / a), A: 0xffff}
}
