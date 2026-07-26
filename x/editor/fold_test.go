package editor

import (
	"strings"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// foldDoc builds an n-line document "line0\nline1\n…".
func foldDoc(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "line" + itoa(i)
	}
	return strings.Join(lines, "\n")
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// newFoldEditor builds an editor showing text inside a laid-out window so the
// entry's fold geometry runs as it would on screen.
func newFoldEditor(t *testing.T, text string) *Editor {
	t.Helper()
	test.NewTempApp(t)
	e := NewEditor()
	e.Gutter = NewCompositeGutter(NewLineNumberGutter(), e.FoldGutter())
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(600, 400))
	e.SetText(text)
	w.Content().Refresh()
	return e
}

func rng(sr, sc, er, ec int) TextRange {
	return TextRange{StartRow: sr, StartCol: sc, EndRow: er, EndCol: ec}
}

// --- model -----------------------------------------------------------------

func TestFold_SetFoldRegionsHonorsCollapsed(t *testing.T) {
	e := newFoldEditor(t, foldDoc(10))
	e.SetFoldRegions([]FoldRegion{
		{Range: rng(1, 0, 4, 0), Collapsed: true},
		{Range: rng(6, 0, 8, 0), Collapsed: false},
	})
	got := e.FoldedRanges()
	if len(got) != 1 || got[0] != rng(1, 0, 4, 0) {
		t.Fatalf("FoldedRanges = %v, want just the collapsed region", got)
	}
	// The Entry hides the collapsed region's interior lines [2,5).
	want := []widget.LineRange{{Start: 2, End: 5}}
	if !lineRangesEq(e.entry.HiddenRanges, want) {
		t.Errorf("HiddenRanges = %v, want %v", e.entry.HiddenRanges, want)
	}
}

func TestFold_FoldUnfoldByRange(t *testing.T) {
	e := newFoldEditor(t, foldDoc(10))
	e.SetFoldRegions([]FoldRegion{{Range: rng(2, 0, 6, 0)}})

	e.Fold(rng(2, 0, 6, 0))
	if len(e.FoldedRanges()) != 1 {
		t.Fatalf("Fold did not collapse the region: %v", e.FoldedRanges())
	}
	if want := []widget.LineRange{{Start: 3, End: 7}}; !lineRangesEq(e.entry.HiddenRanges, want) {
		t.Errorf("HiddenRanges = %v, want %v", e.entry.HiddenRanges, want)
	}

	e.Unfold(rng(2, 0, 6, 0))
	if len(e.FoldedRanges()) != 0 {
		t.Errorf("Unfold did not expand the region: %v", e.FoldedRanges())
	}
	if len(e.entry.HiddenRanges) != 0 {
		t.Errorf("HiddenRanges = %v, want empty after unfold", e.entry.HiddenRanges)
	}
}

func TestFold_FoldRegistersUnknownRange(t *testing.T) {
	e := newFoldEditor(t, foldDoc(10))
	e.Fold(rng(1, 0, 5, 0)) // no prior SetFoldRegions
	if len(e.FoldedRanges()) != 1 {
		t.Fatalf("Fold on an unknown range should register+collapse it: %v", e.FoldedRanges())
	}
	if regs := e.FoldRegions(); len(regs) != 1 || regs[0].Kind != FoldKindCustom {
		t.Errorf("expected one registered custom region, got %v", regs)
	}
}

func TestFold_UnfoldUnknownRangeIsNoop(t *testing.T) {
	e := newFoldEditor(t, foldDoc(10))
	e.Unfold(rng(1, 0, 5, 0))
	if len(e.FoldRegions()) != 0 {
		t.Errorf("Unfold of an unknown range must not register anything: %v", e.FoldRegions())
	}
}

func TestFold_SingleLineRangeNotFoldable(t *testing.T) {
	e := newFoldEditor(t, foldDoc(10))
	e.Fold(rng(3, 0, 3, 8)) // start row == end row: nothing to hide
	if len(e.FoldedRanges()) != 0 {
		t.Errorf("a single-line range must not fold: %v", e.FoldedRanges())
	}
	if len(e.entry.HiddenRanges) != 0 {
		t.Errorf("HiddenRanges = %v, want empty", e.entry.HiddenRanges)
	}
}

func TestFold_AutoFoldRegistersAndCollapses(t *testing.T) {
	e := newFoldEditor(t, foldDoc(20))
	e.AutoFold([]TextRange{rng(2, 0, 10, 0), rng(15, 0, 15, 0)}) // 2nd is single-line, ignored
	folded := e.FoldedRanges()
	if len(folded) != 1 || folded[0] != rng(2, 0, 10, 0) {
		t.Fatalf("AutoFold = %v, want the multi-line range collapsed", folded)
	}
	if want := []widget.LineRange{{Start: 3, End: 11}}; !lineRangesEq(e.entry.HiddenRanges, want) {
		t.Errorf("HiddenRanges = %v, want %v", e.entry.HiddenRanges, want)
	}
}

func TestFold_FoldRegionsCopyIsDefensive(t *testing.T) {
	e := newFoldEditor(t, foldDoc(10))
	e.SetFoldRegions([]FoldRegion{{Range: rng(1, 0, 4, 0), Collapsed: true}})
	regs := e.FoldRegions()
	regs[0].Collapsed = false // mutate the copy
	if !e.FoldRegions()[0].Collapsed {
		t.Error("FoldRegions() must return a defensive copy")
	}
}

// --- fold all / unfold all -------------------------------------------------

func TestFold_FoldAllCollapsesEveryFoldable(t *testing.T) {
	e := newFoldEditor(t, foldDoc(20))
	e.SetFoldRegions([]FoldRegion{
		{Range: rng(1, 0, 4, 0)},
		{Range: rng(6, 0, 9, 0)},
		{Range: rng(12, 0, 12, 5)}, // single-line: not foldable
	})

	e.FoldAll()

	folded := e.FoldedRanges()
	if len(folded) != 2 {
		t.Fatalf("FoldAll should collapse both multi-line regions, got %v", folded)
	}
	want := []widget.LineRange{{Start: 2, End: 5}, {Start: 7, End: 10}}
	if !lineRangesEq(e.entry.HiddenRanges, want) {
		t.Errorf("HiddenRanges = %v, want %v", e.entry.HiddenRanges, want)
	}
}

func TestFold_UnfoldAllExpandsEverything(t *testing.T) {
	e := newFoldEditor(t, foldDoc(20))
	e.SetFoldRegions([]FoldRegion{
		{Range: rng(1, 0, 4, 0), Collapsed: true},
		{Range: rng(6, 0, 9, 0), Collapsed: true},
	})

	e.UnfoldAll()

	if folded := e.FoldedRanges(); len(folded) != 0 {
		t.Errorf("UnfoldAll should expand every region, still folded: %v", folded)
	}
	if len(e.entry.HiddenRanges) != 0 {
		t.Errorf("HiddenRanges = %v, want empty after UnfoldAll", e.entry.HiddenRanges)
	}
	// The regions themselves are retained (only their collapsed flag cleared).
	if len(e.FoldRegions()) != 2 {
		t.Errorf("UnfoldAll must keep regions registered, got %v", e.FoldRegions())
	}
}

// --- reveal auto-unfolds the target ----------------------------------------

func TestFold_RevealRangeUnfoldsTarget(t *testing.T) {
	e := newFoldEditor(t, foldDoc(20))
	e.Fold(rng(3, 0, 8, 0)) // hides rows 4..8

	e.RevealRange(rng(5, 0, 5, 3)) // jump into the folded interior

	if folded := e.FoldedRanges(); len(folded) != 0 {
		t.Errorf("RevealRange into a fold should unfold it, still folded: %v", folded)
	}
	if len(e.entry.HiddenRanges) != 0 {
		t.Errorf("HiddenRanges = %v, want empty after revealing into the fold", e.entry.HiddenRanges)
	}
	if e.entry.CursorRow != 5 {
		t.Errorf("cursor row = %d, want 5 (the revealed line)", e.entry.CursorRow)
	}
}

func TestFold_RevealRangeUnfoldsNested(t *testing.T) {
	e := newFoldEditor(t, foldDoc(20))
	e.SetFoldRegions([]FoldRegion{
		{Range: rng(1, 0, 15, 0), Collapsed: true}, // outer hides 2..15
		{Range: rng(4, 0, 8, 0), Collapsed: true},  // inner hides 5..8
	})

	e.RevealRange(rng(6, 0, 6, 0)) // row 6 is hidden by both

	if folded := e.FoldedRanges(); len(folded) != 0 {
		t.Errorf("revealing a row hidden by nested folds should unfold both, still folded: %v", folded)
	}
}

func TestFold_RevealRangeLeavesUnrelatedFolds(t *testing.T) {
	e := newFoldEditor(t, foldDoc(20))
	e.Fold(rng(3, 0, 8, 0)) // hides rows 4..8

	e.RevealRange(rng(12, 0, 12, 0)) // clear of the fold

	if folded := e.FoldedRanges(); len(folded) != 1 || folded[0] != rng(3, 0, 8, 0) {
		t.Errorf("a reveal clear of the fold must leave it collapsed, got %v", folded)
	}
}

func TestFold_RevealRowsControllerNoopWhenVisible(t *testing.T) {
	e := newFoldEditor(t, foldDoc(20))
	e.Fold(rng(3, 0, 8, 0))
	if e.fold.revealRows(3, 3) { // the header row is visible, not hidden
		t.Error("revealRows on a visible (header) row should report no change")
	}
	if e.fold.revealRows(12, 12) { // clear of the fold
		t.Error("revealRows clear of any fold should report no change")
	}
	if !e.fold.revealRows(5, 5) { // inside the hidden interior
		t.Error("revealRows on a hidden interior row should unfold and report change")
	}
}

// --- geometry: document coordinates stay intact ----------------------------

func TestFold_TextUnchangedByFolding(t *testing.T) {
	doc := foldDoc(30)
	e := newFoldEditor(t, doc)
	e.Fold(rng(5, 0, 20, 0))
	if e.Text() != doc {
		t.Error("folding must not change the document text")
	}
	// A search-style position still resolves against document coordinates: the
	// caret cell for a folded-away line is reachable in the model.
	if e.entry.Text != doc {
		t.Error("entry text changed under folding")
	}
}

func TestFold_HiddenRowsCollapseToZeroHeight(t *testing.T) {
	e := newFoldEditor(t, foldDoc(30))
	e.Fold(rng(5, 0, 10, 0)) // hides logical rows 6..10
	yp, h := e.entry.RowPositions()
	if len(yp) < 30 {
		t.Fatalf("RowPositions returned %d rows, want >= 30", len(yp))
	}
	for row := 6; row <= 10; row++ {
		if h[row] != 0 {
			t.Errorf("folded row %d height = %v, want 0", row, h[row])
		}
	}
	// The header row and the first row after the fold stay full height.
	if h[5] == 0 || h[11] == 0 {
		t.Errorf("visible rows collapsed unexpectedly: h[5]=%v h[11]=%v", h[5], h[11])
	}
}

// --- placeholder -----------------------------------------------------------

func TestFold_PlaceholderOnCollapsedHeader(t *testing.T) {
	e := newFoldEditor(t, foldDoc(10))
	e.SetFoldRegions([]FoldRegion{{Range: rng(2, 0, 6, 0), Collapsed: true, Summary: "5 lines"}})
	ph := e.fold.placeholders()
	if len(ph) != 1 {
		t.Fatalf("want one placeholder, got %v", ph)
	}
	if ph[0].row != 2 || ph[0].text != "5 lines" {
		t.Errorf("placeholder = %+v, want row 2 text %q", ph[0], "5 lines")
	}
	// col sits just past the header line's text ("line2" == 5 runes).
	if ph[0].col != 5 {
		t.Errorf("placeholder col = %d, want 5 (end of header line)", ph[0].col)
	}
}

func TestFold_PlaceholderDefaultsToEllipsis(t *testing.T) {
	e := newFoldEditor(t, foldDoc(10))
	e.Fold(rng(1, 0, 3, 0))
	ph := e.fold.placeholders()
	if len(ph) != 1 || ph[0].text != defaultFoldPlaceholder {
		t.Errorf("placeholder = %v, want default ellipsis", ph)
	}
}

func TestFold_NoPlaceholderWhenExpanded(t *testing.T) {
	e := newFoldEditor(t, foldDoc(10))
	e.SetFoldRegions([]FoldRegion{{Range: rng(2, 0, 6, 0), Collapsed: false}})
	if ph := e.fold.placeholders(); len(ph) != 0 {
		t.Errorf("expanded region must have no placeholder, got %v", ph)
	}
}

// --- fold gutter -----------------------------------------------------------

func TestFoldGutter_ChevronOnlyOnFoldableHeader(t *testing.T) {
	e := newFoldEditor(t, foldDoc(10))
	e.SetFoldRegions([]FoldRegion{{Range: rng(3, 0, 7, 0)}})
	g := e.FoldGutter()
	size := geo.NewSize(g.Width(10, 12), 16)

	if objs := g.Render(4, size, 12); len(objs) == 0 { // line 4 == header row 3
		t.Error("expected a chevron on the foldable header line")
	}
	if objs := g.Render(5, size, 12); objs != nil { // interior line, not a header
		t.Errorf("non-header line should draw no chevron, got %d objects", len(objs))
	}
	if objs := g.Render(1, size, 12); objs != nil {
		t.Errorf("non-foldable line should draw no chevron, got %d objects", len(objs))
	}
}

func TestFoldGutter_TapTogglesFold(t *testing.T) {
	e := newFoldEditor(t, foldDoc(10))
	e.SetFoldRegions([]FoldRegion{{Range: rng(3, 0, 7, 0)}})
	g := e.FoldGutter()

	g.Tapped(4) // header line 4 (row 3)
	if len(e.FoldedRanges()) != 1 {
		t.Fatalf("tap on chevron should fold: %v", e.FoldedRanges())
	}
	g.Tapped(4)
	if len(e.FoldedRanges()) != 0 {
		t.Errorf("second tap should unfold: %v", e.FoldedRanges())
	}
}

func TestFoldGutter_ChevronDirectionDiffersByState(t *testing.T) {
	sz := geo.NewSize(20, 16)
	folded := chevron(sz, 8, true, nil)
	expanded := chevron(sz, 8, false, nil)
	if len(folded) != 2 || len(expanded) != 2 {
		t.Fatalf("chevron should be two strokes, got %d/%d", len(folded), len(expanded))
	}
	// The two orientations must not be identical (right-pointing vs down-pointing):
	// the shared apex (first stroke's endpoint) differs.
	fa, ok1 := folded[0].(*canvas.Line)
	ea, ok2 := expanded[0].(*canvas.Line)
	if !ok1 || !ok2 {
		t.Fatal("chevron strokes should be canvas.Line objects")
	}
	if fa.Position2 == ea.Position2 {
		t.Error("collapsed and expanded chevrons should point differently")
	}
}

// --- analytic geometry matches the fork exactly ----------------------------

// TestFold_AnalyticWindowMatchesExact validates that the O(viewport) rowWindow
// used on large documents reproduces the Entry's exact RowPositions geometry,
// including across a fold — the correctness backstop for the large-file fast path.
func TestFold_AnalyticWindowMatchesExact(t *testing.T) {
	e := newFoldEditor(t, foldDoc(largeDocRowThreshold+500))
	e.Fold(rng(10, 0, 25, 0)) // hides rows 11..25

	yp, h := e.entry.RowPositions()
	viewportH := e.entry.Size().Height
	rows, tops, heights := e.rowWindow(0, viewportH)
	if rows == nil {
		t.Fatal("rowWindow returned nil on a large document")
	}
	if len(rows) == 0 {
		t.Fatal("rowWindow returned no visible rows")
	}
	// Every windowed row must be visible (non-zero height) and its analytic
	// geometry must match the exact per-row geometry.
	prev := -1
	for i, row := range rows {
		if row <= prev {
			t.Fatalf("windowed rows not strictly increasing at %d: %v", i, rows)
		}
		prev = row
		if h[row] == 0 {
			t.Errorf("windowed row %d is a hidden row", row)
		}
		if !almostEqual(tops[i], yp[row]) {
			t.Errorf("row %d top = %v, exact = %v", row, tops[i], yp[row])
		}
		if !almostEqual(heights[i], h[row]) {
			t.Errorf("row %d height = %v, exact = %v", row, heights[i], h[row])
		}
	}
	// The fold must be skipped: no row in [11,25] appears in the window when the
	// window covers that band.
	for _, row := range rows {
		if row >= 11 && row <= 25 {
			t.Errorf("folded row %d appeared in the visible window", row)
		}
	}
}

func TestFold_RowWindowNilBelowThreshold(t *testing.T) {
	e := newFoldEditor(t, foldDoc(100))
	if rows, _, _ := e.rowWindow(0, 400); rows != nil {
		t.Error("rowWindow should return nil (defer to exact path) below the size threshold")
	}
}

// TestFold_GutterSkipsHiddenRowNumbers guards that folded-away rows draw no
// gutter content — otherwise their zero-height line numbers stack into a smudge
// at the fold point (the exact/small-doc path).
func TestFold_GutterSkipsHiddenRowNumbers(t *testing.T) {
	e := newFoldEditor(t, foldDoc(12))
	e.Gutter = NewLineNumberGutter() // line numbers only, so drawn text == row numbers
	e.Refresh()
	e.Fold(rng(2, 0, 8, 0)) // hides logical rows 3..8 (1-based line numbers 4..9)

	rend := e.gutterW.CreateRenderer().(*gutterWidgetRenderer)
	rend.Layout(geo.NewSize(e.gutterW.gutterWidth(), 240))

	drawn := map[string]bool{}
	for _, o := range rend.Objects() {
		if txt, ok := o.(*canvas.Text); ok {
			drawn[txt.Text] = true
		}
	}
	// Header (3) and the first row after the fold (10) stay; the folded interior
	// line numbers must be absent.
	for _, hidden := range []string{"4", "5", "6", "7", "8", "9"} {
		if drawn[hidden] {
			t.Errorf("folded line number %q should not be drawn", hidden)
		}
	}
	for _, shown := range []string{"3", "10"} {
		if !drawn[shown] {
			t.Errorf("visible line number %q should be drawn", shown)
		}
	}
}

// TestFold_RendersWithoutPanic paints a folded editor (default placeholder +
// line-number gutter) through the software painter, guarding that the default
// placeholder is renderable on every backend — a non-Latin default would tofu on
// wasm and can nil-deref the software painter's font-fallback path.
func TestFold_RendersWithoutPanic(t *testing.T) {
	e := newFoldEditor(t, foldDoc(10))
	e.Gutter = NewCompositeGutter(NewLineNumberGutter(), e.FoldGutter())
	e.Refresh()
	e.SetFoldRegions([]FoldRegion{{Range: rng(1, 0, 6, 0), Collapsed: true}}) // default "..." placeholder

	c := test.NewWindow(e)
	t.Cleanup(c.Close)
	c.Resize(geo.NewSize(320, 220))
	// Capture forces a full paint pass; a bad placeholder glyph would panic here.
	if img := c.Canvas().Capture(); img == nil {
		t.Fatal("capture returned nil image")
	}
}

// --- small helpers ---------------------------------------------------------

func lineRangesEq(a, b []widget.LineRange) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func almostEqual(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 0.01
}
