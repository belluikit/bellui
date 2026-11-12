package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
)

// unfocusRecycled drops the canvas focus if it is on item or on an object
// inside it. A collection widget calls it as it puts a row or cell that left
// the view back in its pool, which hands the same widget out for another item:
// a focus kept on it would follow it there, and the keys typed next would edit
// that item (#1175). The canvas also unfocuses an object its frame's walk no
// longer finds (#1089), but a second layout pass before that frame can already
// have reused it.
func unfocusRecycled(item core.CanvasObject) {
	c := bellui.CurrentApp().Driver().CanvasForObject(item)
	if c == nil {
		return
	}
	focused, ok := c.Focused().(core.CanvasObject)
	if !ok || focused == nil {
		return
	}
	inside := false
	intdriver.WalkCompleteObjectTree(item, func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
		inside = o == focused
		return inside
	}, nil)
	if inside {
		c.Unfocus()
	}
}
