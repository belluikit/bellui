//go:build !wasm

package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/internal/driver/mobile/event/key"
)

// The mobile driver exposes hardware-keyboard modifiers via
// desktop.KeyModifierDriver so Entry can extend its selection on Shift+Arrow
// (pre-rebrand tracker #1083). A method-signature drift would break the widget's runtime assertion
// silently, so pin the interface at compile time.
var _ desktop.KeyModifierDriver = (*driver)(nil)

func Test_mobileDriver_CurrentKeyModifiers(t *testing.T) {
	d := &driver{}
	c := newCanvas(bellui.CurrentDevice()).(*canvas)

	assert.Equal(t, event.KeyModifier(0), d.CurrentKeyModifiers())

	// A key event's modifiers become the live set (a Shift+Arrow carries Shift).
	d.typeDownCanvas(c, 0, key.CodeRightArrow, key.ModShift)
	assert.Equal(t, event.KeyModifierShift, d.CurrentKeyModifiers())

	// A subsequent unmodified key clears it.
	d.typeDownCanvas(c, 0, key.CodeRightArrow, 0)
	assert.Equal(t, event.KeyModifier(0), d.CurrentKeyModifiers())
}

func Test_mobileDriver_KeyModifierListener(t *testing.T) {
	d := &driver{}
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	var got []event.KeyModifier
	remove := d.AddKeyModifierListener(func(m event.KeyModifier) { got = append(got, m) })

	d.typeDownCanvas(c, 0, key.CodeLeftArrow, key.ModShift) // 0 → Shift: fires
	d.typeDownCanvas(c, 0, key.CodeLeftArrow, key.ModShift) // Shift → Shift: no change
	d.typeDownCanvas(c, 0, key.CodeLeftArrow, 0)            // Shift → 0: fires
	assert.Equal(t, []event.KeyModifier{event.KeyModifierShift, 0}, got)

	remove()
	d.typeDownCanvas(c, 0, key.CodeLeftArrow, key.ModShift)
	assert.Len(t, got, 2, "removed listener is not called")
}
