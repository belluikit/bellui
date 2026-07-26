package widget_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestSplitButton_New(t *testing.T) {
	test.NewTempApp(t)
	items := []*bellui.MenuItem{
		bellui.NewMenuItem("Option A", nil),
	}
	sb := widget.NewSplitButton("Save", nil, items)
	assert.NotNil(t, sb)
	assert.Equal(t, "Save", sb.Text)
	assert.Len(t, sb.MenuItems, 1)
}

func TestSplitButton_MinSize(t *testing.T) {
	test.NewTempApp(t)
	sb := widget.NewSplitButton("Action", nil, nil)
	min := sb.MinSize()
	assert.Greater(t, min.Width, float32(0))
	assert.GreaterOrEqual(t, min.Height, float32(40))
}

func TestSplitButton_Tapped(t *testing.T) {
	test.NewTempApp(t)
	tapped := false
	sb := widget.NewSplitButton("Go", func() { tapped = true }, nil)
	sb.Resize(sb.MinSize())

	// Tap in the primary area (left side), not the dropdown area (right 36px).
	sb.Tapped(&event.PointEvent{Position: geo.NewPos(10, 10)})
	assert.True(t, tapped)
}

func TestSplitButton_Disabled(t *testing.T) {
	test.NewTempApp(t)
	tapped := false
	sb := widget.NewSplitButton("Go", func() { tapped = true }, nil)
	sb.Disable()

	sb.Tapped(&event.PointEvent{})
	assert.False(t, tapped)
}

func TestSplitButton_DropdownTap_OpensMenu(t *testing.T) {
	test.NewTempApp(t)
	primary := false
	items := []*bellui.MenuItem{bellui.NewMenuItem("A", nil), bellui.NewMenuItem("B", nil)}
	sb := widget.NewSplitButton("Go", func() { primary = true }, items)
	w := test.NewTempWindow(t, sb)
	w.Resize(geo.NewSize(240, 60))

	// A tap in the right-hand dropdown region opens the menu instead of firing
	// the primary action.
	sb.Tapped(&event.PointEvent{Position: geo.NewPos(sb.Size().Width-5, sb.Size().Height/2)})
	assert.False(t, primary, "dropdown tap must not trigger the primary action")
	assert.NotEmpty(t, w.Canvas().Overlays().List(), "dropdown tap opens the menu overlay")
}

func TestSplitButton_EmptyMenuItems_NoMenu(t *testing.T) {
	test.NewTempApp(t)
	sb := widget.NewSplitButton("Go", nil, nil)
	w := test.NewTempWindow(t, sb)
	w.Resize(geo.NewSize(240, 60))

	// With no menu items the dropdown tap is a guarded no-op (no panic, no menu).
	assert.NotPanics(t, func() {
		sb.Tapped(&event.PointEvent{Position: geo.NewPos(sb.Size().Width-5, sb.Size().Height/2)})
	})
	assert.Empty(t, w.Canvas().Overlays().List())
}
