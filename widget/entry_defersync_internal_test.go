package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/data/binding"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// newDeferSyncEntry builds a shown multi-line entry with DeferTextSync on and
// the lazy single-segment fast path active, ready for deferred-edit assertions.
func newDeferSyncEntry(t *testing.T, text string) (*Entry, bellui.Window) {
	t.Helper()
	e := NewMultiLineEntry()
	e.DeferTextSync = true
	e.SetText(text)
	w := test.NewWindow(e)
	w.Resize(geo.NewSize(400, 300))
	p := e.textProvider()
	p.updateRowBounds()
	require.True(t, p.lazyCanEdit(), "expected the lazy fast path to be active")
	require.True(t, p.deferText, "expected DeferTextSync pushed to the provider")
	return e, w
}

// TestEntryDeferTextSync_TypingDefersAndSyncs pins the core pre-rebrand tracker #410 contract:
// with the mode on, typing leaves Text/OnChanged/Validator untouched (the
// buffer and rendering carry the live content) and SyncText materializes,
// notifies once, and restores the identity chain for further fast edits.
func TestEntryDeferTextSync_TypingDefersAndSyncs(t *testing.T) {
	e, w := newDeferSyncEntry(t, "hello\nworld")
	defer w.Close()

	changed := 0
	validated := 0
	var lastChanged string
	e.OnChanged = func(s string) { changed++; lastChanged = s }
	e.Validator = func(string) error { validated++; return nil }
	// The first render after a Validator is set runs a one-time setup
	// validation (ensureValidationSetup); flush it so the assertions below
	// measure per-edit behavior only.
	e.Refresh()
	validated = 0

	p := e.textProvider()
	builds := p.lazyFullBuilds

	e.CursorRow, e.CursorColumn = 0, 5
	e.TypedRune('!')
	e.TypedRune('?')

	// Text and the notification chain are untouched between sync points…
	assert.Equal(t, "hello\nworld", e.Text)
	assert.Zero(t, changed)
	assert.Zero(t, validated)
	assert.True(t, p.deferredDirty)
	// …while the buffer, rendering and selection sources hold the live content.
	assert.Equal(t, "hello!?\nworld", p.buf.String())
	assert.Equal(t, "hello!?", string(p.row(0)))
	assert.Equal(t, "hello!?", p.textRange(0, 7))

	got := e.SyncText()
	assert.Equal(t, "hello!?\nworld", got)
	assert.Equal(t, "hello!?\nworld", e.Text)
	assert.Equal(t, 1, changed)
	assert.Equal(t, "hello!?\nworld", lastChanged)
	assert.Equal(t, 1, validated)
	assert.False(t, p.deferredDirty)

	// The identity chain is re-anchored: more typing stays on the fast path.
	seg := p.Segments[0].(*TextSegment)
	assert.Equal(t, e.Text, seg.Text)
	assert.True(t, p.lazyCanEdit())
	e.TypedRune('x')
	assert.Equal(t, builds, p.lazyFullBuilds, "deferred edits must not degrade to full rebuilds")

	// An idle SyncText is a cheap no-op with no duplicate notification.
	e.SyncText()
	got = e.SyncText()
	assert.Equal(t, "hello!?x\nworld", got)
	assert.Equal(t, 2, changed)
}

// TestEntryDeferTextSync_FocusLostSyncs pins focus loss as a sync point.
func TestEntryDeferTextSync_FocusLostSyncs(t *testing.T) {
	e, w := newDeferSyncEntry(t, "abc")
	defer w.Close()

	changed := 0
	e.OnChanged = func(string) { changed++ }
	e.CursorRow, e.CursorColumn = 0, 3
	e.TypedRune('d')
	assert.Equal(t, "abc", e.Text)
	assert.Zero(t, changed)

	e.FocusLost()
	assert.Equal(t, "abcd", e.Text)
	assert.Equal(t, 1, changed)
	assert.False(t, e.text.deferredDirty)
}

// TestEntryDeferTextSync_SubmitSyncs pins submission as a sync point: the
// pending OnChanged runs first and OnSubmitted sees the current text.
func TestEntryDeferTextSync_SubmitSyncs(t *testing.T) {
	e, w := newDeferSyncEntry(t, "abc")
	defer w.Close()

	var order []string
	e.OnChanged = func(s string) { order = append(order, "changed:"+s) }
	var submitted string
	e.OnSubmitted = func(s string) { submitted = s; order = append(order, "submitted") }

	e.CursorRow, e.CursorColumn = 0, 3
	e.TypedRune('d')
	e.selectKeyDown = true // Shift+Return submits a multi-line entry
	e.typedKeyReturn(e.textProvider(), true)

	assert.Equal(t, "abcd", submitted)
	assert.Equal(t, "abcd", e.Text)
	assert.Equal(t, []string{"changed:abcd", "submitted"}, order)
}

// TestEntryDeferTextSync_UndoRedo pins that history replay defers like any
// other edit and syncs correctly afterwards.
func TestEntryDeferTextSync_UndoRedo(t *testing.T) {
	e, w := newDeferSyncEntry(t, "base")
	defer w.Close()

	changed := 0
	e.OnChanged = func(string) { changed++ }

	e.CursorRow, e.CursorColumn = 0, 4
	e.TypedRune('X')
	assert.Equal(t, "baseX", e.SyncText())
	assert.Equal(t, 1, changed)

	e.Undo()
	assert.Equal(t, "baseX", e.Text, "undo must defer the Text sync")
	assert.Equal(t, 1, changed)
	assert.Equal(t, "base", e.textProvider().buf.String())
	assert.Equal(t, "base", e.SyncText())
	assert.Equal(t, 2, changed)

	e.Redo()
	assert.Equal(t, "baseX", e.SyncText())
	assert.Equal(t, 3, changed)
}

// TestEntryDeferTextSync_SelectionEraseAndCut drives the selection-delete and
// cut paths and checks both defer.
func TestEntryDeferTextSync_SelectionEraseAndCut(t *testing.T) {
	e, w := newDeferSyncEntry(t, "hello world")
	defer w.Close()

	changed := 0
	e.OnChanged = func(string) { changed++ }

	e.Select(0, 0, 0, 6) // "hello "
	e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	assert.Equal(t, "hello world", e.Text)
	assert.Zero(t, changed)
	assert.Equal(t, "world", e.textProvider().buf.String())

	e.Select(0, 0, 0, 2) // "wo"
	cb := bellui.CurrentApp().Clipboard()
	e.cutToClipboard(cb)
	assert.Equal(t, "wo", cb.Content())
	assert.Equal(t, "hello world", e.Text)
	assert.Zero(t, changed)
	assert.Equal(t, "rld", e.textProvider().buf.String())

	assert.Equal(t, "rld", e.SyncText())
	assert.Equal(t, 1, changed)
}

// TestEntryDeferTextSync_PasteAndDeleteWord drives the paste and word-delete
// paths and checks both defer.
func TestEntryDeferTextSync_PasteAndDeleteWord(t *testing.T) {
	e, w := newDeferSyncEntry(t, "one two")
	defer w.Close()

	changed := 0
	e.OnChanged = func(string) { changed++ }

	cb := bellui.CurrentApp().Clipboard()
	cb.SetContent("pasted ")
	e.CursorRow, e.CursorColumn = 0, 0
	e.pasteFromClipboard(cb)
	assert.Equal(t, "one two", e.Text)
	assert.Zero(t, changed)
	assert.Equal(t, "pasted one two", e.textProvider().buf.String())

	e.CursorRow, e.CursorColumn = 0, 0
	e.deleteWord(true)
	assert.Equal(t, "one two", e.Text)
	assert.Zero(t, changed)
	assert.Equal(t, " one two", e.textProvider().buf.String())

	assert.Equal(t, " one two", e.SyncText())
	assert.Equal(t, 1, changed)
}

// TestEntryDeferTextSync_Binding pins that a bound value is written at sync
// points only, not per keystroke.
func TestEntryDeferTextSync_Binding(t *testing.T) {
	e, w := newDeferSyncEntry(t, "ignored")
	defer w.Close()

	data := binding.NewString()
	require.NoError(t, data.Set("bound"))
	e.Bind(data)
	waitForBinding()
	require.Equal(t, "bound", e.Text)
	p := e.textProvider()
	p.updateRowBounds()
	require.True(t, p.lazyCanEdit())

	e.CursorRow, e.CursorColumn = 0, 5
	e.TypedRune('!')
	waitForBinding()
	v, err := data.Get()
	require.NoError(t, err)
	assert.Equal(t, "bound", v, "binding must not be written per keystroke")

	e.SyncText()
	waitForBinding()
	v, err = data.Get()
	require.NoError(t, err)
	assert.Equal(t, "bound!", v)
}

// TestEntryDeferTextSync_ComposeSyncExcludesPreedit pins that a sync during a
// live IME composition reports the committed text only, and that the preedit
// keeps rendering and later commits correctly.
func TestEntryDeferTextSync_ComposeSyncExcludesPreedit(t *testing.T) {
	e, w := newDeferSyncEntry(t, "ab")
	defer w.Close()

	e.CursorRow, e.CursorColumn = 0, 2
	e.TypedRune('c')

	e.Compose(&event.PreeditEvent{Text: "にほ", CursorPos: 2})
	require.Positive(t, e.preeditApplied, "expected the preedit spliced into the buffer")
	assert.Equal(t, "abcにほ", e.textProvider().buf.String())

	got := e.SyncText()
	assert.Equal(t, "abc", got, "sync must exclude in-progress preedit")
	assert.Equal(t, "abc", e.Text)
	assert.True(t, e.text.deferredDirty, "identity stays deferred while composing")

	e.ComposeCommit("日本")
	assert.Equal(t, "abc日本", e.SyncText())
	assert.False(t, e.composing)
}

// TestEntryDeferTextSync_WrappedTyping exercises the deferred path with word
// wrapping on (the lazy wrapped row model) as a smoke test.
func TestEntryDeferTextSync_WrappedTyping(t *testing.T) {
	e := NewMultiLineEntry()
	e.DeferTextSync = true
	e.Wrapping = bellui.TextWrapWord
	e.SetText("some words that wrap around the narrow view\nsecond line")
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(120, 200))
	p := e.textProvider()
	p.updateRowBounds()
	require.True(t, p.lazyCanEdit())

	e.CursorRow, e.CursorColumn = 0, 4
	e.TypedRune('X')
	assert.Equal(t, "some words that wrap around the narrow view\nsecond line", e.Text)
	assert.Equal(t, "someX words that wrap around the narrow view\nsecond line", e.SyncText())
}

// TestEntryDeferTextSync_PasteEmptyClipboardDefers pins the pre-rebrand tracker #427 fix's
// deferred branch: an empty-clipboard paste over a selection erases through
// the buffer and defers Text/OnChanged to the next sync point.
func TestEntryDeferTextSync_PasteEmptyClipboardDefers(t *testing.T) {
	e, w := newDeferSyncEntry(t, "hello world")
	defer w.Close()

	changed := 0
	e.OnChanged = func(string) { changed++ }

	e.Select(0, 0, 0, 6) // "hello "
	cb := bellui.CurrentApp().Clipboard()
	cb.SetContent("")
	e.pasteFromClipboard(cb)

	assert.Equal(t, "hello world", e.Text)
	assert.Zero(t, changed)
	assert.Equal(t, "world", e.textProvider().buf.String())

	assert.Equal(t, "world", e.SyncText())
	assert.Equal(t, 1, changed)

	e.Undo()
	assert.Equal(t, "hello world", e.SyncText())
}

// TestEntryDeferTextSync_ExternalTextWriteWins documents the staleness
// contract: writing Text directly (then refreshing) while edits are pending
// discards the deferred edits — the external write wins.
func TestEntryDeferTextSync_ExternalTextWriteWins(t *testing.T) {
	e, w := newDeferSyncEntry(t, "abc")
	defer w.Close()

	e.TypedRune('d')
	assert.True(t, e.text.deferredDirty)

	e.SetText("replaced")
	assert.Equal(t, "replaced", e.Text)
	assert.False(t, e.text.deferredDirty)
	assert.Equal(t, "replaced", e.textProvider().buf.String())
	assert.Equal(t, "replaced", e.SyncText())
}
