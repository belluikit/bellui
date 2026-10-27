//go:build !wasm

package mobile

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/widget"
)

// revealFocused scrolls the scroll containers that hold the focused object
// until the object is inside the canvas's interactive area. An on-screen
// keyboard shrinks that area from the bottom, and nothing brought the field
// it was opened for back into view: you typed into a field you could not see
// (#654). The innermost container scrolls first; each one only as far as it
// must, and an object taller than the space shows its top.
func (c *canvas) revealFocused() {
	obj, ok := c.Focused().(core.CanvasObject)
	if !ok {
		return
	}
	type scrollAt struct {
		scroll *widget.Scroll
		pos    geo.Position
	}
	var stack, holders []scrollAt
	var objPos geo.Position
	found := false
	trees := append(c.Overlays().List(), c.Content())
	for _, tree := range trees {
		intdriver.WalkVisibleObjectTree(tree, func(o core.CanvasObject, pos, _ geo.Position, _ geo.Size) bool {
			if o == obj {
				holders, objPos, found = append([]scrollAt(nil), stack...), pos, true
				return true
			}
			if s, ok := o.(*widget.Scroll); ok {
				stack = append(stack, scrollAt{s, pos})
			}
			return false
		}, func(o core.CanvasObject, _ geo.Position, _ core.CanvasObject) {
			if _, ok := o.(*widget.Scroll); ok && len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		})
		if found {
			break
		}
		stack = stack[:0]
	}
	if !found {
		return
	}

	areaPos, areaSize := c.InteractiveArea()
	top, bottom := objPos.Y, objPos.Y+obj.Size().Height
	for i := len(holders) - 1; i >= 0; i-- {
		h := holders[i]
		viewTop := max(h.pos.Y, areaPos.Y)
		viewBottom := min(h.pos.Y+h.scroll.Size().Height, areaPos.Y+areaSize.Height)
		var delta float32
		switch {
		case bottom > viewBottom:
			delta = min(bottom-viewBottom, top-viewTop) // never past the object's top
		case top < viewTop:
			delta = top - viewTop
		}
		if delta == 0 {
			continue
		}
		old := h.scroll.Offset
		h.scroll.ScrollToOffset(old.AddXY(0, delta))
		moved := h.scroll.Offset.Y - old.Y // clamped at the content's ends
		top -= moved
		bottom -= moved
	}
}
