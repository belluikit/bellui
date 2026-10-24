package dagwidget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// A one-finger touch pan of the graph coasts after release, as the two-finger
// pan does through touch-scroll momentum; connecting drags stop where the
// finger lifts (#751). The interface is matched structurally, so this module
// does not need a toolkit release that declares event.MomentumDraggable.
func TestDAGCanvas_OnlyPanningHasDragMomentum(t *testing.T) {
	c := newTestCanvas(t)
	var m interface{ DragMomentum() bool } = c

	assert.False(t, m.DragMomentum(), "no drag, nothing to coast")
	c.Dragged(&event.DragEvent{Dragged: geo.NewDelta(10, 0)})
	assert.True(t, m.DragMomentum(), "a pan coasts")
	c.DragEnd()
	assert.False(t, m.DragMomentum())

	c.dragState = dragConnecting
	assert.False(t, m.DragMomentum(), "dragging a connection does not")
	c.dragState = dragNone
}
