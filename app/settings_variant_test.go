package app

import (
	"os"
	"path/filepath"
	"testing"

	internalapp "github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An explicit SetThemeVariant (what widget.ThemeSelection.Apply calls) survives
// an OS appearance change. The platform callbacks (macOS themeChanged, mobile
// configuration change, wasm prefers-color-scheme, settings-file reload) all
// run setupTheme, which reset the variant to the OS value (#339).
func TestSettings_ExplicitVariantSurvivesOSChange(t *testing.T) {
	set := &settings{}
	set.setupTheme()

	chosen := theme.VariantDark
	if internalapp.DefaultVariant() == theme.VariantDark {
		chosen = theme.VariantLight
	}
	set.SetThemeVariant(chosen)
	assert.Equal(t, chosen, set.ThemeVariant())

	set.setupTheme() // what an OS appearance change runs
	assert.Equal(t, chosen, set.ThemeVariant(), "OS change overrode the app's explicit variant")

	// Nor does the system-wide settings file, as with an explicit SetTheme.
	set.schema.ThemeName = "light"
	if chosen == theme.VariantLight {
		set.schema.ThemeName = "dark"
	}
	set.setupTheme()
	assert.Equal(t, chosen, set.ThemeVariant())
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
