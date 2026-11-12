package dialog

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	internalwidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
)

// TestColorButton_PressFeedback is #1091: a colour in the picker shows the
// pressed state over the whole swatch from the press to the release.
func TestColorButton_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	b := newColorButton(color.White, func(color.Color) {})
	b.Resize(geo.NewSquareSize(32))
	cache.Renderer(b)
	press := internalwidget.PressFeedbackOf(&b.PressTarget)

	b.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(4, 6)}})
	assert.True(t, press.Pressed(), "no pressed state while the finger is down")
	assert.Equal(t, b.Size(), press.Ripple.Size())
	assert.Equal(t, geo.NewPos(4, 6), press.Ripple.Center)
	b.TouchUp(&mobile.TouchEvent{})
	assert.False(t, press.Pressed())

	b.TouchDown(&mobile.TouchEvent{})
	b.TouchCancel(&mobile.TouchEvent{})
	assert.False(t, press.Ripple.Visible(), "a touch that became a scroll left the colour pressed")

	b.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
	b.HoverOut()
	assert.False(t, press.Pressed(), "a press dragged off the colour stayed")
}
