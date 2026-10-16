//go:build !no_glfw && !ci && !mobile

package glfw

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/glfw"
	"github.com/stretchr/testify/assert"
)

// activatableStub is a focusable that declares Space activates it, so the driver
// should fire the keyboard-activation signal for it (pre-rebrand tracker #700).
type activatableStub struct {
	canvas.Rectangle
}

func (a *activatableStub) TypedRune(rune)           {}
func (a *activatableStub) TypedKey(*event.KeyEvent) {}
func (a *activatableStub) FocusGained()             {}
func (a *activatableStub) FocusLost()               {}

func (a *activatableStub) ActivatedByKey(ev *event.KeyEvent) bool {
	return ev.Name == event.KeySpace
}

// TestWindow_KeyboardActivationNotifies asserts the glfw driver fires
// core.NotifyActivation(obj, viaKeyboard=true) when a focused event.Activatable
// receives its activation key, and does NOT fire for a plain focusable that
// consumes the key (parity with the pointer-tap activation path, pre-rebrand tracker #700).
func TestWindow_KeyboardActivationNotifies(t *testing.T) {
	var gotObj core.CanvasObject
	var gotKbd bool
	count := 0
	old := core.OnActivation
	core.OnActivation = func(obj core.CanvasObject, viaKeyboard bool) {
		gotObj = obj
		gotKbd = viaKeyboard
		count++
	}
	defer func() { core.OnActivation = old }()

	w := createWindow("Test")

	act := &activatableStub{}
	w.SetContent(act)
	w.Canvas().Focus(act)
	w.keyPressed(w.viewport, glfw.KeySpace, 0, glfw.Press, 0)

	assert.Equal(t, 1, count, "Space on an Activatable fires one activation")
	assert.True(t, gotKbd, "activation is flagged as keyboard-sourced")
	assert.Equal(t, act, gotObj, "the focused object is reported")

	// A non-activation key on the same object must not fire.
	count = 0
	w.keyPressed(w.viewport, glfw.KeyDown, 0, glfw.Press, 0)
	assert.Equal(t, 0, count, "navigation keys do not activate")

	// A plain focusable (not event.Activatable) never fires the signal.
	count = 0
	plain := &focusable{}
	w.SetContent(plain)
	w.Canvas().Focus(plain)
	w.keyPressed(w.viewport, glfw.KeySpace, 0, glfw.Press, 0)
	assert.Equal(t, 0, count, "a plain focusable is not keyboard-activatable")
}
