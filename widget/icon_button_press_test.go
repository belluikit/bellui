package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// TestIconButton_PressFeedback is #1085: the icon button had no tap feedback
// at all, and only a hover state, which touch screens do not have. It now
// shows the pressed state from the press to the release.
func TestIconButton_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	b := NewIconButton(theme.HomeIcon(), func() {})
	b.Resize(geo.NewSquareSize(40))
	press := cache.Renderer(b).(*iconButtonRenderer).press

	b.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(5, 5)}})
	assert.True(t, press.Pressed(), "no pressed state while the finger is down")
	assert.True(t, press.Ripple.Visible())
	assert.Equal(t, float32(20), press.Ripple.CornerRadius, "the pressed state is not the button's shape")
	b.TouchUp(&mobile.TouchEvent{})
	assert.False(t, press.Pressed())

	b.TouchDown(&mobile.TouchEvent{})
	b.TouchCancel(&mobile.TouchEvent{})
	assert.False(t, press.Ripple.Visible(), "a touch that became a scroll left the button pressed")

	b.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
	b.HoverOut()
	assert.False(t, press.Pressed(), "a press dragged off the button stayed")

	b.Tapped(nil) // from code: no press before it
	assert.NotZero(t, press.Ripple.Radius, "a tap from code showed no press")

	b.Disable()
	b.TouchDown(&mobile.TouchEvent{})
	assert.False(t, press.Pressed(), "a disabled button showed a press")
}

// TestIconButton_PressColor: the pressed state is the icon's colour.
func TestIconButton_PressColor(t *testing.T) {
	test.NewTempApp(t)
	th := test.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	for style, want := range map[IconButtonStyle]bellui.ThemeColorName{
		IconButtonStyleStandard:    theme.ColorNameOnSurfaceVariant,
		IconButtonStyleFilled:      theme.ColorNameForegroundOnPrimary,
		IconButtonStyleFilledTonal: theme.ColorNameOnSecondaryContainer,
		IconButtonStyleOutlined:    theme.ColorNameOnSurfaceVariant,
	} {
		b := NewIconButton(theme.HomeIcon(), nil)
		b.Style = style
		b.Resize(geo.NewSquareSize(40))
		press := cache.Renderer(b).(*iconButtonRenderer).press
		b.TouchDown(&mobile.TouchEvent{})
		wr, wg, wb, _ := th.Color(want, v).RGBA()
		r, g, bb, _ := press.Ripple.Color.RGBA()
		assert.Equal(t, [3]uint32{wr >> 8, wg >> 8, wb >> 8}, [3]uint32{r >> 8, g >> 8, bb >> 8}, "style %d", style)
	}
}
