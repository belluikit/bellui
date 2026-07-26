package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestEntry_CaretRect(t *testing.T) {
	test.NewTempApp(t)

	entry := widget.NewEntry()
	entry.MultiLine = true
	entry.TextStyle.Monospace = true
	entry.SetText("abcdef\n\tindented\nαβγδ")

	// X grows monotonically across columns on row 0, and the step is uniform
	// for a monospace font.
	r0 := entry.CaretRect(0, 0)
	r1 := entry.CaretRect(0, 1)
	r2 := entry.CaretRect(0, 2)
	assert.Greater(t, r1.Pos.X, r0.Pos.X, "column 1 should be right of column 0")
	assert.Greater(t, r2.Pos.X, r1.Pos.X, "column 2 should be right of column 1")
	assert.InDelta(t, r1.Pos.X-r0.Pos.X, r2.Pos.X-r1.Pos.X, 0.01, "monospace step should be uniform")

	// The caret cell has a positive width and height.
	assert.Greater(t, r0.Size.Width, float32(0))
	assert.Greater(t, r0.Size.Height, float32(0))

	// Lower rows sit below higher ones, by a uniform stride.
	row0 := entry.CaretRect(0, 0)
	row1 := entry.CaretRect(1, 0)
	row2 := entry.CaretRect(2, 0)
	assert.Greater(t, row1.Pos.Y, row0.Pos.Y)
	assert.InDelta(t, row1.Pos.Y-row0.Pos.Y, row2.Pos.Y-row1.Pos.Y, 0.01, "row stride should be uniform")

	// A leading tab advances X past a single space-width column.
	tabCol1 := entry.CaretRect(1, 1) // just after the tab on row 1
	space := r1.Pos.X - r0.Pos.X
	assert.Greater(t, tabCol1.Pos.X-entry.CaretRect(1, 0).Pos.X, space, "a tab should be wider than one space")

	// Multi-byte runes (row 2 = αβγδ) advance one column per rune.
	mb0 := entry.CaretRect(2, 0)
	mb1 := entry.CaretRect(2, 1)
	assert.Greater(t, mb1.Pos.X, mb0.Pos.X)

	// CaretRect agrees with the live cursor position at the same logical spot.
	entry.CursorRow, entry.CursorColumn = 0, 3
	assert.Equal(t, entry.CursorPosition(), entry.CaretRect(0, 3).Pos,
		"CaretRect.Pos must match CursorPosition at the same (row,col)")
}

func TestEntry_CaretRect_OutOfRange(t *testing.T) {
	test.NewTempApp(t)

	entry := widget.NewEntry()
	entry.MultiLine = true
	entry.SetText("abc\ndef")

	// Negative, past-last-row, and past-end-of-line all yield the zero rect.
	assert.True(t, entry.CaretRect(-1, 0).IsZero(), "negative row")
	assert.True(t, entry.CaretRect(0, -1).IsZero(), "negative col")
	assert.True(t, entry.CaretRect(5, 0).IsZero(), "row past end")
	assert.True(t, entry.CaretRect(0, 4).IsZero(), "col past end of line")

	// In-range positions, including end-of-line (col == len), are non-zero.
	assert.False(t, entry.CaretRect(0, 0).IsZero())
	assert.False(t, entry.CaretRect(0, 3).IsZero(), "end-of-line caret is valid")
}
