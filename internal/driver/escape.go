package driver

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
)

// HandleEscape gives the Escape key its default meaning — close the innermost
// pop-up, menu or dialog — and reports whether it consumed the key. Drivers
// call it before delivering the key press, so a canvas-hosted dialog dismisses
// even though nothing inside it is focused.
//
// The focused widget gets first refusal through [event.EscapeHandler]: while a
// text field is composing, or a cell is being edited in place, Escape belongs
// to that widget and no overlay closes. Everything else falls through, which is
// why a focused text field inside a dialog does not trap the user in it.
func HandleEscape(c core.Canvas) bool {
	if c == nil {
		return false
	}
	if h, ok := c.Focused().(event.EscapeHandler); ok && h.HandlesEscape() {
		return false
	}

	overlays := c.Overlays()
	if overlays == nil {
		return false
	}

	// Innermost first: the overlay the user is looking at is the one they mean
	// to back out of.
	list := overlays.List()
	for i := len(list) - 1; i >= 0; i-- {
		if o, ok := list[i].(event.EscapeDismissible); ok && o.DismissOnEscape() {
			return true
		}
	}
	return false
}
