package glfw

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/event"
)

// TestGLDriver_KeyModifierListeners pins the #460 notification contract:
// listeners fire in registration order on actual state changes only, and a
// remove function detaches exactly its own listener.
func TestGLDriver_KeyModifierListeners(t *testing.T) {
	d := &gLDriver{}
	assert.Equal(t, event.KeyModifier(0), d.CurrentKeyModifiers())

	var a, b []event.KeyModifier
	removeA := d.AddKeyModifierListener(func(m event.KeyModifier) { a = append(a, m) })
	d.AddKeyModifierListener(func(m event.KeyModifier) { b = append(b, m) })

	d.setCurrentKeyModifiers(event.KeyModifierControl)
	d.setCurrentKeyModifiers(event.KeyModifierControl) // unchanged: no re-notify
	d.setCurrentKeyModifiers(event.KeyModifierControl | event.KeyModifierShift)

	want := []event.KeyModifier{
		event.KeyModifierControl,
		event.KeyModifierControl | event.KeyModifierShift,
	}
	assert.Equal(t, want, a)
	assert.Equal(t, want, b)
	assert.Equal(t, event.KeyModifierControl|event.KeyModifierShift, d.CurrentKeyModifiers())

	removeA()
	removeA() // removing twice is harmless
	d.setCurrentKeyModifiers(0)
	assert.Equal(t, want, a, "removed listener no longer fires")
	assert.Equal(t, append(want, 0), b, "remaining listener still fires")
}
