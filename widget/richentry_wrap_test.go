package widget

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// wrappedRichEntry lays out a word-wrapping RichEntry narrow enough to force the
// given segments onto multiple visual rows.
func wrappedRichEntry(t *testing.T, segs []RichTextSegment) (*RichEntry, bellui.Window) {
	t.Helper()
	e := NewRichEntry()
	e.Wrapping = bellui.TextWrapWord
	e.SetSegments(segs)
	w := test.NewWindow(e)
	w.Resize(geo.NewSize(120, 200))
	require.Greater(t, e.provider.rows(), 1, "content must actually wrap for this test")
	return e, w
}

// TestRichEntry_WrapRowModelMatchesProvider is the core pre-rebrand tracker #653 invariant: the
// wrapping-aware row model tracks the provider's real rowBounds exactly — same
// row count, starting at 0, covering every position, contiguous with at most the
// one consumed separator between rows.
func TestRichEntry_WrapRowModelMatchesProvider(t *testing.T) {
	test.NewTempApp(t)
	e, w := wrappedRichEntry(t, []RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "the quick brown fox jumps over the lazy dog"},
	})
	defer w.Close()

	rows := e.doc.activeRowModel()
	require.Equal(t, e.provider.rows(), len(rows), "row model matches the provider's row count")
	assert.Equal(t, 0, rows[0].start, "first row starts at the document start")
	assert.Equal(t, e.doc.len(), rows[len(rows)-1].end, "last row reaches the document end")

	for r := 0; r < len(rows)-1; r++ {
		assert.LessOrEqual(t, rows[r].end, rows[r+1].start, "rows are ordered and non-overlapping")
		gap := rows[r+1].start - rows[r].end
		assert.LessOrEqual(t, gap, 1, "at most one separator rune (the wrapped space) sits between rows")
		assert.GreaterOrEqual(t, gap, 0)
	}
}

// TestRichEntry_WrapOffsetRoundTrip walks every caret offset and checks the
// offset<->(row,col) mapping is self-consistent under wrapping: each offset lands
// on a real row whose bounds contain the round-tripped offset, and re-mapping is
// idempotent.
func TestRichEntry_WrapOffsetRoundTrip(t *testing.T) {
	test.NewTempApp(t)
	e, w := wrappedRichEntry(t, []RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "the quick brown fox jumps over the lazy dog"},
	})
	defer w.Close()

	rows := e.doc.activeRowModel()
	for off := 0; off <= e.doc.len(); off++ {
		row, col := e.doc.offsetToRowCol(off)
		require.GreaterOrEqual(t, row, 0)
		require.Less(t, row, len(rows))
		back := e.doc.rowColToOffset(row, col)
		assert.GreaterOrEqual(t, back, rows[row].start, "round-trip stays within its row (offset %d)", off)
		assert.LessOrEqual(t, back, rows[row].end)

		r2, c2 := e.doc.offsetToRowCol(back)
		assert.Equal(t, row, r2, "row mapping is idempotent (offset %d)", off)
		assert.Equal(t, col, c2, "col mapping is idempotent (offset %d)", off)
	}
}

// TestRichEntry_WrapCaretDownUpNavigatesVisualRows verifies arrow Down/Up move
// the caret between *visual* rows of one wrapped logical line, rather than jumping
// to the document end (the before pre-rebrand tracker #653 newline-count behaviour, which saw one row).
func TestRichEntry_WrapCaretDownUpNavigatesVisualRows(t *testing.T) {
	test.NewTempApp(t)
	e, w := wrappedRichEntry(t, []RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "the quick brown fox jumps over the lazy dog"},
	})
	defer w.Close()

	e.moveCaret(0, false) // row 0, col 0
	e.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	row, _ := e.doc.offsetToRowCol(e.caret)
	assert.Equal(t, 1, row, "Down moves to the next visual row")
	assert.Greater(t, e.caret, 0)
	assert.Less(t, e.caret, e.doc.len(), "caret is not stranded at the document end")

	e.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	row, _ = e.doc.offsetToRowCol(e.caret)
	assert.Equal(t, 0, row, "Up returns to the first visual row")
}

// TestRichEntry_WrapHomeEndUseVisualRow verifies Home/End land on the visual row
// boundaries, not the logical-line extremes.
func TestRichEntry_WrapHomeEndUseVisualRow(t *testing.T) {
	test.NewTempApp(t)
	e, w := wrappedRichEntry(t, []RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "the quick brown fox jumps over the lazy dog"},
	})
	defer w.Close()

	rows := e.doc.activeRowModel()
	// Put the caret somewhere in the middle of row 1, then Home/End.
	mid := (rows[1].start + rows[1].end) / 2
	e.moveCaret(mid, false)
	e.TypedKey(&event.KeyEvent{Name: event.KeyHome})
	assert.Equal(t, rows[1].start, e.caret, "Home goes to the visual row start")
	e.TypedKey(&event.KeyEvent{Name: event.KeyEnd})
	assert.Equal(t, rows[1].end, e.caret, "End goes to the visual row end")
}

// TestRichEntry_WrapMultiSegment checks the row model stays correct when a wrap
// falls across styled runs (a bold run in the middle) — the segBase-of-first-
// segment mapping must still place every row at the right document position.
func TestRichEntry_WrapMultiSegment(t *testing.T) {
	test.NewTempApp(t)
	e, w := wrappedRichEntry(t, []RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "the quick "},
		&TextSegment{Style: boldStyle(), Text: "brown fox jumps"},
		&TextSegment{Style: normalStyle(), Text: " over the lazy dog"},
	})
	defer w.Close()

	rows := e.doc.activeRowModel()
	require.Equal(t, e.provider.rows(), len(rows))
	assert.Equal(t, 0, rows[0].start)
	assert.Equal(t, e.doc.len(), rows[len(rows)-1].end)
	// starts strictly increase, so no row collapses onto another.
	for r := 0; r < len(rows)-1; r++ {
		assert.Less(t, rows[r].start, rows[r+1].start, "row starts advance monotonically")
	}
}

// TestRichEntry_WrapWithBlockImage checks a block image keeps its own single-
// position row inside otherwise-wrapping content, and that positions on both
// sides map to the right rows.
func TestRichEntry_WrapWithBlockImage(t *testing.T) {
	test.NewTempApp(t)
	e, w := wrappedRichEntry(t, []RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "the quick brown fox jumps"},
		&ImageSegment{Image: image.NewNRGBA(image.Rect(0, 0, 8, 8))},
		&TextSegment{Style: normalStyle(), Text: "over the lazy dog again"},
	})
	defer w.Close()

	rows := e.doc.activeRowModel()
	require.Equal(t, e.provider.rows(), len(rows))
	assert.Equal(t, e.doc.len(), rows[len(rows)-1].end)

	// Exactly one image row, occupying a single position.
	imgRows := 0
	for r, rw := range rows {
		if rw.image {
			imgRows++
			assert.Equal(t, 1, rw.end-rw.start, "an image row is one position")
			assert.True(t, e.doc.rowIsImage(r))
		}
	}
	assert.Equal(t, 1, imgRows, "the one block image is its own row")
}

// TestRichEntry_WrapOffStillNewlineModel guards that the default (wrap off) path
// is untouched: it uses the newline-count row model, one row per '\n' line.
func TestRichEntry_WrapOffStillNewlineModel(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "alpha\nbeta\ngamma"}})
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 120))

	assert.Nil(t, e.doc.wrapRows, "wrap off installs no wrapping row model")
	assert.Equal(t, 3, e.doc.rowCount(), "one row per newline-separated line")
}

// TestRichEntry_RowModelMemoInvalidatesOnEdit guards the pre-rebrand tracker #671 wrap-off row-model
// memo: after an edit that changes the row structure, the cached model must be
// rebuilt, not served stale.
func TestRichEntry_RowModelMemoInvalidatesOnEdit(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "one line"}})
	require.Equal(t, 1, e.doc.rowCount())

	e.moveCaret(e.doc.len(), false)
	e.insertRunes([]rune("\nsecond")) // add a newline → a new row
	assert.Equal(t, 2, e.doc.rowCount(), "row model reflects the inserted newline")

	e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace}) // delete "second" + newline
	assert.Equal(t, 1, e.doc.rowCount(), "removing the newline collapses back to one row")
}

// TestRichEntry_IMECaretOnWrappedRow is the pre-rebrand tracker #891 fix: while composing in a
// wrapped document, the caret (and thus the IME candidate window) must resolve to
// the correct visual row, not row 0. Before, caretDoc used a wrap-off temp doc
// during composition, so a no-newline wrapped line collapsed to one row and the
// candidate window sat at the top.
func TestRichEntry_IMECaretOnWrappedRow(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.Wrapping = bellui.TextWrapWord
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "the quick brown fox jumps over the lazy dog"},
	})
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(120, 200))
	require.Greater(t, e.provider.rows(), 2, "content wraps to several rows")

	rows := e.doc.activeRowModel()
	e.moveCaret(rows[2].start+1, false) // caret on a deep visual row
	e.FocusGained()
	e.Compose(&event.PreeditEvent{Text: "ni", CursorPos: 2})

	cd, caret := e.caretDoc()
	row, _ := cd.offsetToRowCol(caret)
	assert.Positive(t, row, "composing caret resolves to a deep visual row, not row 0")

	rect := e.PreeditCursorRectangle()
	y0, _ := e.provider.RowGeometry(0)
	assert.Greater(t, rect.Pos.Y, y0, "candidate rectangle sits below the first visual row")

	e.ComposeCancel()
}

// TestRichEntry_IMECaretWrapOffUnchanged guards that the wrap-off composition path
// is unchanged: the caret doc uses the newline model, and a caret on the second
// logical line resolves to row 1.
func TestRichEntry_IMECaretWrapOffUnchanged(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "alpha\nbeta"}})
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 120))

	e.moveCaret(7, false) // inside "beta" (second line)
	e.FocusGained()
	e.Compose(&event.PreeditEvent{Text: "x", CursorPos: 1})

	cd, caret := e.caretDoc()
	row, _ := cd.offsetToRowCol(caret)
	assert.Equal(t, 1, row, "wrap-off composing caret stays on the newline-derived row")
	assert.Nil(t, cd.wrapRows, "wrap-off composition installs no wrap model")
	e.ComposeCancel()
}
