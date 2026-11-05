//go:build (ci || android || ios || mobile) && (!windows || !ci) && !wasm

package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// positionTaker takes touch scrolls and records where each one landed in its
// own coordinates.
type positionTaker struct {
	belluicanvas.Rectangle
	got []geo.Position
}

func (t *positionTaker) Scrolled(ev *event.ScrollEvent) { t.TryScrolled(ev) }

func (t *positionTaker) TryScrolled(ev *event.ScrollEvent) bool {
	t.got = append(t.got, ev.Position)
	return true
}

// A touch scroll over a translated object hands it the position in its own
// coordinates, through its world transform, as the hit-test found it (#1003).
// It used to subtract the layout position, off by the translation.
func Test_canvas_TouchScrollPositionUnderTransform(t *testing.T) {
	taker := &positionTaker{}
	taker.SetMinSize(geo.NewSize(100, 100))
	taker.SetTransform(geo.NewTranslation(30, 40))
	c := claimCanvas(container.NewWithoutLayout(taker), geo.NewSize(300, 300))
	taker.Resize(geo.NewSize(100, 100))
	taker.Move(geo.NewPos(10, 20))

	// The middle of where it is drawn, inside the canvas's content.
	on := c.Content().Position().AddXY(10+30+50, 20+40+50)
	swipe(c, 1, on, on.SubtractXY(0, 30))

	// Every step is offered at the gesture's origin, the taker's middle.
	if assert.NotEmpty(t, taker.got, "the drawn object took the scroll") {
		for _, got := range taker.got {
			assert.Equal(t, geo.NewPos(50, 50), got)
		}
	}
}
