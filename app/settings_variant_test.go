package app

import (
	"testing"

	"github.com/belluikit/bellui"
	internalapp "github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

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

// Choosing System in a ThemeSelection follows the OS live, while Light and
// Dark stay pinned under the same OS change (#695). Before, Apply resolved
// System with SetThemeVariant, which pinned the OS appearance of that moment.
func TestSettings_ThemeSelectionSystemFollowsOS(t *testing.T) {
	os := theme.VariantLight
	osVariant = func() bellui.ThemeVariant { return os }
	t.Cleanup(func() { osVariant = internalapp.DefaultVariant })

	set := &settings{}
	set.setupTheme()
	prev := bellui.CurrentApp()
	bellui.SetCurrentApp(&settingsOnlyApp{set: set})
	t.Cleanup(func() { bellui.SetCurrentApp(prev) })

	changeOS := func(v bellui.ThemeVariant) {
		os = v
		set.setupTheme() // what an OS appearance change runs
	}

	widget.ThemeSelection{Variant: widget.ThemeVariantDark}.Apply()
	changeOS(theme.VariantLight)
	assert.Equal(t, theme.VariantDark, set.ThemeVariant(), "Dark did not stay pinned")

	// System after a pin withdraws it: the OS appearance applies at once…
	widget.ThemeSelection{Variant: widget.ThemeVariantSystem}.Apply()
	assert.Equal(t, theme.VariantLight, set.ThemeVariant())
	// …and every later OS change is followed.
	changeOS(theme.VariantDark)
	assert.Equal(t, theme.VariantDark, set.ThemeVariant(), "System did not follow the OS")
	changeOS(theme.VariantLight)
	assert.Equal(t, theme.VariantLight, set.ThemeVariant(), "System did not follow the OS back")

	widget.ThemeSelection{Variant: widget.ThemeVariantLight}.Apply()
	changeOS(theme.VariantDark)
	assert.Equal(t, theme.VariantLight, set.ThemeVariant(), "Light did not stay pinned")
}

// settingsOnlyApp is the current app for ThemeSelection.Apply, which reads
// nothing but Settings.
type settingsOnlyApp struct {
	bellui.App
	set *settings
}

func (a *settingsOnlyApp) Settings() bellui.Settings { return a.set }
