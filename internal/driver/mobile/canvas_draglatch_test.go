//go:build (!windows || !ci) && !wasm

package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// Drag-target latching (pre-rebrand tracker #945 device finding): a drag latches onto the
// Draggable under the gesture ORIGIN, matching the desktop driver — a fast
// first move can jump clear across a small Draggable (a selection handle, a
// slider thumb) and must not start dragging whatever lies beyond it.

type dragRecorder struct {
	widget.BaseWidget
	drags int
}

func newDragRecorder() *dragRecorder {
	r := &dragRecorder{}
	r.ExtendBaseWidget(r)
	return r
}

func (r *dragRecorder) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(widget.NewLabel(""))
}

func (r *dragRecorder) Dragged(*event.DragEvent) { r.drags++ }
func (r *dragRecorder) DragEnd()                 {}

func Test_canvas_DragLatchesAtGestureOrigin(t *testing.T) {
	small := newDragRecorder() // a small handle-like draggable
	big := newDragRecorder()   // a large draggable underneath/beside it

	cont := container.NewWithoutLayout(big, small)
	big.Resize(geo.NewSize(200, 80))
	big.Move(geo.NewPos(0, 0))
	small.Resize(geo.NewSize(20, 20))
	small.Move(geo.NewPos(10, 30))

	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(cont)
	c.Resize(geo.NewSize(200, 80))

	drag := func(wid event.Draggable, ev *event.DragEvent) {
		wid.Dragged(ev)
	}

	// Touch down on the small draggable, then jump far past it in one move.
	c.TapDown(geo.NewPos(20, 40), 0)
	c.TapMove(geo.NewPos(120, 40), 0, drag)
	c.TapMove(geo.NewPos(150, 40), 0, drag)

	assert.Positive(t, small.drags,
		"the drag belongs to the draggable under the touch-down point")
	assert.Zero(t, big.drags,
		"a fast first move must not hand the drag to the object it lands on")
}
