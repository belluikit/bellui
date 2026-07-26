package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/event"
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
