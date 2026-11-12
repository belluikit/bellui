package container

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// TestBorderButton_PressFeedback is #1091: a press on an inner window's frame
// button, including on the touch ring around it on mobile, shows the button's
// pressed state from the press, and the ring's HoverOut ends it.
func TestBorderButton_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	b := newBorderButton(theme.CancelIcon(), modeClose, test.Theme(), func() {})
	b.Resize(b.MinSize())
	cache.Renderer(b)
	cache.Renderer(b.b)
	pressed := func() bool { return buttonPressed(b.b) }

	b.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(1, 1)}})
	assert.True(t, pressed(), "a touch on the frame button's ring showed no press")
	want := geo.NewPos(1, 1)
	if bellui.CurrentDevice().IsMobile() {
		// The ring is the padding around the button.
		pad := b.Theme().Size(theme.SizeNamePadding)
		want = want.SubtractXY(pad, pad)
	}
	t.Logf("mobile=%v", bellui.CurrentDevice().IsMobile())
	assert.Equal(t, want, buttonRipple(b.b).Center, "the press is not placed in the button's coordinates")
	b.TouchUp(&mobile.TouchEvent{})
	assert.False(t, pressed())

	b.TouchDown(&mobile.TouchEvent{})
	b.TouchCancel(&mobile.TouchEvent{})
	assert.False(t, pressed())

	b.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
	assert.True(t, pressed())
	b.HoverOut()
	assert.False(t, pressed(), "a press dragged off the ring stayed")
}

// buttonPressed reports whether b's press ripple shows. The test app runs
// animations to their end, so a released press shows none.
func buttonPressed(b *widget.Button) bool {
	r := buttonRipple(b)
	return r != nil && r.Visible()
}

func buttonRipple(b *widget.Button) *canvas.Ripple {
	for _, o := range cache.Renderer(b).Objects() {
		if r, ok := o.(*canvas.Ripple); ok {
			return r
		}
	}
	return nil
}
