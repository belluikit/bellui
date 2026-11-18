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

// statusCases are the status importances and their roles: colour, contrast
// colour, and tonal container pair.
var statusCases = []struct {
	name                         string
	imp                          Importance
	col, on, container, onContnr bellui.ThemeColorName
}{
	{"danger", DangerImportance, theme.ColorNameError, theme.ColorNameForegroundOnError, theme.ColorNameErrorContainer, theme.ColorNameOnErrorContainer},
	{"success", SuccessImportance, theme.ColorNameSuccess, theme.ColorNameForegroundOnSuccess, theme.ColorNameSuccess, theme.ColorNameForegroundOnSuccess},
	{"warning", WarningImportance, theme.ColorNameWarning, theme.ColorNameForegroundOnWarning, theme.ColorNameWarning, theme.ColorNameForegroundOnWarning},
}

// TestButton_DangerInEveryStyle is #1327: DangerImportance only coloured a
// ButtonStyleDefault button, so a destructive action could not be a red text
// or outlined button. It now colours the label, the icon and the outline in
// every style, and so do Success and Warning.
func TestButton_DangerInEveryStyle(t *testing.T) {
	test.NewTempApp(t)
	for _, st := range statusCases {
		for _, tc := range []struct {
			name    string
			style   ButtonStyle
			fg, bg  bellui.ThemeColorName
			outline bellui.ThemeColorName
		}{
			{"default", ButtonStyleDefault, st.on, st.col, ""},
			{"filled", ButtonStyleFilled, st.on, st.col, ""},
			{"filled tonal", ButtonStyleFilledTonal, st.onContnr, st.container, ""},
			{"elevated", ButtonStyleElevated, st.col, theme.ColorNameSurfaceContainerLow, ""},
			{"outlined", ButtonStyleOutlined, st.col, theme.ColorNameButton, st.col},
			{"text", ButtonStyleText, st.col, "", ""},
		} {
			t.Run(st.name+"/"+tc.name, func(t *testing.T) {
				b := NewButtonWithIcon("Delete", theme.DeleteIcon(), nil)
				b.Style = tc.style
				b.Importance = st.imp
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

// High and Low still apply to ButtonStyleDefault only.
func TestButton_PlainImportanceIgnoredOutsideDefault(t *testing.T) {
	test.NewTempApp(t)
	for _, imp := range []Importance{HighImportance, LowImportance} {
		b := NewButton("Save", nil)
		b.Style = ButtonStyleOutlined
		b.Importance = imp
		r := test.TempWidgetRenderer(t, b).(*buttonRenderer)

		assert.Equal(t, theme.ColorNamePrimary, r.label.Segments[0].(*TextSegment).Style.ColorName)
		assert.Equal(t, colorOf(theme.ColorNameOutline), r.background.StrokeColor)
	}
}

// TestIconButton_Danger is #1327: an icon button had no importance, so a
// trash icon could not be red. Success and Warning colour it the same way.
func TestIconButton_Danger(t *testing.T) {
	test.NewTempApp(t)
	for _, st := range statusCases {
		for _, tc := range []struct {
			name    string
			style   IconButtonStyle
			fg, bg  bellui.ThemeColorName
			outline bellui.ThemeColorName
		}{
			{"standard", IconButtonStyleStandard, st.col, theme.ColorNameButton, ""},
			{"filled", IconButtonStyleFilled, st.on, st.col, ""},
			{"filled tonal", IconButtonStyleFilledTonal, st.onContnr, st.container, ""},
			{"outlined", IconButtonStyleOutlined, st.col, "", st.col},
		} {
			t.Run(st.name+"/"+tc.name, func(t *testing.T) {
				b := NewIconButton(theme.DeleteIcon(), nil)
				b.Style = tc.style
				b.Importance = st.imp
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
}

// A disabled Danger icon button draws as any disabled one does: the outline
// of an Outlined one is not left red.
func TestIconButton_DangerDisabled(t *testing.T) {
	test.NewTempApp(t)
	b := NewIconButton(theme.DeleteIcon(), nil)
	b.Style = IconButtonStyleOutlined
	b.Importance = DangerImportance
	b.Disable()
	r := cache.Renderer(b).(*iconButtonRenderer)

	assert.Equal(t, theme.ColorNameDisabled, themeColorOf(t, r.icon))
	assert.Equal(t, colorOf(theme.ColorNameOutlineVariant), r.bg.StrokeColor)
}

// Medium, High and Low keep the style's colours.
func TestIconButton_PlainImportanceKeepsStyleColours(t *testing.T) {
	test.NewTempApp(t)
	for _, imp := range []Importance{MediumImportance, HighImportance, LowImportance} {
		b := NewIconButton(theme.DeleteIcon(), nil)
		b.Style = IconButtonStyleOutlined
		b.Importance = imp
		r := cache.Renderer(b).(*iconButtonRenderer)
		assert.Equal(t, theme.ColorNameOnSurfaceVariant, themeColorOf(t, r.icon))
		assert.Equal(t, colorOf(theme.ColorNameOutlineVariant), r.bg.StrokeColor)
	}
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
