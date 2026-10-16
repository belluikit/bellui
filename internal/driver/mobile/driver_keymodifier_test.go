package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/internal/driver/mobile/event/key"
)

func TestKeyEventModifier(t *testing.T) {
	assert.Equal(t, event.KeyModifier(0), keyEventModifier(0))
	assert.Equal(t, event.KeyModifierShift, keyEventModifier(key.ModShift))
	assert.Equal(t, event.KeyModifierControl, keyEventModifier(key.ModControl))
	assert.Equal(t, event.KeyModifierAlt, keyEventModifier(key.ModAlt))
	assert.Equal(t, event.KeyModifierSuper, keyEventModifier(key.ModMeta))
	assert.Equal(t, event.KeyModifierShift|event.KeyModifierControl,
		keyEventModifier(key.ModShift|key.ModControl))
}

type keyModRecorder struct {
	belluicanvas.Rectangle
	keys []*event.KeyEvent
}

func (r *keyModRecorder) FocusGained()   {}
func (r *keyModRecorder) FocusLost()     {}
func (r *keyModRecorder) TypedRune(rune) {}
func (r *keyModRecorder) TypedKey(ev *event.KeyEvent) {
	r.keys = append(r.keys, ev)
}

// Test_mobileDriver_TypedKeyCarriesModifier pins pre-rebrand tracker #1042 on the mobile driver:
// hardware-keyboard modifiers reported by the platform reach TypedKey.
func Test_mobileDriver_TypedKeyCarriesModifier(t *testing.T) {
	d := &driver{}
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	rec := &keyModRecorder{}
	c.SetContent(rec)
	c.Focus(rec)

	d.typeDownCanvas(c, 0, key.CodeA, 0)
	d.typeDownCanvas(c, 0, key.CodeA, key.ModShift|key.ModMeta)

	require.Len(t, rec.keys, 2)
	assert.Equal(t, event.KeyA, rec.keys[0].Name)
	assert.Equal(t, event.KeyModifier(0), rec.keys[0].Modifier)
	assert.Equal(t, event.KeyA, rec.keys[1].Name)
	assert.Equal(t, event.KeyModifierShift|event.KeyModifierSuper, rec.keys[1].Modifier)
}
