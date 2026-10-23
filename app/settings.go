package app

import (
	"os"
	"path/filepath"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/bellui/theme"
)

// SettingsSchema is used for loading and storing global settings
type SettingsSchema struct {
	// these items are used for global settings load
	ThemeName         string  `json:"theme"`
	Scale             float32 `json:"scale"`
	PrimaryColor      string  `json:"primary_color"`
	CloudName         string  `json:"cloud_name"`
	CloudConfig       string  `json:"cloud_config"`
	DisableAnimations bool    `json:"no_animations"`
}

// StoragePath returns the location of the settings storage — the PXF store
// (settings.pxf) that replaced the legacy settings.json (pre-rebrand tracker #872).
func (sc *SettingsSchema) StoragePath() string {
	return filepath.Join(app.RootConfigDir(), "settings.pxf")
}

// Declare conformity with Settings interface
var (
	_ bellui.Settings             = (*settings)(nil)
	_ bellui.PrimaryColorSettings = (*settings)(nil)
)

type settings struct {
	theme          bellui.Theme
	themeSpecified bool
	variant        bellui.ThemeVariant
	// variantSpecified records an explicit SetThemeVariant: like
	// themeSpecified for the theme, it keeps setupTheme (run on OS appearance
	// changes and settings-file reloads) from replacing the app's choice.
	variantSpecified bool
	// primary is the colour from an explicit SetPrimaryColor, kept apart from
	// schema so a settings-file reload does not replace it.
	primary          string
	primarySpecified bool

	listeners []func(bellui.Settings)
	watcher   any // normally *fsnotify.Watcher or nil - avoid import in this file

	schema SettingsSchema
}

func (s *settings) BuildType() bellui.BuildType {
	return build.Mode
}

func (s *settings) PrimaryColor() string {
	if s.primarySpecified {
		return s.primary
	}
	return s.schema.PrimaryColor
}

// SetPrimaryColor sets the named primary color preference for the running app
// and re-applies the current theme. Unlike the settings app it does not write
// the system-wide settings file — apps that want to remember the choice should
// store it in their own Preferences (see widget.SaveThemeSelection). The choice
// is kept when the system-wide settings file changes.
//
// Since: 1.0
func (s *settings) SetPrimaryColor(name string) {
	s.primary = name
	s.primarySpecified = true
	s.apply()
}

func (s *settings) Theme() bellui.Theme {
	if s == nil {
		bellui.LogError("Attempt to access current Bellui theme when no app is started", nil)
		return nil
	}
	return s.theme
}

func (s *settings) SetTheme(theme bellui.Theme) {
	s.themeSpecified = true
	s.applyTheme(theme, s.variant)
}

func (s *settings) ShowAnimations() bool {
	return !s.schema.DisableAnimations && !build.NoAnimations
}

func (s *settings) ThemeVariant() bellui.ThemeVariant {
	return s.variant
}

// SetThemeVariant sets the variant for the running app. The choice is kept
// across later OS appearance changes and settings-file reloads, as SetTheme's
// is.
func (s *settings) SetThemeVariant(variant bellui.ThemeVariant) {
	s.variantSpecified = true
	s.applyTheme(s.theme, variant)
}

func (s *settings) applyTheme(theme bellui.Theme, variant bellui.ThemeVariant) {
	s.variant = variant
	s.theme = theme
	s.apply()
}

func (s *settings) Scale() float32 {
	if s.schema.Scale < 0.0 {
		return 1.0 // catching any really old data still using the `-1`  value for "auto" scale
	}
	return s.schema.Scale
}

func (s *settings) AddListener(listener func(bellui.Settings)) {
	s.listeners = append(s.listeners, listener)
}

func (s *settings) apply() {
	for _, l := range s.listeners {
		l(s)
	}
}

func (s *settings) fileChanged() {
	s.load()
	s.apply()
}

func (s *settings) setupTheme() {
	name := s.schema.ThemeName
	if env := os.Getenv("BELLUI_THEME"); env != "" {
		name = env
	}

	variant := app.DefaultVariant()
	effectiveTheme := s.theme
	if !s.themeSpecified {
		effectiveTheme = theme.DefaultTheme()
	}
	switch name {
	case "light":
		variant = theme.VariantLight
	case "dark":
		variant = theme.VariantDark
	}
	if s.variantSpecified {
		variant = s.variant
	}

	s.applyTheme(effectiveTheme, variant)
}

func loadSettings() *settings {
	s := &settings{}
	s.load()

	return s
}
