package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestBadge_New(t *testing.T) {
	test.NewTempApp(t)
	icon := widget.NewIconButton(theme.MailComposeIcon(), nil)
	b := widget.NewBadge(icon)
	assert.NotNil(t, b)
	assert.Equal(t, 0, b.Count)
}

func TestBadge_WithCount(t *testing.T) {
	test.NewTempApp(t)
	icon := widget.NewIconButton(theme.MailComposeIcon(), nil)
	b := widget.NewBadgeWithCount(icon, 42)
	assert.Equal(t, 42, b.Count)
}

func TestBadge_MinSize(t *testing.T) {
	test.NewTempApp(t)
	icon := widget.NewIconButton(theme.MailComposeIcon(), nil)
	b := widget.NewBadge(icon)
	min := b.MinSize()
	assert.Greater(t, min.Width, float32(0))
	assert.Greater(t, min.Height, float32(0))
}

func TestBadge_Renderer(t *testing.T) {
	test.NewTempApp(t)
	icon := widget.NewIconButton(theme.MailComposeIcon(), nil)
	b := widget.NewBadgeWithCount(icon, 5)
	assert.NotPanics(t, func() { test.TempWidgetRenderer(t, b) })
}
