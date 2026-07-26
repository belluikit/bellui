package widget

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// newBigDocEntry builds a shown multi-line entry over a large single-segment
// document with the lazy row model active, mirroring BenchmarkEntryTypeBigDoc.
func newBigDocEntry(t *testing.T, lines int, wrap bellui.TextWrap) *Entry {
	t.Helper()
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	e.Wrapping = wrap
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 300))
	e.SetText(strings.Repeat("the quick brown fox jumps over the lazy dog\n", lines))
	e.textProvider().updateRowBounds()
	return e
}

// TestLazyEdit_NoFullRebuildPerKeystroke pins the #380/#400 identity chain: after
// the initial build, interactive edits (typing, backspace, delete, newline) go
// through the incremental fast path and never trigger a full content rebuild of
// the lazy model. A regression here is a silent perf cliff, not a correctness
// failure — this test is the only guard.
func TestLazyEdit_NoFullRebuildPerKeystroke(t *testing.T) {
	e := newBigDocEntry(t, 5000, bellui.TextWrapOff)
	p := e.textProvider()
	require.True(t, p.lazyCanEdit(), "fast edit path must be active")

	base := p.lazyFullBuilds

	e.CursorRow, e.CursorColumn = 0, 0
	for i := 0; i < 20; i++ {
		e.TypedRune('x')
	}
	e.TypedKey(&event.KeyEvent{Name: event.KeyEnter})
	for i := 0; i < 5; i++ {
		e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	}
	e.TypedKey(&event.KeyEvent{Name: event.KeyDelete})

	assert.Equal(t, base, p.lazyFullBuilds,
		"interactive edits must stay on the incremental path (no full rebuilds)")
	assert.True(t, p.lazyCanEdit(), "fast edit path must survive the edits")
}

// TestLazyEdit_UndoRedoStaysIncremental pins the #401 delta-native undo replay:
// undo and redo splice the provider through the fast path, so they trigger no
// full rebuild of the lazy model (each used to force one per application), and
// still reproduce the exact text and caret of the string-replay implementation.
func TestLazyEdit_UndoRedoStaysIncremental(t *testing.T) {
	e := newBigDocEntry(t, 5000, bellui.TextWrapOff)
	p := e.textProvider()
	require.True(t, p.lazyCanEdit())

	before := e.Text
	e.CursorRow, e.CursorColumn = 2, 3
	for _, r := range "hello" {
		e.TypedRune(r)
	}
	e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	typed := e.Text

	base := p.lazyFullBuilds
	e.Undo() // backspace undone
	e.Undo() // "hello" undone (merged into one action)
	assert.Equal(t, before, e.Text, "undo restores the original text")
	e.Redo()
	e.Redo()
	assert.Equal(t, typed, e.Text, "redo replays both edits")
	assert.Equal(t, base, p.lazyFullBuilds, "undo/redo must stay on the incremental path")
	assert.True(t, p.lazyCanEdit(), "fast edit path must survive undo/redo")
}

// TestLazyEdit_ComposeStaysIncremental pins the #402 preedit-through-the-buffer
// path: IME composition updates, commits and cancels splice the provider's
// buffer, so composing on a large document triggers no full rebuild per event
// (each used to force one), while Text/undo stay untouched until commit.
func TestLazyEdit_ComposeStaysIncremental(t *testing.T) {
	e := newBigDocEntry(t, 5000, bellui.TextWrapOff)
	p := e.textProvider()
	require.True(t, p.lazyCanEdit())
	before := e.Text

	e.CursorRow, e.CursorColumn = 3, 2
	base := p.lazyFullBuilds

	e.Compose(&event.PreeditEvent{Text: "に", CursorPos: 1})
	assert.Equal(t, before, e.Text, "preedit must not touch Text")
	e.Compose(&event.PreeditEvent{Text: "にほ", CursorPos: 2})
	e.Compose(&event.PreeditEvent{Text: "にほん", CursorPos: 3})
	assert.Equal(t, base, p.lazyFullBuilds, "compose updates must stay incremental")

	e.ComposeCommit("日本")
	assert.Equal(t, base, p.lazyFullBuilds, "commit must stay incremental")
	assert.NotEqual(t, before, e.Text, "committed text lands in Text")
	assert.NotContains(t, e.Text, "にほん", "preedit glyphs must not leak into Text")

	e.Undo()
	assert.Equal(t, before, e.Text, "commit is a single undoable edit")
	assert.Equal(t, base, p.lazyFullBuilds, "undo of a commit stays incremental")

	// Cancel path: compose then cancel restores the original text with no rebuild.
	e.Compose(&event.PreeditEvent{Text: "あ", CursorPos: 1})
	e.ComposeCancel()
	assert.Equal(t, before, e.Text)
	assert.Equal(t, before, p.String(), "provider restored after cancel")
	assert.Equal(t, base, p.lazyFullBuilds, "cancel stays incremental")
	assert.True(t, p.lazyCanEdit(), "fast path survives the composition cycle")
}

// TestLazyEdit_SetTextRebuildsOnce guards the other direction: a genuine content
// replacement rebuilds the model exactly once (no double build via SyncState).
func TestLazyEdit_SetTextRebuildsOnce(t *testing.T) {
	e := newBigDocEntry(t, 1000, bellui.TextWrapOff)
	p := e.textProvider()

	base := p.lazyFullBuilds
	e.SetText("completely new\ncontent")
	p.updateRowBounds()
	assert.Equal(t, base+1, p.lazyFullBuilds, "SetText must rebuild the buffer exactly once")
}

// TestLazyEdit_RowBoundsMatchRebuild is the line-relative lineWrap oracle: after a
// random edit sequence on a wrapped document, every visual row's absolute
// boundary equals what a from-scratch rebuild computes. This pins the
// line-relative -> absolute conversion in lazyRowBoundary and the retained
// measurements of lines outside the edited span (#400).
func TestLazyEdit_RowBoundsMatchRebuild(t *testing.T) {
	e := newBigDocEntry(t, 300, bellui.TextWrapWord) // below eager threshold: all lines measured
	p := e.textProvider()
	require.True(t, p.lazyCanEdit())

	rng := rand.New(rand.NewSource(42))
	inserts := []string{"x", "hello world", "\n", "a\nb", "é界🙂", " "}
	for step := 0; step < 60; step++ {
		n := p.len()
		if rng.Intn(3) == 0 && n > 0 { // delete
			low := rng.Intn(n)
			high := low + rng.Intn(min(n-low, 40)+1)
			p.deleteFromTo(low, high)
		} else { // insert
			pos := rng.Intn(n + 1)
			p.insertAt(pos, []rune(inserts[rng.Intn(len(inserts))]))
		}
		e.updateText(p.String(), false)
	}

	// Snapshot the incrementally maintained row boundaries.
	rows := p.rows()
	type bounds struct{ begin, end int }
	got := make([]bounds, 0, rows)
	for r := 0; r < rows; r++ {
		b := p.rowBoundary(r)
		require.NotNilf(t, b, "row %d", r)
		got = append(got, bounds{b.begin, b.end})
	}

	// Force a from-scratch rebuild and compare.
	p.lazyStr = "\x00invalid"
	p.rowBoundsVersion++
	p.updateRowBounds()
	require.Equal(t, rows, p.rows(), "row count after rebuild")
	for r := 0; r < rows; r++ {
		b := p.rowBoundary(r)
		require.NotNilf(t, b, "rebuilt row %d", r)
		assert.Equalf(t, bounds{b.begin, b.end}, got[r], "row %d bounds: incremental vs rebuild", r)
	}
}
