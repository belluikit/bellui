//go:build !wasm

package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/internal/driver/mobile/event/textedit"
	"github.com/belluikit/bellui/internal/textinput"
	"github.com/belluikit/bellui/widget"
)

// A textedit.Event with Selection set is a pure selection change (pre-rebrand tracker #1030): it
// moves the editor's selection through the text-input seam and leaves the
// text untouched — the IME's own caret moves and Select All ride this.
func TestHandleTextEdit_SelectionEvent(t *testing.T) {
	d := &driver{textInput: textinput.NewSession(keyboardStateBackend{})}
	entry := widget.NewEntry()
	entry.SetText("hello world")
	d.textInput.Bind(entry)

	d.handleTextEdit(textedit.Event{Start: 2, End: 7, Selection: true})
	start, end := entry.AccessibleSelection()
	assert.Equal(t, 2, start)
	assert.Equal(t, 7, end)
	assert.Equal(t, "hello world", entry.Text, "a selection event never edits text")

	// Equal offsets place a bare caret.
	d.handleTextEdit(textedit.Event{Start: 4, End: 4, Selection: true})
	start, end = entry.AccessibleSelection()
	assert.Equal(t, 4, start)
	assert.Equal(t, 4, end)

	// A plain event still replaces the range.
	d.handleTextEdit(textedit.Event{Start: 0, End: 5, Text: "bye"})
	assert.Equal(t, "bye world", entry.Text)
}
