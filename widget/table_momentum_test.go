//go:build ci || android || ios || mobile

package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/event"
)

// A Table's clip embeds a Scroll, which asks for momentum; its drags resize
// columns and rows, which must stop where the finger lifts (#751).
func TestTable_ClipHasNoDragMomentum(t *testing.T) {
	var m event.MomentumDraggable = newClip(&Table{}, canvas.NewRectangle(nil))
	assert.False(t, m.DragMomentum())
}
