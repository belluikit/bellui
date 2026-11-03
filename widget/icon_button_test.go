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

func TestIconButton_ButtonSize(t *testing.T) {
	test.NewTempApp(t)
	th := test.Theme()
	for _, tt := range []struct {
		name            string
		size            widget.IconButtonSize
		container, icon float32
	}{
		{"default", widget.IconButtonSizeDefault, th.Size(theme.SizeNameIconButtonSize), th.Size(theme.SizeNameIconSize)},
		{"xsmall", widget.IconButtonSizeXSmall, 32, 20},
		{"small", widget.IconButtonSizeSmall, 40, 24},
		{"medium", widget.IconButtonSizeMedium, 56, 24},
		{"large", widget.IconButtonSizeLarge, 96, 32},
		{"xlarge", widget.IconButtonSizeXLarge, 136, 40},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := widget.NewIconButton(theme.CancelIcon(), nil)
			b.ButtonSize = tt.size
			assert.Equal(t, geo.NewSize(tt.container, tt.container), b.MinSize())

			b.Resize(b.MinSize())
			r := test.TempWidgetRenderer(t, b)
			objs := r.Objects()
			bg := objs[0].(*canvas.Rectangle)
			icon := objs[1].(*canvas.Image)
			assert.Equal(t, tt.container/2, bg.CornerRadius)
			assert.Equal(t, geo.NewSize(tt.icon, tt.icon), icon.Size())
			off := (tt.container - tt.icon) / 2
			assert.Equal(t, geo.NewPos(off, off), icon.Position())
		})
	}
}
