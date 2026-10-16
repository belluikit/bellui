package editor

import (
	"strings"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// newDeferredEditor returns a shown editor with DeferTextSync on, laid out so
// the fork's lazy fast path (and with it the deferred edit path) is active.
func newDeferredEditor(t *testing.T, text string) *Editor {
	t.Helper()
	e := NewEditor()
	e.DeferTextSync = true
	e.SetText(text)
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 300))
	return e
}

// TestDeferSync_TypingDefersOnChanged pins the core contract: typing within a
// row leaves Entry.Text and OnChanged untouched, and Editor.Text() is a sync
// point that materializes and notifies once with the full current text.
func TestDeferSync_TypingDefersOnChanged(t *testing.T) {
	test.NewTempApp(t)
	e := newDeferredEditor(t, "hello\nworld")

	changed := 0
	var last string
	e.OnChanged = func(s string) { changed++; last = s }

	e.SetCursorPos(0, 5)
	e.entry.TypedRune('!')
	e.entry.TypedRune('?')

	if e.entry.Text != "hello\nworld" {
		t.Fatalf("expected deferred edits to leave Entry.Text stale, got %q", e.entry.Text)
	}
	if changed != 0 {
		t.Fatalf("OnChanged fired %d times between sync points, want 0", changed)
	}

	if got := e.Text(); got != "hello!?\nworld" {
		t.Fatalf("Text() after sync = %q, want %q", got, "hello!?\nworld")
	}
	if changed != 1 || last != "hello!?\nworld" {
		t.Errorf("sync point: changed=%d last=%q, want one call with the full text", changed, last)
	}
	// A second read is a no-op sync: no further notifications.
	_ = e.Text()
	if changed != 1 {
		t.Errorf("idle Text() re-fired OnChanged (%d calls)", changed)
	}
}

// TestDeferSync_EnterSyncsLineCount: a newline insert is a sync point, so the
// gutter's raw-read line count is exact immediately after Enter.
func TestDeferSync_EnterSyncsLineCount(t *testing.T) {
	test.NewTempApp(t)
	e := newDeferredEditor(t, "aaa\nbbb")

	e.SetCursorPos(0, 3)
	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyReturn})

	if got := e.lineCount(); got != 3 {
		t.Errorf("lineCount after Enter = %d, want 3", got)
	}
	if e.entry.Text != "aaa\n\nbbb" {
		t.Errorf("Entry.Text after Enter = %q, want synced %q", e.entry.Text, "aaa\n\nbbb")
	}
}

// TestDeferSync_BackspaceMidRowStaysDeferred: a single-rune backspace strictly
// inside a line cannot change the line structure, so it does not sync.
func TestDeferSync_BackspaceMidRowStaysDeferred(t *testing.T) {
	test.NewTempApp(t)
	e := newDeferredEditor(t, "hello\nworld")

	changed := 0
	e.OnChanged = func(string) { changed++ }

	e.SetCursorPos(0, 5)
	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})

	if changed != 0 || e.entry.Text != "hello\nworld" {
		t.Fatalf("mid-row backspace synced (changed=%d, text=%q), want deferred", changed, e.entry.Text)
	}
	if got := e.Text(); got != "hell\nworld" {
		t.Errorf("Text() = %q, want %q", got, "hell\nworld")
	}
}

// TestDeferSync_BackspaceLineJoinSyncs: backspace at column 0 removes the
// joining newline, so it must sync to keep the line count exact.
func TestDeferSync_BackspaceLineJoinSyncs(t *testing.T) {
	test.NewTempApp(t)
	e := newDeferredEditor(t, "aaa\nbbb")

	e.SetCursorPos(1, 0)
	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})

	if e.entry.Text != "aaabbb" {
		t.Errorf("Entry.Text after line-join backspace = %q, want synced %q", e.entry.Text, "aaabbb")
	}
	if got := e.lineCount(); got != 1 {
		t.Errorf("lineCount = %d, want 1", got)
	}
}

// TestDeferSync_DeleteForwardAtEOLSyncs: deleting the newline at end of line
// joins two rows with the caret static at (0, 3) — the live line-count
// comparison (pre-rebrand tracker #149) catches it and syncs, even though neither the caret row
// nor any BeforeChange flag signals it.
func TestDeferSync_DeleteForwardAtEOLSyncs(t *testing.T) {
	test.NewTempApp(t)
	e := newDeferredEditor(t, "aaa\nbbb")

	e.SetCursorPos(0, 3)
	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyDelete})

	if e.entry.Text != "aaabbb" {
		t.Errorf("Entry.Text after delete-forward at EOL = %q, want synced %q", e.entry.Text, "aaabbb")
	}
	if got := e.lineCount(); got != 1 {
		t.Errorf("lineCount = %d, want 1", got)
	}
}

// TestDeferSync_DeleteForwardMidRowStaysDeferred (pre-rebrand tracker #149): a mid-row
// delete-forward removes no newline, so it stays deferred — before the live
// line-count comparison every deletion synced conservatively, making
// held-down Delete O(document) per repeat.
func TestDeferSync_DeleteForwardMidRowStaysDeferred(t *testing.T) {
	test.NewTempApp(t)
	e := newDeferredEditor(t, "hello\nworld")

	changed := 0
	e.OnChanged = func(string) { changed++ }

	e.SetCursorPos(0, 0)
	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyDelete})

	if changed != 0 || e.entry.Text != "hello\nworld" {
		t.Fatalf("mid-row delete-forward synced (changed=%d, text=%q), want deferred", changed, e.entry.Text)
	}
	if got := e.Text(); got != "ello\nworld" {
		t.Errorf("Text() = %q, want %q", got, "ello\nworld")
	}
}

// TestDeferSync_GutterLineCountLiveWhileDeferred (pre-rebrand tracker #149): the gutter's row
// count reads the live Entry.LineCount, so it is exact even while the Text
// field is stale between sync points.
func TestDeferSync_GutterLineCountLiveWhileDeferred(t *testing.T) {
	test.NewTempApp(t)
	e := newDeferredEditor(t, "hello")

	changed := 0
	e.OnChanged = func(string) { changed++ }

	// Force a deferred state with the completion/protection features off:
	// mid-row typing never syncs.
	e.SetCursorPos(0, 5)
	e.entry.TypedRune('!')
	if changed != 0 {
		t.Fatalf("typing synced immediately (changed=%d), want deferred", changed)
	}
	if e.gutterW == nil {
		t.Fatal("expected the default line-number gutter")
	}
	if got := e.gutterW.rowCount(); got != 1 {
		t.Errorf("gutter rowCount = %d, want 1 (live count)", got)
	}
}

// TestDeferSync_RowChangeSyncs: moving the caret off the edited row is a sync
// point, bounding the anchor-tracking diff to a single row.
func TestDeferSync_RowChangeSyncs(t *testing.T) {
	test.NewTempApp(t)
	e := newDeferredEditor(t, "aaa\nbbb")

	changed := 0
	e.OnChanged = func(string) { changed++ }

	e.SetCursorPos(0, 3)
	e.entry.TypedRune('x')
	if changed != 0 {
		t.Fatalf("typing synced immediately (changed=%d), want deferred", changed)
	}

	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	if changed != 1 || e.entry.Text != "aaax\nbbb" {
		t.Errorf("row change: changed=%d text=%q, want 1 sync with %q", changed, e.entry.Text, "aaax\nbbb")
	}
}

// TestDeferSync_UndoSyncs: undo replays history without passing BeforeChange;
// the shortcut dispatcher flags the sync instead.
func TestDeferSync_UndoSyncs(t *testing.T) {
	test.NewTempApp(t)
	e := newDeferredEditor(t, "hello")

	e.SetCursorPos(0, 5)
	e.entry.TypedRune('!')
	if got := e.Text(); got != "hello!" { // sync so Text has the rune to undo past
		t.Fatalf("Text() = %q, want %q", got, "hello!")
	}

	e.entry.TypedShortcut(&bellui.ShortcutUndo{})
	if e.entry.Text != "hello" {
		t.Errorf("Entry.Text after undo = %q, want synced %q", e.entry.Text, "hello")
	}

	e.entry.TypedShortcut(&bellui.ShortcutRedo{})
	if e.entry.Text != "hello!" {
		t.Errorf("Entry.Text after redo = %q, want synced %q", e.entry.Text, "hello!")
	}
}

// TestDeferSync_ProtectedRangesForcePerEditSync: with protected ranges active
// every edit is a sync point, so enforcement and anchor-tracking behave
// exactly as with the mode off.
func TestDeferSync_ProtectedRangesForcePerEditSync(t *testing.T) {
	test.NewTempApp(t)
	e := newDeferredEditor(t, "aaa\nSECRET\nbbb")
	e.SetProtectedRanges([]TextRange{{StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 6}})

	// An edit inside the range vetoes.
	e.SetCursorPos(1, 3)
	e.entry.TypedRune('x')
	if got := e.Text(); got != "aaa\nSECRET\nbbb" {
		t.Fatalf("protected edit applied under deferral, text = %q", got)
	}

	// An allowed edit syncs immediately (per-edit sync while ranges are set)…
	e.SetCursorPos(0, 3)
	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	if e.entry.Text != "aaa\n\nSECRET\nbbb" {
		t.Fatalf("Entry.Text = %q, want synced %q", e.entry.Text, "aaa\n\nSECRET\nbbb")
	}

	// …and the anchor-tracked range shifted with the inserted line: the moved
	// location enforces, the vacated one does not.
	want := TextRange{StartRow: 2, StartCol: 0, EndRow: 2, EndCol: 6}
	if got := e.ProtectedRanges(); len(got) != 1 || got[0] != want {
		t.Fatalf("ProtectedRanges after Enter above = %v, want [%v]", got, want)
	}
	e.SetCursorPos(2, 3)
	e.entry.TypedRune('x')
	if got := e.Text(); got != "aaa\n\nSECRET\nbbb" {
		t.Errorf("edit inside the shifted range should veto, text = %q", got)
	}
}

// TestDeferSync_CompletionProviderForcesPerEditSync: an installed completion
// provider needs current text on every keystroke, so each edit syncs.
func TestDeferSync_CompletionProviderForcesPerEditSync(t *testing.T) {
	test.NewTempApp(t)
	e := newDeferredEditor(t, "hello")
	e.SetCompletionProvider(&WordListCompletionProvider{Words: []string{"hexagon"}})

	changed := 0
	e.OnChanged = func(string) { changed++ }

	e.SetCursorPos(0, 5)
	e.entry.TypedRune('!')
	if changed != 1 || e.entry.Text != "hello!" {
		t.Errorf("with a completion provider: changed=%d text=%q, want per-edit sync", changed, e.entry.Text)
	}
}

// TestDeferSync_SetDeferTextSyncOffFlushes: disabling the mode at runtime
// flushes pending edits, then per-keystroke behavior resumes.
func TestDeferSync_SetDeferTextSyncOffFlushes(t *testing.T) {
	test.NewTempApp(t)
	e := newDeferredEditor(t, "hello")

	changed := 0
	e.OnChanged = func(string) { changed++ }

	e.SetCursorPos(0, 5)
	e.entry.TypedRune('!')
	if changed != 0 {
		t.Fatalf("typing synced immediately (changed=%d), want deferred", changed)
	}

	e.SetDeferTextSync(false)
	if changed != 1 || e.entry.Text != "hello!" {
		t.Fatalf("disable: changed=%d text=%q, want flushed %q", changed, e.entry.Text, "hello!")
	}

	e.entry.TypedRune('?')
	if changed != 2 || e.entry.Text != "hello!?" {
		t.Errorf("after disable: changed=%d text=%q, want per-keystroke sync", changed, e.entry.Text)
	}
}

// TestDeferSync_OffByDefault: a fresh editor keeps the historical
// per-keystroke OnChanged contract.
func TestDeferSync_OffByDefault(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	if e.DeferTextSync {
		t.Fatal("DeferTextSync must default off")
	}
	e.SetText("hello")
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 300))

	changed := 0
	e.OnChanged = func(string) { changed++ }
	e.SetCursorPos(0, 5)
	e.entry.TypedRune('!')
	e.entry.TypedRune('?')
	if changed != 2 || e.entry.Text != "hello!?" {
		t.Errorf("default mode: changed=%d text=%q, want per-keystroke sync", changed, e.entry.Text)
	}
}

// benchmarkKeystroke measures the full editor per-keystroke path (entry edit +
// OnChanged pipeline when it fires) on a ~1 MB, 50k-line document — the pre-rebrand tracker #129
// latency check comparing deferred and default modes.
func benchmarkKeystroke(b *testing.B, deferSync bool) {
	test.NewTempApp(b)
	e := NewEditor()
	e.DeferTextSync = deferSync
	e.SetText(strings.Repeat("0123456789abcdef\n", 50_000))
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(800, 600))
	e.SetCursorPos(100, 0)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.entry.TypedRune('x')
	}
}

func BenchmarkKeystroke_Deferred(b *testing.B) { benchmarkKeystroke(b, true) }
func BenchmarkKeystroke_Default(b *testing.B)  { benchmarkKeystroke(b, false) }

// benchmarkBackspace measures held-down Backspace through normal-length rows
// (line joins included) — the pre-rebrand tracker #149 case: with the live line-count comparison
// replacing the conservative any-deletion sync, only the presses that
// actually remove a newline materialize; the mid-row majority stays O(edit).
// The default mode materializes the whole document on every press.
func benchmarkBackspace(b *testing.B, deferSync bool) {
	test.NewTempApp(b)
	e := NewEditor()
	e.DeferTextSync = deferSync
	e.SetText(strings.Repeat("0123456789abcdef\n", 50_000))
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(800, 600))
	e.SetCursorPos(25_000, 16)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.entry.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	}
}

func BenchmarkBackspace_Deferred(b *testing.B) { benchmarkBackspace(b, true) }
func BenchmarkBackspace_Default(b *testing.B)  { benchmarkBackspace(b, false) }
