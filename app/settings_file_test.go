//go:build !wasm

// These tests assert the on-disk settings load: loadFromFile on
// testdata/*.pxf, which settings_wasm.go stubs to a no-op — and the browser
// has no testdata directory to read from anyway. Portable settings tests
// stay untagged in settings_test.go (#77's split convention; #79).
// A localStorage-backed wasm counterpart is future feature work, tracked in
// the issue filed from #79.

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/belluikit/bellui"
	internalapp "github.com/belluikit/bellui/internal/app"
	internalTest "github.com/belluikit/bellui/internal/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingsLoad(t *testing.T) {
	settings := &settings{}

	err := settings.loadFromFile(filepath.Join("testdata", "light-theme.pxf"))
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "light", settings.schema.ThemeName)

	err = settings.loadFromFile(filepath.Join("testdata", "dark-theme.pxf"))
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "dark", settings.schema.ThemeName)
}

func TestOverrideTheme_IgnoresSettingsChange(t *testing.T) {
	// check that a file-load does not overwrite our value
	set := &settings{}
	err := os.Setenv("BELLUI_THEME", "light")
	if err != nil {
		t.Error(err)
	}
	set.setupTheme()
	assert.Equal(t, theme.DefaultTheme(), set.Theme())
	assert.Equal(t, theme.VariantLight, set.ThemeVariant())

	err = set.loadFromFile(filepath.Join("testdata", "dark-theme.pxf"))
	if err != nil {
		t.Error(err)
	}
	set.setupTheme()
	assert.Equal(t, theme.DefaultTheme(), set.Theme())
	assert.Equal(t, theme.VariantLight, set.ThemeVariant())
	err = os.Setenv("BELLUI_THEME", "")
	if err != nil {
		t.Error(err)
	}
}

func TestCustomTheme(t *testing.T) {
	type customTheme struct {
		bellui.Theme
	}
	set := &settings{}
	ctheme := &customTheme{internalTest.LightTheme(theme.DefaultTheme())}
	set.SetTheme(ctheme)

	set.setupTheme()
	assert.Equal(t, set.Theme(), ctheme)
	assert.Equal(t, internalapp.DefaultVariant(), set.ThemeVariant())

	err := set.loadFromFile(filepath.Join("testdata", "light-theme.pxf"))
	if err != nil {
		t.Error(err)
	}
	set.setupTheme()
	assert.Equal(t, set.Theme(), ctheme)
	assert.Equal(t, theme.VariantLight, set.ThemeVariant())

	err = set.loadFromFile(filepath.Join("testdata", "dark-theme.pxf"))
	if err != nil {
		t.Error(err)
	}
	set.setupTheme()
	assert.Equal(t, set.Theme(), ctheme)
	assert.Equal(t, theme.VariantDark, set.ThemeVariant())

	err = os.Setenv("BELLUI_THEME", "light")
	if err != nil {
		t.Error(err)
	}
	set.setupTheme()
	assert.Equal(t, set.Theme(), ctheme)
	assert.Equal(t, theme.VariantLight, set.ThemeVariant())

	err = os.Setenv("BELLUI_THEME", "dark")
	if err != nil {
		t.Error(err)
	}
	set.setupTheme()
	assert.Equal(t, set.Theme(), ctheme)
	assert.Equal(t, theme.VariantDark, set.ThemeVariant())

	err = os.Setenv("BELLUI_THEME", "")
	if err != nil {
		t.Error(err)
	}
	set.setupTheme()
	assert.Equal(t, set.Theme(), ctheme)
	assert.Equal(t, theme.VariantDark, set.ThemeVariant())
}

// An in-app SetPrimaryColor (ThemeSelection.Apply's other half) survives a
// reload of the system-wide settings file, which replaced the whole schema,
// primary colour included (found alongside #339's variant reset).
func TestSettings_ExplicitPrimarySurvivesFileReload(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.pxf")
	require.NoError(t, os.WriteFile(p, []byte("primary_color = \"blue\"\n"), 0o600))

	set := &settings{}
	set.SetPrimaryColor(theme.ColorRed)
	require.NoError(t, set.loadFromFile(p))
	assert.Equal(t, theme.ColorRed, set.PrimaryColor(), "file reload overrode the app's explicit primary")

	// Without an explicit choice the file's colour applies.
	set = &settings{}
	require.NoError(t, set.loadFromFile(p))
	assert.Equal(t, "blue", set.PrimaryColor())
}
