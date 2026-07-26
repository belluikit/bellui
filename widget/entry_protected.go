package widget

import "github.com/belluikit/bellui/event"

// clampCaretFromOpaqueForKey biases the caret-opaque clamp to a navigation key's
// direction: backward keys (left/up/home/page-up) push toward a span's Start,
// forward keys (right/down/end/page-down) toward its End. Other keys are
// ignored. It is extracted from TypedKey to keep that dispatcher's cyclomatic
// complexity within budget.
func (e *Entry) clampCaretFromOpaqueForKey(key event.KeyName) {
	switch key {
	case event.KeyLeft, event.KeyUp, event.KeyHome, event.KeyPageUp:
		e.clampCaretFromOpaque(-1)
	case event.KeyRight, event.KeyDown, event.KeyEnd, event.KeyPageDown:
		e.clampCaretFromOpaque(+1)
	}
}

// TextRange is a half-open span of an [Entry]'s text expressed as rune offsets
// into Text: Start is inclusive, End exclusive. Start == End is an empty range
// (a caret position). It is the unit passed to [Entry.BeforeChange].
//
// Since: 2.10
type TextRange struct {
	Start, End int
}

// ProtectedRange marks a span of an [Entry]'s text that the widget refuses to
// mutate. Offsets are rune indices into Text, half-open [Start, End). An edit
// overlapping the span is rejected across every edit path, and undo/redo cannot
// re-introduce it.
//
// When CaretOpaque is true the caret is additionally barred from entering the
// span: arrow navigation, clicks and mobile taps cannot land strictly inside it
// (used for fully-masked / redacted regions). The caret may still rest on
// either boundary. Touch placement is enforced through the same
// updateMousePointer path as desktop clicks.
//
// Offsets are rune indices into the current Text; the caller owns reconciling
// them when Text changes.
//
// Since: 2.10
type ProtectedRange struct {
	Start, End  int
	CaretOpaque bool
}

// bounds returns the protected span as an ordered [low, high) rune range.
func (p ProtectedRange) bounds() (int, int) {
	if p.Start <= p.End {
		return p.Start, p.End
	}
	return p.End, p.Start
}

// editAllowed reports whether replacing the text in the rune range [start, end)
// with next is permitted. It enforces [Entry.ProtectedRanges] first and then
// consults [Entry.BeforeChange]. It performs no mutation. The range is ordered
// internally, so callers may pass start/end in either order.
//
// It is the single gate every text-mutating path funnels through; when both the
// hook and the slice are unset it always returns true (today's behavior).
func (e *Entry) editAllowed(start, end int, next string) bool {
	if start > end {
		start, end = end, start
	}
	for i := range e.ProtectedRanges {
		ps, pe := e.ProtectedRanges[i].bounds()
		if editTouchesRange(start, end, ps, pe) {
			return false
		}
	}
	if e.BeforeChange != nil {
		if !e.BeforeChange(TextRange{Start: start, End: end}, next) {
			return false
		}
	}
	return true
}

// editTouchesRange reports whether an edit replacing [start, end) intrudes on
// the protected span [ps, pe). A deletion or replacement (start < end) is
// blocked when it overlaps the span at all. A pure insertion (start == end) is
// blocked only when it falls strictly inside the span; an insertion exactly at
// either boundary is allowed so text can grow up against a protected span.
func editTouchesRange(start, end, ps, pe int) bool {
	if ps >= pe {
		return false // an empty span protects nothing
	}
	if start == end {
		return start > ps && start < pe
	}
	return start < pe && end > ps
}

// editTargetForInsert returns the rune range a caret insertion would replace:
// the active selection when one exists, otherwise the empty range at the caret.
func (e *Entry) editTargetForInsert() (int, int) {
	if e.sel != nil && e.sel.selecting {
		return e.sel.selection()
	}
	p := e.CursorTextOffset()
	return p, p
}

// hasOpaqueRanges reports whether any protected range bars caret entry.
func (e *Entry) hasOpaqueRanges() bool {
	for i := range e.ProtectedRanges {
		if e.ProtectedRanges[i].CaretOpaque {
			return true
		}
	}
	return false
}

// clampCaretFromOpaque pushes the caret out of any CaretOpaque protected span it
// currently sits strictly inside, moving it to the span boundary in the
// direction of travel: dir < 0 toward Start, dir > 0 toward End, dir == 0 to the
// nearer boundary. It updates CursorRow/CursorColumn and the selection mirror.
// No-op when there are no opaque ranges or the caret is already clear.
func (e *Entry) clampCaretFromOpaque(dir int) {
	if !e.hasOpaqueRanges() {
		return
	}
	pos := e.CursorTextOffset()
	moved := false
	// Iterate to a fixed point so a caret pushed out of one span but into an
	// adjacent one keeps moving. Bounded by the range count to guarantee
	// termination even with pathological overlapping spans.
	for pass := 0; pass <= len(e.ProtectedRanges); pass++ {
		changed := false
		for i := range e.ProtectedRanges {
			pr := e.ProtectedRanges[i]
			if !pr.CaretOpaque {
				continue
			}
			ps, pe := pr.bounds()
			if pos <= ps || pos >= pe {
				continue
			}
			switch {
			case dir < 0:
				pos = ps
			case dir > 0:
				pos = pe
			default:
				if pos-ps <= pe-pos {
					pos = ps
				} else {
					pos = pe
				}
			}
			changed, moved = true, true
		}
		if !changed {
			break
		}
	}
	if moved {
		e.CursorRow, e.CursorColumn = e.rowColFromTextPos(pos)
		e.syncSelectable()
	}
}
