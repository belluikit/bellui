package widget

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// countingHighlighter is a trivial chained LineHighlighter that records how many
// lines it was asked to highlight. State chains forward: a line containing
// "OPEN" flips the state to 1 until a line containing "CLOSE".
func countingHighlighter(calls *int) func(line string, state int) ([]StyleRun, int) {
	return func(line string, state int) ([]StyleRun, int) {
		*calls++
		out := state
		if strings.Contains(line, "OPEN") {
			out = 1
		}
		if strings.Contains(line, "CLOSE") {
			out = 0
		}
		return []StyleRun{{Text: line}}, out
	}
}

func newHighlightedBigEntry(t *testing.T, lines int) (*Entry, *int) {
	t.Helper()
	test.NewTempApp(t)
	calls := new(int)
	e := NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapOff
	e.LineHighlighter = countingHighlighter(calls)
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 300))
	e.SetText(strings.Repeat("some code line with tokens\n", lines))
	e.textProvider().updateRowBounds()
	return e, calls
}

// TestHighlight_SetTextIsWindowed is the fycoder#106 / #401 acceptance: setting
// a large document runs the app-supplied highlighter only for the rows actually
// requested (one screenful), not for every line up front.
func TestHighlight_SetTextIsWindowed(t *testing.T) {
	e, calls := newHighlightedBigEntry(t, 5000)

	// Simulate the renderer requesting the visible band.
	for row := 0; row < 40; row++ {
		e.HighlightRunsForRow(row)
	}
	require.NotNil(t, e.HighlightRunsForRow(10), "visible rows are highlighted")
	assert.Less(t, *calls, 200,
		"SetText+first window highlighted %d lines — must be O(window), not all 5000", *calls)
}

// TestHighlight_KeystrokeIsBounded pins the per-edit highlight cost: after the
// visible band is computed, a keystroke re-runs the highlighter only until the
// state chain converges (the edited line, in the common case) — never a
// document-wide re-scan (#401, kills the strings.Split-per-keystroke).
func TestHighlight_KeystrokeIsBounded(t *testing.T) {
	e, calls := newHighlightedBigEntry(t, 5000)
	for row := 0; row < 40; row++ {
		e.HighlightRunsForRow(row)
	}

	e.CursorRow, e.CursorColumn = 5, 4
	before := *calls
	e.TypedRune('x')
	for row := 0; row < 40; row++ {
		e.HighlightRunsForRow(row)
	}
	assert.Less(t, *calls-before, 40,
		"one keystroke re-highlighted %d lines — must be edit-local", *calls-before)
	assert.NotNil(t, e.HighlightRunsForRow(5))
}

// TestHighlight_ChainedStateAcrossEdit verifies correctness of the incremental
// chain: an edit that opens a multi-line construct re-highlights downstream
// lines until the state converges, so stale cached runs are never served.
func TestHighlight_ChainedStateAcrossEdit(t *testing.T) {
	test.NewTempApp(t)
	calls := new(int)
	e := NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapOff
	e.LineHighlighter = func(line string, state int) ([]StyleRun, int) {
		(*calls)++
		out := state
		if strings.Contains(line, "OPEN") {
			out = 1
		}
		if strings.Contains(line, "CLOSE") {
			out = 0
		}
		// Encode the incoming state in the run so the test can observe it.
		return []StyleRun{{Text: line, Bold: state == 1}}, out
	}
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 300))
	e.SetText(strings.Repeat("plain line\n", 2999) + "last line")
	e.textProvider().updateRowBounds()

	for row := 0; row < 20; row++ {
		e.HighlightRunsForRow(row)
	}
	require.False(t, e.HighlightRunsForRow(10)[0].Bold, "state 0 before the edit")

	// Type OPEN into line 5: lines 6+ must now see state 1.
	e.CursorRow, e.CursorColumn = 5, 0
	for _, r := range "OPEN " {
		e.TypedRune(r)
	}
	for row := 0; row < 20; row++ {
		e.HighlightRunsForRow(row)
	}
	assert.True(t, e.HighlightRunsForRow(10)[0].Bold, "downstream lines re-chained to state 1")
	assert.False(t, e.HighlightRunsForRow(3)[0].Bold, "upstream lines untouched")
}

// TestHighlight_DeepEditAboveViewportIsBounded is the #412 acceptance: with a
// deep viewport already chained (the one-time first-jump cost), an edit far
// above it re-chains only until the state converges back into the trusted
// stretch, then jumps — O(edit + convergence + window) walk steps, never a
// per-line walk back to the viewport.
func TestHighlight_DeepEditAboveViewportIsBounded(t *testing.T) {
	e, _ := newHighlightedBigEntry(t, 5000)

	// First deep jump: chains from line 0 (unavoidable, documented).
	for row := 4960; row < 5000; row++ {
		e.HighlightRunsForRow(row)
	}
	require.GreaterOrEqual(t, e.hlWalkSteps, 4960, "first deep jump chains from line 0")

	// Edit near the top, then re-request the deep band.
	e.CursorRow, e.CursorColumn = 5, 4
	e.TypedRune('x')
	before := e.hlWalkSteps
	for row := 4960; row < 5000; row++ {
		e.HighlightRunsForRow(row)
	}
	assert.Less(t, e.hlWalkSteps-before, 40,
		"deep re-chain after a top edit walked %d lines — must jump the trusted stretch", e.hlWalkSteps-before)
	assert.NotNil(t, e.HighlightRunsForRow(4990))
	assert.Equal(t, "somex code line with tokens", e.hlCache[5].text)
}

// TestHighlight_DeepJumpNeverServesStaleState proves the #412 jump is gated on
// state convergence: an edit that flips downstream state re-runs the
// highlighter through every affected line (no jump over stale entries), and
// once a CLOSE line restores the state the walk converges there — deep lines
// keep their still-valid runs without being re-read.
func TestHighlight_DeepJumpNeverServesStaleState(t *testing.T) {
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapOff
	e.LineHighlighter = func(line string, state int) ([]StyleRun, int) {
		out := state
		if strings.Contains(line, "OPEN") {
			out = 1
		}
		if strings.Contains(line, "CLOSE") {
			out = 0
		}
		return []StyleRun{{Text: line, Bold: state == 1}}, out
	}
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 300))
	lines := make([]string, 3000)
	for i := range lines {
		lines[i] = "plain line"
	}
	lines[100] = "a CLOSE marker"
	e.SetText(strings.Join(lines, "\n"))
	e.textProvider().updateRowBounds()

	for row := 2960; row < 3000; row++ {
		e.HighlightRunsForRow(row)
	}
	require.False(t, e.HighlightRunsForRow(2980)[0].Bold, "state 0 before the edit")

	// Type OPEN into line 5: lines 6..100 must re-chain to state 1, the walk
	// converges at the CLOSE line and jumps the rest.
	e.CursorRow, e.CursorColumn = 5, 0
	before := e.hlWalkSteps
	for _, r := range "OPEN " {
		e.TypedRune(r)
	}
	for row := 2960; row < 3000; row++ {
		e.HighlightRunsForRow(row)
	}
	assert.True(t, e.HighlightRunsForRow(50)[0].Bold, "lines below the edit re-chained to state 1")
	assert.True(t, e.HighlightRunsForRow(100)[0].Bold, "the CLOSE line itself entered with state 1")
	assert.False(t, e.HighlightRunsForRow(101)[0].Bold, "state restored after CLOSE")
	assert.False(t, e.HighlightRunsForRow(2980)[0].Bold, "deep lines still state 0")
	assert.Less(t, e.hlWalkSteps-before, 700,
		"walked %d lines for an edit converging at line 100 — must jump after convergence", e.hlWalkSteps-before)
}

// TestHighlight_DeepJumpSurvivesNewlineSplices pins the trusted-stretch index
// maintenance across line-count changes: splitting and merging lines above a
// chained deep viewport shifts the stretch, so the jump stays aligned and the
// deep cache entries keep matching their logical lines (#412).
func TestHighlight_DeepJumpSurvivesNewlineSplices(t *testing.T) {
	e, _ := newHighlightedBigEntry(t, 5000)
	for row := 4960; row < 5000; row++ {
		e.HighlightRunsForRow(row)
	}

	// Split line 2, then re-request the deep band (now one row further down).
	e.CursorRow, e.CursorColumn = 2, 4
	e.TypedKey(&event.KeyEvent{Name: event.KeyEnter})
	before := e.hlWalkSteps
	for row := 4961; row < 5001; row++ {
		e.HighlightRunsForRow(row)
	}
	assert.Less(t, e.hlWalkSteps-before, 40, "re-chain after newline split must jump")
	require.Equal(t, 5002, len(e.hlCache)) // 5000 lines + trailing empty + the split
	assert.Equal(t, "some", e.hlCache[2].text)
	assert.Equal(t, " code line with tokens", e.hlCache[3].text)
	assert.Equal(t, "some code line with tokens", e.hlCache[4990].text)

	// Merge back; the deep band shifts up again.
	e.CursorRow, e.CursorColumn = 3, 0
	e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	before = e.hlWalkSteps
	for row := 4960; row < 5000; row++ {
		e.HighlightRunsForRow(row)
	}
	assert.Less(t, e.hlWalkSteps-before, 40, "re-chain after line merge must jump")
	require.Equal(t, 5001, len(e.hlCache))
	assert.Equal(t, "some code line with tokens", e.hlCache[2].text)
	assert.Equal(t, "some code line with tokens", e.hlCache[4990].text)
}

// batchSpliceRows lands one insert per row directly on the provider before a
// single text sync and render — the programmatic batch-edit shape (undo
// replay, find-replace-all, a formatter patch list) where no viewport
// re-chain runs between the splices, unlike interactive typing where every
// keystroke's refresh re-verifies the prefix (#418). Rows must be descending
// so earlier splices don't shift later positions.
func batchSpliceRows(e *Entry, rows []int, col int, insert string) {
	p := e.textProvider()
	for _, row := range rows {
		p.insertAt(textPosFromRowCol(row, col, p), []rune(insert))
	}
	e.updateText(p.String(), false)
	e.Refresh()
}

// TestHighlight_ScatteredBatchEditsStayBounded is the #418 acceptance: K
// scattered edits landing between renders each leave their own trusted
// stretch, so the next deep re-chain walks O(K·convergence + window) lines —
// the single-stretch #412 bookkeeping kept only the largest fragment and fell
// back to a per-line compare walk over the rest.
func TestHighlight_ScatteredBatchEditsStayBounded(t *testing.T) {
	e, _ := newHighlightedBigEntry(t, 5000)
	for row := 4960; row < 5000; row++ {
		e.HighlightRunsForRow(row)
	}

	batchSpliceRows(e, []int{4000, 2000, 100}, 4, "x")
	require.Equal(t, 3, len(e.hlTrusted), "each scattered edit keeps its own stretch")

	before := e.hlWalkSteps
	for row := 4960; row < 5000; row++ {
		e.HighlightRunsForRow(row)
	}
	assert.Less(t, e.hlWalkSteps-before, 40,
		"deep re-chain after 3 scattered edits walked %d lines — must jump every stretch", e.hlWalkSteps-before)
	for _, row := range []int{100, 2000, 4000} {
		assert.Equalf(t, "somex code line with tokens", e.hlCache[row].text, "edited line %d", row)
	}
	assert.Equal(t, "some code line with tokens", e.hlCache[4990].text)
}

// TestHighlight_ScatteredEditsNeverServeStaleState proves multi-stretch jumps
// stay gated on state convergence: with two stretches tracked, an edit that
// flips downstream state re-runs the highlighter through the first stretch's
// lines until a CLOSE restores the state, then jumps mid-stretch — and the
// second stretch (above the other batch edit) is still jumped, never serving
// stale runs (#418).
func TestHighlight_ScatteredEditsNeverServeStaleState(t *testing.T) {
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapOff
	e.LineHighlighter = func(line string, state int) ([]StyleRun, int) {
		out := state
		if strings.Contains(line, "OPEN") {
			out = 1
		}
		if strings.Contains(line, "CLOSE") {
			out = 0
		}
		return []StyleRun{{Text: line, Bold: state == 1}}, out
	}
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 300))
	lines := make([]string, 3000)
	for i := range lines {
		lines[i] = "plain line"
	}
	lines[1500] = "a CLOSE marker"
	e.SetText(strings.Join(lines, "\n"))
	e.textProvider().updateRowBounds()
	for row := 2960; row < 3000; row++ {
		e.HighlightRunsForRow(row)
	}

	// Batch: an edit deep down, then OPEN near the top — two stretches.
	p := e.textProvider()
	p.insertAt(textPosFromRowCol(2500, 4, p), []rune{'x'})
	p.insertAt(textPosFromRowCol(5, 0, p), []rune("OPEN "))
	e.updateText(p.String(), false)
	e.Refresh()
	require.Equal(t, 2, len(e.hlTrusted))

	before := e.hlWalkSteps
	for row := 2960; row < 3000; row++ {
		e.HighlightRunsForRow(row)
	}
	assert.True(t, e.HighlightRunsForRow(1000)[0].Bold, "lines below OPEN re-chained to state 1")
	assert.False(t, e.HighlightRunsForRow(1501)[0].Bold, "state restored after CLOSE")
	assert.False(t, e.HighlightRunsForRow(2600)[0].Bold, "lines above the deep edit still state 0")
	assert.Equal(t, "plaixn line", e.hlCache[2500].text)
	walked := e.hlWalkSteps - before
	assert.Greater(t, walked, 1400, "the flipped-state lines must all be re-read")
	assert.Less(t, walked, 1600,
		"walked %d lines — must jump mid-stretch at the CLOSE convergence and over the second stretch", walked)
}

// TestHighlight_TrustedStretchListIsBounded pins the eviction path: more
// scattered edits than hlMaxTrustedStretches drops the shortest stretches
// (their lines fall back to the compare walk) while correctness is unaffected.
func TestHighlight_TrustedStretchListIsBounded(t *testing.T) {
	e, _ := newHighlightedBigEntry(t, 5000)
	for row := 4960; row < 5000; row++ {
		e.HighlightRunsForRow(row)
	}

	rows := make([]int, 0, 12)
	for row := 4800; row >= 400; row -= 400 { // 12 scattered edits
		rows = append(rows, row)
	}
	batchSpliceRows(e, rows, 4, "x")
	require.Equal(t, hlMaxTrustedStretches, len(e.hlTrusted), "list capped by evicting the shortest")

	for row := 4960; row < 5000; row++ {
		e.HighlightRunsForRow(row)
	}
	for row := 400; row <= 4800; row += 400 {
		assert.Equalf(t, "somex code line with tokens", e.hlCache[row].text, "edited line %d", row)
	}
	assert.Equal(t, "some code line with tokens", e.hlCache[4990].text)
}

// zeroPrefillBudget pins hlPrefillTick to exactly one slice per call so the
// #417 tests advance deterministically regardless of machine speed.
func zeroPrefillBudget(t *testing.T) {
	t.Helper()
	old := hlPrefillFrameBudget
	hlPrefillFrameBudget = 0
	t.Cleanup(func() { hlPrefillFrameBudget = old })
}

// drainHlPrefill drives the background prefill (#417) to completion the way
// frame ticks would on a real driver (the test driver never ticks animations),
// asserting the progress guarantee: every tick either advances the watermark
// or stops the driver, so prefill always terminates.
func drainHlPrefill(t *testing.T, e *Entry) {
	t.Helper()
	for i := 0; e.hlPrefill != nil; i++ {
		require.Less(t, i, 10000, "prefill did not terminate")
		before := e.hlComputedThrough
		e.hlPrefillTick()
		if e.hlPrefill != nil {
			require.Greater(t, e.hlComputedThrough, before, "a tick must make progress")
		}
	}
}

// TestHighlight_PrefillCompletesAfterSetText is the #417 acceptance: after
// SetText the background prefill chains the whole document in bounded slices,
// so a later deep jump finds the watermark already past it and walks nothing.
func TestHighlight_PrefillCompletesAfterSetText(t *testing.T) {
	zeroPrefillBudget(t)
	e, calls := newHighlightedBigEntry(t, 5000)

	// The first on-demand request detects the fresh buffer and arms prefill.
	e.HighlightRunsForRow(0)
	require.NotNil(t, e.hlPrefill, "prefill armed on fresh content")

	drainHlPrefill(t, e)
	assert.Equal(t, e.text.buf.LineCount(), e.hlComputedThrough,
		"prefill chained the whole document")
	assert.LessOrEqual(t, *calls, 5001+1, "prefill highlights each line exactly once")

	// Deep jump after prefill: the band is below the watermark, zero chain walk.
	before := e.hlWalkSteps
	for row := 4960; row < 5000; row++ {
		require.NotNil(t, e.HighlightRunsForRow(row))
	}
	assert.Equal(t, before, e.hlWalkSteps, "deep jump after prefill walks no lines")
}

// TestHighlight_PrefillSyncCostIsGap pins the partial-prefill acceptance: a
// deep jump arriving before prefill has finished chains only the gap between
// the watermark and the viewport, not the document.
func TestHighlight_PrefillSyncCostIsGap(t *testing.T) {
	zeroPrefillBudget(t)
	e, _ := newHighlightedBigEntry(t, 5000)
	e.HighlightRunsForRow(0)

	// Let prefill advance partway, then stop ticking (as if the jump arrived).
	for e.hlPrefill != nil && e.hlComputedThrough < 3000 {
		e.hlPrefillTick()
	}
	watermark := e.hlComputedThrough
	require.GreaterOrEqual(t, watermark, 3000)
	require.Less(t, watermark, 4960, "prefill must still be partway for this test")

	before := e.hlWalkSteps
	for row := 4960; row < 5000; row++ {
		e.HighlightRunsForRow(row)
	}
	assert.LessOrEqual(t, e.hlWalkSteps-before, 5000-watermark,
		"deep jump must chain only the unprefilled gap")
}

// TestHighlight_PrefillRestartsOnSetText verifies the driver re-arms for a
// content replacement and re-chains against the new buffer. Arming happens in
// the SetText path itself, not the render path: a repaint can serve cached
// rows without consulting the highlighter, so waiting for a render-side
// HighlightRunsForRow would leave the prefill unarmed.
func TestHighlight_PrefillRestartsOnSetText(t *testing.T) {
	zeroPrefillBudget(t)
	e, _ := newHighlightedBigEntry(t, 3000)
	drainHlPrefill(t, e)
	require.Nil(t, e.hlPrefill)

	e.SetText(strings.Repeat("fresh content line\n", 2000))
	require.NotNil(t, e.hlPrefill, "prefill re-armed by SetText, no render needed")
	e.textProvider().updateRowBounds()

	drainHlPrefill(t, e)
	assert.Equal(t, e.text.buf.LineCount(), e.hlComputedThrough)
	assert.Equal(t, "fresh content line", e.hlCache[1500].text,
		"prefill chained the new content, not the old")
}

// TestHighlight_PrefillRestartsOnIdenticalSetText pins the subtle probe-found
// case: SetText with value-identical text keeps the provider's buffer object
// (updateRowBoundsLazy reuses it on a value-equal compare), so a buffer-swap
// check alone never fires — yet the cache and watermark were voided, and the
// next deep jump would stall without a re-armed prefill.
func TestHighlight_PrefillRestartsOnIdenticalSetText(t *testing.T) {
	zeroPrefillBudget(t)
	e, _ := newHighlightedBigEntry(t, 3000)
	drainHlPrefill(t, e)
	require.Nil(t, e.hlPrefill)
	buf := e.text.buf

	e.SetText(strings.Repeat("some code line with tokens\n", 3000))
	require.Same(t, buf, e.text.buf, "identical text must reuse the buffer object")
	require.NotNil(t, e.hlPrefill, "prefill re-armed despite the surviving buffer")

	drainHlPrefill(t, e)
	assert.Equal(t, e.text.buf.LineCount(), e.hlComputedThrough)
}

// TestHighlight_PrefillSurvivesEditBelowWatermark pins mid-prefill edits: an
// edit lowers the watermark (the refresh's own re-chain then jumps the trusted
// stretch, #412), and subsequent ticks resume and still finish with a correct,
// fully-chained cache — no restart bookkeeping needed.
func TestHighlight_PrefillSurvivesEditBelowWatermark(t *testing.T) {
	zeroPrefillBudget(t)
	e, _ := newHighlightedBigEntry(t, 5000)
	e.HighlightRunsForRow(0)
	for e.hlPrefill != nil && e.hlComputedThrough < 2000 {
		e.hlPrefillTick()
	}
	require.NotNil(t, e.hlPrefill, "prefill still running partway through")

	e.CursorRow, e.CursorColumn = 5, 4
	e.TypedRune('x')
	require.Less(t, e.hlComputedThrough, 4000, "edit must not chain ahead of the prefill")

	drainHlPrefill(t, e)
	assert.Equal(t, e.text.buf.LineCount(), e.hlComputedThrough)
	assert.Equal(t, "somex code line with tokens", e.hlCache[5].text)
	assert.Equal(t, "some code line with tokens", e.hlCache[4990].text)
}

// TestHighlight_PrefillStopsWithoutHighlighter covers the lifecycle guards: no
// prefill without a LineHighlighter, and clearing the highlighter (or reaching
// a non-lazy provider) stops a running driver.
func TestHighlight_PrefillStopsWithoutHighlighter(t *testing.T) {
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapOff
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 300))
	e.SetText(strings.Repeat("plain line\n", 1000))
	e.textProvider().updateRowBounds()
	e.HighlightRunsForRow(0)
	require.Nil(t, e.hlPrefill, "no prefill when LineHighlighter is nil")

	// Arm it, then clear the highlighter: the next tick self-stops.
	e.LineHighlighter = func(line string, state int) ([]StyleRun, int) {
		return []StyleRun{{Text: line}}, state
	}
	e.HighlightRunsForRow(0)
	require.NotNil(t, e.hlPrefill)
	e.LineHighlighter = nil
	e.hlPrefillTick()
	assert.Nil(t, e.hlPrefill, "tick stops when the highlighter is gone")
}

// TestHighlight_PrefillStopsOnRendererDestroy is the #421 acceptance: a
// renderer Destroy (discarded widget, or cache eviction of a hidden one)
// stops the prefill driver, so its animation closure no longer holds the
// Entry — rope buffer, hlCache and all — alive until the chain completes.
func TestHighlight_PrefillStopsOnRendererDestroy(t *testing.T) {
	zeroPrefillBudget(t)
	e, _ := newHighlightedBigEntry(t, 5000)
	e.HighlightRunsForRow(0)
	require.NotNil(t, e.hlPrefill, "prefill armed on fresh content")

	cache.Renderer(e).Destroy()
	assert.Nil(t, e.hlPrefill, "Destroy must stop the prefill driver")
}

// TestHighlight_PrefillReArmsOnReShow pins the Destroy-stop's mandatory twin
// (#421): eviction also hits live, temporarily-hidden widgets, and on re-show
// the buffer object is unchanged so no other arming seam fires — without the
// CreateRenderer re-arm the #417 fix regresses and the next deep jump stalls.
// The chain resumes from the watermark; a completed chain does not re-arm.
func TestHighlight_PrefillReArmsOnReShow(t *testing.T) {
	zeroPrefillBudget(t)
	e, calls := newHighlightedBigEntry(t, 5000)
	e.HighlightRunsForRow(0)
	for e.hlPrefill != nil && e.hlComputedThrough < 2000 {
		e.hlPrefillTick()
	}
	require.NotNil(t, e.hlPrefill, "prefill still running partway through")

	cache.Renderer(e).Destroy()
	require.Nil(t, e.hlPrefill)
	watermark := e.hlComputedThrough
	before := *calls

	e.CreateRenderer()
	require.NotNil(t, e.hlPrefill, "unfinished chain re-arms on re-show")
	drainHlPrefill(t, e)
	assert.Equal(t, e.text.buf.LineCount(), e.hlComputedThrough)
	assert.LessOrEqual(t, *calls-before, e.text.buf.LineCount()-watermark,
		"resumed chain highlights only the lines above the watermark")

	// A finished chain must not re-arm on the next re-show.
	cache.Renderer(e).Destroy()
	e.CreateRenderer()
	assert.Nil(t, e.hlPrefill, "no re-arm when the chain is already complete")
}

// TestHighlight_NewlineEditKeepsAlignment pins the hlCache splice performed by
// noteLazyLinesEdited: inserting and deleting newlines keeps cache entries
// aligned with their logical lines (no off-by-one stale runs).
func TestHighlight_NewlineEditKeepsAlignment(t *testing.T) {
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapOff
	e.LineHighlighter = func(line string, state int) ([]StyleRun, int) {
		return []StyleRun{{Text: line}}, state
	}
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 300))
	e.SetText("alpha\nbravo\ncharlie\ndelta")
	e.textProvider().updateRowBounds()
	for row := 0; row < 4; row++ {
		e.HighlightRunsForRow(row)
	}

	// Split "bravo" -> "bra" / "vo": every line's cached text must re-align.
	e.CursorRow, e.CursorColumn = 1, 3
	e.TypedKey(&event.KeyEvent{Name: event.KeyEnter})
	for row := 0; row < 5; row++ {
		e.HighlightRunsForRow(row)
	}
	require.Equal(t, 5, len(e.hlCache))
	for i, want := range []string{"alpha", "bra", "vo", "charlie", "delta"} {
		assert.Equalf(t, want, e.hlCache[i].text, "line %d cache text after split", i)
	}

	// Merge back with backspace at the split point.
	e.CursorRow, e.CursorColumn = 2, 0
	e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	for row := 0; row < 4; row++ {
		e.HighlightRunsForRow(row)
	}
	require.Equal(t, 4, len(e.hlCache))
	for i, want := range []string{"alpha", "bravo", "charlie", "delta"} {
		assert.Equalf(t, want, e.hlCache[i].text, "line %d cache text after merge", i)
	}
}

// TestHighlight_DeepJumpChainIsCapped is the #167 guard: a render request for a
// row far below the highlight watermark — a collapsed fold's footer jumping into
// view, or a deep scroll before the prefill has caught up — must not chain the
// whole prefix synchronously. Auto-folding a 437k-row @dataset exposed its footer
// row and hitched ~1.15s highlighting every intervening line in one blocking pass.
// The synchronous chain is now bounded to maxSyncHighlightChain and the background
// prefill converges the remainder.
func TestHighlight_DeepJumpChainIsCapped(t *testing.T) {
	e, _ := newHighlightedBigEntry(t, maxSyncHighlightChain*3)

	// A fresh document leaves the watermark at the initial visible band. Request a
	// row far below it in one shot, as a collapsed fold's footer would.
	deep := maxSyncHighlightChain*3 - 5
	before := e.hlWalkSteps
	e.HighlightRunsForRow(deep)
	walked := e.hlWalkSteps - before
	assert.LessOrEqual(t, walked, maxSyncHighlightChain+1,
		"a deep render request chained %d lines synchronously — must cap at %d (#167)",
		walked, maxSyncHighlightChain)

	// The background prefill still converges the whole document, so the deep row
	// picks up its highlight once the watermark reaches it.
	drainHlPrefill(t, e)
	assert.Equal(t, e.text.buf.LineCount(), e.hlComputedThrough,
		"prefill converges the whole document after a capped deep jump")
	require.NotNil(t, e.HighlightRunsForRow(deep),
		"the deep row is highlighted once the prefill has converged")
}

// TestHighlight_CappedDeepRowIsNotStale guards the #167 cap's correctness
// contract: a deep row above the (capped) watermark must render *unstyled*, never
// with stale runs left in the cache from a prior pass. The synchronous chain is
// bounded before it can reach and revalidate a downstream trusted stretch (#412),
// so an upstream state-changing edit that invalidates that stretch's entering
// state must not leak its now-wrong cached runs to the renderer — the row reports
// nil until the background prefill re-chains it. Encoded via Bold = entering state
// "open", so a stale (state-0, not-bold) run is distinguishable from the correct
// (state-1, bold) one.
func TestHighlight_CappedDeepRowIsNotStale(t *testing.T) {
	zeroPrefillBudget(t)
	test.NewTempApp(t)

	e := NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapOff
	e.LineHighlighter = func(line string, state int) ([]StyleRun, int) {
		out := state
		if strings.Contains(line, "OPEN") {
			out = 1
		}
		return []StyleRun{{Text: line, Bold: state == 1}}, out
	}
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 300))
	e.SetText(strings.Repeat("plain code line\n", maxSyncHighlightChain*3))
	e.textProvider().updateRowBounds()

	// Fully chain the document so every downstream row holds a cached (not-bold)
	// run from the all-closed state.
	e.HighlightRunsForRow(0)
	drainHlPrefill(t, e)
	deep := maxSyncHighlightChain*3 - 5
	require.NotNil(t, e.HighlightRunsForRow(deep))
	require.False(t, e.HighlightRunsForRow(deep)[0].Bold, "precondition: deep row starts unstyled/closed")

	// An in-place edit at line 0 opens the state for the whole rest of the
	// document, lowering the watermark and stranding the downstream cache as a
	// trusted stretch whose entering state is now wrong.
	e.CursorRow, e.CursorColumn = 0, 0
	for _, r := range "OPEN " {
		e.TypedRune(r)
	}
	require.Less(t, e.hlComputedThrough, deep, "edit lowered the watermark below the deep row")

	// The deep row is now above the capped watermark; a render request must report
	// unstyled rather than the stale not-bold run the cache still holds.
	assert.Nil(t, e.HighlightRunsForRow(deep),
		"a deep row above the capped watermark must render unstyled, not serve stale cached runs (#167)")

	// Once prefill re-chains the document the row picks up its correct, bold run.
	drainHlPrefill(t, e)
	runs := e.HighlightRunsForRow(deep)
	require.NotNil(t, runs, "deep row highlighted after prefill converges")
	assert.True(t, runs[0].Bold, "deep row reflects the opened state once re-chained")
}

// TestHighlight_PrefillRepaintsWhenCrossingViewport is the #693 refinement: the
// background prefill advances only highlight *state*, so a deep visible row above
// the watermark (e.g. a collapsed fold's footer) stays unstyled until a repaint.
// The prefill must request that repaint itself as its watermark climbs across the
// viewport, rather than relying on the animation's frame cadence or an unrelated
// later refresh.
func TestHighlight_PrefillRepaintsWhenCrossingViewport(t *testing.T) {
	zeroPrefillBudget(t)
	e, _ := newHighlightedBigEntry(t, 5000)

	// Place the viewport deep in the document, well above the top-of-file band.
	rowH := e.textProvider().charMinSize(false, e.TextStyle, e.Theme().Size(theme.SizeNameText)).Height
	require.Positive(t, rowH)
	e.textProvider().SetVisibleRange(rowH*2000, 300)
	vs, ve := e.textProvider().visibleRowRange()
	require.Greater(t, vs, 100, "viewport is deep in the document")

	// Restart the chain at the top and drive the prefill up to the viewport.
	e.stopHlPrefill()
	e.hlComputedThrough = 0
	e.hlPrefillRepaints = 0
	e.startHlPrefill()
	for i := 0; e.hlPrefill != nil && e.hlComputedThrough <= ve; i++ {
		require.Less(t, i, 10000, "prefill did not reach the viewport")
		e.hlPrefillTick()
	}
	assert.Positive(t, e.hlPrefillRepaints,
		"prefill must repaint the text as its watermark crosses the viewport (rows %d..%d)", vs, ve)
}

// TestHighlight_PrefillNoRepaintBelowViewport guards that the repaint is bounded:
// while the watermark climbs far below the viewport it requests no repaints (the
// newly-highlighted band does not overlap the visible rows).
func TestHighlight_PrefillNoRepaintBelowViewport(t *testing.T) {
	zeroPrefillBudget(t)
	e, _ := newHighlightedBigEntry(t, 5000)

	rowH := e.textProvider().charMinSize(false, e.TextStyle, e.Theme().Size(theme.SizeNameText)).Height
	e.textProvider().SetVisibleRange(rowH*4000, 300) // viewport near the very bottom
	vs, _ := e.textProvider().visibleRowRange()
	require.Greater(t, vs, 3000)

	e.stopHlPrefill()
	e.hlComputedThrough = 0
	e.hlPrefillRepaints = 0
	e.startHlPrefill()
	// Advance a handful of slices, all well below the viewport.
	for i := 0; i < 5 && e.hlPrefill != nil; i++ {
		e.hlPrefillTick()
	}
	require.Less(t, e.hlComputedThrough, vs, "watermark still below the viewport")
	assert.Zero(t, e.hlPrefillRepaints, "no repaint while the watermark is below the viewport")
}
