package widget_test

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// pressText is a widget made outside bellui, from the exported names alone:
// a label that shows bellui's pressed state (#1449).
type pressText struct {
	widget.BaseWidget
	widget.PressTarget
	ripple *canvas.Ripple
	off    bool // takes no press, as a disabled widget
	taps   int
}

func newPressText() *pressText {
	p := &pressText{}
	p.ExtendBaseWidget(p)
	return p
}

func (p *pressText) CreateRenderer() core.WidgetRenderer {
	p.ripple = widget.AttachPress(&p.PressTarget, func() (widget.PressShape, bool) {
		return widget.PressShape{Size: p.Size(), CornerRadius: 4, Content: color.Black}, !p.off
	})
	return widget.NewSimpleRenderer(container.NewStack(p.ripple, widget.NewLabel("Morning Light")))
}

func (p *pressText) Tapped(ev *event.PointEvent) {
	p.taps++
	widget.TapPress(&p.PressTarget, ev)
}

func (p *pressText) HoverOut() { widget.CancelPress(&p.PressTarget) }

var (
	_ mobile.Touchable  = (*pressText)(nil)
	_ desktop.Mouseable = (*pressText)(nil)
)

func TestPressTarget_Exported(t *testing.T) {
	test.NewTempApp(t)
	p := newPressText()
	w := test.NewWindow(p)
	defer w.Close()
	w.Resize(geo.NewSize(200, 60))
	assert.False(t, p.ripple.Visible(), "the ripple shows before any press")

	// A pointer press shows it from where it went down, over the shape.
	p.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: geo.NewPos(30, 10)}, Button: desktop.MouseButtonPrimary})
	assert.True(t, p.ripple.Visible(), "a mouse press shows no ripple")
	assert.Equal(t, geo.NewPos(30, 10), p.ripple.Center)
	assert.Equal(t, p.Size(), p.ripple.Size())

	// The pointer leaves: nothing is left.
	p.HoverOut()
	assert.False(t, p.ripple.Visible(), "CancelPress left the ripple showing")

	// A tap with no press before it, from code or a key, shows one too. The
	// test app runs the release to its end at once, so the ripple is hidden
	// again by now; its radius shows it spread.
	p.ripple.Radius = 0
	p.Tapped(nil)
	assert.Equal(t, 1, p.taps)
	assert.NotZero(t, p.ripple.Radius, "a tap with no press showed no ripple")
	widget.CancelPress(&p.PressTarget)

	// A finger shows it as the mouse does.
	p.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(5, 5)}})
	assert.True(t, p.ripple.Visible(), "a touch shows no ripple")
	p.TouchCancel(nil)
	assert.False(t, p.ripple.Visible(), "a cancelled touch left the ripple showing")

	// While the shape says no, nothing shows.
	p.off = true
	p.ripple.Radius = 0
	p.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(5, 5)}})
	assert.False(t, p.ripple.Visible(), "a widget that takes no press showed a ripple")
	widget.TapPress(&p.PressTarget, nil)
	assert.Zero(t, p.ripple.Radius, "a widget that takes no press showed a tap")
}
