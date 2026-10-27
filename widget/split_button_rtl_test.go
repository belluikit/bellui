package widget_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// splitButtonInWindow puts a SplitButton at its own size in the middle of a
// wide window, so a menu aligned to either edge of it stays on the canvas.
func splitButtonInWindow(t *testing.T, primary *bool) (*widget.SplitButton, bellui.Window) {
	t.Helper()
	items := []*bellui.MenuItem{bellui.NewMenuItem("Save As...", nil), bellui.NewMenuItem("Export PDF", nil)}
	sb := widget.NewSplitButton("Save", func() { *primary = true }, items)
	w := test.NewTempWindow(t, container.NewCenter(sb))
	w.Resize(geo.NewSize(600, 300))
	return sb, w
}

// shownMenu is the pop-up menu the button opened, or nil. The overlay on the
// canvas is the menu's container; the menu is one of its objects.
func shownMenu(t *testing.T, w bellui.Window) *widget.PopUpMenu {
	t.Helper()
	for _, o := range w.Canvas().Overlays().List() {
		ow, ok := o.(core.Widget)
		if !ok || !o.Visible() {
			continue
		}
		for _, c := range test.TempWidgetRenderer(t, ow).Objects() {
			if m, ok := c.(*widget.PopUpMenu); ok {
				return m
			}
		}
	}
	return nil
}

// Under RTL the dropdown sits at the left edge, where the layout mirrors it,
// and the menu lines up with the button's right edge, its start edge (#835).
func TestSplitButton_RTL_DropdownAndMenu(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	var primary bool
	sb, w := splitButtonInWindow(t, &primary)
	mid := sb.Size().Height / 2

	sb.Tapped(&event.PointEvent{Position: geo.NewPos(sb.Size().Width-5, mid)})
	assert.True(t, primary, "RTL: the right end is the primary action")
	assert.Nil(t, shownMenu(t, w), "RTL: the right end opens no menu")

	primary = false
	sb.Tapped(&event.PointEvent{Position: geo.NewPos(5, mid)})
	assert.False(t, primary, "RTL: the left end is the dropdown, not the primary action")
	m := shownMenu(t, w)
	require.NotNil(t, m, "RTL: the left end opens the menu")

	btnRight := absX(sb) + sb.Size().Width
	menuRight := m.Position().X + m.Size().Width
	assert.InDelta(t, btnRight, menuRight, 0.5, "RTL: the menu's right edge lines up with the button's")
}

func TestSplitButton_LTR_MenuAlignsLeft(t *testing.T) {
	test.NewTempApp(t)
	var primary bool
	sb, w := splitButtonInWindow(t, &primary)

	sb.Tapped(&event.PointEvent{Position: geo.NewPos(sb.Size().Width-5, sb.Size().Height/2)})
	m := shownMenu(t, w)
	require.NotNil(t, m)
	assert.InDelta(t, absX(sb), m.Position().X, 0.5, "LTR: the menu's left edge lines up with the button's")
}

// A right-aligned menu wider than the room to the left of the button's right
// edge stays on the canvas: it starts at its left edge.
func TestSplitButton_RTL_MenuStaysOnCanvas(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	items := []*bellui.MenuItem{bellui.NewMenuItem("A menu item much wider than the button it opens from", nil)}
	sb := widget.NewSplitButton("Go", nil, items)
	// Mirrored under RTL: the spacer goes right and the button to the left edge.
	w := test.NewTempWindow(t, container.NewHBox(layout.NewSpacer(), sb))
	w.Resize(geo.NewSize(600, 300))

	sb.Tapped(&event.PointEvent{Position: geo.NewPos(5, sb.Size().Height/2)})
	m := shownMenu(t, w)
	require.NotNil(t, m)
	require.Greater(t, m.Size().Width, absX(sb)+sb.Size().Width, "the menu must be wider than the room it is right-aligned into")
	assert.GreaterOrEqual(t, m.Position().X, float32(0), "the menu stays on the canvas")
}

// A right-to-left scope in a left-to-right app counts the same as an RTL app:
// the direction is read where the button sits, not app-wide (#696).
func TestSplitButton_RTLScope_OpensFromTheLeftEnd(t *testing.T) {
	test.NewTempApp(t)
	var primary bool
	items := []*bellui.MenuItem{bellui.NewMenuItem("Save As...", nil)}
	sb := widget.NewSplitButton("Save", func() { primary = true }, items)
	w := test.NewTempWindow(t, container.NewCenter(container.NewDirectionScope(sb, bellui.DirectionRTL)))
	w.Resize(geo.NewSize(600, 300))

	sb.Tapped(&event.PointEvent{Position: geo.NewPos(5, sb.Size().Height/2)})
	assert.False(t, primary, "scoped RTL: the left end is the dropdown")
	m := shownMenu(t, w)
	require.NotNil(t, m, "scoped RTL: the left end opens the menu")
	assert.InDelta(t, absX(sb)+sb.Size().Width, m.Position().X+m.Size().Width, 0.5,
		"scoped RTL: the menu's right edge lines up with the button's")
}

func absX(o core.CanvasObject) float32 {
	return bellui.CurrentApp().Driver().AbsolutePositionForObject(o).X
}
