package widget

import (
	"strings"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// TestTextGrid_StyleAfterShrinkWhileScrolled is the regression for a
// real-app crash ("index out of range [96] with length 1"): a
// console-style buffer clear+repopulate within one UI callback, while
// the grid was scrolled, leaves pooled row widgets pointing at document
// rows that no longer exist. Styling the fresh first row then panicked
// twice over: textGridContent.refreshCell indexed the widget pool
// positionally (visible[0] can show ANY document row after scrolling —
// and the old guard also skipped the last visible row), and
// textGridRow.refreshCell indexed Rows[t.row] without the stale-row
// guard refreshCells already has.
func TestTextGrid_StyleAfterShrinkWhileScrolled(t *testing.T) {
	test.NewTempApp(t)
	g := NewTextGrid()
	g.Scroll = bellui.ScrollBoth
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(300, 120))

	g.Append(strings.Repeat("row of text\n", 199) + "last row")
	g.ScrollToBottom()
	g.Refresh()

	// Clear + repopulate in one callback (no relayout in between), with
	// a pool entry forced into the crash shape: showing a high document
	// row whose last laid-out content was short.
	g.Rows = nil
	g.Append("$ govm record …")
	if len(g.content.visible) > 0 {
		wid := g.content.visible[0].(*textGridRow)
		wid.row = 96
		wid.cols = 0
	}
	g.SetStyleRange(0, 0, 0, 5, TextGridStyleWhitespace) // panicked before the fix
}

// TestTextGrid_RefreshCellLastVisibleRow guards the off-by-one fixed in
// textGridContent.refreshCell: styling the last visible row must reach
// its widget (the old positional guard returned early for it).
func TestTextGrid_RefreshCellLastVisibleRow(t *testing.T) {
	test.NewTempApp(t)
	g := NewTextGrid()
	g.Scroll = bellui.ScrollBoth
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(300, 120))

	g.SetText("aa\nbb\ncc")
	last := len(g.content.visible) - 1
	if last < 0 {
		t.Skip("no visible rows in test driver")
	}
	wid := g.content.visible[last].(*textGridRow)
	g.SetStyle(wid.row, 0, TextGridStyleWhitespace)
	if wid.row < len(g.Rows) && len(g.Rows[wid.row].Cells) > 0 {
		if g.Rows[wid.row].Cells[0].Style != TextGridStyleWhitespace {
			t.Fatal("style not applied to the last visible row's cell")
		}
	}
}
