package theme

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
)

var oldTheme = &legacyTheme{}

func TestFromLegacy(t *testing.T) {
	newTheme := FromLegacy(oldTheme)
	assert.NotNil(t, newTheme)
	assert.Equal(t, oldTheme, newTheme.(*legacyWrapper).old)
}

func TestLegacyWrapper_Color(t *testing.T) {
	newTheme := FromLegacy(oldTheme)
	assert.Equal(t, oldTheme.BackgroundColor(), newTheme.Color(ColorNameBackground, VariantLight))
	assert.Equal(t, oldTheme.ShadowColor(), newTheme.Color(ColorNameShadow, VariantLight))
	assert.Equal(t, oldTheme.TextColor(), newTheme.Color(ColorNameForeground, VariantLight))
}

func TestLegacyWrapper_Font(t *testing.T) {
	newTheme := FromLegacy(oldTheme)
	assert.Equal(t, oldTheme.TextFont(), newTheme.Font(bellui.TextStyle{}))
	assert.Equal(t, oldTheme.TextBoldFont(), newTheme.Font(bellui.TextStyle{Bold: true}))
	assert.Equal(t, oldTheme.TextItalicFont(), newTheme.Font(bellui.TextStyle{Italic: true}))
	assert.Equal(t, oldTheme.TextMonospaceFont(), newTheme.Font(bellui.TextStyle{Monospace: true}))
}

func TestLegacyWrapper_Size(t *testing.T) {
	newTheme := FromLegacy(oldTheme)
	assert.Equal(t, oldTheme.IconInlineSize(), int(newTheme.Size(SizeNameInlineIcon)))
	assert.Equal(t, oldTheme.Padding(), int(newTheme.Size(SizeNamePadding)))
	assert.Equal(t, oldTheme.TextSize(), int(newTheme.Size(SizeNameText)))
}

var _ bellui.LegacyTheme = (*legacyTheme)(nil)

type legacyTheme struct{}

func (t *legacyTheme) BackgroundColor() color.Color {
	return Color(ColorNameBackground)
}

func (t *legacyTheme) ButtonColor() color.Color {
	return Color(ColorNameButton)
}

func (t *legacyTheme) DisabledButtonColor() color.Color {
	return Color(ColorNameDisabledButton)
}

func (t *legacyTheme) DisabledTextColor() color.Color {
	return Color(ColorNameDisabled)
}

func (t *legacyTheme) FocusColor() color.Color {
	return Color(ColorNameFocus)
}

func (t *legacyTheme) HoverColor() color.Color {
	return Color(ColorNameHover)
}

func (t *legacyTheme) PlaceHolderColor() color.Color {
	return Color(ColorNamePlaceHolder)
}

func (t *legacyTheme) PrimaryColor() color.Color {
	return Color(ColorNamePrimary)
}

func (t *legacyTheme) ScrollBarColor() color.Color {
	return Color(ColorNameScrollBar)
}

func (t *legacyTheme) ShadowColor() color.Color {
	return Color(ColorNameShadow)
}

func (t *legacyTheme) TextColor() color.Color {
	return Color(ColorNameForeground)
}

func (t *legacyTheme) TextSize() int {
	return int(TextSize())
}

func (t *legacyTheme) TextFont() bellui.Resource {
	return TextFont()
}

func (t *legacyTheme) TextBoldFont() bellui.Resource {
	return TextBoldFont()
}

func (t *legacyTheme) TextItalicFont() bellui.Resource {
	return TextItalicFont()
}

func (t *legacyTheme) TextBoldItalicFont() bellui.Resource {
	return TextBoldItalicFont()
}

func (t *legacyTheme) TextMonospaceFont() bellui.Resource {
	return TextMonospaceFont()
}

func (t *legacyTheme) Padding() int {
	return int(Padding())
}

func (t *legacyTheme) IconInlineSize() int {
	return int(IconInlineSize())
}

func (t *legacyTheme) ScrollBarSize() int {
	return int(ScrollBarSize())
}

func (t *legacyTheme) ScrollBarSmallSize() int {
	return int(ScrollBarSmallSize())
}
