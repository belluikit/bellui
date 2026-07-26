package theme_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
)

func Test_BackgroundColor(t *testing.T) {
	t.Run("dark theme", func(t *testing.T) {
		bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
		assert.Equal(t, theme.Current().Color(theme.ColorNameBackground, theme.VariantDark), theme.Color(theme.ColorNameBackground), "wrong dark theme background color")
	})
	t.Run("light theme", func(t *testing.T) {
		bellui.CurrentApp().Settings().SetTheme(theme.LightTheme())
		bg := theme.Color(theme.ColorNameBackground)
		assert.Equal(t, theme.Current().Color(theme.ColorNameBackground, theme.VariantLight), bg, "wrong light theme background color")
	})
}

func Test_ButtonColor(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	c := theme.Color(theme.ColorNameButton)
	assert.Equal(t, theme.DarkTheme().Color(theme.ColorNameButton, theme.VariantDark), c, "wrong button color")
}

func Test_DisabledTextColor(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	c := theme.Color(theme.ColorNameDisabled)
	assert.Equal(t, theme.DarkTheme().Color(theme.ColorNameDisabled, theme.VariantDark), c, "wrong disabled text color")
}

func Test_FocusColor(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	c := theme.Color(theme.ColorNameFocus)
	assert.Equal(t, theme.DarkTheme().Color(theme.ColorNameFocus, theme.VariantDark), c, "wrong focus color")
}

func Test_HoverColor(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	c := theme.Color(theme.ColorNameHover)
	assert.Equal(t, theme.DarkTheme().Color(theme.ColorNameHover, theme.VariantDark), c, "wrong hover color")
}

func Test_PlaceHolderColor(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	c := theme.Color(theme.ColorNamePlaceHolder)
	assert.Equal(t, theme.DarkTheme().Color(theme.ColorNamePlaceHolder, theme.VariantDark), c, "wrong placeholder color")
}

func Test_PrimaryColor(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	c := theme.Color(theme.ColorNamePrimary)
	assert.Equal(t, theme.DarkTheme().Color(theme.ColorNamePrimary, theme.VariantDark), c, "wrong primary color")
}

func Test_ScrollBarColor(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	c := theme.Color(theme.ColorNameScrollBar)
	assert.Equal(t, theme.DarkTheme().Color(theme.ColorNameScrollBar, theme.VariantDark), c, "wrong scrollbar color")
}

func Test_TextColor(t *testing.T) {
	bellui.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	c := theme.Color(theme.ColorNameForeground)
	assert.Equal(t, theme.DarkTheme().Color(theme.ColorNameForeground, theme.VariantDark), c, "wrong text color")
}
