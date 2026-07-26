package widget

import (
	"github.com/belluikit/bellui"
)

// ShowCompletion displays an application-driven completion popup anchored at the
// Entry's caret. The optional title is shown as a header above the items; pass
// an empty string for no header. When the user accepts an item (Enter, Space or
// tap), OnCompletionSelected is called. Passing no items hides any open popup.
//
// The popup is placed just below the caret line, flipping above it when there is
// not enough room beneath, so it never covers the text being edited. It is the
// caller's responsibility to refresh or hide the popup as the edited text
// changes (for example from OnChanged).
//
// This is for app-level suggestions (history, snippets, language-server
// completions). It is separate from the platform IME candidate window, which the
// operating system renders and positions itself.
//
// Since: 2.10
func (e *Entry) ShowCompletion(title string, items []CompletionItem) {
	if len(items) == 0 {
		e.HideCompletion()
		return
	}

	d := bellui.CurrentApp().Driver()
	c := d.CanvasForObject(e)
	if c == nil {
		return
	}

	if e.completion == nil {
		e.completion = NewCompletionPopup(c, items)
		// The popup is the focused overlay while open, but it forwards typing
		// and editing keys back to this Entry so the user keeps editing and the
		// list filters live; navigation/accept keys it handles itself (#361).
		e.completion.host = e
	} else {
		e.completion.SetItems(items)
	}
	e.completion.SetTitle(title)
	e.completion.OnSelected = func(_ int, item CompletionItem) {
		if e.OnCompletionSelected != nil {
			e.OnCompletionSelected(item)
		}
	}

	// PreeditCursorRectangle reports the caret in content coordinates (already
	// adjusted for scroll); translate it to canvas-absolute for the popup.
	caret := e.PreeditCursorRectangle()
	caret.Pos = caret.Pos.Add(d.AbsolutePositionForObject(e))
	e.completion.ShowAtCaret(caret)
}

// HideCompletion hides the completion popup if one is showing.
//
// Since: 2.10
func (e *Entry) HideCompletion() {
	if e.completion != nil {
		e.completion.Hide()
	}
}
