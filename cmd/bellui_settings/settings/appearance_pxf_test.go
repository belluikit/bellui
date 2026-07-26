package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/belluikit/bellui/app"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSettings_SaveLoadPXF exercises the real writer path: saveToFile writes the
// PXF store, and app's loadFromFile reads it back (#872).
func TestSettings_SaveLoadPXF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.pxf")

	s := &Settings{belluiSettings: app.SettingsSchema{
		ThemeName:         "dark",
		Scale:             1.5,
		PrimaryColor:      "blue",
		DisableAnimations: true,
	}}
	require.NoError(t, s.saveToFile(path))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// It is valid PXF that decodes back to the same values via the shared codec.
	var out app.SettingsSchema
	require.NoError(t, out.UnmarshalPXF(data))
	assert.Equal(t, s.belluiSettings, out)

	// No temp file is left behind by the atomic write.
	_, err = os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err))
}

// TestSettings_SaveRemovesLegacyJSON verifies a stale settings.json left by the
// pre-#872 build is deleted once the PXF store is written.
func TestSettings_SaveRemovesLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.pxf")
	legacy := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(legacy, []byte(`{"theme":"light"}`), 0o644))

	s := &Settings{belluiSettings: app.SettingsSchema{ThemeName: "dark"}}
	require.NoError(t, s.saveToFile(path))

	_, err := os.Stat(legacy)
	assert.True(t, os.IsNotExist(err), "legacy settings.json should be removed after a PXF save")
}
