package widget

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
)

func displayText(e *RichEntry) string {
	out := ""
	for _, s := range e.displaySegments() {
		if ts, ok := s.(*TextSegment); ok {
			out += ts.Text
		}
	}
	return out
}

func TestRichEntry_ComposePreeditDoesNotTouchDocument(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "ab"}})
	w := test.NewWindow(e)
	defer w.Close()
	e.FocusGained()

	changed := 0
	e.OnChanged = func() { changed++ }
	e.caret, e.anchor = 1, 1

	e.Compose(&event.PreeditEvent{Text: "ni", CursorPos: 2})
	assert.True(t, e.composing)
	assert.Equal(t, "ab", e.String(), "document is untouched during composition")
	assert.Equal(t, "anib", displayText(e), "preedit is shown inline at the caret")
	assert.Equal(t, 0, changed, "composition fires no change callback")
}

func TestRichEntry_ComposeCommitInsertsOneEdit(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "ab"}})
	w := test.NewWindow(e)
	defer w.Close()
	e.FocusGained()
	e.caret, e.anchor = 1, 1

	e.Compose(&event.PreeditEvent{Text: "ni", CursorPos: 2})
	e.ComposeCommit("你")
	assert.False(t, e.composing)
	assert.Equal(t, "a你b", e.String())
	assert.Equal(t, 2, e.caret)

	e.Undo() // the commit is one undo step
	assert.Equal(t, "ab", e.String())
}

func TestRichEntry_ComposeCancelDiscards(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "ab"}})
	w := test.NewWindow(e)
	defer w.Close()
	e.FocusGained()
	e.caret, e.anchor = 1, 1

	e.Compose(&event.PreeditEvent{Text: "ni", CursorPos: 2})
	e.ComposeCancel()
	assert.False(t, e.composing)
	assert.Equal(t, "ab", e.String())
	assert.Equal(t, "ab", displayText(e), "preedit is gone after cancel")
}

func TestRichEntry_TypedRuneCommitsComposition(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: ""}})
	w := test.NewWindow(e)
	defer w.Close()
	e.FocusGained()

	e.Compose(&event.PreeditEvent{Text: "n", CursorPos: 1})
	e.TypedRune('X') // commits while composing
	assert.False(t, e.composing)
	assert.Equal(t, "X", e.String())
}

func TestRichEntry_ComposeReplacesSelection(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "abcd"}})
	w := test.NewWindow(e)
	defer w.Close()
	e.FocusGained()
	e.selecting, e.anchor, e.caret = true, 1, 3 // "bc"

	e.Compose(&event.PreeditEvent{Text: "x", CursorPos: 1})
	assert.Equal(t, "ad", e.String(), "selection removed when composition starts")
	e.ComposeCommit("y")
	assert.Equal(t, "ayd", e.String())
}

func TestRichEntry_ComposeCaretTracksPreedit(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "ab"}})
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(200, 60))
	e.FocusGained()
	e.caret, e.anchor = 1, 1

	xNoPreedit, _ := e.caretRect()
	e.Compose(&event.PreeditEvent{Text: "mmmm", CursorPos: 4})
	xPreeditEnd, _ := e.caretRect()
	assert.Greater(t, xPreeditEnd.X, xNoPreedit.X, "caret advances into the preedit")

	// PreeditPositioner reports the same caret rect for the IME candidate window.
	r := e.PreeditCursorRectangle()
	assert.Equal(t, xPreeditEnd.X, r.Pos.X)
}

func TestRichEntry_ComposeViaTestHelpers(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: ""}})
	w := test.NewWindow(e)
	defer w.Close()
	w.Canvas().Focus(e)

	test.Compose(w.Canvas(), "ni", 2)
	assert.True(t, e.composing)
	test.ComposeCommit(w.Canvas(), "你")
	assert.Equal(t, "你", e.String())
}
