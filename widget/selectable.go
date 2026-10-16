package widget

import (
	"math"
	"strings"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	paint "github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

type selectable struct {
	BaseWidget
	cursorRow, cursorColumn int

	// selectRow and selectColumn represent the selection start location
	// The selection will span from selectRow/Column to CursorRow/Column -- note that the cursor
	// position may occur before or after the select start position in the text.
	selectRow, selectColumn int

	focussed, selecting, selectEnded, password, keepSelection bool
	sizeName                                                  bellui.ThemeSizeName
	style                                                     bellui.TextStyle
	alignment                                                 bellui.TextAlign

	provider *RichText
	theme    bellui.Theme
	focus    event.Focusable

	// host, when set, is an editor (RichEntry) that embeds this selectable purely
	// as a selection-highlight/geometry layer and owns its own caret/selection
	// model. Because a selectable is a child object that structurally implements
	// the pointer interfaces, it would otherwise shadow the host's own
	// Tapped/Dragged handlers (the host is the parent, so the driver routes
	// pointer events to the deeper child) — the caret would never move on click.
	// When host is set the pointer handlers forward to it instead of driving the
	// selectable's own cursor model. Read-only selectable Label/RichText leave it
	// nil and handle pointer input directly.
	host selectableHost

	// Mobile touch-selection state (pre-rebrand tracker #944, the pre-rebrand tracker #758 follow-up): selHandles is
	// the draggable-handle overlay shown while a standalone selectable has a
	// live touch-initiated selection; touchPointer notes whether the most
	// recent pointer-down was a touch; touchSelection marks the selection as
	// touch-initiated (keyboard/programmatic selections summon no handles).
	// See selectable_handles.go.
	selHandles                   *entrySelectionHandles
	touchPointer, touchSelection bool

	onSelectChanged func(string) // called when selected text changes

	// doubleTappedAtUnixMillis stores the time the entry was last DoubleTapped
	// used for deciding whether the next MouseDown/TouchDown is a triple-tap or not
	doubleTappedAtUnixMillis int64
}

// selectableHost is the editor (RichEntry) that owns pointer input when a
// selectable is embedded only for highlight rendering. Its methods are the
// pointer gestures the selectable forwards instead of handling itself.
type selectableHost interface {
	Tapped(*event.PointEvent)
	DoubleTapped(*event.PointEvent)
	Dragged(*event.DragEvent)
	DragEnd()
}

func (s *selectable) CreateRenderer() core.WidgetRenderer {
	return &selectableRenderer{sel: s}
}

func (s *selectable) Cursor() desktop.Cursor {
	return desktop.TextCursor
}

func (s *selectable) DoubleTapped(p *event.PointEvent) {
	if s.host != nil {
		s.host.DoubleTapped(p)
		return
	}
	s.doubleTappedAtUnixMillis = time.Now().UnixMilli()
	s.selectAll()
	if s.touchPointer {
		s.markTouchSelection()
	}
}

func (s *selectable) DragEnd() {
	if s.host != nil {
		s.host.DragEnd()
		return
	}
	if s.cursorColumn == s.selectColumn && s.cursorRow == s.selectRow {
		s.selecting = false
	}

	shouldRefresh := !s.selecting
	if shouldRefresh {
		s.Refresh()
	}
	s.selectEnded = true
	s.notifySelectChanged()

	// A finished touch drag-select offers the Copy menu for the new
	// selection, like a handle adjustment (native toolbar parity, pre-rebrand tracker #945).
	// Touch-gated: desktop mouse selection is unchanged.
	if s.touchPointer && s.touchSelection && s.selecting && bellui.CurrentDevice().IsMobile() {
		if a, _ := s.selection(); a >= 0 {
			rect := s.selectionHandleCaretRect(s.cursorRow, s.cursorColumn)
			pos := rect.Pos.AddXY(0, rect.Size.Height)
			abs := bellui.CurrentApp().Driver().AbsolutePositionForObject(s.super()).Add(pos)
			s.TappedSecondary(&event.PointEvent{AbsolutePosition: abs, Position: pos})
		}
	}
}

func (s *selectable) Dragged(d *event.DragEvent) {
	if s.host != nil {
		s.host.Dragged(d)
		return
	}
	s.dragged(d)
}

func (s *selectable) dragged(d *event.DragEvent) {
	// Ignore drags that originate inside the gutter (line numbers / breakpoints).
	if gw := s.provider.GutterWidth; gw > 0 {
		startX := d.Position.X - d.Dragged.DX
		if startX < gw {
			return
		}
	}

	if !s.selecting || s.selectEnded {
		s.selectEnded = false
		s.updateMousePointer(d.Position)

		startPos := d.Position.Subtract(d.Dragged)
		s.selectRow, s.selectColumn, _ = s.getRowCol(startPos)
		s.selecting = true

		s.grabFocus()
	}

	s.updateMousePointer(d.Position)
	// A drag confined to a standalone block row gives the same zero-column position
	// for start and end (the row has one caret column), so it would select nothing.
	// Treat any drag within a block row as selecting the whole atomic block, with
	// the synthetic trailing column selectAll uses (pre-rebrand issue #56).
	if s.selectRow == s.cursorRow && s.selectColumn == s.cursorColumn && s.provider.isBlockRow(s.cursorRow) {
		s.cursorColumn = 1
	}
	if s.touchPointer && s.selecting {
		s.markTouchSelection() // a touch drag-select gets handles (pre-rebrand tracker #944)
	}
	s.Refresh()
}

func (s *selectable) MouseDown(m *desktop.MouseEvent) {
	s.touchPointer = false
	if s.host != nil {
		// The host owns caret/selection and handles focus in its Tapped/Dragged.
		// Grab focus on press for responsiveness; do nothing else so the host's
		// own model — not the selectable's — drives the click. It does see the
		// press, so it can track the pointer type (pre-rebrand tracker #888).
		if h, ok := s.host.(desktop.Mouseable); ok {
			h.MouseDown(m)
		}
		s.grabFocus()
		return
	}
	// Ignore clicks inside the gutter (line numbers / breakpoints).
	if gw := s.provider.GutterWidth; gw > 0 && m.Position.X < gw {
		return
	}
	if isTripleTap(s.doubleTappedAtUnixMillis, time.Now().UnixMilli()) {
		s.selectCurrentRow(false)
		return
	}
	s.grabFocus()
	if s.selecting && m.Button == desktop.MouseButtonPrimary {
		s.selecting = false
	}
}

func (s *selectable) MouseUp(ev *desktop.MouseEvent) {
	if s.host != nil {
		return // the host's Tapped (fired on release) moves the caret
	}
	if ev.Button == desktop.MouseButtonSecondary {
		return
	}

	start, _ := s.selection()
	if (start == -1 || (s.selectRow == s.cursorRow && s.selectColumn == s.cursorColumn)) && s.selecting {
		s.selecting = false
	}
	s.Refresh()
}

// SelectedText returns the text currently selected in this Entry.
// If there is no selection it will return the empty string.
func (s *selectable) SelectedText() string {
	if s == nil || !s.selecting {
		return ""
	}

	noSelection := s.cursorRow == s.selectRow && s.cursorColumn == s.selectColumn
	if noSelection {
		return ""
	}

	// Determine selection range in (row, col) coordinates.
	rowA, colA := s.cursorRow, s.cursorColumn
	rowB, colB := s.selectRow, s.selectColumn
	if rowA > rowB || (rowA == rowB && colA > colB) {
		rowA, colA = s.selectRow, s.selectColumn
		rowB, colB = s.cursorRow, s.cursorColumn
	}

	// Fast path: a single text segment (a plain Label, or a single-segment RichText —
	// the common selectable case). Slice the source text by offset so characters that
	// word-wrap consumes at a wrap point (a space dropped from both adjacent visual
	// rows) are preserved, along with hard newlines (pre-rebrand tracker #161). This also matches
	// Entry.SelectedText and is exact under the lazy row model (pre-rebrand tracker #157).
	if _, ok := s.provider.singleSegment(); ok {
		a := textPosFromRowCol(rowA, colA, s.provider)
		b := textPosFromRowCol(rowB, colB, s.provider)
		// textRange reads from the lazy buffer when live (no full-document
		// decode, pre-rebrand tracker #401) and decodes the segment text otherwise.
		return s.provider.textRange(a, b)
	}

	// Build selected text directly from row content. rowSelectedText spans whole
	// rows by default; constrain only the first and last rows to the selected
	// columns. It also splices in any inline non-text segment (link, image) whose
	// opaque box is covered by the selection, so the copied text matches the
	// highlighted rectangles (pre-rebrand issue #46).
	var buf strings.Builder
	for r := rowA; r <= rowB && r < s.provider.rows(); r++ {
		rowLen := s.provider.rowLength(r)
		if rowLen == 0 {
			// Zero-column row: the text-based selection model has no column here. A
			// standalone block non-text segment (image/table/separator) is
			// all-or-nothing — since we only get here for a non-degenerate selection,
			// any block row the selection reaches (including the last/only row) is
			// covered, so include its Textual() (pre-rebrand issues #50, pre-rebrand tracker #53). An ordinary blank
			// text row yields "" here, so its newline behaviour is unchanged.
			buf.WriteString(s.provider.rowBlockText(r))
			s.appendRowJoiner(&buf, r, rowB)
			continue
		}
		startCol := 0
		endCol := rowLen
		if r == rowA {
			startCol = colA
		}
		if r == rowB {
			endCol = colB
		}
		if startCol > rowLen {
			startCol = rowLen
		}
		if endCol > rowLen {
			endCol = rowLen
		}
		buf.WriteString(s.provider.rowSelectedText(r, startCol, endCol))
		s.appendRowJoiner(&buf, r, rowB)
	}
	return buf.String()
}

// appendRowJoiner writes what goes between visual row r and the next selected row:
// a newline at a hard logical-line end, or — when r and r+1 are a soft wrap of the
// same logical line — the run word-wrap consumed at the break (so the copied text
// keeps its spaces, pre-rebrand tracker #161). Nothing is written after the last selected row.
func (s *selectable) appendRowJoiner(buf *strings.Builder, r, rowB int) {
	if r >= rowB {
		return
	}
	if s.provider.rowIsLogicalLineEnd(r) {
		buf.WriteByte('\n')
		return
	}
	if sep, ok := s.provider.softWrapGap(r); ok {
		buf.WriteString(sep)
	}
}

func (s *selectable) notifySelectChanged() {
	if s.onSelectChanged != nil {
		s.onSelectChanged(s.SelectedText())
	}
}

func (s *selectable) Tapped(p *event.PointEvent) {
	if s.host != nil {
		s.host.Tapped(p) // moves the host's caret to the tapped position
		return
	}
	if !bellui.CurrentDevice().IsMobile() {
		return
	}

	if s.doubleTappedAtUnixMillis != 0 {
		s.doubleTappedAtUnixMillis = 0
		return // was a triple (TappedDouble plus Tapped)
	}
	s.selecting = false
	s.Refresh()
}

func (s *selectable) TappedSecondary(ev *event.PointEvent) {
	if s.host != nil {
		// The embedding editor offers its own, richer context menu (Cut/Paste/
		// Undo need the editor's document model); the selectable's read-only
		// Copy menu would also desync the host's selection state (pre-rebrand tracker #888).
		if st, ok := s.host.(event.SecondaryTappable); ok {
			st.TappedSecondary(ev)
		}
		return
	}
	app := bellui.CurrentApp()
	c := app.Driver().CanvasForObject(s.focus.(core.CanvasObject))
	if c == nil {
		return
	}

	m := bellui.NewMenu("",
		bellui.NewMenuItem(lang.L("Copy"), func() {
			app.Clipboard().SetContent(s.SelectedText())
		}),
		bellui.NewMenuItem(lang.L("Select all"), func() {
			s.selectAll()
			if s.touchPointer {
				s.markTouchSelection() // Select all from the touch menu gets handles
			}
		}))
	ShowPopUpMenuAtPosition(m, c, ev.AbsolutePosition)
}

func (s *selectable) TouchCancel(m *mobile.TouchEvent) {
	s.TouchUp(m)
}

func (s *selectable) TouchDown(m *mobile.TouchEvent) {
	s.touchPointer = true
	if s.host != nil {
		// The embedding editor gates its own selection handles on the pointer
		// type, so it must see the touch too (mirrors Tapped/Dragged, pre-rebrand tracker #888).
		if h, ok := s.host.(mobile.Touchable); ok {
			h.TouchDown(m)
		}
		return
	}
	if isTripleTap(s.doubleTappedAtUnixMillis, time.Now().UnixMilli()) {
		s.selectCurrentRow(true)
		return
	}
}

func (s *selectable) TouchUp(*mobile.TouchEvent) {
}

func (s *selectable) TypedShortcut(sh core.Shortcut) {
	switch sh.(type) {
	case *bellui.ShortcutCopy:
		bellui.CurrentApp().Clipboard().SetContent(s.SelectedText())
	case *bellui.ShortcutSelectAll:
		s.selectAll()
	}
}

// selectAll selects all text in the provider.
func (s *selectable) selectAll() {
	rows := s.provider.rows()
	if rows == 0 {
		return
	}
	s.selectRow = 0
	s.selectColumn = 0
	last := rows - 1
	s.cursorRow = last
	s.cursorColumn = s.provider.rowLength(last)
	// A standalone block row (image/table/separator) has no caret column, so when
	// it is the only content the selection above would be degenerate (0,0)==(0,0)
	// and select nothing. Give it a synthetic trailing column so the block is
	// enclosed and thus copied/highlighted (pre-rebrand issue #53). Multi-row select-all is
	// already non-degenerate, so a trailing block on row `last` is covered as-is.
	if s.cursorRow == s.selectRow && s.cursorColumn == s.selectColumn && s.provider.isBlockRow(last) {
		s.cursorColumn = 1
	}
	s.selecting = true
	s.grabFocus()
	s.Refresh()
}

// alignOffsetX returns the horizontal offset for the given row to account
// for center or trailing text alignment. For leading alignment it returns 0.
func (s *selectable) alignOffsetX(row int) float32 {
	// Resolve against the active layout direction so the selection highlight
	// tracks the text edge under RTL (no-op for the default LTR layout).
	align := lang.ResolveAlign(s.alignment)
	if align == bellui.TextAlignLeading || s.provider == nil {
		return 0
	}
	th := s.theme
	textSize := th.Size(s.getSizeName())
	innerPad := th.Size(theme.SizeNameInnerPadding)
	// Measure each segment in its own style/size so multi-segment rows centre or
	// trail exactly; a single-style measurement drifts when segments differ in
	// size or weight (pre-rebrand issue #42).
	textWidth := s.provider.rowWidth(row, textSize) + 2*innerPad
	containerWidth := s.Size().Width
	if containerWidth <= textWidth {
		return 0
	}
	switch align {
	case bellui.TextAlignCenter:
		return (containerWidth - textWidth) / 2
	case bellui.TextAlignTrailing:
		return containerWidth - textWidth
	}
	return 0
}

// cursorColAt returns the logical column a click at pos selects, plus the caret
// affinity of the clicked side (downstream true = forward edge). Affinity is only
// meaningful on a bidi row at a direction boundary; the pure-LTR path always
// reports downstream.
func (s *selectable) cursorColAt(row int, text []rune, pos geo.Position) (col int, downstream bool) {
	th := s.theme
	textSize := th.Size(s.getSizeName())
	innerPad := th.Size(theme.SizeNameInnerPadding)

	// Rows mixing text with inline non-text atoms (inline images, pre-rebrand tracker #665) need
	// atom-aware hit-testing: the plain LTR scan below measures *through* the
	// image (landing every hit after it one image-width off) and the bidi
	// HitTestAffinity returns box-blind text-only columns, so a click on or past
	// an image resolves wrong (pre-rebrand tracker #983). Resolve both column spaces from the row's
	// layout — bidi first (RTL/mixed), else the LTR atom-aware walk — and return
	// the one the host addresses content in: the document space (each atom one
	// position) for an editor host like RichEntry, the text-only space otherwise.
	if s.provider.rowHasInlineBox(row) {
		if textCol, docCol, down, ok := s.provider.bidiInlineHitTest(row, pos.X, innerPad); ok {
			if s.host != nil {
				return docCol, down
			}
			return textCol, down
		}
		if textCol, docCol, down, ok := s.provider.inlineColumnAt(row, pos.X, innerPad); ok {
			if s.host != nil {
				return docCol, down
			}
			return textCol, down
		}
	}

	// In bidi text the visual x of a click maps non-monotonically to logical
	// offsets, so hit-test against the row's visual layout (which spans segment
	// boundaries for multi-segment RichText rows). Pure-LTR text keeps the
	// prefix-width scan below.
	if layout, ok := s.provider.rowVisualLayout(row); ok {
		off, aff := layout.HitTestAffinity(pos.X - innerPad)
		return off, aff == paint.AffinityDownstream
	}

	for i := 0; i < len(text); i++ {
		str := string(text[0:i])
		wid := bellui.MeasureText(str, textSize, s.style).Width
		charWid := bellui.MeasureText(string(text[i]), textSize, s.style).Width
		if pos.X < innerPad+wid+(charWid/2) {
			return i, true
		}
	}
	return len(text), true
}

// getRowCol maps a click position to a (row, column) and the caret affinity of
// the clicked side (downstream true = forward edge; always true off a bidi
// direction boundary). Callers that do not track affinity can ignore it.
func (s *selectable) getRowCol(p geo.Position) (row, col int, downstream bool) {
	th := s.theme
	textSize := th.Size(s.getSizeName())
	innerPad := th.Size(theme.SizeNameInnerPadding)

	// Map the click Y to a row: O(1) analytic inverse for the lazy/uniform-stride
	// model (pre-rebrand tracker #160), cached-geometry scan otherwise. -1 ⇒ fall back to uniform below.
	numRows := s.provider.rows()
	downstream = true
	row = s.provider.rowAtY(p.Y)
	if row == -1 {
		// Fallback: uniform row height (Label path, or before first Layout).
		rowHeight := s.provider.charMinSize(false, s.style, textSize).Height
		row = int(math.Floor(float64(p.Y-innerPad+th.Size(theme.SizeNameLineSpacing)) / float64(rowHeight)))
	}
	_ = textSize

	if row < 0 {
		row = 0
	} else if row >= numRows {
		row = numRows - 1
		col = s.provider.rowLength(row)
	} else {
		// Adjust X for text alignment and gutter offset before column hit-testing.
		gutterOff := s.provider.GutterWidth
		adjusted := geo.NewPos(p.X-s.alignOffsetX(row)-gutterOff, p.Y)
		col, downstream = s.cursorColAt(row, s.provider.row(row), adjusted)
	}

	return row, col, downstream
}

// Selects the row where the cursorColumn is currently positioned
func (s *selectable) selectCurrentRow(focus bool) {
	s.grabFocus()
	provider := s.provider
	s.selectRow = s.cursorRow
	s.selectColumn = 0
	s.cursorColumn = provider.rowLength(s.cursorRow)
	// A standalone block row (image/table/separator) has no caret column, so the
	// range above would be degenerate (0 == 0) and select nothing. Give it a
	// synthetic trailing column so the atomic block is enclosed, mirroring
	// selectAll (pre-rebrand issue #56).
	if s.cursorColumn == s.selectColumn && provider.isBlockRow(s.cursorRow) {
		s.cursorColumn = 1
	}
	s.Refresh()
}

// selection returns the start and end text positions for the selected span of text
// Note: this functionality depends on the relationship between the selection start row/col and
// the current cursor row/column.
// eg: (whitespace for clarity, '_' denotes cursor)
//
//	"T  e  s [t  i]_n  g" == 3, 5
//	"T  e  s_[t  i] n  g" == 3, 5
//	"T  e_[s  t  i] n  g" == 2, 5
func (s *selectable) selection() (int, int) {
	noSelection := !s.selecting || (s.cursorRow == s.selectRow && s.cursorColumn == s.selectColumn)

	if noSelection {
		return -1, -1
	}

	// Find the selection start
	rowA, colA := s.cursorRow, s.cursorColumn
	rowB, colB := s.selectRow, s.selectColumn
	// Reposition if the cursors row is more than select start row, or if the row is the same and
	// the cursors col is more that the select start column
	if rowA > s.selectRow || (rowA == s.selectRow && colA > s.selectColumn) {
		rowA, colA = s.selectRow, s.selectColumn
		rowB, colB = s.cursorRow, s.cursorColumn
	}

	return textPosFromRowCol(rowA, colA, s.provider), textPosFromRowCol(rowB, colB, s.provider)
}

// textPosFromRowCol obtains a textual position from a given row and col.
// This uses the rowBoundary begin offset which works correctly for Entry
// (single segment per row). For RichText selection, SelectedText() uses
// row-based extraction instead of String() indexing.
func textPosFromRowCol(row, col int, prov *RichText) int {
	b := prov.rowBoundary(row)
	if b == nil {
		return col
	}
	return b.begin + col
}

// rowColFromTextPos obtains the row,col for a rune offset into the provider —
// the inverse of textPosFromRowCol. Shared by Entry.rowColFromTextPos and the
// selectable accessibility seam (pre-rebrand tracker #556). wrapping is the owning widget's text
// wrap mode, needed to resolve positions that sit exactly on a soft wrap.
func rowColFromTextPos(pos int, provider *RichText, wrapping bellui.TextWrap) (row, col int) {
	if provider.lazy {
		// Lazy model (pre-rebrand tracker #157): O(log lines + one line measure) instead of scanning
		// (and thereby measuring) every row.
		return provider.lazyRowColFromTextPos(pos)
	}
	canWrap := wrapping == bellui.TextWrapBreak || wrapping == bellui.TextWrapWord
	totalRows := provider.rows()
	for i := 0; i < totalRows; i++ {
		b := provider.rowBoundary(i)
		if b == nil {
			continue
		}
		if b.begin <= pos {
			if b.end < pos {
				row++
			}
			col = pos - b.begin
			// if this gap is at `pos` and is a line wrap, increment (safe to access boundary i-1)
			if canWrap && b.begin == pos && pos != 0 && provider.rowBoundary(i-1).end == b.begin && row < (totalRows-1) {
				row++
			}
		} else {
			break
		}
	}
	return row, col
}

// accessibleSelection returns the selection as a normalized half-open rune
// range into the provider text; when nothing is selected both offsets are
// the cursor position (the [core.AccessibleTexter] contract, pre-rebrand tracker #556).
func (s *selectable) accessibleSelection() (start, end int) {
	if s == nil {
		return 0, 0
	}
	if a, b := s.selection(); a >= 0 {
		return a, b
	}
	return s.accessibleCaret(), s.accessibleCaret()
}

// accessibleCaret returns the rune offset of the cursor (the moving end of a
// selection drag, or where the user last interacted).
func (s *selectable) accessibleCaret() int {
	if s == nil {
		return 0
	}
	return textPosFromRowCol(s.cursorRow, s.cursorColumn, s.provider)
}

// setAccessibleSelection selects the clamped rune range [start, end); equal
// offsets clear the selection and park the cursor there.
func (s *selectable) setAccessibleSelection(start, end int, wrapping bellui.TextWrap) {
	if s == nil || s.provider == nil {
		return
	}
	if start > end {
		start, end = end, start
	}
	n := s.provider.len()
	start = clampInt(start, 0, n)
	end = clampInt(end, 0, n)

	s.selectRow, s.selectColumn = rowColFromTextPos(start, s.provider, wrapping)
	s.cursorRow, s.cursorColumn = rowColFromTextPos(end, s.provider, wrapping)
	s.selecting = start != end
	s.Refresh()
	s.notifySelectChanged()
}

func (s *selectable) updateMousePointer(p geo.Position) {
	row, col, _ := s.getRowCol(p)
	s.cursorRow, s.cursorColumn = row, col

	if !s.selecting {
		s.selectRow = row
		s.selectColumn = col
	}
}

func (s *selectable) getSizeName() bellui.ThemeSizeName {
	if s.sizeName != "" {
		return s.sizeName
	}
	return theme.SizeNameText
}

type selectableRenderer struct {
	sel *selectable

	selections []core.CanvasObject
}

func (r *selectableRenderer) Destroy() {
}

func (r *selectableRenderer) Layout(geo.Size) {
}

func (r *selectableRenderer) MinSize() geo.Size {
	return geo.Size{}
}

func (r *selectableRenderer) Objects() []core.CanvasObject {
	return r.selections
}

func (r *selectableRenderer) Refresh() {
	r.buildSelection()
	selections := r.selections
	v := bellui.CurrentApp().Settings().ThemeVariant()

	selectionColor := r.sel.theme.Color(theme.ColorNameSelection, v)
	for _, selection := range selections {
		rect := selection.(*canvas.Rectangle)
		rect.FillColor = selectionColor

		if r.sel.focussed || r.sel.keepSelection {
			rect.Show()
		} else {
			rect.Hide()
		}
	}

	// The central mobile selection-handle seam (pre-rebrand tracker #944): every selection or
	// focus mutation funnels through this Refresh (a no-op off mobile and
	// for editor-embedded selectables).
	r.sel.updateSelectionHandles()

	canvas.Refresh(r.sel.impl)
}

// This process builds a slice of rectangles:
// - one entry per row of text
// - ordered by row order as they occur in multiline text
// This process could be optimized in the scenario where the user is selecting upwards:
// If the upwards case instead produces an order-reversed slice then only the newest rectangle would
// require movement and resizing. The existing solution creates a new rectangle and then moves/resizes
// all rectangles to comply with the occurrence order as stated above.
func (r *selectableRenderer) buildSelection() {
	th := r.sel.theme
	v := bellui.CurrentApp().Settings().ThemeVariant()
	textSize := th.Size(r.sel.getSizeName())

	cursorRow, cursorCol := r.sel.cursorRow, r.sel.cursorColumn
	selectRow, selectCol := -1, -1
	if r.sel.selecting {
		selectRow = r.sel.selectRow
		selectCol = r.sel.selectColumn
	}

	if selectRow == -1 || (cursorRow == selectRow && cursorCol == selectCol) {
		r.selections = r.selections[:0]
		return
	}

	provider := r.sel.provider
	innerPad := th.Size(theme.SizeNameInnerPadding)
	defaultLineHeight := r.sel.provider.charMinSize(r.sel.password, r.sel.style, textSize).Height

	// Convert column, row into x,y (accounting for text alignment offset).
	// Uses cached RowGeometry for accurate Y positions in variable-height layouts.
	getCoordinates := func(column int, row int) (float32, float32) {
		// Selection endpoints are not affinity-tracked; use the forward edge.
		sz := provider.lineSizeToColumn(column, row, textSize, innerPad, true)
		xw := sz.Width
		// A row with an inline image atom counts columns in document-position space,
		// which lineSizeToColumn (text-rune columns) mis-measures — it steps through
		// the image. Use the atom-aware walk for the x so the highlight edge lands
		// correctly across an inline image (pre-rebrand tracker #893); the height still comes from sz.
		if ix, ok := provider.inlineColumnX(row, column, textSize, innerPad); ok {
			xw = ix
		}
		x := xw + r.sel.alignOffsetX(row) + provider.GutterWidth
		ry, _ := provider.RowGeometry(row)
		y := ry
		if y == 0 && row > 0 {
			// Fallback: uniform row height.
			y = sz.Height*float32(row) - th.Size(theme.SizeNameInputBorder) + innerPad
		}
		return x, y
	}

	minmax := func(a, b int) (int, int) {
		if a < b {
			return a, b
		}
		return b, a
	}

	// The remainder of the function calculates the set of boxes and add them to r.selection

	selectStartRow, selectEndRow := minmax(selectRow, cursorRow)
	selectStartCol, selectEndCol := minmax(selectCol, cursorCol)
	if selectRow < cursorRow {
		selectStartCol, selectEndCol = selectCol, cursorCol
	}
	if selectRow > cursorRow {
		selectStartCol, selectEndCol = cursorCol, selectCol
	}
	rowCount := selectEndRow - selectStartRow + 1

	// When the provider is virtualised (pre-rebrand tracker #119), only build rectangles for the
	// rows in the materialised window — building (and measuring) a rect for every
	// selected row would re-introduce the O(rows) cost on select-all. Rect Y/width
	// come from analytic RowGeometry, so off-window rows are correctly omitted and
	// rebuilt on scroll (Entry refreshes the selection from updateTextViewport).
	bandStart, bandEnd := selectStartRow, selectEndRow+1
	if provider.canVirtualize() {
		if ws, we := provider.visibleRowRange(); we > ws {
			if ws > bandStart {
				bandStart = ws
			}
			if we < bandEnd {
				bandEnd = we
			}
		}
	}

	// Build the rectangles for every selected row. A row of LTR text yields a
	// single rectangle; a row containing RTL/mixed text yields one rectangle per
	// disjoint visual span, since a logically-contiguous selection is not
	// visually contiguous there. Collect specs first, then reconcile the rect
	// pool, so the count may differ from rowCount.
	type selRect struct {
		x, y, w, h float32
	}
	var rects []selRect
	for i := 0; i < rowCount; i++ {
		// determine starting/ending columns for this row
		row := selectStartRow + i
		if row < bandStart || row >= bandEnd {
			continue // outside the virtualised window — rebuilt on scroll
		}
		if provider.rowHidden(row) {
			continue // folded row (pre-rebrand tracker #141): part of the model, but draws no rectangle
		}
		startCol, endCol := selectStartCol, selectEndCol
		if selectStartRow < row {
			startCol = 0
		}
		if selectEndRow > row {
			endCol = provider.rowLength(row)
		}

		_, rh := provider.RowGeometry(row)
		if rh <= 0 {
			rh = defaultLineHeight
		}

		// Standalone block non-text row (image/table/separator): it has no text
		// columns, so the column-based path would draw a zero-width sliver. Fill the
		// whole box width instead, all-or-nothing. The loop only runs for a real
		// selection, so any block row it reaches (including the last/only row) is
		// covered — matching SelectedText (pre-rebrand issues #50, pre-rebrand tracker #53). A blank text row is not
		// a block row, so it falls through to the legacy path.
		if provider.isBlockRow(row) {
			if w := provider.rowWidth(row, textSize); w > 0 {
				_, y := getCoordinates(0, row)
				leftX := r.sel.alignOffsetX(row) + provider.GutterWidth + innerPad - provider.inset.Width
				rects = append(rects, selRect{x: leftX - 1, y: y, w: w + 1, h: rh})
			}
			continue
		}

		// A bidi row holding inline atoms needs the document-column-aware span
		// path first: the plain RTL spans index text-only columns and would
		// both mis-map the endpoints (atoms count a position each) and decide
		// atom highlighting by the wrong rule (pre-rebrand tracker #913).
		spans, spansOK := provider.bidiInlineSelectionSpans(row, startCol, endCol, innerPad)
		if !spansOK {
			spans, spansOK = provider.rtlSelectionSpans(row, startCol, endCol, innerPad)
		}
		if spansOK {
			_, y := getCoordinates(startCol, row)
			base := r.sel.alignOffsetX(row) + provider.GutterWidth
			for _, sp := range spans {
				rects = append(rects, selRect{x: base + sp[0] - 1, y: y, w: sp[1] - sp[0] + 1, h: rh})
			}
			continue
		}

		// LTR fast path: a single rectangle from startCol to endCol.
		x1, y1 := getCoordinates(startCol, row)
		x2, _ := getCoordinates(endCol, row)
		rects = append(rects, selRect{x: x1 - 1, y: y1, w: x2 - x1 + 1, h: rh})
	}

	// reconcile the rectangle pool with the computed specs
	if len(r.selections) > len(rects) {
		r.selections = r.selections[:len(rects)]
	}
	for i, rc := range rects {
		if len(r.selections) <= i {
			r.selections = append(r.selections, canvas.NewRectangle(th.Color(theme.ColorNameSelection, v)))
		}
		r.selections[i].Resize(geo.NewSize(rc.w, rc.h))
		r.selections[i].Move(geo.NewPos(rc.x, rc.y))
	}
}

func (s *selectable) grabFocus() {
	if c := bellui.CurrentApp().Driver().CanvasForObject(s.focus.(core.CanvasObject)); c != nil {
		c.Focus(s.focus)
	}
}

func isTripleTap(double, nowMilli int64) bool {
	return nowMilli-double <= bellui.DoubleTapDelay().Milliseconds()
}
