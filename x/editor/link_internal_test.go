package editor

import (
	"testing"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// shortcutModKey returns the platform's link-modifier key name (Cmd on macOS,
// Ctrl elsewhere) so tests drive the same modifier the editor tracks.
func shortcutModKey() event.KeyName {
	if event.KeyModifierShortcutDefault == event.KeyModifierSuper {
		return desktop.KeySuperLeft
	}
	return desktop.KeyControlLeft
}

func clickAt(pos geo.Position, mod event.KeyModifier) *desktop.MouseEvent {
	return &desktop.MouseEvent{
		PointEvent: event.PointEvent{Position: pos},
		Button:     desktop.MouseButtonPrimary,
		Modifier:   mod,
	}
}

// TestNavigation_CmdClickFiresHook verifies a Cmd/Ctrl+click reports the clicked
// (row, col), and a plain click does not.
func TestNavigation_CmdClickFiresHook(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz\nqux")

	gotRow, gotCol, calls := -1, -1, 0
	e.OnRequestNavigation(func(row, col int) {
		gotRow, gotCol, calls = row, col, calls+1
	})

	target := cellCenter(e, 0, 3)

	// Plain click: no navigation.
	e.entry.onMouseDown(clickAt(target, 0))
	if calls != 0 {
		t.Fatalf("a plain click should not navigate, got %d calls", calls)
	}

	// Cmd/Ctrl+click: fires with the clicked position.
	e.entry.onMouseDown(clickAt(target, event.KeyModifierShortcutDefault))
	if calls != 1 || gotRow != 0 || gotCol != 3 {
		t.Fatalf("cmd+click → calls=%d pos=(%d,%d), want 1 (0,3)", calls, gotRow, gotCol)
	}
}

func TestNavigation_NoHookPlainBehavior(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar")
	// No OnRequestNavigation set: a cmd+click must not panic and does nothing.
	e.entry.onMouseDown(clickAt(cellCenter(e, 0, 2), event.KeyModifierShortcutDefault))
}

// TestLink_CmdHoverShowsUnderlineAndPointer verifies a navigable symbol under the
// pointer, with the modifier held, lights a solid-underline link + pointer cursor.
func TestLink_CmdHoverShowsUnderlineAndPointer(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetLinkProvider(&WordLinkProvider{
		Navigable: map[string]bool{"foobar": true},
		Text:      func() string { return e.Text() },
	})

	// Hover over "foobar" without the modifier → no link.
	e.link.onMouseMoved(cellCenter(e, 0, 2))
	if e.link.active() {
		t.Fatal("no link should light without the modifier held")
	}
	if e.hoverCap.Cursor() != desktop.TextCursor {
		t.Error("cursor should be the text I-beam without a link")
	}

	// Press the modifier while hovering → link lights without moving the mouse.
	e.entry.onKeyDown(&event.KeyEvent{Name: shortcutModKey()})
	if !e.link.active() {
		t.Fatal("link should light on Cmd-hover over a navigable symbol")
	}
	if got := e.linkDeco.SpansList; len(got) != 1 ||
		got[0].Kind != SpanUnderline || got[0].UnderlineStyle != UnderlineSolid {
		t.Errorf("link should be a solid underline, got %v", got)
	}
	if got := e.linkDeco.SpansList[0].Range; got != (TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 6}) {
		t.Errorf("link range should cover 'foobar', got %v", got)
	}
	if e.hoverCap.Cursor() != desktop.PointerCursor {
		t.Error("cursor should be a pointer over a lit link")
	}

	// Release the modifier → link clears.
	e.entry.onKeyUp(&event.KeyEvent{Name: shortcutModKey()})
	if e.link.active() || e.linkDeco.SpansList != nil {
		t.Error("releasing the modifier should clear the link")
	}
}

func TestLink_NonNavigableSymbolNoLink(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetLinkProvider(&WordLinkProvider{
		Navigable: map[string]bool{"foobar": true},
		Text:      func() string { return e.Text() },
	})
	e.entry.onKeyDown(&event.KeyEvent{Name: shortcutModKey()})

	// "baz" (cols 7..10) is not navigable.
	e.link.onMouseMoved(cellCenter(e, 0, 8))
	if e.link.active() {
		t.Error("a non-navigable symbol should not light a link")
	}
}

func TestLink_MoveAwayClearsLink(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetLinkProvider(&WordLinkProvider{
		Navigable: map[string]bool{"foobar": true},
		Text:      func() string { return e.Text() },
	})
	e.entry.onKeyDown(&event.KeyEvent{Name: shortcutModKey()})

	e.link.onMouseMoved(cellCenter(e, 0, 2)) // over foobar → lit
	if !e.link.active() {
		t.Fatal("link should be lit over foobar")
	}
	e.link.onMouseMoved(cellCenter(e, 0, 8)) // over baz → cleared
	if e.link.active() {
		t.Error("moving onto a non-navigable symbol should clear the link")
	}
	// Leaving the text entirely also clears.
	e.link.onMouseMoved(cellCenter(e, 0, 2))
	e.link.onMouseOut()
	if e.link.active() {
		t.Error("leaving the text area should clear the link")
	}
}

func TestLink_ClearingProviderRemovesLink(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar")
	e.SetLinkProvider(&WordLinkProvider{
		Navigable: map[string]bool{"foobar": true},
		Text:      func() string { return e.Text() },
	})
	e.entry.onKeyDown(&event.KeyEvent{Name: shortcutModKey()})
	e.link.onMouseMoved(cellCenter(e, 0, 2))
	if !e.link.active() {
		t.Fatal("link should be lit")
	}
	e.SetLinkProvider(nil)
	if e.link.active() || e.linkDeco.SpansList != nil {
		t.Error("clearing the provider should remove the link")
	}
}

// TestGoToDefinition_AtCursorFiresAtSymbolStart verifies the keyboard entry
// point resolves the navigable symbol under the caret and fires the navigation
// hook at that symbol's start, regardless of the caret's column within it.
func TestGoToDefinition_AtCursorFiresAtSymbolStart(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetLinkProvider(&WordLinkProvider{
		Navigable: map[string]bool{"foobar": true},
		Text:      func() string { return e.Text() },
	})

	gotRow, gotCol, calls := -1, -1, 0
	e.OnRequestNavigation(func(row, col int) { gotRow, gotCol, calls = row, col, calls+1 })

	e.SetCursorPos(0, 3) // mid-"foobar"
	e.GoToDefinitionAtCursor()

	if calls != 1 || gotRow != 0 || gotCol != 0 {
		t.Fatalf("go-to-def at cursor → calls=%d pos=(%d,%d), want 1 (0,0)", calls, gotRow, gotCol)
	}
}

// TestGoToDefinition_NonNavigableNoFire verifies that, with a provider installed,
// the caret on a non-navigable symbol does not navigate.
func TestGoToDefinition_NonNavigableNoFire(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetLinkProvider(&WordLinkProvider{
		Navigable: map[string]bool{"foobar": true},
		Text:      func() string { return e.Text() },
	})

	calls := 0
	e.OnRequestNavigation(func(int, int) { calls++ })

	e.SetCursorPos(0, 8) // mid-"baz", not navigable
	e.GoToDefinitionAtCursor()
	if calls != 0 {
		t.Fatalf("a non-navigable caret should not navigate, got %d calls", calls)
	}
}

// TestGoToDefinition_NoProviderFiresAtCaret verifies that with no LinkProvider
// the gesture fires unconditionally at the caret, matching provider-free
// Cmd/Ctrl+click.
func TestGoToDefinition_NoProviderFiresAtCaret(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")

	gotRow, gotCol, calls := -1, -1, 0
	e.OnRequestNavigation(func(row, col int) { gotRow, gotCol, calls = row, col, calls+1 })

	e.SetCursorPos(0, 8)
	e.GoToDefinitionAtCursor()
	if calls != 1 || gotRow != 0 || gotCol != 8 {
		t.Fatalf("provider-free go-to-def → calls=%d pos=(%d,%d), want 1 (0,8)", calls, gotRow, gotCol)
	}
}

// TestGoToDefinition_NoHookNoop verifies the gesture is a no-op (no panic) with
// no navigation callback installed.
func TestGoToDefinition_NoHookNoop(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar")
	e.SetCursorPos(0, 2)
	e.GoToDefinitionAtCursor() // must not panic
}

// TestGoToDefinition_DefaultF12KeyDispatch verifies the modifier-less F12 default
// reaches the action through the key dispatcher (TypedKey), since the platform
// driver never synthesizes a shortcut for a bare key.
func TestGoToDefinition_DefaultF12KeyDispatch(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetLinkProvider(&WordLinkProvider{
		Navigable: map[string]bool{"foobar": true},
		Text:      func() string { return e.Text() },
	})

	calls := 0
	e.OnRequestNavigation(func(int, int) { calls++ })

	e.SetCursorPos(0, 3)
	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyF12})
	if calls != 1 {
		t.Fatalf("bare F12 should fire go-to-def via the key dispatcher, got %d calls", calls)
	}
}

// TestGoToDefinition_ModifiedChordRoutesShortcut verifies a host rebinding to a
// modified chord routes through the shortcut registry (TypedShortcut) instead.
func TestGoToDefinition_ModifiedChordRoutesShortcut(t *testing.T) {
	chord := desktop.CustomShortcut{KeyName: event.KeyF12, Modifier: event.KeyModifierShortcutDefault}
	e := renderedWordEditor(t, func(e *Editor) {
		e.KeyMap.GoToDefinition = chord
	}, "foobar")
	e.SetText("foobar baz")
	e.SetLinkProvider(&WordLinkProvider{
		Navigable: map[string]bool{"foobar": true},
		Text:      func() string { return e.Text() },
	})

	calls := 0
	e.OnRequestNavigation(func(int, int) { calls++ })

	e.SetCursorPos(0, 3)
	// The bare-key dispatcher must NOT fire for a modified binding.
	e.entry.TypedKey(&event.KeyEvent{Name: event.KeyF12})
	if calls != 0 {
		t.Fatalf("bare F12 should not fire when rebound to a modified chord, got %d", calls)
	}
	// The chord routes through the shortcut registry.
	e.entry.TypedShortcut(&chord)
	if calls != 1 {
		t.Fatalf("the modified chord should fire go-to-def via the shortcut registry, got %d", calls)
	}
}

// TestLink_UnfocusedDriverModifierLightsLink verifies the pre-rebrand tracker #132 affordance: with
// the window-wide modifier state reporting Cmd/Ctrl held, a hover lights the
// link WITHOUT the Entry ever seeing a key event (i.e. while unfocused).
func TestLink_UnfocusedDriverModifierLightsLink(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetLinkProvider(&WordLinkProvider{
		Navigable: map[string]bool{"foobar": true},
		Text:      func() string { return e.Text() },
	})

	held := event.KeyModifierShortcutDefault
	e.link.modifierSource = func() (event.KeyModifier, bool) { return held, true }

	// No Entry key events at all — the driver state alone lights the link.
	e.link.onMouseMoved(cellCenter(e, 0, 2))
	if !e.link.active() {
		t.Fatal("driver-held modifier should light the link without Entry focus")
	}
	if e.hoverCap.Cursor() != desktop.PointerCursor {
		t.Error("cursor should be a pointer over a lit link")
	}

	// Modifier released (per the driver) → next move clears.
	held = 0
	e.link.onMouseMoved(cellCenter(e, 0, 3))
	if e.link.active() {
		t.Error("driver-released modifier should clear the link on the next move")
	}
}

// TestLink_DriverUnavailableKeepsTrackedState verifies the graceful degradation:
// when the driver cannot report modifiers (wasm/mobile/tests), the Entry-focused
// KeyDown/Up tracking stands — the before pre-rebrand tracker #132 behavior.
func TestLink_DriverUnavailableKeepsTrackedState(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetLinkProvider(&WordLinkProvider{
		Navigable: map[string]bool{"foobar": true},
		Text:      func() string { return e.Text() },
	})
	e.link.modifierSource = func() (event.KeyModifier, bool) { return 0, false }

	e.entry.onKeyDown(&event.KeyEvent{Name: shortcutModKey()})
	e.link.onMouseMoved(cellCenter(e, 0, 2))
	if !e.link.active() {
		t.Error("with no driver state, the Entry-tracked modifier should still light the link")
	}
}

// TestLink_DriverOverridesStaleTrackedState verifies the driver state wins over
// a stale Entry-tracked value — e.g. the modifier was released while the editor
// was unfocused, so the Entry never saw the KeyUp.
func TestLink_DriverOverridesStaleTrackedState(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetLinkProvider(&WordLinkProvider{
		Navigable: map[string]bool{"foobar": true},
		Text:      func() string { return e.Text() },
	})
	e.link.modifierSource = func() (event.KeyModifier, bool) { return 0, true }

	// The Entry saw the press (focused), then focus moved away and the release
	// happened elsewhere: tracked state says held, driver says released.
	e.entry.onKeyDown(&event.KeyEvent{Name: shortcutModKey()})
	e.link.onMouseMoved(cellCenter(e, 0, 2))
	if e.link.active() {
		t.Error("the driver's released state should override the stale tracked press")
	}
}

// TestLink_ModifierChangeReactsWithoutMove (pre-rebrand tracker #155): the driver modifier-change
// callback lights and clears the link at the resting pointer position — no
// mouse move, no Entry key events (i.e. it works unfocused).
func TestLink_ModifierChangeReactsWithoutMove(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar baz")
	e.SetLinkProvider(&WordLinkProvider{
		Navigable: map[string]bool{"foobar": true},
		Text:      func() string { return e.Text() },
	})

	// Pointer rests over the navigable symbol, modifier not held.
	e.link.onMouseMoved(cellCenter(e, 0, 2))
	if e.link.active() {
		t.Fatal("no link should light without the modifier held")
	}

	// The driver reports the modifier pressed: the link lights in place.
	e.link.onModifiersChanged(event.KeyModifierShortcutDefault)
	if !e.link.active() {
		t.Fatal("a modifier-change notification should light the link without a mouse move")
	}
	if e.hoverCap.Cursor() != desktop.PointerCursor {
		t.Error("cursor should be a pointer over a lit link")
	}

	// Released: the link clears in place.
	e.link.onModifiersChanged(0)
	if e.link.active() {
		t.Error("a modifier-release notification should clear the link without a mouse move")
	}
}

// TestLink_WatchModifiersDegradesWithoutDriver: the test driver implements no
// modifier tracking, so registration reports nil and the editor keeps the
// polling + Entry-tracked fallback (also proves CreateRenderer arms nothing
// under tests).
func TestLink_WatchModifiersDegradesWithoutDriver(t *testing.T) {
	test.NewTempApp(t)
	e := renderedPopoverEditor(t, "foobar")
	if e.linkModWatchStop != nil {
		t.Error("no modifier listener should be registered under the test driver")
	}
	if remove := e.link.watchModifiers(); remove != nil {
		t.Error("watchModifiers should report nil without a KeyModifierDriver")
	}
}

func TestWordLinkProvider(t *testing.T) {
	text := "alpha beta"
	p := &WordLinkProvider{
		Navigable: map[string]bool{"alpha": true},
		Text:      func() string { return text },
	}
	if rng, ok := p.IsNavigable(0, 2); !ok ||
		rng != (TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 5}) {
		t.Errorf("alpha should be navigable over [0,5), got %v ok=%v", rng, ok)
	}
	if _, ok := p.IsNavigable(0, 7); ok {
		t.Error("beta is not in the set and should not be navigable")
	}
}
