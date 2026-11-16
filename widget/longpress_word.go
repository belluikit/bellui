package widget

// pressedWord is the word a long-press selected. A drag in the same touch
// extends the selection from it a whole word at a time, as native text fields
// do, instead of moving the caret to the finger: a held finger creeps a few
// points once the long-press has fired, and moving the caret with it shrank
// the word to a fragment (#1267). Entry, the selectable behind Label and
// RichText, and RichEntry each keep one, set by LongPressed and cleared when
// the drag ends or the next press starts.
type pressedWord struct {
	active          bool
	row, start, end int
}

func (w *pressedWord) set(row, start, end int) {
	*w = pressedWord{active: true, row: row, start: start, end: end}
}

func (w *pressedWord) clear() { w.active = false }

// extend returns the selection for a drag at row, col: the word alone while
// the finger is inside it, otherwise the word through the whole word under the
// finger, anchored at the word's far edge. A finger on the separators between
// words adds none of the word beyond them. rowText reads a row's runes.
func (w *pressedWord) extend(row, col int, rowText func(int) []rune) (anchorRow, anchorCol, caretRow, caretCol int) {
	switch {
	case row < w.row || row == w.row && col < w.start:
		return w.row, w.end, row, wordEdge(rowText(row), col, false)
	case row > w.row || row == w.row && col > w.end:
		return w.row, w.start, row, wordEdge(rowText(row), col, true)
	}
	return w.row, w.start, w.row, w.end
}

// wordEdge returns the end (forward) or the start of the word at col. On a
// run of separators it returns the run's edge nearer the pressed word: its
// start going forward, its end going back.
func wordEdge(row []rune, col int, forward bool) int {
	start, end := getTextWhitespaceRegion(row, col, false)
	if start < 0 || end < 0 {
		return col
	}
	if probe := min(col, len(row)-1); probe >= 0 && isWordSeparator(row[probe]) {
		forward = !forward
	}
	if forward {
		return end
	}
	return start
}
