//go:build ci || android || ios || mobile

package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// A Scroll asks for momentum after a touch release (#751), unless it scrolls
// in no direction.
func TestScroll_DragMomentum(t *testing.T) {
	var m event.MomentumDraggable = NewVScroll(canvas.NewRectangle(nil))
	assert.True(t, m.DragMomentum())

	none := NewScroll(canvas.NewRectangle(nil))
	none.Direction = ScrollNone
	assert.False(t, none.DragMomentum())
}

// A Scroll set to ScrollNone does not move its content on a drag delivered
// to it directly, as it already ignores the wheel (#1217: a disabled DataGrid
// turns its body scroller off this way). Set back to a direction, the same
// drag scrolls.
func TestScroll_DraggedHonoursScrollNone(t *testing.T) {
	rect := canvas.NewRectangle(nil)
	rect.SetMinSize(geo.NewSize(100, 1000))
	s := NewScroll(rect)
	s.Resize(geo.NewSize(100, 100))
	drag := &event.DragEvent{Dragged: geo.NewDelta(0, -50)}

	s.Direction = ScrollNone
	s.Dragged(drag)
	s.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -50)})
	assert.Equal(t, geo.Position{}, s.Offset, "a ScrollNone scroll does not move")

	s.Direction = ScrollBoth
	s.Dragged(drag)
	assert.Equal(t, float32(50), s.Offset.Y, "with a direction, the drag scrolls")
}
