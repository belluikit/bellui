package widget

import "github.com/belluikit/bellui/event"

// skipHiddenRowForKey biases the fold-skip to a navigation key's direction:
// backward keys (up/left/home/page-up) land on the visible row before a fold,
// forward keys (down/right/end/page-down) on the row after it. Other keys are
// ignored. It is extracted from TypedKey to keep that dispatcher's cyclomatic
// complexity within budget.
func (e *Entry) skipHiddenRowForKey(key event.KeyName) {
	switch key {
	case event.KeyUp, event.KeyLeft, event.KeyHome, event.KeyPageUp:
		e.skipHiddenRow(-1)
	case event.KeyDown, event.KeyRight, event.KeyEnd, event.KeyPageDown:
		e.skipHiddenRow(+1)
	}
}

// skipHiddenRow moves the caret off a folded row onto the nearest visible row in
// the direction of travel (#141): dir > 0 (down / forward) lands on the first
// visible row after the fold, dir <= 0 (up / backward) on the last visible row
// before it. The column is clamped to the destination row. It is a no-op when no
// rows are folded or the caret is already on a visible row.
func (e *Entry) skipHiddenRow(dir int) {
	provider := e.textProvider()
	if !provider.anyHidden() || !provider.rowHidden(e.CursorRow) {
		return
	}

	var row int
	if dir > 0 {
		row = provider.firstVisibleAtOrAfter(e.CursorRow)
		if row >= provider.rows() {
			row = provider.lastVisibleAtOrBefore(e.CursorRow)
		}
	} else {
		row = provider.lastVisibleAtOrBefore(e.CursorRow)
		if row < 0 {
			row = provider.firstVisibleAtOrAfter(e.CursorRow)
		}
	}
	if row < 0 || row >= provider.rows() {
		return
	}

	e.CursorRow = row
	if rl := provider.rowLength(row); e.CursorColumn > rl {
		e.CursorColumn = rl
	}
	e.syncSelectable()
}
