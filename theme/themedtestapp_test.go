package theme_test

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
)

type themedApp struct {
	primaryColor string
	theme        bellui.Theme
	variant      bellui.ThemeVariant
}

func (t *themedApp) BuildType() bellui.BuildType {
	return bellui.BuildStandard
}

func (t *themedApp) NewWindow(title string) bellui.Window {
	return nil
}

func (t *themedApp) Icon() bellui.Resource {
	return nil
}

func (t *themedApp) SetIcon(bellui.Resource) {
}

func (t *themedApp) Run() {
}

func (t *themedApp) Quit() {
}

func (t *themedApp) Driver() bellui.Driver {
	return nil
}

func (t *themedApp) UniqueID() string {
	return ""
}

func (t *themedApp) Settings() bellui.Settings {
	return t
}

func (t *themedApp) Storage() bellui.Storage {
	return nil
}

func (t *themedApp) Preferences() bellui.Preferences {
	return nil
}

func (t *themedApp) Lifecycle() bellui.Lifecycle {
	return nil
}

func (t *themedApp) Metadata() bellui.AppMetadata {
	return bellui.AppMetadata{}
}

func (t *themedApp) PrimaryColor() string {
	if t.primaryColor != "" {
		return t.primaryColor
	}

	return theme.ColorBlue
}

func (t *themedApp) Theme() bellui.Theme {
	return t.theme
}

func (t *themedApp) SetTheme(theme bellui.Theme) {
	t.theme = theme
}

func (t *themedApp) ThemeVariant() bellui.ThemeVariant {
	return t.variant // The null value is theme.VariantDark
}

func (t *themedApp) SetThemeVariant(v bellui.ThemeVariant) {
	t.variant = v
}

func (t *themedApp) Scale() float32 {
	return 1.0
}

func (t *themedApp) ShowAnimations() bool {
	return true
}

func (t *themedApp) AddChangeListener(chan bellui.Settings) {
}

func (t *themedApp) AddListener(func(bellui.Settings)) {
}

func (t *themedApp) Clipboard() bellui.Clipboard {
	return nil
}
