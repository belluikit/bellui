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
	// to back out of. An implementing overlay may decline — a snackbar counting
	// down to its own dismissal — and pass the key to what is beneath it. An
	// overlay that does not implement the interface ends the walk instead:
	// menus and completion lists close themselves through the focus path, and
	// reaching past the surface the user is looking at would cancel the dialog
	// underneath a still-open dropdown.
	list := overlays.List()
	for i := len(list) - 1; i >= 0; i-- {
		o, ok := list[i].(event.EscapeDismissible)
		if !ok {
			return false
		}
		if o.DismissOnEscape() {
			return true
		}
	}
	return false
}
