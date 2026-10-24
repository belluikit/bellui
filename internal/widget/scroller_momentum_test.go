//go:build ci || android || ios || mobile

package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/event"
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
