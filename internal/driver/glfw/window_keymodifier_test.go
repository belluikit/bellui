//go:build !no_glfw && !ci && !mobile

package glfw

import (
	"testing"

	"github.com/belluikit/glfw"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

type modifierKeyRecorder struct {
	focusable
	keys  []*event.KeyEvent
	downs []*event.KeyEvent
}

func (r *modifierKeyRecorder) TypedKey(ev *event.KeyEvent) {
	r.keys = append(r.keys, ev)
}

func (r *modifierKeyRecorder) KeyDown(ev *event.KeyEvent) {
	r.downs = append(r.downs, ev)
}

func (r *modifierKeyRecorder) KeyUp(*event.KeyEvent) {
}

// TestWindow_TypedKeyCarriesModifier pins pre-rebrand tracker #1042: the KeyEvent delivered to a
// focused widget's TypedKey and to Canvas.SetOnTypedKey reports the modifiers
// held at event time, so a handler can tell Shift+F11 from F11. Control/Alt/
// Super combinations keep routing to the shortcut system before TypedKey
// (unchanged), but desktop.Keyable's KeyDown sees them with the modifier set.
func TestWindow_TypedKeyCarriesModifier(t *testing.T) {
	w := createWindow("Test")
	content := &modifierKeyRecorder{}
	content.SetMinSize(geo.NewSize(10, 10))
	w.SetContent(content)
	repaintWindow(w)

	w.Canvas().Focus(content)
	w.keyPressed(nil, glfw.KeyF11, 0, glfw.Press, 0)
	w.keyPressed(nil, glfw.KeyF11, 0, glfw.Press, glfw.ModShift)

	require.Len(t, content.keys, 2)
	assert.Equal(t, event.KeyF11, content.keys[0].Name)
	assert.Equal(t, event.KeyModifier(0), content.keys[0].Modifier)
	assert.Equal(t, event.KeyF11, content.keys[1].Name)
	assert.Equal(t, event.KeyModifierShift, content.keys[1].Modifier)

	// A Control combination is consumed as a shortcut before TypedKey, but the
	// KeyDown event (desktop.Keyable) still reports the held modifiers.
	w.keyPressed(nil, glfw.KeyF11, 0, glfw.Press, glfw.ModControl)
	require.Len(t, content.downs, 3)
	assert.Equal(t, event.KeyModifierControl, content.downs[2].Modifier)
	assert.Len(t, content.keys, 2, "Control combos keep routing to shortcuts, not TypedKey")

	var typed []*event.KeyEvent
	w.Canvas().Unfocus()
	w.Canvas().SetOnTypedKey(func(ev *event.KeyEvent) { typed = append(typed, ev) })
	w.keyPressed(nil, glfw.KeyF11, 0, glfw.Press, glfw.ModShift)

	require.Len(t, typed, 1)
	assert.Equal(t, event.KeyF11, typed[0].Name)
	assert.Equal(t, event.KeyModifierShift, typed[0].Modifier)
}
