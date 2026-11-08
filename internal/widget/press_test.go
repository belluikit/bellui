package widget

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
)

var pressContent = color.NRGBA{R: 0x21, G: 0x96, B: 0xf3, A: 0xff}

// TestPressFeedback is #1085: the pressed state shows from the press, in the
// content colour at the pressed opacity, spreads to cover the shape, and goes
// after the release or a cancel. The test app runs each animation to its end
// as it starts it, as the test app does.
func TestPressFeedback(t *testing.T) {
	withPressApp(t, true)
	f := NewPressFeedback()
	size := geo.NewSize(100, 40)

	f.Press(geo.NewPos(10, 20), size, 20, pressContent)
	assert.True(t, f.Pressed())
	assert.True(t, f.Ripple.Visible(), "no pressed state while the pointer is down")
	assert.Equal(t, color.NRGBA64{R: 0x2121, G: 0x9696, B: 0xf3f3, A: 0xffff}, f.Ripple.Color)
	assert.InDelta(t, 0.12, f.Ripple.Opacity, 0.005)
	assert.InDelta(t, coverRadius(geo.NewPos(10, 20), size), f.Ripple.Radius, 0.01, "the press did not spread to cover the shape")
	assert.Equal(t, geo.NewPos(10, 20), f.Ripple.Center)
	assert.Equal(t, float32(20), f.Ripple.CornerRadius)
	assert.Equal(t, size, f.Ripple.Size())

	f.Release()
	assert.False(t, f.Pressed())
	assert.False(t, f.Ripple.Visible(), "the release did not fade out")

	f.Press(geo.NewPos(50, 20), size, 20, pressContent)
	f.Cancel()
	assert.False(t, f.Pressed())
	assert.False(t, f.Ripple.Visible(), "a cancelled press left its state behind")

	f.Release() // no press: nothing to release
	assert.False(t, f.Ripple.Visible())
}

// TestPressFeedbackTap: an activation with no press before it shows a press
// and its release, and one during a press adds nothing.
func TestPressFeedbackTap(t *testing.T) {
	withPressApp(t, true)
	f := NewPressFeedback()
	size := geo.NewSize(100, 40)

	f.Tap(geo.NewPos(50, 20), size, 20, pressContent)
	assert.False(t, f.Pressed())
	assert.False(t, f.Ripple.Visible())
	assert.InDelta(t, coverRadius(geo.NewPos(50, 20), size), f.Ripple.Radius, 0.01, "the tap showed no press")

	f.Press(geo.NewPos(10, 20), size, 20, pressContent)
	f.Tap(geo.NewPos(90, 20), size, 20, pressContent)
	assert.True(t, f.Pressed(), "a tap during a press ended it")
	assert.Equal(t, geo.NewPos(10, 20), f.Ripple.Center)
}

// TestPressFeedbackWithoutAnimations: with animations off, the state layer
// covers the shape at once and goes at the release, with no ripple.
func TestPressFeedbackWithoutAnimations(t *testing.T) {
	withPressApp(t, false)
	f := NewPressFeedback()
	size := geo.NewSize(100, 40)

	f.Press(geo.NewPos(10, 20), size, 20, pressContent)
	assert.True(t, f.Ripple.Visible())
	assert.Nil(t, f.anim, "a press animated with animations off")
	assert.InDelta(t, coverRadius(geo.NewPos(10, 20), size), f.Ripple.Radius, 0.01)

	f.Release()
	assert.Nil(t, f.anim, "a release animated with animations off")
	assert.False(t, f.Ripple.Visible())
}

// withPressApp stands in for the app: animations shown or not, each run to
// its end as it starts.
func withPressApp(t *testing.T, animations bool) {
	show, start := showAnimations, startAnimation
	showAnimations = func() bool { return animations }
	startAnimation = func(a *bellui.Animation) { a.Tick(1) }
	t.Cleanup(func() { showAnimations, startAnimation = show, start })
}

// TestPressFeedbackReleaseFades: after a quick tap, whose press had no time
// to spread, the release first spreads the ripple over the shape at full
// pressed opacity and only then fades it, so the tap is seen once the finger
// lifts. The old feedback faded as it grew, from 12%.
func TestPressFeedbackReleaseFades(t *testing.T) {
	withPressApp(t, true)
	startAnimation = func(*bellui.Animation) {} // the test ticks it
	f := NewPressFeedback()
	size := geo.NewSize(100, 40)
	full := coverRadius(geo.NewPos(10, 20), size)

	f.Press(geo.NewPos(10, 20), size, 20, pressContent)
	f.anim.Tick(0.1) // lifted early
	from, opacity := f.Ripple.Radius, f.Ripple.Opacity
	f.Release()

	f.anim.Tick(0.25)
	assert.Greater(t, f.Ripple.Radius, from, "the release did not spread the ripple")
	assert.Equal(t, opacity, f.Ripple.Opacity, "the release faded before it covered the shape")
	f.anim.Tick(0.5)
	assert.InDelta(t, full, f.Ripple.Radius, 0.01)
	assert.Equal(t, opacity, f.Ripple.Opacity)
	f.anim.Tick(0.75)
	assert.InDelta(t, opacity/2, f.Ripple.Opacity, 0.001)
	assert.True(t, f.Ripple.Visible())
	f.anim.Tick(1)
	assert.False(t, f.Ripple.Visible())
}
