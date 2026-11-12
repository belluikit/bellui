package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// assertIndicatorShape checks that a press on a navigation item, anywhere on
// it, shows on its indicator pill, in the colour of its icon (#1091).
func assertIndicatorShape(t *testing.T, item pressEvents, t0 *widget.PressTarget, indicator interface {
	Position() geo.Position
	Size() geo.Size
}, radius float32, want bellui.ThemeColorName,
) {
	t.Helper()
	item.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: indicator.Position().Add(geo.NewPos(5, 6))}})
	defer item.TouchCancel(nil)
	r := widget.PressFeedbackOf(t0).Ripple
	assert.Equal(t, indicator.Position(), r.Position(), "the pressed state is not on the indicator")
	assert.Equal(t, indicator.Size(), r.Size())
	assert.Equal(t, radius, r.CornerRadius)
	assert.Equal(t, geo.NewPos(5, 6), r.Center, "the ripple does not start at the press")
	assertSameRGB(t, test.Theme().Color(want, bellui.CurrentApp().Settings().ThemeVariant()), r.Color)
}

func TestNavigationBarItem_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	bar := NewNavigationBar([]*NavigationBarItem{{Label: "Home", Icon: theme.HomeIcon()}, {Label: "Mail", Icon: theme.MailComposeIcon()}}, func(int) {})
	bar.Resize(geo.NewSize(300, bar.MinSize().Height))
	items := cache.Renderer(bar).(*navBarRenderer).items
	for i, want := range []bellui.ThemeColorName{theme.ColorNameOnSecondaryContainer, theme.ColorNameOnSurfaceVariant} {
		item := items[i]
		assertPressCycle(t, item, &item.pressTarget, item.HoverOut)
		ind := cache.Renderer(item).(*navBarItemRenderer).indicator
		assertIndicatorShape(t, item, &item.pressTarget, ind, ind.CornerRadius, want)
	}
	assertTapFromCode(t, &items[1].pressTarget, func() { items[1].PerformAccessibleAction("select") })
}

func TestNavigationRailItem_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	rail := NewNavigationRail([]*NavigationRailItem{{Label: "Home", Icon: theme.HomeIcon()}, {Label: "Mail", Icon: theme.MailComposeIcon()}}, func(int) {})
	rail.Resize(rail.MinSize())
	items := cache.Renderer(rail).(*navRailRenderer).items
	for i, want := range []bellui.ThemeColorName{theme.ColorNameOnSecondaryContainer, theme.ColorNameOnSurfaceVariant} {
		item := items[i]
		assertPressCycle(t, item, &item.pressTarget, item.HoverOut)
		ind := cache.Renderer(item).(*navRailItemRenderer).indicator
		assertIndicatorShape(t, item, &item.pressTarget, ind, ind.CornerRadius, want)
	}
	assertTapFromCode(t, &items[1].pressTarget, func() { items[1].PerformAccessibleAction("select") })
}

func TestNavigationDrawerItem_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	d := NewNavigationDrawer([]*NavigationDrawerItem{{Label: "Home", Icon: theme.HomeIcon()}, {Label: "Mail", Icon: theme.MailComposeIcon()}}, func(int) {})
	w := test.NewTempWindow(t, NewLabel(""))
	w.Resize(geo.NewSize(400, 600))
	d.Show(w.Canvas())
	var items []*drawerItemWidget
	for _, o := range test.LaidOutObjects(d.popUp) {
		if item, ok := o.(*drawerItemWidget); ok {
			items = append(items, item)
		}
	}
	if !assert.Len(t, items, 2) {
		return
	}
	for i, want := range []bellui.ThemeColorName{theme.ColorNameOnSecondaryContainer, theme.ColorNameOnSurfaceVariant} {
		item := items[i]
		assertPressCycle(t, item, &item.pressTarget, item.HoverOut)
		// The whole item is its indicator.
		bounds := canvas.NewRectangle(nil)
		bounds.Resize(item.Size())
		assertIndicatorShape(t, item, &item.pressTarget, bounds, item.bg.CornerRadius, want)
	}
}
