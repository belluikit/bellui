package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/internal/driver/mobile/event/scroll"
)

// A mouse or touchpad on Android is not a finger (#1510): its hovers and
// button changes are dropped, its wheel is a scroll, and a touchpad's
// two-finger swipe is a touchpad scroll. Only fingers on the screen stay
// touches.
func TestClassifyMotion(t *testing.T) {
	const pointer1 = 1 << 8 // a second pointer's index in the action
	for _, tc := range []struct {
		name          string
		action, class int32
		want          motionKind
	}{
		{"finger down", motionActionDown, 0, motionTouch},
		{"finger move", motionActionMove, 0, motionTouch},
		{"second finger down", 5 | pointer1, 0, motionTouch},
		{"finger cancel", motionActionCancel, 0, motionTouch},
		{"mouse hover move", motionActionHoverMove, 0, motionIgnore},
		{"mouse hover enter", motionActionHoverEnter, 0, motionIgnore},
		{"mouse hover exit", motionActionHoverExit, 0, motionIgnore},
		{"button press", motionActionButtonPress, 0, motionIgnore},
		{"button release", motionActionButtonRelease, 0, motionIgnore},
		{"wheel", motionActionScroll, 0, motionWheel},
		{"touchpad swipe down", motionActionDown, motionClassTwoFingerSwipe, motionTouchpad},
		{"touchpad swipe move", motionActionMove, motionClassTwoFingerSwipe, motionTouchpad},
		{"touchpad swipe up", motionActionUp, motionClassTwoFingerSwipe, motionTouchpad},
		// A touchpad pinch (classification 5) arrives as two fingers, and
		// bellui's touch pinch handles it (measured on pixel143).
		{"touchpad pinch", 5 | pointer1, 5, motionTouch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, classifyMotion(tc.action, tc.class))
		})
	}
}

// A wheel notch toward the user (AXIS_VSCROLL -1) scrolls down, a negative DY
// as on the desktop, and a notch to the right (AXIS_HSCROLL +1) is a negative
// DX. Each notch scrolls the device's own step.
func TestWheelScroll(t *testing.T) {
	e := wheelScroll(10, 20, -1, 0, 168, 150)
	assert.Equal(t, scroll.Event{X: 10, Y: 20, DY: -168}, e)

	e = wheelScroll(10, 20, 0, 1, 168, 150)
	assert.Equal(t, float32(-150), e.DX)
	assert.Zero(t, e.DY)
	assert.False(t, e.Touchpad)
	assert.Equal(t, scroll.PhaseNone, e.Phase)
}

// A two-finger swipe begins at its DOWN, moves with the content following the
// fingers, and ends at its UP or CANCEL. Fingers moving down or right report
// negative distances, measured on the pixel143 emulator, and scroll toward
// the top or the left, a positive delta.
func TestTouchpadScroll(t *testing.T) {
	began := touchpadScroll(motionActionDown, 5, 6, 0, 0)
	assert.Equal(t, scroll.Event{X: 5, Y: 6, Touchpad: true, Phase: scroll.PhaseBegan}, began)

	moved := touchpadScroll(motionActionMove, 5, 6, -4, -10)
	assert.Equal(t, scroll.PhaseChanged, moved.Phase)
	assert.Equal(t, float32(4), moved.DX)
	assert.Equal(t, float32(10), moved.DY)

	for _, a := range []int32{motionActionUp, motionActionCancel} {
		ended := touchpadScroll(a, 5, 6, -3, -3)
		assert.Equal(t, scroll.PhaseEnded, ended.Phase)
		assert.Zero(t, ended.DX, "an end carries no movement")
		assert.Zero(t, ended.DY)
	}
}
