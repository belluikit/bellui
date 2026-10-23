package app

import (
	"testing"

	internalapp "github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
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
