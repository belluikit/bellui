package widget

import (
	"image"
	"testing"
	"unsafe"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/stretchr/testify/assert"
)

func TestRichEntry_AccessibleText_Memoized(t *testing.T) {
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Text: "hello world"}})

	a := e.AccessibleText()
	b := e.AccessibleText()
	assert.Equal(t, "hello world", a)
	// Repeated reads return the same string object, not a rebuilt one — this is
	// what keeps the per-paint-frame mobile sync O(1) (#783).
	assert.True(t, unsafe.StringData(a) == unsafe.StringData(b),
		"AccessibleText should be memoized between reads")

	// An interactive edit invalidates the memo; the next read reflects it and is
	// a fresh object.
	e.SetAccessibleSelection(0, 5)
	e.ReplaceAccessibleSelection("HELLO")
	c := e.AccessibleText()
	assert.Equal(t, "HELLO world", c)
	assert.True(t, unsafe.StringData(a) != unsafe.StringData(c),
		"the memo must refresh after an edit")

	// Programmatic replacement (SetSegments) invalidates too.
	e.SetSegments([]RichTextSegment{&TextSegment{Text: "new"}})
	assert.Equal(t, "new", e.AccessibleText())
}

// IME composition and repaints must NOT drop the AccessibleText memo (#794):
// the preedit is excluded from the committed rune stream, so a screen reader
// tracking the caret during CJK composition would otherwise rebuild the whole
// string on every preedit keystroke over a large document. Only a committed edit
// refreshes it.
func TestRichEntry_AccessibleText_MemoSurvivesCompositionAndRepaint(t *testing.T) {
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Text: "hello world"}})
	e.SetAccessibleSelection(5, 5)

	base := e.AccessibleText()

	// A repaint (renderer refresh) leaves the committed content unchanged.
	e.Refresh()
	assert.True(t, unsafe.StringData(e.AccessibleText()) == unsafe.StringData(base),
		"a repaint must not invalidate the memo")

	// A full composition cycle (set preedit, update it, then cancel) never
	// touches the committed document, so the memo must persist throughout.
	e.Compose(&event.PreeditEvent{Text: "n", CursorPos: 1})
	e.Compose(&event.PreeditEvent{Text: "ni", CursorPos: 2})
	e.Compose(&event.PreeditEvent{Text: "nih", CursorPos: 3})
	cur := e.AccessibleText()
	assert.Equal(t, "hello world", cur, "AccessibleText excludes the preedit")
	assert.True(t, unsafe.StringData(cur) == unsafe.StringData(base),
		"composition must not invalidate the memo")

	e.ComposeCancel()
	assert.True(t, unsafe.StringData(e.AccessibleText()) == unsafe.StringData(base),
		"cancelling composition must not invalidate the memo")

	// Committing the composition inserts one edit — that refreshes the memo.
	e.Compose(&event.PreeditEvent{Text: "ni", CursorPos: 2})
	e.ComposeCommit("你")
	got := e.AccessibleText()
	assert.Equal(t, "hello你 world", got)
	assert.True(t, unsafe.StringData(got) != unsafe.StringData(base),
		"a committed edit must refresh the memo")
}

func TestRichEntry_AccessibleTexter_TextAndSelection(t *testing.T) {
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Text: "hello"}})

	assert.Equal(t, "hello", e.AccessibleText())

	// Caret and empty selection.
	e.SetAccessibleSelection(2, 2)
	assert.Equal(t, 2, e.AccessibleCaret())
	s, en := e.AccessibleSelection()
	assert.Equal(t, [2]int{2, 2}, [2]int{s, en})

	// Non-empty selection, normalized.
	e.SetAccessibleSelection(4, 1)
	s, en = e.AccessibleSelection()
	assert.Equal(t, [2]int{1, 4}, [2]int{s, en})
}

func TestRichEntry_AccessibleTexter_ReplaceSelection(t *testing.T) {
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Text: "hello world"}})

	// Replace the "world" range (#748 replace-selection semantics).
	e.SetAccessibleSelection(6, 11)
	e.ReplaceAccessibleSelection("there")
	assert.Equal(t, "hello there", e.AccessibleText())

	// Insert at the caret.
	e.SetAccessibleSelection(5, 5)
	e.ReplaceAccessibleSelection(",")
	assert.Equal(t, "hello, there", e.AccessibleText())

	// Empty replacement deletes the selection.
	e.SetAccessibleSelection(0, 6)
	e.ReplaceAccessibleSelection("")
	assert.Equal(t, " there", e.AccessibleText())
}

// Atoms (images) occupy exactly one caret position, mirrored as U+FFFC so
// accessible rune offsets stay aligned with the document's global offsets.
func TestRichEntry_AccessibleTexter_AtomAlignment(t *testing.T) {
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Text: "ab"}})
	e.SetAccessibleSelection(1, 1)
	e.InsertImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))

	txt := e.AccessibleText()
	// "a" + object-replacement char + "b"
	assert.Equal(t, 3, len([]rune(txt)))
	assert.Equal(t, "a￼b", txt)

	// A rune offset past the atom addresses "b".
	e.SetAccessibleSelection(2, 3)
	e.ReplaceAccessibleSelection("B")
	assert.Equal(t, "a￼B", e.AccessibleText())
}

// RichEntry must satisfy the seam's editor contract so Session.Bind binds it.
func TestRichEntry_ImplementsAccessibleTexter(t *testing.T) {
	var _ core.AccessibleTexter = NewRichEntry()
}
