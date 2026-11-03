package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestIconButton_New(t *testing.T) {
	test.NewTempApp(t)
	b := widget.NewIconButton(theme.CancelIcon(), nil)
	assert.NotNil(t, b)
	assert.Equal(t, widget.IconButtonStyleStandard, b.Style)
}

func TestIconButton_MinSize(t *testing.T) {
	test.NewTempApp(t)
	b := widget.NewIconButton(theme.CancelIcon(), nil)
	min := b.MinSize()
	assert.GreaterOrEqual(t, min.Width, float32(40))
	assert.GreaterOrEqual(t, min.Height, float32(40))
}

func TestIconButton_Styles(t *testing.T) {
	test.NewTempApp(t)
	for _, style := range []widget.IconButtonStyle{
		widget.IconButtonStyleStandard,
		widget.IconButtonStyleFilled,
		widget.IconButtonStyleFilledTonal,
		widget.IconButtonStyleOutlined,
	} {
		b := widget.NewIconButton(theme.CancelIcon(), nil)
		b.Style = style
		assert.NotPanics(t, func() { test.TempWidgetRenderer(t, b) })
	}
}

func TestIconButton_Tapped(t *testing.T) {
	test.NewTempApp(t)
	tapped := false
	b := widget.NewIconButton(theme.CancelIcon(), func() { tapped = true })

	test.Tap(b)
	assert.True(t, tapped)
}

func TestIconButton_Disabled(t *testing.T) {
	test.NewTempApp(t)
	tapped := false
	b := widget.NewIconButton(theme.CancelIcon(), func() { tapped = true })
	b.Disable()

	b.Tapped(&event.PointEvent{})
	assert.False(t, tapped)
}

// iconButtonIconSize returns the size the renderer gives the icon image.
func iconButtonIconSize(t *testing.T, b *widget.IconButton) geo.Size {
	t.Helper()
	r := test.TempWidgetRenderer(t, b)
	b.Resize(b.MinSize())
	r.Layout(b.MinSize())
	for _, o := range r.Objects() {
		if img, ok := o.(*canvas.Image); ok {
			return img.Size()
		}
	}
	t.Fatal("icon button renderer has no image")
	return geo.Size{}
}

func TestIconButton_ButtonSizes(t *testing.T) {
	test.NewTempApp(t)
	for _, tc := range []struct {
		name            string
		size            widget.IconButtonSize
		container, icon float32
	}{
		{"default", widget.IconButtonSizeDefault, 40, 24},
		{"extra small", widget.IconButtonSizeExtraSmall, 32, 20},
		{"small", widget.IconButtonSizeSmall, 40, 24},
		{"medium", widget.IconButtonSizeMedium, 56, 24},
		{"large", widget.IconButtonSizeLarge, 96, 32},
		{"extra large", widget.IconButtonSizeExtraLarge, 136, 40},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := widget.NewIconButton(theme.CancelIcon(), nil)
			b.ButtonSize = tc.size
			assert.Equal(t, geo.NewSize(tc.container, tc.container), b.MinSize())
			assert.Equal(t, geo.NewSize(tc.icon, tc.icon), iconButtonIconSize(t, b))
			bg := test.TempWidgetRenderer(t, b).Objects()[0].(*canvas.Rectangle)
			assert.Equal(t, tc.container/2, bg.CornerRadius)
		})
	}
}

// The default size keeps reading the theme, so a dense theme still shrinks
// it; the explicit sizes are fixed by the spec.
func TestIconButton_DefaultSizeFollowsTheme(t *testing.T) {
	a := test.NewTempApp(t)
	a.Settings().SetTheme(theme.DenseTheme())

	b := widget.NewIconButton(theme.CancelIcon(), nil)
	assert.Equal(t, geo.NewSize(34, 34), b.MinSize())
	assert.Equal(t, geo.NewSize(20, 20), iconButtonIconSize(t, b))

	b = widget.NewIconButton(theme.CancelIcon(), nil)
	b.ButtonSize = widget.IconButtonSizeLarge
	assert.Equal(t, geo.NewSize(96, 96), b.MinSize())
}
