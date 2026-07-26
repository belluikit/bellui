package widget

import (
	"sort"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// Entry exposes its visual layout to assistive technologies (#990): the
// soft-wrapped line structure the user actually sees, and on-screen geometry
// for text ranges. Platform bridges consume this through
// [core.AccessibleTextLayouter] for what-you-see line navigation and for the
// range-geometry queries behind screen-reader magnifiers and cursor tracking;
// without it they fall back to logical (newline-delimited) lines and
// element-level bounds.
var _ core.AccessibleTextLayouter = (*Entry)(nil)

// AccessibleLineCount returns the number of visual (soft-wrapped) lines of
// the laid-out content (see [core.AccessibleTextLayouter]).
func (e *Entry) AccessibleLineCount() int {
	return e.textProvider().rows()
}

// AccessibleLineForOffset returns the zero-based visual line containing the
// rune offset, clamped to the content (see [core.AccessibleTextLayouter]).
func (e *Entry) AccessibleLineForOffset(off int) int {
	row, _ := e.rowColFromTextPos(off)
	return row
}

// AccessibleLineRange returns the half-open rune range of the zero-based
// visual line (see [core.AccessibleTextLayouter]): end is the start of the
// following line — covering a terminating newline or the soft-wrap boundary
// — or the content length on the last line.
func (e *Entry) AccessibleLineRange(line int) (start, end int, ok bool) {
	provider := e.textProvider()
	n := provider.rows()
	if line < 0 || line >= n {
		return 0, 0, false
	}
	start = textPosFromRowCol(line, 0, provider)
	if line+1 < n {
		end = textPosFromRowCol(line+1, 0, provider)
	} else {
		end = provider.len()
	}
	return start, end, true
}

// AccessibleRangeBounds returns one rectangle per visible visual line touched
// by the rune range, in the Entry's own coordinate space adjusted for its
// scroll offset — the same space as [Entry.PreeditCursorRectangle] (see
// [core.AccessibleTextLayouter]). Lines scrolled out of the viewport are
// omitted, so the work and the result stay bounded by the viewport no matter
// how large the range: the first visible row is found by binary search over
// the (monotonically increasing) row tops, and the walk stops at the first
// row below the fold. Each rectangle spans the caret rectangles at the
// range's endpoints within that row — an approximation on bidirectional rows,
// where reordered runs may extend past the endpoint span.
func (e *Entry) AccessibleRangeBounds(start, end int) []geo.Rectangle {
	provider := e.textProvider()
	rows := provider.rows()
	if rows == 0 {
		return nil
	}
	if start > end {
		start, end = end, start
	}
	l := provider.len()
	start = clampInt(start, 0, l)
	end = clampInt(end, 0, l)

	startRow, startCol := e.rowColFromTextPos(start)
	endRow, endCol := e.rowColFromTextPos(end)
	if endRow >= rows {
		endRow, endCol = rows-1, provider.rowLength(rows-1)
	}

	var scrollOff geo.Position
	if e.scroll != nil {
		scrollOff = e.scroll.Offset
	}
	view := e.Size()

	// First candidate row whose bottom reaches into the viewport. Row tops are
	// monotone in the row index (folded rows collapse to zero height), so the
	// rows above the fold form a prefix.
	first := startRow + sort.Search(endRow-startRow+1, func(i int) bool {
		r := e.caretRect(startRow+i, 0, true)
		return r.Pos.Y+r.Size.Height > scrollOff.Y
	})

	var out []geo.Rectangle
	for row := first; row <= endRow && row < rows; row++ {
		if provider.rowHidden(row) {
			continue // a folded row is not visible (#141)
		}
		colFrom := 0
		if row == startRow {
			colFrom = startCol
		}
		colTo := provider.rowLength(row)
		if row == endRow {
			colTo = endCol
		}
		if colTo < colFrom {
			colTo = colFrom
		}

		r1 := e.caretRect(row, colFrom, true)
		if r1.Pos.Y-scrollOff.Y >= view.Height {
			break // below the viewport; later rows only move further down
		}
		rect := r1
		if colTo > colFrom {
			r2 := e.caretRect(row, colTo, true)
			x1 := min(r1.Pos.X, r2.Pos.X)
			x2 := max(r1.Pos.X, r2.Pos.X)
			if x2 == x1 {
				x2 = x1 + r1.Size.Width
			}
			rect = geo.Rectangle{Pos: geo.NewPos(x1, r1.Pos.Y), Size: geo.NewSize(x2-x1, r1.Size.Height)}
		}
		rect.Pos = rect.Pos.Subtract(scrollOff)

		// Clip to the widget viewport; a line clipped away entirely is omitted.
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
