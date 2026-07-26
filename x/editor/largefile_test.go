package editor

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// Phase 5 deliverable 5: editing a large document must stay interactive because
// highlight + decoration + gutter + fold work scales with the viewport, not the
// document. These tests guard that property structurally (object counts and rows
// visited are viewport-bounded regardless of document size) and with a generous
// wall-clock budget as a regression catch.

// largeFileLines is the benchmark document size — the roadmap's 50k-line case.
const largeFileLines = 50000

// buildLargeEditor lays out an editor showing an n-line document at a fixed
// viewport, with the full gutter stack and a row decorator installed so every
// per-row render path is exercised.
func buildLargeEditor(t testing.TB, n int) *Editor {
	t.Helper()
	test.NewTempApp(t)
	e := NewEditor()
	e.Gutter = NewCompositeGutter(NewLineNumberGutter(), e.FoldGutter())
	e.Decorators = []RowDecorator{NewCurrentLineDecorator(1)}
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(800, 600))
	e.SetText(foldDoc(n))
	w.Content().Refresh()
	return e
}

// scrollTo mirrors an editor scroll: sync the offset the gutter and overlay read.
func scrollTo(e *Editor, y float32) {
	e.scrollY = y
	e.gutterW.scrollY = y
	e.decOverlay.scrollY = y
}

// gutterObjectCount renders the gutter at the laid-out size and returns how many
// canvas objects it emits (a proxy for per-frame work).
func gutterObjectCount(e *Editor) int {
	r := e.gutterW.CreateRenderer().(*gutterWidgetRenderer)
	r.Layout(geo.NewSize(e.gutterW.gutterWidth(), 600))
	return len(r.Objects())
}

func TestLargeFile_VirtualisationActive(t *testing.T) {
	e := buildLargeEditor(t, largeFileLines)
	if e.Text() == "" || strings.Count(e.Text(), "\n") != largeFileLines-1 {
		t.Fatalf("document not loaded: %d lines", strings.Count(e.Text(), "\n")+1)
	}
	// The analytic window path must engage on a document this large.
	if rows, _, _ := e.rowWindow(0, 600); rows == nil {
		t.Fatal("rowWindow returned nil on a 50k-line document (fast path disabled)")
	}
}

// TestLargeFile_GutterWorkIsViewportBounded is the core structural guard: the
// number of gutter objects for the same viewport must not grow with the document.
// A 50k-line file and a 5k-line file, scrolled to the same offset, render the same
// (viewport-bounded) amount of work — proving O(viewport), not O(document).
func TestLargeFile_GutterWorkIsViewportBounded(t *testing.T) {
	small := buildLargeEditor(t, 5000)
	large := buildLargeEditor(t, largeFileLines)

	// Scroll both to a deep, identical offset so both show a full viewport of rows.
	_, strideSmall := small.entry.RowMetrics()
	_, strideLarge := large.entry.RowMetrics()
	if strideSmall <= 0 || strideSmall != strideLarge {
		t.Fatalf("strides differ (%v vs %v); cannot compare windows", strideSmall, strideLarge)
	}
	off := 2000 * strideSmall
	scrollTo(small, off)
	scrollTo(large, off)

	cs := gutterObjectCount(small)
	cl := gutterObjectCount(large)
	if cs == 0 || cl == 0 {
		t.Fatalf("no gutter objects rendered (small=%d large=%d)", cs, cl)
	}
	if cs != cl {
		t.Errorf("gutter work not viewport-bounded: 5k lines=%d objects, 50k lines=%d objects", cs, cl)
	}
	// And the object count itself is bounded to a viewport's worth of rows, not
	// thousands. 600px / ~row height gives a few dozen rows, each a handful of objects.
	if cl > 400 {
		t.Errorf("gutter emitted %d objects for one viewport — expected viewport-bounded", cl)
	}
}

// TestLargeFile_RenderBudget is the concrete latency guard: a full external
// render cycle (gutter + decorator overlay) at the bottom of a 50k-line document
// must complete well within a single 60fps frame (16.6ms). Measured cost is
// ~0.05ms; the budget leaves a ~100x margin so it is robust on slow/CI hardware
// (including the race detector) while still catching an O(document) regression,
// which was ~15ms per gutter refresh before windowing.
func TestLargeFile_RenderBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing test in -short mode")
	}
	e := buildLargeEditor(t, largeFileLines)
	_, stride := e.entry.RowMetrics()
	scrollTo(e, float32(largeFileLines-40)*stride) // near the bottom

	const iters = 50
	start := time.Now()
	for i := 0; i < iters; i++ {
		e.gutterW.Refresh()
		e.decOverlay.Refresh()
	}
	perCycle := time.Since(start) / iters

	const budget = 16 * time.Millisecond
	t.Logf("large-file external render cycle: %v (budget %v)", perCycle, budget)
	if perCycle > budget {
		t.Errorf("render cycle %v exceeds the %v frame budget — O(document) regression?", perCycle, budget)
	}
}

// TestLargeFile_AutoFoldHugeRegion covers pxfed's case: auto-collapse a huge
// region on load. The fold applies, FoldedRanges reflects it, and the Entry hides
// the interior — all without touching the document text.
func TestLargeFile_AutoFoldHugeRegion(t *testing.T) {
	e := buildLargeEditor(t, largeFileLines)
	doc := e.Text()

	huge := rng(100, 0, 40000, 0)
	e.AutoFold([]TextRange{huge})

	if folded := e.FoldedRanges(); len(folded) != 1 || folded[0] != huge {
		t.Fatalf("AutoFold did not collapse the huge region: %v", e.FoldedRanges())
	}
	if e.Text() != doc {
		t.Error("AutoFold changed the document text")
	}
	// Interior lines are hidden; the visible-row count drops by ~the folded span.
	hidden := e.fold.mergedHidden()
	if got := visibleRowCount(e.lineCount(), hidden); got >= largeFileLines-30000 {
		t.Errorf("visible rows = %d, expected the huge region collapsed away", got)
	}
	// Rendering the folded document is still viewport-bounded.
	scrollTo(e, 0)
	if n := gutterObjectCount(e); n > 400 {
		t.Errorf("folded large document emitted %d gutter objects, expected viewport-bounded", n)
	}
}

// TestLargeFile_HighlighterIsWindowed is the latency guard for #106: SetText of a
// large document must invoke the Highlighter only for the rows the renderer shows,
// not every line. Before the windowed-highlight fyne stack (fyne#401), SetText
// chained the LineHighlighter through all 50k lines synchronously — the ~6.5s wasm
// stall the issue reported (~88% of the large-file freeze). fycoder itself never
// looped the document; it only wires the LineHighlighter, so this guards that the
// fyne dependency keeps highlighting O(viewport) on open.
//
// The background highlight-chain prefill (fyne#417) is animation-driven and does
// not tick under the headless test driver, so the count observed here reflects the
// synchronous on-open window alone.
func TestLargeFile_HighlighterIsWindowed(t *testing.T) {
	test.NewTempApp(t)

	var calls int64
	e := NewEditor()
	e.Highlighter = HighlighterFunc(func(line string, state int) ([]StyleRun, int) {
		atomic.AddInt64(&calls, 1)
		return []StyleRun{{Text: line}}, 0
	})
	e.Gutter = NewCompositeGutter(NewLineNumberGutter(), e.FoldGutter())
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(800, 600))
	e.SetText(foldDoc(largeFileLines))
	w.Content().Refresh()

	n := atomic.LoadInt64(&calls)
	if n == 0 {
		t.Fatal("highlighter never invoked — visible window not rendered")
	}
	// A 600px viewport is a few dozen rows. Allow generous slack for layout
	// passes and chain re-verification, but far below the O(document) count
	// (largeFileLines) a non-windowed highlighter would produce.
	if n > 500 {
		t.Errorf("highlighter invoked %d times on SetText of %d lines — expected viewport-bounded (#106 windowing regression)", n, largeFileLines)
	}
	t.Logf("highlighter invoked %d times on SetText of %d lines (viewport-bounded)", n, largeFileLines)
}

// BenchmarkLargeFileRenderCycle measures the external render cost (gutter +
// overlay) at the bottom of a 50k-line document — the benchmark the roadmap asks
// to establish for the large-file case.
func BenchmarkLargeFileRenderCycle(b *testing.B) {
	e := buildLargeEditor(b, largeFileLines)
	_, stride := e.entry.RowMetrics()
	scrollTo(e, float32(largeFileLines-40)*stride)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.gutterW.Refresh()
		e.decOverlay.Refresh()
	}
}
