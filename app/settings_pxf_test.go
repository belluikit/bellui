package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSettingsSchema_PXFRoundTrip verifies every SettingsSchema field survives a
// MarshalPXF/UnmarshalPXF round-trip (the settings.pxf store, pre-rebrand tracker #872).
func TestSettingsSchema_PXFRoundTrip(t *testing.T) {
	in := &SettingsSchema{
		ThemeName:         "dark",
		Scale:             1.25,
		PrimaryColor:      "orange",
		CloudName:         "myprovider",
		CloudConfig:       "config-blob",
		DisableAnimations: true,
	}

	data, err := in.MarshalPXF()
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var out SettingsSchema
	require.NoError(t, out.UnmarshalPXF(data))
	assert.Equal(t, *in, out)
}

// TestSettingsSchema_PXFEmpty checks the zero value round-trips to the zero value
// (an absent field decodes to its Go zero, matching proto3 semantics).
func TestSettingsSchema_PXFEmpty(t *testing.T) {
	data, err := (&SettingsSchema{}).MarshalPXF()
	require.NoError(t, err)

	var out SettingsSchema
	require.NoError(t, out.UnmarshalPXF(data))
	assert.Equal(t, SettingsSchema{}, out)
}

// TestSettingsSchema_PXFInvalid checks that non-PXF bytes (e.g. the legacy JSON
// object the migration dropped) are rejected rather than silently accepted.
func TestSettingsSchema_PXFInvalid(t *testing.T) {
	var out SettingsSchema
	assert.Error(t, out.UnmarshalPXF([]byte(`{"theme":"dark"}`)))
}
