package widget

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
)

// blockSegment is a standalone block (Inline() == false) non-text RichTextSegment
// with a controllable Textual() and visual size, standing in for an image/table on
// its own row (pre-rebrand issue #50).
type blockSegment struct {
	text          string
	width, height float32
}

func (s *blockSegment) Inline() bool             { return false }
func (s *blockSegment) Textual() string          { return s.text }
func (s *blockSegment) Update(core.CanvasObject) {}

func (s *blockSegment) Visual() core.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(geo.NewSize(s.width, s.height))
	return r
}

func (s *blockSegment) Select(geo.Position, geo.Position) {}
func (s *blockSegment) SelectedText() string              { return "" }
func (s *blockSegment) Unselect()                         {}

// blockRowRectWidth builds the selection and returns the width of the rectangle
// drawn on the given row's y, or 0 if none covers it.
func blockRowRectWidth(t *testing.T, rt *RichText, row int) float32 {
	t.Helper()
	rr := cache.Renderer(rt.selection).(*selectableRenderer)
	rr.buildSelection()
	y, _ := rt.RowGeometry(row)
	for _, o := range rr.selections {
		if o.Size().Width > 0 && o.Position().Y == y {
			return o.Size().Width
		}
	}
	return 0
}

func newBlockRichText(t *testing.T, before, after string) (*RichText, *blockSegment) {
	t.Helper()
	blk := &blockSegment{text: "IMG", width: 40, height: 30}
	rt := NewRichText(
		&TextSegment{Text: before, Style: RichTextStyleParagraph},
		blk,
		&TextSegment{Text: after, Style: RichTextStyleParagraph},
	)
	rt.Wrapping = bellui.TextWrapOff
	rt.Selectable = true
	w := test.NewTempWindow(t, rt)
	w.Resize(geo.NewSize(320, 200))
	require.Equal(t, 3, rt.rows())
	require.Equal(t, 0, rt.rowLength(1), "the block row must have no text columns")
	return rt, blk
}

// TestSelectable_BlockSegment_CopyAndHighlight verifies that a selection fully
// spanning a standalone block non-text segment copies its Textual() in logical
// order and draws a full-row highlight for it (pre-rebrand issue #50, LTR).
func TestSelectable_BlockSegment_CopyAndHighlight(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rt, blk := newBlockRichText(t, "before", "after")

	// Full span (row0 → end of row2): the block is interior, so included.
	got := selectRange(rt, 0, 0, 2, rt.rowLength(2))
	assert.Equal(t, "before\nIMG\nafter", got)

	w := blockRowRectWidth(t, rt, 1)
	assert.InDelta(t, blk.width+1, w, 1.0, "the block row must get a full-width highlight")
}

// TestSelectable_BlockSegment_LastRowIncluded verifies that a selection reaching
// the block row as its last row now covers it — a block is atomic, so reaching its
// row selects it (pre-rebrand issue #53). Copy and highlight stay in sync.
func TestSelectable_BlockSegment_LastRowIncluded(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rt, blk := newBlockRichText(t, "before", "after")

	// Select from row0 down onto the block row (its only caret col, 0).
	got := selectRange(rt, 0, 0, 1, 0)
	assert.Equal(t, "before\nIMG", got)

	assert.InDelta(t, blk.width+1, blockRowRectWidth(t, rt, 1), 1.0,
		"a block row reached by the selection must be highlighted")
}

// TestSelectable_BlockSegment_SelectAllTrailing verifies select-all on content
// ending with a block row copies and highlights the trailing block (pre-rebrand issue #53).
func TestSelectable_BlockSegment_SelectAllTrailing(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	blk := &blockSegment{text: "IMG", width: 40, height: 30}
	rt := NewRichText(
		&TextSegment{Text: "before", Style: RichTextStyleParagraph},
		blk,
	)
	rt.Wrapping = bellui.TextWrapOff
	rt.Selectable = true
	w := test.NewTempWindow(t, rt)
	w.Resize(geo.NewSize(320, 200))
	require.Equal(t, 2, rt.rows())
	require.True(t, rt.isBlockRow(1), "row 1 must be a standalone block row")

	rt.selection.selectAll()
	assert.Equal(t, "before\nIMG", rt.SelectedText())
	assert.InDelta(t, blk.width+1, blockRowRectWidth(t, rt, 1), 1.0,
		"select-all must highlight the trailing block")
}

// TestSelectable_BlockSegment_TripleTapSelects verifies a triple-tap
// (selectCurrentRow) on a standalone block row selects and highlights it — the
// zero-column range needs the synthetic trailing column (pre-rebrand issue #56).
func TestSelectable_BlockSegment_TripleTapSelects(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rt, blk := newBlockRichText(t, "before", "after")
	sel := rt.selection
	sel.selecting = true // a double-tap precedes a triple-tap in the real flow
	sel.cursorRow = 1    // cursor on the block row
	sel.selectCurrentRow(false)

	assert.Equal(t, "IMG", rt.SelectedText())
	assert.InDelta(t, blk.width+1, blockRowRectWidth(t, rt, 1), 1.0,
		"triple-tap must select & highlight the block row")
}

// TestSelectable_BlockSegment_DragSelects verifies a drag confined to a standalone
// block row selects and highlights it (pre-rebrand issue #56).
func TestSelectable_BlockSegment_DragSelects(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rt, blk := newBlockRichText(t, "before", "after")
	y, h := rt.RowGeometry(1)
	pos := geo.NewPos(25, y+h/2)
	rt.selection.dragged(&event.DragEvent{
		PointEvent: event.PointEvent{Position: pos},
		Dragged:    geo.Delta{DX: 15, DY: 0}, // start and end both within the block row
	})

	assert.Equal(t, "IMG", rt.SelectedText())
	assert.InDelta(t, blk.width+1, blockRowRectWidth(t, rt, 1), 1.0,
		"a drag within the block row must select & highlight it")
}

// TestSelectable_BlockSegment_SelectAllOnlyRow verifies select-all on a RichText
// whose only row is a standalone block copies and highlights it — the single-row
// case needs a synthetic trailing column to form a non-degenerate range (pre-rebrand tracker #53).
func TestSelectable_BlockSegment_SelectAllOnlyRow(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	blk := &blockSegment{text: "IMG", width: 40, height: 30}
	rt := NewRichText(blk)
	rt.Wrapping = bellui.TextWrapOff
	rt.Selectable = true
	w := test.NewTempWindow(t, rt)
	w.Resize(geo.NewSize(320, 200))
	require.Equal(t, 1, rt.rows())
	require.True(t, rt.isBlockRow(0), "the only row must be a standalone block row")

	rt.selection.selectAll()
	assert.Equal(t, "IMG", rt.SelectedText())
	assert.InDelta(t, blk.width+1, blockRowRectWidth(t, rt, 0), 1.0,
		"select-all must highlight the only (block) row")
}

// TestSelectable_BlockSegment_RTLSurrounding verifies the same splicing with RTL
// text rows around the block (acceptance covers LTR and RTL surrounding rows).
func TestSelectable_BlockSegment_RTLSurrounding(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rt, _ := newBlockRichText(t, "אבג", "דהו")

	got := selectRange(rt, 0, 0, 2, rt.rowLength(2))
	assert.Equal(t, "אבג\nIMG\nדהו", got)
}
