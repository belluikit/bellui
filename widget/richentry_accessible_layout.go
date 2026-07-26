package widget

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// RichEntry exposes its visual layout to assistive technologies (#1116, Phase 1
// of #1115): the rendered (visual) line structure the user sees and the
// on-screen geometry of text ranges. Platform bridges consume this through
// [core.AccessibleTextLayouter] for what-you-see line navigation and for the
// range-geometry queries behind the VoiceOver editing ring, screen-reader
// magnifiers and cursor tracking. It mirrors [Entry]'s implementation
// (widget/entry_accessible_layout.go), wiring RichEntry's existing document
// row model and atom/image/bidi-aware caret geometry into the interface.
//
// Unlike Entry, RichEntry never scrolls its own content — it lays out the full
// document and relies on an enclosing [container.Scroll] for scrolling — so its
// caret geometry is already in viewport (== content) coordinates with no scroll
// offset to subtract, exactly like [RichEntry.PreeditCursorRectangle].
var _ core.AccessibleTextLayouter = (*RichEntry)(nil)

// AccessibleLineCount returns the number of rendered (visual) rows — one per
// wrapped line and one per block image (see [core.AccessibleTextLayouter]).
func (e *RichEntry) AccessibleLineCount() int {
	return e.doc.rowCount()
}

// AccessibleLineForOffset returns the zero-based visual row containing the rune
// offset, clamped to the content (see [core.AccessibleTextLayouter]).
func (e *RichEntry) AccessibleLineForOffset(off int) int {
	off = clampInt(off, 0, e.doc.len())
	row, _ := e.doc.offsetToRowCol(off)
	return row
}

// AccessibleLineRange returns the half-open rune range of the zero-based visual
// row (see [core.AccessibleTextLayouter]): end is the start of the following
// row — covering a terminating newline or the soft-wrap boundary that sits in
// the one-position gap between rows — or the content length on the last row.
func (e *RichEntry) AccessibleLineRange(line int) (start, end int, ok bool) {
	rows := e.doc.activeRowModel()
	n := len(rows)
	if line < 0 || line >= n {
		return 0, 0, false
	}
	start = rows[line].start
	if line+1 < n {
		end = rows[line+1].start
	} else {
		end = e.doc.len()
	}
	if end < start {
		end = start
	}
	return start, end, true
}

// AccessibleRangeBounds returns one rectangle per visible visual row touched by
// the rune range, in RichEntry's own coordinate space — the same space as
// [RichEntry.PreeditCursorRectangle] (see [core.AccessibleTextLayouter]). Rows
// are laid out top to bottom, so once a row starts below the widget viewport the
// walk stops; each rectangle is clipped to the viewport and a row clipped away
// entirely is omitted. Each rectangle spans the caret rectangles at the range's
// endpoints within that row — an approximation on bidirectional rows, where
// reordered runs may extend past the endpoint span.
//
// RichEntry does not virtualize, so a range spanning many rows in a widget as
// tall as its content yields one rectangle per row; the enclosing
// [container.Scroll] clips the display. In practice the bridges query small
// ranges (the selection/marked range for the VoiceOver ring), so this is
// bounded by the selection, not the document.
func (e *RichEntry) AccessibleRangeBounds(start, end int) []geo.Rectangle {
	rows := e.doc.activeRowModel()
	rowCount := len(rows)
	if rowCount == 0 {
		return nil
	}
	if start > end {
		start, end = end, start
	}
	l := e.doc.len()
	start = clampInt(start, 0, l)
	end = clampInt(end, 0, l)

	startRow, startCol := e.doc.offsetToRowCol(start)
	endRow, endCol := e.doc.offsetToRowCol(end)
	if endRow >= rowCount {
		endRow = rowCount - 1
		endCol = rows[endRow].end - rows[endRow].start
	}

	view := e.Size()

	var out []geo.Rectangle
	for row := startRow; row <= endRow && row < rowCount; row++ {
		rw := rows[row]
		colFrom := 0
		if row == startRow {
			colFrom = startCol
		}
		colTo := rw.end - rw.start
		if row == endRow {
			colTo = endCol
		}
		if colTo < colFrom {
			colTo = colFrom
		}

		p1, s1 := e.caretRectAt(row, colFrom, rw.image)
		if p1.Y >= view.Height {
			break // below the viewport; later rows only move further down
		}
		rect := geo.Rectangle{Pos: p1, Size: s1}
		if colTo > colFrom {
			p2, _ := e.caretRectAt(row, colTo, rw.image)
			x1 := min(p1.X, p2.X)
			x2 := max(p1.X, p2.X)
			if x2 == x1 {
				x2 = x1 + s1.Width
			}
			rect = geo.Rectangle{Pos: geo.NewPos(x1, p1.Y), Size: geo.NewSize(x2-x1, s1.Height)}
		}

		// Clip to the widget viewport; a row clipped away entirely is omitted.
		x1 := max(rect.Pos.X, 0)
		y1 := max(rect.Pos.Y, 0)
		x2 := min(rect.Pos.X+rect.Size.Width, view.Width)
		y2 := min(rect.Pos.Y+rect.Size.Height, view.Height)
		if x2 <= x1 || y2 <= y1 {
			continue
		}
		out = append(out, geo.Rectangle{Pos: geo.NewPos(x1, y1), Size: geo.NewSize(x2-x1, y2-y1)})
	}
	return out
}
