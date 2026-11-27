//go:build !no_glfw && !mobile && !wasm

package glfw

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/glfw"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// GLFW's sources and phases map onto bellui's; a platform that cannot tell
// the device keeps reporting a mouse wheel, as every scroll did before (#391).
func TestConvertScrollSourceAndPhase(t *testing.T) {
	assert.Equal(t, event.ScrollSourceTouchpad, convertScrollSource(glfw.ScrollSourceTouchpad))
	assert.Equal(t, event.ScrollSourceMouseWheel, convertScrollSource(glfw.ScrollSourceWheel))
	assert.Equal(t, event.ScrollSourceMouseWheel, convertScrollSource(glfw.ScrollSourceUnknown))

	for g, want := range map[glfw.GesturePhase]gesturePhase{
		glfw.GesturePhaseNone:      gestureNone,
		glfw.GesturePhaseBegan:     gestureBegan,
		glfw.GesturePhaseChanged:   gestureChanged,
		glfw.GesturePhaseEnded:     gestureEnded,
		glfw.GesturePhaseCancelled: gestureCancelled,
		glfw.GesturePhaseMomentum:  gestureMomentum,
	} {
		assert.Equal(t, want, convertGesturePhase(g), "GLFW phase %d", g)
	}
}

// A wheel turned with shift held scrolls sideways off macOS, as before, now
// read from the event's own modifiers.
func TestWindow_ShiftWheelScrollsSideways(t *testing.T) {
	target := newGestureTarget()
	w := keyWindow(target)
	w.mousePos = geo.NewPos(50, 50)
	w.mouseScrolledEx(nil, 0, 1, glfw.ScrollSourceWheel, glfw.GesturePhaseNone, glfw.ModShift)
	require.Len(t, target.scrolls, 1)
	d := target.scrolls[0].Scrolled
	if runtime.GOOS == "darwin" {
		assert.Zero(t, d.DX, "macOS turns it sideways itself")
		assert.NotZero(t, d.DY)
	} else {
		assert.NotZero(t, d.DX, "shift turned the wheel sideways")
		assert.Zero(t, d.DY)
	}
}
