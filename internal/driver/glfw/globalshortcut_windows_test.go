//go:build windows

package glfw

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/event"
)

func TestWinHotKeyModifiers(t *testing.T) {
	assert.Equal(t, uint32(0), winHotKeyModifiers(0))
	assert.Equal(t, uint32(modWin), winHotKeyModifiers(event.KeyModifierSuper))
	assert.Equal(t, uint32(modShift), winHotKeyModifiers(event.KeyModifierShift))
	assert.Equal(t, uint32(modAlt), winHotKeyModifiers(event.KeyModifierAlt))
	assert.Equal(t, uint32(modControl), winHotKeyModifiers(event.KeyModifierControl))
	assert.Equal(t, uint32(modControl|modShift),
		winHotKeyModifiers(event.KeyModifierControl|event.KeyModifierShift))
}

func TestWinVirtualKeyCodes(t *testing.T) {
	// Spot-check a few well-known VK_ codes.
	assert.Equal(t, uint32(0x41), winVirtualKeyCodes[event.KeyA])
	assert.Equal(t, uint32(0x20), winVirtualKeyCodes[event.KeySpace])
	assert.Equal(t, uint32(0x70), winVirtualKeyCodes[event.KeyF1])
	assert.Equal(t, uint32(0x30), winVirtualKeyCodes[event.Key0])

	// Every letter and digit is mappable.
	for _, k := range []event.KeyName{
		event.KeyA, event.KeyM, event.KeyZ, event.Key0, event.Key9,
	} {
		_, ok := winVirtualKeyCodes[k]
		assert.True(t, ok, "expected %q to be mappable", k)
	}

	// A key with no hotkey mapping is absent.
	_, ok := winVirtualKeyCodes[event.KeyInsert]
	assert.False(t, ok)
}
