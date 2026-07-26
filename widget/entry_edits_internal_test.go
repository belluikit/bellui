package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// newLazyEntry builds a shown multi-line entry with the lazy single-segment
// fast path active (the newDeferSyncEntry sibling without DeferTextSync).
func newLazyEntry(t *testing.T, text string) (*Entry, bellui.Window) {
	t.Helper()
	e := NewMultiLineEntry()
	e.SetText(text)
	w := test.NewWindow(e)
	w.Resize(geo.NewSize(400, 300))
	p := e.textProvider()
	p.updateRowBounds()
	require.True(t, p.lazyCanEdit(), "expected the lazy fast path to be active")
	return e, w
}

func TestEntrySelectionRange(t *testing.T) {
	e := NewMultiLineEntry()
	e.SetText("héllo\nwörld")

	_, ok := e.SelectionRange()
	assert.False(t, ok, "no selection yet")

	// Forward selection: anchor before caret.
	e.Select(0, 2, 1, 3)
	r, ok := e.SelectionRange()
	require.True(t, ok)
	assert.Equal(t, TextRange{Start: 2, End: 9}, r)
	assert.Equal(t, "llo\nwör", e.SelectedText())
	assert.Len(t, []rune(e.SelectedText()), r.End-r.Start)
	// The caret is at the End side; the anchor is recoverable by elimination.
	assert.Equal(t, r.End, e.CursorTextOffset())

	// Backward selection normalizes to Start <= End, caret at the Start side.
	e.Select(1, 3, 0, 2)
	r, ok = e.SelectionRange()
	require.True(t, ok)
	assert.Equal(t, TextRange{Start: 2, End: 9}, r)
	assert.Equal(t, r.Start, e.CursorTextOffset())

	e.Unselect()
	_, ok = e.SelectionRange()
	assert.False(t, ok, "cleared selection")
}

func TestEntryInsertText(t *testing.T) {
	e := NewEntry()
	e.SetText("héllo")
	e.CursorColumn = 2

	changed := 0
	e.OnChanged = func(string) { changed++ }

	e.InsertText("ab c")
	assert.Equal(t, "héab cllo", e.Text)
	assert.Equal(t, 6, e.CursorTextOffset())
	assert.Equal(t, 1, changed, "a multi-rune insert notifies once")

	// One edit, one undo step.
	e.Undo()
	assert.Equal(t, "héllo", e.Text)
	e.Redo()
	assert.Equal(t, "héab cllo", e.Text)

	e.InsertText("")
	assert.Equal(t, "héab cllo", e.Text, "empty insert is a no-op")

	e.ReadOnly = true
	e.InsertText("x")
	assert.Equal(t, "héab cllo", e.Text, "read-only rejects the insert")
}

func TestEntryInsertText_ReplacesSelectionAndGates(t *testing.T) {
	e := NewEntry()
	e.SetText("abcdef")
	e.Select(0, 1, 0, 3)

	e.BeforeChange = func(sel TextRange, next string) bool {
		assert.Equal(t, TextRange{Start: 1, End: 3}, sel)
		return next != "veto"
	}
	e.InsertText("veto")
	assert.Equal(t, "abcdef", e.Text, "vetoed insert leaves text untouched")
	assert.Equal(t, "bc", e.SelectedText(), "vetoed insert leaves the selection")

	e.InsertText("X")
	assert.Equal(t, "aXdef", e.Text)
	assert.Equal(t, 2, e.CursorTextOffset())
}

func TestEntryApplyEdits_BlockIndent(t *testing.T) {
	// The #433 consumer scenario: indent every selected line as one undo step.
	e := NewMultiLineEntry()
	e.SetText("aa\nbb\ncc")

	changed := 0
	var lastChanged string
	e.OnChanged = func(s string) { changed++; lastChanged = s }

	ok := e.ApplyEdits([]TextEdit{
		{Start: 0, End: 0, Text: "\t"},
		{Start: 3, End: 3, Text: "\t"},
		{Start: 6, End: 6, Text: "\t"},
	})
	require.True(t, ok)
	assert.Equal(t, "\taa\n\tbb\n\tcc", e.Text)
	assert.Equal(t, 1, changed, "the batch notifies once")
	assert.Equal(t, "\taa\n\tbb\n\tcc", lastChanged)
	assert.Equal(t, 9, e.CursorTextOffset(), "caret after the last inserted indent")

	// The whole batch is exactly one undo step.
	assert.True(t, e.undoStack.CanUndo())
	e.Undo()
	assert.Equal(t, "aa\nbb\ncc", e.Text)
	assert.False(t, e.undoStack.CanUndo(), "one undo press reverted the whole batch")
	assert.Equal(t, 6, e.CursorTextOffset(), "undo caret at the last edit's original offset")

	e.Redo()
	assert.Equal(t, "\taa\n\tbb\n\tcc", e.Text)
	assert.Equal(t, 3, changed, "undo and redo notified once each")
}

func TestEntryApplyEdits_ReplaceAndDelete(t *testing.T) {
	e := NewEntry()
	e.SetText("abcdef")

	// Given out of order; replace + pure deletion in one batch.
	ok := e.ApplyEdits([]TextEdit{
		{Start: 4, End: 5}, // delete "e"
		{Start: 1, End: 2, Text: "XX"},
	})
	require.True(t, ok)
	assert.Equal(t, "aXXcdf", e.Text)
	assert.Equal(t, 5, e.CursorTextOffset(), "caret at the last edit, shifted by earlier deltas")

	e.Undo()
	assert.Equal(t, "abcdef", e.Text)
	e.Redo()
	assert.Equal(t, "aXXcdf", e.Text)
	assert.Equal(t, 5, e.CursorTextOffset())
}

func TestEntryApplyEdits_ClearsSelection(t *testing.T) {
	e := NewMultiLineEntry()
	e.SetText("aa\nbb")
	e.Select(0, 0, 1, 2)

	require.True(t, e.ApplyEdits([]TextEdit{{Start: 0, End: 0, Text: "\t"}}))
	_, ok := e.SelectionRange()
	assert.False(t, ok, "a batch edit drops the selection")
	assert.Equal(t, "", e.SelectedText())
}

func TestEntryApplyEdits_SameOffsetInsertsKeepOrder(t *testing.T) {
	e := NewEntry()
	e.SetText("abcd")

	require.True(t, e.ApplyEdits([]TextEdit{
		{Start: 2, End: 2, Text: "X"},
		{Start: 2, End: 2, Text: "Y"},
	}))
	assert.Equal(t, "abXYcd", e.Text)
}

func TestEntryApplyEdits_ClampsAndSwapsBounds(t *testing.T) {
	e := NewEntry()
	e.SetText("abcd")

	require.True(t, e.ApplyEdits([]TextEdit{
		{Start: 3, End: 1, Text: "Z"},    // reversed bounds
		{Start: 4, End: 99, Text: "end"}, // End clamped to the document
	}))
	assert.Equal(t, "aZdend", e.Text)
}

func TestEntryApplyEdits_Rejections(t *testing.T) {
	e := NewEntry()
	e.SetText("abcdef")

	assert.False(t, e.ApplyEdits(nil), "empty batch")
	assert.False(t, e.ApplyEdits([]TextEdit{{Start: 2, End: 2}}), "all-no-op batch")
	assert.False(t, e.ApplyEdits([]TextEdit{
		{Start: 1, End: 4, Text: "x"},
		{Start: 3, End: 5, Text: "y"},
	}), "overlapping ranges")
	assert.Equal(t, "abcdef", e.Text)
	assert.False(t, e.undoStack.CanUndo(), "rejected batches never reach the undo stack")

	e.ReadOnly = true
	assert.False(t, e.ApplyEdits([]TextEdit{{Start: 0, End: 1, Text: "x"}}), "read-only")
	e.ReadOnly = false

	// Touching ranges are not overlapping.
	assert.True(t, e.ApplyEdits([]TextEdit{
		{Start: 1, End: 3, Text: "x"},
		{Start: 3, End: 5, Text: "y"},
	}))
	assert.Equal(t, "axyf", e.Text)
}

func TestEntryApplyEdits_VetoIsAtomic(t *testing.T) {
	e := NewEntry()
	e.SetText("abcdef")

	changed := 0
	e.OnChanged = func(string) { changed++ }

	// BeforeChange vetoes the second edit: nothing may be applied.
	e.BeforeChange = func(sel TextRange, next string) bool { return sel.Start < 3 }
	assert.False(t, e.ApplyEdits([]TextEdit{
		{Start: 0, End: 1, Text: "x"},
		{Start: 4, End: 5, Text: "y"},
	}))
	assert.Equal(t, "abcdef", e.Text)
	assert.Zero(t, changed)
	assert.False(t, e.undoStack.CanUndo())
	e.BeforeChange = nil

	// A protected range overlapping any edit rejects the whole batch too.
	e.ProtectedRanges = []ProtectedRange{{Start: 4, End: 6}}
	assert.False(t, e.ApplyEdits([]TextEdit{
		{Start: 0, End: 1, Text: "x"},
		{Start: 4, End: 5, Text: "y"},
	}))
	assert.Equal(t, "abcdef", e.Text)
	assert.False(t, e.undoStack.CanUndo())
}

func TestEntryApplyEdits_Deferred(t *testing.T) {
	e, w := newDeferSyncEntry(t, "aa\nbb\ncc")
	defer w.Close()

	changed := 0
	e.OnChanged = func(string) { changed++ }
	p := e.textProvider()

	require.True(t, e.ApplyEdits([]TextEdit{
		{Start: 0, End: 0, Text: "\t"},
		{Start: 3, End: 3, Text: "\t"},
		{Start: 6, End: 6, Text: "\t"},
	}))

	// The batch stayed in the buffer: Text and notifications wait for a sync
	// point while rendering reads the live content.
	assert.Equal(t, "aa\nbb\ncc", e.Text)
	assert.Zero(t, changed)
	assert.True(t, p.deferredDirty)
	assert.Equal(t, "\taa\n\tbb\n\tcc", p.buf.String())

	got := e.SyncText()
	assert.Equal(t, "\taa\n\tbb\n\tcc", got)
	assert.Equal(t, 1, changed, "the sync point notifies once for the whole batch")

	// Still one undo step; history replay defers like any other edit.
	e.Undo()
	assert.Equal(t, "aa\nbb\ncc", p.buf.String())
	assert.Equal(t, "aa\nbb\ncc", e.SyncText())
	assert.False(t, e.undoStack.CanUndo())
}

func TestEntryApplyEdits_KeepsLazyFastPath(t *testing.T) {
	e, w := newLazyEntry(t, "aa\nbb\ncc")
	defer w.Close()
	p := e.textProvider()
	builds := p.lazyFullBuilds

	require.True(t, e.ApplyEdits([]TextEdit{
		{Start: 0, End: 0, Text: "\t"},
		{Start: 3, End: 4, Text: "B"},
	}))
	assert.Equal(t, "\taa\nBb\ncc", e.Text)

	// The identity chain survived the batch: subsequent edits stay incremental.
	e.CursorRow, e.CursorColumn = 0, 0
	e.TypedRune('x')
	e.Undo()
	e.Redo()
	assert.Equal(t, builds, p.lazyFullBuilds, "batch edits must not degrade to full rebuilds")
}
