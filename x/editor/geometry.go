package editor

import (
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
)

// TextRange is a half-open [Start, End) range over the document, in 0-based
// (row, col) coordinates. It is the shared range type across the editor: span
// decorations, protected ranges, redactions, fold regions, diagnostics, and the
// LSP-style completion/signature/hover layer.
type TextRange struct {
	StartRow, StartCol, EndRow, EndCol int
}

// IsEmpty reports whether the range covers no characters.
func (r TextRange) IsEmpty() bool {
	return r.StartRow > r.EndRow ||
		(r.StartRow == r.EndRow && r.StartCol >= r.EndCol)
}

// Contains reports whether (row, col) falls within the half-open range: at or
// after the start, strictly before the end.
func (r TextRange) Contains(row, col int) bool {
	if row < r.StartRow || row > r.EndRow {
		return false
	}
	if row == r.StartRow && col < r.StartCol {
		return false
	}
	if row == r.EndRow && col >= r.EndCol {
		return false
	}
	return true
}

// Union returns the smallest range covering both r and other. An empty operand
// is ignored, so unioning with the zero range returns the other unchanged; the
// union of two empty ranges is empty.
func (r TextRange) Union(other TextRange) TextRange {
	if r.IsEmpty() {
		return other
	}
	if other.IsEmpty() {
		return r
	}
	sr, sc := r.StartRow, r.StartCol
	if posLess(other.StartRow, other.StartCol, sr, sc) {
		sr, sc = other.StartRow, other.StartCol
	}
	er, ec := r.EndRow, r.EndCol
	if posLess(er, ec, other.EndRow, other.EndCol) {
		er, ec = other.EndRow, other.EndCol
	}
	return TextRange{StartRow: sr, StartCol: sc, EndRow: er, EndCol: ec}
}

// posLess reports whether (r1, c1) precedes (r2, c2) in document order.
func posLess(r1, c1, r2, c2 int) bool {
	return r1 < r2 || (r1 == r2 && c1 < c2)
}

// largeDocRowThreshold is the logical-line count above which the gutter and
// decorator overlay switch from exact [widget.Entry.RowPositions] (an O(document)
// call — ~14ms on 50k lines) to the analytic O(viewport) [Editor.rowWindow]. Below
// it, the exact path is cheap and preserves every existing behavior (including the
// phantom line numbers drawn below short content).
const largeDocRowThreshold = 4000

// lineCount returns the document's logical-line count via the fork's O(1)
// [widget.Entry.LineCount] (pre-rebrand tracker #149, pre-rebrand tracker #459) — read from the buffer's line
// index, so it is live even between [Editor.DeferTextSync] sync points and
// never scans the document.
func (e *Editor) lineCount() int {
	return e.entry.LineCount()
}

// analyticGeometryOK reports whether the editor's content permits analytic,
// uniform-stride row geometry: single-segment wrap-off text, where a logical line
// is exactly one visual row. The editor's entry is always single-segment and
// defaults to wrap-off, so this holds unless a wrap mode has been forced on.
func (e *Editor) analyticGeometryOK() bool {
	return e.entry.Wrapping != bellui.TextWrapWord && e.entry.Wrapping != bellui.TextWrapBreak
}

// rowWindow returns the logical rows visible in the viewport band
// [scrollY, scrollY+viewportH], each with its content-space top Y and height —
// the same coordinate space [widget.Entry.RowPositions] uses (callers subtract
// their own scrollY). It is O(window): geometry is derived from the uniform
// visible-row stride ([widget.Entry.RowMetrics]) and the active folds, so it never
// walks the whole document. Exact for the editor's single-segment wrap-off content.
//
// It returns nil to signal "use the exact RowPositions path instead" — for small
// documents (below [largeDocRowThreshold], where the exact path is cheap and
// preserves existing behavior) or when analytic geometry does not apply (wrapping
// on) or metrics are not ready.
func (e *Editor) rowWindow(scrollY, viewportH float32) (rows []int, tops []float32, heights []float32) {
	total := e.lineCount()
	if total <= largeDocRowThreshold || !e.analyticGeometryOK() {
		return nil, nil, nil
	}
	topOffset, stride := e.entry.RowMetrics()
	if stride <= 0 {
		return nil, nil, nil
	}
	hidden := e.fold.mergedHidden()
	visRows := visibleRowCount(total, hidden)

	firstV := 0
	if scrollY > topOffset {
		firstV = int((scrollY - topOffset) / stride)
	}
	if firstV < 0 {
		firstV = 0
	}
	for v := firstV; v < visRows; v++ {
		top := topOffset + float32(v)*stride
		if top-scrollY > viewportH {
			break
		}
		rows = append(rows, visibleIndexToRow(v, total, hidden))
		tops = append(tops, top)
		heights = append(heights, stride)
	}
	// A non-nil, possibly-empty slice signals "large-doc window computed"; the
	// caller must not fall back to the exact path. Guarantee non-nil.
	if rows == nil {
		rows = []int{}
	}
	return rows, tops, heights
}

// rowAtContentY maps a content-space Y (a viewport click plus scrollY) to its
// logical row, using the same analytic geometry as [Editor.rowWindow]. ok is
// false when analytic geometry does not apply (the caller should use the exact
// RowPositions scan) or the point is past the last visible row.
func (e *Editor) rowAtContentY(contentY float32) (row int, ok bool) {
	total := e.lineCount()
	if total <= largeDocRowThreshold || !e.analyticGeometryOK() {
		return 0, false
	}
	topOffset, stride := e.entry.RowMetrics()
	if stride <= 0 {
		return 0, false
	}
	if contentY < topOffset {
		return 0, false
	}
	v := int((contentY - topOffset) / stride)
	hidden := e.fold.mergedHidden()
	if v < 0 || v >= visibleRowCount(total, hidden) {
		return 0, false
	}
	return visibleIndexToRow(v, total, hidden), true
}

// PositionAt returns the on-screen rectangle of the character cell at
// (row, col), in the editor's viewport coordinate space (the Entry's scroll
// offset is already applied). The rectangle's width is one character cell and
// its height is the row height.
//
// The zero [geo.Rectangle] is returned when (row, col) is out of range or is
// scrolled out of the vertical viewport — callers anchoring popovers should
// treat a zero rectangle as "not visible".
func (e *Editor) PositionAt(row, col int) geo.Rectangle {
	rect := e.entry.CaretRect(row, col)
	if rect.IsZero() {
		return geo.Rectangle{}
	}
	rect.Pos.X -= e.scrollX
	rect.Pos.Y -= e.scrollY

	// Cull positions scrolled out of the vertical viewport.
	if h := e.entry.Size().Height; h > 0 {
		if rect.Pos.Y+rect.Size.Height < 0 || rect.Pos.Y > h {
			return geo.Rectangle{}
		}
	}
	return rect
}

// positionAtPoint maps a viewport-space point (e.g. a mouse position relative to
// the editor's text area) back to a document (row, col). It is the inverse of
// [Editor.PositionAt], derived from the same fork geometry (`RowPositions` for
// the row, `CaretRect` scanning for the column), so hover and other pointer
// features need no extra fork support.
//
// ok is false when the point falls outside any laid-out row (e.g. below the last
// line or before the first layout pass).
func (e *Editor) positionAtPoint(pos geo.Position) (row, col int, ok bool) {
	yPos, heights := e.entry.RowPositions()
	if len(yPos) == 0 {
		return 0, 0, false
	}
	contentY := pos.Y + e.scrollY
	row = -1
	for i := range yPos {
		if i < len(heights) && contentY >= yPos[i] && contentY < yPos[i]+heights[i] {
			row = i
			break
		}
	}
	if row < 0 {
		return 0, 0, false
	}

	// Raw read (pre-rebrand tracker #129): this is the hover/pointer render path, so it tolerates
	// the deferred mode's bounded staleness (same-row, newline-free edits) —
	// only the actively edited row's column mapping can be momentarily off.
	lines := strings.Split(e.entry.Text, "\n")
	if row >= len(lines) {
		return 0, 0, false
	}
	n := len([]rune(lines[row]))

	contentX := pos.X + e.scrollX
	for c := 0; c <= n; c++ {
		r := e.entry.CaretRect(row, c)
		if r.IsZero() || r.Pos.X > contentX {
			break
		}
		col = c
	}
	return row, col, true
}

// RangeRects returns one rectangle per visual row spanned by r, in the
// editor's viewport coordinate space. A multi-row range yields a rect from the
// start column to end-of-line on the first row, full-line rects for interior
// rows, and a rect from column 0 to the end column on the last row.
//
// Rows that are out of range are skipped; an empty range returns nil. Unlike
// [Editor.PositionAt], rows scrolled out of view are still returned (with
// off-screen coordinates) so the decorator overlay can clip them uniformly.
func (e *Editor) RangeRects(r TextRange) []geo.Rectangle {
	if r.IsEmpty() {
		return nil
	}
	// Raw read (pre-rebrand tracker #129): decorator render path — see positionAtPoint.
	lines := strings.Split(e.entry.Text, "\n")

	var rects []geo.Rectangle
	for row := r.StartRow; row <= r.EndRow; row++ {
		if row < 0 || row >= len(lines) {
			continue
		}
		startCol := 0
		if row == r.StartRow {
			startCol = r.StartCol
		}
		endCol := len([]rune(lines[row]))
		if row == r.EndRow {
			endCol = r.EndCol
		}
		if endCol < startCol {
			continue
		}

		left := e.entry.CaretRect(row, startCol)
		if left.IsZero() {
			continue
		}
		x1 := left.Pos.X + left.Size.Width
		if right := e.entry.CaretRect(row, endCol); !right.IsZero() {
			x1 = right.Pos.X
		}
		w := x1 - left.Pos.X
		if w <= 0 {
			// Empty line within a multi-row range: show a single cell so the
			// span remains visible across the blank row.
			w = left.Size.Width
		}
		rects = append(rects, geo.NewRectangle(
			left.Pos.X-e.scrollX,
			left.Pos.Y-e.scrollY,
			w,
			left.Size.Height,
		))
	}
	return rects
}
