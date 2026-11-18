package widget

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// TestButton_DangerInEveryStyle is #1327: DangerImportance only coloured a
// ButtonStyleDefault button, so a destructive action could not be a red text
// or outlined button. It now colours the label, the icon and the outline in
// every style.
func TestButton_DangerInEveryStyle(t *testing.T) {
	test.NewTempApp(t)
	for _, tc := range []struct {
		name    string
		style   ButtonStyle
		fg, bg  bellui.ThemeColorName
		outline bellui.ThemeColorName
	}{
		{"default", ButtonStyleDefault, theme.ColorNameForegroundOnError, theme.ColorNameError, ""},
		{"filled", ButtonStyleFilled, theme.ColorNameForegroundOnError, theme.ColorNameError, ""},
		{"filled tonal", ButtonStyleFilledTonal, theme.ColorNameOnErrorContainer, theme.ColorNameErrorContainer, ""},
		{"elevated", ButtonStyleElevated, theme.ColorNameError, theme.ColorNameSurfaceContainerLow, ""},
		{"outlined", ButtonStyleOutlined, theme.ColorNameError, theme.ColorNameButton, theme.ColorNameError},
		{"text", ButtonStyleText, theme.ColorNameError, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := NewButtonWithIcon("Delete", theme.DeleteIcon(), nil)
			b.Style = tc.style
			b.Importance = DangerImportance
			r := test.TempWidgetRenderer(t, b).(*buttonRenderer)

			assert.Equal(t, tc.fg, r.label.Segments[0].(*TextSegment).Style.ColorName, "label")
			assert.Equal(t, tc.fg, themeColorOf(t, r.icon), "icon")
			assert.Equal(t, colorOf(tc.bg), r.background.FillColor, "background")
			if tc.outline != "" {
				assert.Equal(t, colorOf(tc.outline), r.background.StrokeColor, "outline")
			}
		})
	}
}

// A disabled Danger button draws as any disabled button does: the outline of
// an Outlined one is not left red.
func TestButton_DangerDisabled(t *testing.T) {
	test.NewTempApp(t)
	b := NewButton("Delete", nil)
	b.Style = ButtonStyleOutlined
	b.Importance = DangerImportance
	b.Disable()
	r := test.TempWidgetRenderer(t, b).(*buttonRenderer)

	assert.Equal(t, theme.ColorNameDisabled, r.label.Segments[0].(*TextSegment).Style.ColorName)
	assert.Equal(t, colorOf(theme.ColorNameOutline), r.background.StrokeColor)
}

// The other importances still apply to ButtonStyleDefault only.
func TestButton_NonDangerImportanceIgnoredOutsideDefault(t *testing.T) {
	test.NewTempApp(t)
	b := NewButton("Save", nil)
	b.Style = ButtonStyleText
	b.Importance = SuccessImportance
	r := test.TempWidgetRenderer(t, b).(*buttonRenderer)

	assert.Equal(t, theme.ColorNamePrimary, r.label.Segments[0].(*TextSegment).Style.ColorName)
}

// TestIconButton_Danger is #1327: an icon button had no importance, so a
// trash icon could not be red.
func TestIconButton_Danger(t *testing.T) {
	test.NewTempApp(t)
	for _, tc := range []struct {
		name    string
		style   IconButtonStyle
		fg, bg  bellui.ThemeColorName
		outline bellui.ThemeColorName
	}{
		{"standard", IconButtonStyleStandard, theme.ColorNameError, theme.ColorNameButton, ""},
		{"filled", IconButtonStyleFilled, theme.ColorNameForegroundOnError, theme.ColorNameError, ""},
		{"filled tonal", IconButtonStyleFilledTonal, theme.ColorNameOnErrorContainer, theme.ColorNameErrorContainer, ""},
		{"outlined", IconButtonStyleOutlined, theme.ColorNameError, "", theme.ColorNameError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := NewIconButton(theme.DeleteIcon(), nil)
			b.Style = tc.style
			b.Importance = DangerImportance
			r := cache.Renderer(b).(*iconButtonRenderer)

			assert.Equal(t, tc.fg, themeColorOf(t, r.icon), "icon")
			assert.Equal(t, colorOf(tc.fg), r.contentColor(), "press colour")
			assert.Equal(t, colorOf(tc.bg), r.bg.FillColor, "background")
			if tc.outline != "" {
				assert.Equal(t, colorOf(tc.outline), r.bg.StrokeColor, "outline")
			}
		})
	}
}

// Without an importance an icon button keeps its style's colours.
func TestIconButton_MediumImportanceKeepsStyleColours(t *testing.T) {
	test.NewTempApp(t)
	b := NewIconButton(theme.DeleteIcon(), nil)
	r := cache.Renderer(b).(*iconButtonRenderer)
	assert.Equal(t, theme.ColorNameOnSurfaceVariant, themeColorOf(t, r.icon))
}

// themeColorOf is the colour name an image's themed resource is drawn in.
func themeColorOf(t *testing.T, img *canvas.Image) bellui.ThemeColorName {
	t.Helper()
	require.NotNil(t, img)
	res, ok := img.Resource.(bellui.ThemedResource)
	require.True(t, ok, "icon resource %T is not themed", img.Resource)
	return res.ThemeColorName()
}

// colorOf is the current theme's colour for name; the empty name is
// transparent, as the renderers draw it.
func colorOf(name bellui.ThemeColorName) color.Color {
	if name == "" {
		return color.Transparent
	}
	return theme.Color(name)
}
