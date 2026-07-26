package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestFAB_New(t *testing.T) {
	test.NewTempApp(t)
	fab := widget.NewFAB(theme.HomeIcon(), nil)
	assert.NotNil(t, fab)
	assert.Empty(t, fab.Text)
}

func TestExtendedFAB_New(t *testing.T) {
	test.NewTempApp(t)
	fab := widget.NewExtendedFAB("Create", theme.ContentAddIcon(), nil)
	assert.NotNil(t, fab)
	assert.Equal(t, "Create", fab.Text)
}

func TestFAB_MinSize(t *testing.T) {
	test.NewTempApp(t)
	fab := widget.NewFAB(theme.HomeIcon(), nil)
	min := fab.MinSize()
	assert.GreaterOrEqual(t, min.Width, float32(56))
	assert.GreaterOrEqual(t, min.Height, float32(56))

	ext := widget.NewExtendedFAB("Create", theme.ContentAddIcon(), nil)
	extMin := ext.MinSize()
	assert.Greater(t, extMin.Width, min.Width, "Extended FAB should be wider")
}

func TestFAB_Tapped(t *testing.T) {
	test.NewTempApp(t)
	tapped := false
	fab := widget.NewFAB(theme.HomeIcon(), func() { tapped = true })

	test.Tap(fab)
	assert.True(t, tapped)
}

func TestFAB_Disabled(t *testing.T) {
	test.NewTempApp(t)
	tapped := false
	fab := widget.NewFAB(theme.HomeIcon(), func() { tapped = true })
	fab.Disable()

	fab.Tapped(&event.PointEvent{})
	assert.False(t, tapped, "Disabled FAB should not fire callback")
}
