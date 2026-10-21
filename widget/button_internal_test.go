package widget

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestButton_RefreshKeepsTextRenderer pins #446: refreshing a button whose
// text has not changed keeps its label's renderer and parses no font.
func TestButton_RefreshKeepsTextRenderer(t *testing.T) {
	painter.ClearFontCache()
	cache.ResetThemeCaches()
	parses := painter.FontParses()
	button := NewButton("Test", nil)
	render := test.TempWidgetRenderer(t, button).(*buttonRenderer)
	require.Greater(t, painter.FontParses(), parses, "the label was not measured with the painter's fonts")
	button.Refresh() // warm up

	label := cache.Renderer(render.label)
	parses = painter.FontParses()
	for range 10 {
		button.Refresh()
	}
	assert.Same(t, label, cache.Renderer(render.label), "the label's renderer was re-created")
	assert.Equal(t, parses, painter.FontParses(), "a font was parsed again")
}

func TestButton_Style(t *testing.T) {
	button := NewButton("Test", nil)
	render := test.TempWidgetRenderer(t, button).(*buttonRenderer)
	render.applyTheme()
	bg := render.background.FillColor

	button.Importance = HighImportance
	render.applyTheme()
	assert.NotEqual(t, bg, render.background.FillColor)
}

func TestButton_DisabledColor(t *testing.T) {
	button := NewButton("Test", nil)
	render := test.TempWidgetRenderer(t, button).(*buttonRenderer)
	render.applyTheme()
	bg := render.background.FillColor
	button.Importance = MediumImportance
	render.applyTheme()
	assert.Equal(t, bg, render.background.FillColor)

	button.Disable()
	render.applyTheme()
	assert.Equal(t, theme.Color(theme.ColorNameDisabledButton), render.background.FillColor)
}

func TestButton_Hover_Math(t *testing.T) {
	button := NewButtonWithIcon("Test", theme.HomeIcon(), func() {})
	render := test.TempWidgetRenderer(t, button).(*buttonRenderer)
	button.hovered = true

	// Use the renderer's own color-name resolution so the test stays valid
	// across theme and style changes (e.g. MD3 FilledTonal default).
	th := render.button.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	fgName, bgName, _ := render.buttonColorNames()

	bg := th.Color(bgName, v)
	fgColor := th.Color(fgName, v)
	expected := blendColor(bg, theme.StateLayerHover(fgColor))

	render.applyTheme()

	assert.Equal(t, color.NRGBAModel.Convert(expected), color.NRGBAModel.Convert(render.background.FillColor))
}

func TestButton_DisabledIcon(t *testing.T) {
	button := NewButtonWithIcon("Test", theme.CancelIcon(), nil)
	render := test.TempWidgetRenderer(t, button).(*buttonRenderer)
	assert.Equal(t, render.icon.Resource.Name(), theme.CancelIcon().Name())

	button.Disable()
	assert.True(t, strings.HasPrefix(render.icon.Resource.Name(), "disabled_"))

	button.Enable()
	assert.Equal(t, render.icon.Resource.Name(), theme.CancelIcon().Name())
}

func TestButton_DisabledIconChangeUsingSetIcon(t *testing.T) {
	button := NewButtonWithIcon("Test", theme.CancelIcon(), nil)
	render := test.TempWidgetRenderer(t, button).(*buttonRenderer)
	assert.Equal(t, render.icon.Resource.Name(), theme.CancelIcon().Name())

	// assert we are using the disabled original icon
	button.Disable()
	cancelBaseName := strings.TrimPrefix(theme.CancelIcon().Name(), "foreground_")
	assert.Equal(t, render.icon.Resource.Name(), fmt.Sprintf("disabled_%v", cancelBaseName))

	// re-enable, then change the icon
	button.Enable()
	button.SetIcon(theme.SearchIcon())
	assert.Equal(t, render.icon.Resource.Name(), theme.SearchIcon().Name())

	// assert we are using the disabled new icon
	button.Disable()
	searchBaseName := strings.TrimPrefix(theme.SearchIcon().Name(), "foreground_")
	assert.Equal(t, render.icon.Resource.Name(), fmt.Sprintf("disabled_%v", searchBaseName))
}

func TestButton_DisabledIconChangedDirectly(t *testing.T) {
	button := NewButtonWithIcon("Test", theme.CancelIcon(), nil)
	render := test.TempWidgetRenderer(t, button).(*buttonRenderer)
	assert.Equal(t, render.icon.Resource.Name(), theme.CancelIcon().Name())

	// assert we are using the disabled original icon
	button.Disable()
	cancelBaseName := strings.TrimPrefix(theme.CancelIcon().Name(), "foreground_")
	assert.Equal(t, render.icon.Resource.Name(), fmt.Sprintf("disabled_%v", cancelBaseName))

	// re-enable, then change the icon
	button.Enable()
	button.Icon = theme.SearchIcon()
	render.Refresh()
	assert.Equal(t, render.icon.Resource.Name(), theme.SearchIcon().Name())

	// assert we are using the disabled new icon
	button.Disable()
	searchBaseName := strings.TrimPrefix(theme.SearchIcon().Name(), "foreground_")
	assert.Equal(t, render.icon.Resource.Name(), fmt.Sprintf("disabled_%v", searchBaseName))
}

func TestButton_Focus(t *testing.T) {
	tapped := false
	button := NewButton("Test", func() {
		tapped = true
	})
	render := test.TempWidgetRenderer(t, button).(*buttonRenderer)

	th := render.button.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	fgName, bgName, _ := render.buttonColorNames()
	bgColor := th.Color(bgName, v)

	render.applyTheme()
	assert.Equal(t, bgColor, render.background.FillColor)

	assert.False(t, tapped)
	button.FocusGained()
	render.Refresh() // force update without waiting

	fgColor := th.Color(fgName, v)
	assert.Equal(t, blendColor(bgColor, theme.StateLayerFocus(fgColor)), render.background.FillColor)
	button.TypedKey(&event.KeyEvent{Name: event.KeySpace})
	assert.True(t, tapped)

	button.FocusLost()
	render.applyTheme()
	assert.Equal(t, bgColor, render.background.FillColor)
}

func TestButtonRenderer_Layout(t *testing.T) {
	button := NewButtonWithIcon("Test", theme.CancelIcon(), nil)
	render := test.TempWidgetRenderer(t, button).(*buttonRenderer)
	render.Layout(render.MinSize())

	assert.Less(t, render.icon.Position().X, render.label.Position().X)
	assert.Equal(t, theme.InnerPadding(), render.icon.Position().X)
	assert.Equal(t, theme.InnerPadding(), render.MinSize().Width-render.label.Position().X-render.label.Size().Width)
}

func TestButtonRenderer_Layout_Stretch(t *testing.T) {
	button := NewButtonWithIcon("Test", theme.CancelIcon(), nil)
	extra := geo.NewSize(100, 100)
	button.Resize(button.MinSize().Add(extra))
	render := test.TempWidgetRenderer(t, button).(*buttonRenderer)

	textHeight := render.label.MinSize().Height
	minIconHeight := geo.Max(theme.IconInlineSize(), textHeight)

	// The content group is centered within the oversized button.
	// Horizontal centering: (buttonWidth - contentGroupWidth) / 2
	assert.Equal(t, 50+theme.InnerPadding(), render.icon.Position().X, "icon x")
	// Vertical centering depends on the button's MinSize height (which may
	// include an MD3 minimum-height constraint), not just content + padding.
	contentGroupH := geo.Max(minIconHeight, textHeight)
	expectedY := (button.Size().Height - contentGroupH) / 2
	assert.Equal(t, expectedY, render.icon.Position().Y, "icon y")
	assert.Equal(t, theme.IconInlineSize(), render.icon.Size().Width, "icon width")
	assert.Equal(t, minIconHeight, render.icon.Size().Height, "icon height")
	assert.Equal(t, 50+theme.InnerPadding()+theme.Padding()+theme.IconInlineSize(), render.label.Position().X, "label x")
	assert.Equal(t, render.label.MinSize().Width, render.label.Size().Width, "label size")
}

func TestButtonRenderer_Layout_NoText(t *testing.T) {
	button := NewButtonWithIcon("", theme.CancelIcon(), nil)
	render := test.TempWidgetRenderer(t, button).(*buttonRenderer)

	button.Resize(geo.NewSize(100, 100))

	assert.Equal(t, 50-theme.IconInlineSize()/2, render.icon.Position().X)
	assert.Equal(t, 50-theme.IconInlineSize()/2, render.icon.Position().Y)
}

func TestButtonRenderer_ApplyTheme(t *testing.T) {
	button := &Button{}
	render := test.TempWidgetRenderer(t, button).(*buttonRenderer)
	textRender := test.TempWidgetRenderer(t, render.label).(*textRenderer)

	textSize := textRender.Objects()[0].(*canvas.Text).TextSize
	customTextSize := textSize
	test.WithTestTheme(t, func() {
		button.Refresh()
		customTextSize = textRender.Objects()[0].(*canvas.Text).TextSize
	})

	assert.NotEqual(t, textSize, customTextSize)
}

func TestButtonRenderer_TapAnimation(t *testing.T) {
	test.NewTempApp(t)
	test.ApplyTheme(t, test.NewTheme())

	button := NewButton("Hi", func() {})
	w := test.NewWindow(button)
	defer w.Close()
	w.Resize(geo.NewSize(50, 50).Add(geo.NewSize(20, 20)))
	button.Resize(geo.NewSize(50, 50))

	render1 := test.TempWidgetRenderer(t, button).(*buttonRenderer)
	test.Tap(button)
	button.tapAnim.Tick(0.5)
	test.AssertImageMatches(t, "button/tap_animation.png", w.Canvas().Capture())

	cache.DestroyRenderer(button)
	button.Refresh()

	render2 := test.TempWidgetRenderer(t, button).(*buttonRenderer)

	assert.NotEqual(t, render1, render2)

	// Verify the animation still works after renderer recreation.
	test.Tap(button)
	assert.NotNil(t, button.tapAnim)
}

// An Elevated button casts a shadow, MD3 level 1 at rest and 2 under the
// pointer, and none when disabled; the other styles sit flat (#609).
func TestButton_ElevatedShadow(t *testing.T) {
	test.NewTempApp(t)

	shadowOf := func(b *Button) *canvas.BoxShadow {
		b.Resize(b.MinSize())
		return cache.Renderer(b).(*buttonRenderer).shadow
	}
	for _, style := range []ButtonStyle{ButtonStyleDefault, ButtonStyleFilled, ButtonStyleFilledTonal, ButtonStyleOutlined, ButtonStyleText} {
		b := NewButton("flat", nil)
		b.Style = style
		assert.False(t, shadowOf(b).Visible(), "style %d sits flat", style)
	}

	b := NewButton("raised", nil)
	b.Style = ButtonStyleElevated
	s := shadowOf(b)
	require.True(t, s.Visible())
	assert.Equal(t, float32(1.5), s.BlurRadius, "level 1 at rest")
	assert.Equal(t, float32(0.5), s.OffsetY)
	assert.Equal(t, s.Size(), b.Size())
	assert.Equal(t, cache.Renderer(b).(*buttonRenderer).background.CornerRadius, s.CornerRadius, "the shadow follows the pill")

	b.HoverIn(&event.HoverEvent{})
	assert.Equal(t, float32(3), s.BlurRadius, "level 2 under the pointer")
	assert.Equal(t, float32(1), s.OffsetY)
	b.HoverOut()
	assert.Equal(t, float32(1.5), s.BlurRadius)

	b.Disable()
	assert.False(t, s.Visible(), "a disabled button sits flat")
	b.Enable()
	assert.True(t, s.Visible())
}
