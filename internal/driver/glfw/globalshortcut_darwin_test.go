//go:build darwin

package glfw

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/event"
)

func TestMacCarbonModifiers(t *testing.T) {
	const (
		cmdKey     = 0x0100
		shiftKey   = 0x0200
		optionKey  = 0x0800
		controlKey = 0x1000
	)

	assert.Equal(t, uint32(0), macCarbonModifiers(0))
	assert.Equal(t, uint32(cmdKey), macCarbonModifiers(event.KeyModifierSuper))
	assert.Equal(t, uint32(shiftKey), macCarbonModifiers(event.KeyModifierShift))
	assert.Equal(t, uint32(optionKey), macCarbonModifiers(event.KeyModifierAlt))
	assert.Equal(t, uint32(controlKey), macCarbonModifiers(event.KeyModifierControl))
	assert.Equal(t, uint32(cmdKey|shiftKey),
		macCarbonModifiers(event.KeyModifierSuper|event.KeyModifierShift))
}

func TestMacVirtualKeyCodes(t *testing.T) {
	// Spot-check a few well-known kVK_ codes.
	assert.Equal(t, uint32(0x00), macVirtualKeyCodes[event.KeyA])
	assert.Equal(t, uint32(0x31), macVirtualKeyCodes[event.KeySpace])
	assert.Equal(t, uint32(0x7A), macVirtualKeyCodes[event.KeyF1])
	assert.Equal(t, uint32(0x1D), macVirtualKeyCodes[event.Key0])

	// Every letter and digit is mappable.
	for _, k := range []event.KeyName{
		event.KeyA, event.KeyM, event.KeyZ, event.Key0, event.Key9,
	} {
		_, ok := macVirtualKeyCodes[k]
		assert.True(t, ok, "expected %q to be mappable", k)
	}

	// A key with no hotkey mapping is absent.
	_, ok := macVirtualKeyCodes[event.KeyInsert]
	assert.False(t, ok)
}
