package widget

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
)

func typeString(e *RichEntry, s string) {
	for _, r := range s {
		e.TypedRune(r)
	}
}

func TestRichEntry_UndoRedoTyping(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	w := test.NewWindow(e)
	defer w.Close()
	e.FocusGained()

	typeString(e, "hello")
	assert.Equal(t, "hello", e.String())

	e.Undo() // one merged step for the contiguous word
	assert.Equal(t, "", e.String())
	assert.Equal(t, 0, e.caret)

	e.Redo()
	assert.Equal(t, "hello", e.String())
	assert.Equal(t, 5, e.caret)
}

func TestRichEntry_UndoStopsAtWordBoundaries(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	w := test.NewWindow(e)
	defer w.Close()
	e.FocusGained()

	typeString(e, "foo bar")
	e.Undo() // removes "bar" (the run after the space)
	assert.Equal(t, "foo ", e.String())
	e.Undo() // removes "foo " up to (and incl.) the space-terminated run
	assert.Equal(t, "", e.String())
}

func TestRichEntry_UndoRedoPreservesStyle(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	w := test.NewWindow(e)
	defer w.Close()

	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "Hello World"}})
	e.selecting, e.anchor, e.caret = true, 6, 11 // "World"
	e.ApplyStyle(boldStyle())
	assert.Equal(t, []string{"Hello ", "World"}, segTexts(e.Segments()))

	e.Undo() // style change reverts, runs merge back
	assert.Equal(t, []string{"Hello World"}, segTexts(e.Segments()))
	assert.Equal(t, normalStyle(), e.Segments()[0].(*TextSegment).Style)

	e.Redo() // bold "World" restored with its style
	assert.Equal(t, []string{"Hello ", "World"}, segTexts(e.Segments()))
	assert.Equal(t, boldStyle(), e.Segments()[1].(*TextSegment).Style)
}

func TestRichEntry_UndoTypeOverSelectionRestoresStyledContent(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	w := test.NewWindow(e)
	defer w.Close()

	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "Hello "},
		&TextSegment{Style: boldStyle(), Text: "World"},
	})
	// Select the bold "World" and type over it.
	e.selecting, e.anchor, e.caret = true, 6, 11
	e.TypedRune('!')
	assert.Equal(t, "Hello !", e.String())

	e.Undo() // the bold "World" comes back with its style intact
	assert.Equal(t, "Hello World", e.String())
	assert.Equal(t, []string{"Hello ", "World"}, segTexts(e.Segments()))
	assert.Equal(t, boldStyle(), e.Segments()[1].(*TextSegment).Style)
}

func TestRichEntry_UndoRedoDelete(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	w := test.NewWindow(e)
	defer w.Close()

	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "abcdef"}})
	e.selecting, e.anchor, e.caret = false, 3, 3
	e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace}) // delete 'c'
	assert.Equal(t, "abdef", e.String())
	assert.Equal(t, 2, e.caret)

	e.Undo()
	assert.Equal(t, "abcdef", e.String())
	assert.Equal(t, 3, e.caret)
	e.Redo()
	assert.Equal(t, "abdef", e.String())
}

func TestRichEntry_NewEditTruncatesRedo(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	w := test.NewWindow(e)
	defer w.Close()
	e.FocusGained()

	typeString(e, "ab ")
	typeString(e, "cd")
	e.Undo() // drop "cd"
	assert.Equal(t, "ab ", e.String())
	typeString(e, "xy") // new edit — redo of "cd" is gone
	assert.Equal(t, "ab xy", e.String())
	e.Redo()
	assert.Equal(t, "ab xy", e.String(), "redo after a new edit is a no-op")
}

func TestRichEntry_SetSegmentsClearsUndo(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	w := test.NewWindow(e)
	defer w.Close()
	e.FocusGained()

	typeString(e, "abc")
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "fresh"}})
	e.Undo() // must not resurrect the pre-SetSegments content
	assert.Equal(t, "fresh", e.String())
}

func TestRichEntry_UndoRedoNoOpWhenEmpty(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	w := test.NewWindow(e)
	defer w.Close()

	e.Undo()
	e.Redo()
	assert.Equal(t, "", e.String())
}

// merge granularity is exercised via the stack directly for pointer-weight runs.
func TestRichEntryUndo_MergePointerWeightRuns(t *testing.T) {
	bold := RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Weight: bellui.Ptr(bellui.WeightBold)}}
	e1 := &richEntryEdit{at: 0, after: []RichTextSegment{&TextSegment{Style: bold, Text: "a"}}, caretAfter: 1}
	e2 := &richEntryEdit{at: 1, after: []RichTextSegment{&TextSegment{Style: RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Weight: bellui.Ptr(bellui.WeightBold)}}, Text: "b"}}, caretAfter: 2}
	assert.True(t, e1.tryMerge(e2), "value-equal pointer-weight insertions should merge")
	assert.Equal(t, 2, segsPositionLen(e1.after))
}
