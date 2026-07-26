package widget

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// TestEntry_NonVirtualGutterAlignsAcrossFold covers #144: in the non-virtualised
// full-layout path, the built-in line-number gutter and the syntax-highlight
// fragments must stay correctly numbered and positioned across a fold. A folded
// row carries no line number, and every visible row's gutter number and text
// fragment sit at the collapsed row geometry (rowYPos), so nothing drifts.
func TestEntry_NonVirtualGutterAlignsAcrossFold(t *testing.T) {
	test.NewTempApp(t)

	entry := &Entry{
		MultiLine: true,
		Wrapping:  bellui.TextWrapOff,
		Scroll:    bellui.ScrollNone, // forces the non-virtual full-layout path
		LineHighlighter: func(line string, state int) ([]StyleRun, int) {
			return []StyleRun{{Text: line}}, 0
		},
	}
	w := test.NewWindow(entry)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(300, 220))
	entry.Resize(geo.NewSize(280, 200))

	entry.SetText("aaa\nbbb\nccc\nddd\neee")

	// Fold the 2nd logical row (index 1, "bbb").
	entry.HiddenRanges = []LineRange{{Start: 1, End: 2}}
	entry.Refresh()

	require.False(t, entry.text.canVirtualize(), "scroll-none entry must use the non-virtual path")

	// Collect gutter line-number fragments (monospace, numeric) and the visible
	// content text fragments (non-transparent) from the provider renderer.
	r := textRendererOf(entry)
	gutterY := map[string]float32{}  // line number -> Y
	contentY := map[string]float32{} // row text -> Y
	for _, o := range r.Objects() {
		txt, ok := o.(*canvas.Text)
		if !ok {
			continue
		}
		if txt.TextStyle.Monospace {
			if _, err := strconv.Atoi(txt.Text); err == nil {
				gutterY[txt.Text] = txt.Position().Y
			}
			continue
		}
		// Visible rows have their original made transparent and a colored
		// fragment appended; pick up only the visible (alpha > 0) fragments.
		if _, _, _, a := txt.Color.RGBA(); a > 0 && txt.Text != "" {
			contentY[txt.Text] = txt.Position().Y
		}
	}

	// The folded row's line number must not render; every visible one must.
	assert.NotContains(t, gutterY, "2", "folded row's gutter number must be absent")
	for _, n := range []string{"1", "3", "4", "5"} {
		assert.Contains(t, gutterY, n, "visible row %s must have a gutter number", n)
	}

	// Each visible gutter number and its text fragment sit at the collapsed row
	// geometry exposed by RowPositions (the same rowYPos the layout pass cached).
	yPos, _ := entry.RowPositions()
	require.GreaterOrEqual(t, len(yPos), 5)

	rowText := []string{"aaa", "bbb", "ccc", "ddd", "eee"}
	for numStr, y := range gutterY {
		n, _ := strconv.Atoi(numStr)
		assert.InDelta(t, yPos[n-1], y, 0.5,
			"gutter number %s Y must match rowYPos[%d]", numStr, n-1)
		assert.InDelta(t, yPos[n-1], contentY[rowText[n-1]], 0.5,
			"content %q Y must match rowYPos[%d]", rowText[n-1], n-1)
	}

	// Row 2 collapses to zero height, so line 3 sits exactly one stride below
	// line 1 — proving the gutter follows the collapsed geometry across the fold.
	_, stride := entry.RowMetrics()
	assert.InDelta(t, gutterY["1"]+stride, gutterY["3"], 0.5,
		"line 3 must sit one stride below line 1 with row 2 folded")
}
