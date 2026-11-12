package container

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	intwidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

func assertSameRGB(t *testing.T, want, got color.Color) {
	t.Helper()
	wr, wg, wb, _ := want.RGBA()
	r, g, b, _ := got.RGBA()
	assert.Equal(t, [3]uint32{wr >> 8, wg >> 8, wb >> 8}, [3]uint32{r >> 8, g >> 8, b >> 8})
}

// TestTabButton_PressFeedback is #1091: a tab shows the pressed state from
// the press to the release, over the whole tab, in its label's colour.
func TestTabButton_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	v := bellui.CurrentApp().Settings().ThemeVariant()
	tabs := NewAppTabs(NewTabItem("One", widget.NewLabel("1")), NewTabItem("Two", widget.NewLabel("2")))
	w := test.NewTempWindow(t, tabs)
	w.Resize(geo.NewSize(300, 200))
	for i, want := range []bellui.ThemeColorName{theme.ColorNamePrimary, theme.ColorNameForeground} {
		b := tabButtonAt(t, tabs, i)
		press := intwidget.PressFeedbackOf(&b.PressTarget)
		b.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(7, 8)}})
		assert.True(t, press.Pressed(), "no pressed state while the finger is down")
		assert.Equal(t, b.Size(), press.Ripple.Size(), "the pressed state does not fill the tab")
		assert.Equal(t, geo.NewPos(7, 8), press.Ripple.Center)
		assertSameRGB(t, test.Theme().Color(want, v), press.Ripple.Color)
		b.TouchUp(&mobile.TouchEvent{})
		assert.False(t, press.Pressed())

		b.TouchDown(&mobile.TouchEvent{})
		b.TouchCancel(&mobile.TouchEvent{})
		assert.False(t, press.Ripple.Visible(), "a touch that became a scroll left the tab pressed")

		b.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
		b.HoverOut()
		assert.False(t, press.Pressed(), "a press dragged off the tab stayed")
	}

	b := tabButtonAt(t, tabs, 1)
	press := intwidget.PressFeedbackOf(&b.PressTarget)
	press.Ripple.Radius = 0
	b.PerformAccessibleAction("select")
	assert.NotZero(t, press.Ripple.Radius, "selecting a tab from code showed no press")

	b.Disable()
	b.TouchDown(&mobile.TouchEvent{})
	assert.False(t, press.Pressed(), "a disabled tab showed a press")
}

// TestTabCloseButton_PressFeedback: a tab's close button shows a circle of
// its own, from its centre, and a press on it does not press the tab.
func TestTabCloseButton_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	tabs := NewDocTabs(NewTabItem("One", widget.NewLabel("1")))
	w := test.NewTempWindow(t, tabs)
	w.Resize(geo.NewSize(300, 200))
	b := tabButtonAt(t, tabs, 0)
	close := cache.Renderer(b).(*tabButtonRenderer).close
	press := intwidget.PressFeedbackOf(&close.PressTarget)

	close.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(1, 2)}})
	assert.True(t, press.Pressed())
	assert.Equal(t, close.Size(), press.Ripple.Size())
	assert.Equal(t, close.Size().Width/2, press.Ripple.CornerRadius, "the close button's pressed state is not a circle")
	assert.Equal(t, geo.NewPos(close.Size().Width/2, close.Size().Height/2), press.Ripple.Center)
	assertSameRGB(t, test.Theme().Color(theme.ColorNamePrimary, bellui.CurrentApp().Settings().ThemeVariant()), press.Ripple.Color)
	assert.False(t, intwidget.PressFeedbackOf(&b.PressTarget).Pressed(), "a press on the close button pressed the tab")
	close.TouchUp(&mobile.TouchEvent{})
	assert.False(t, press.Pressed())

	close.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
	close.HoverOut()
	assert.False(t, press.Pressed(), "a press dragged off the close button stayed")
}

// tabButtonAt is the button of tab i in tabs' bar.
func tabButtonAt(t *testing.T, tabs core.Widget, i int) *tabButton {
	t.Helper()
	var found []*tabButton
	for _, o := range test.LaidOutObjects(tabs) {
		if b, ok := o.(*tabButton); ok {
			found = append(found, b)
		}
	}
	if len(found) <= i {
		t.Fatalf("no tab button %d among %d", i, len(found))
	}
	cache.Renderer(found[i])
	return found[i]
}
