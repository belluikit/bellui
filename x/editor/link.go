package editor

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// LinkProvider reports whether the symbol at a document position is navigable —
// i.e. has a definition the host can jump to. It backs the Cmd/Ctrl-hover link
// affordance (a solid underline + pointer cursor over the symbol).
//
// It is optional and LSP-shaped but LSP-free: a host can back it with a symbol
// table, a definition-provider bridge, or a static set. Without one, only
// [Editor.OnRequestNavigation] (Cmd/Ctrl+click) is active — no link underlines.
//
// IsNavigable is called on the UI goroutine during pointer movement and should
// return promptly.
type LinkProvider interface {
	// IsNavigable reports whether (row, col) sits on a navigable symbol and, if
	// so, the range to underline as a link. ok is false for a non-navigable
	// position (the returned range is then ignored).
	IsNavigable(row, col int) (rng TextRange, ok bool)
}

// SetLinkProvider installs (or clears, with nil) the link provider that drives
// Cmd/Ctrl-hover link underlines. Clearing it removes any visible link
// decoration. Cmd/Ctrl+click navigation via [Editor.OnRequestNavigation] is
// independent and works with no provider.
func (e *Editor) SetLinkProvider(p LinkProvider) {
	e.link.provider = p
	if p == nil {
		e.link.clear()
	}
}

// OnRequestNavigation installs the callback fired when the user Cmd/Ctrl+clicks
// a position (the "go to definition" gesture). The host resolves the target —
// possibly in another document — and calls [Editor.RevealRange], or opens a new
// document and reveals there. Pass nil to clear it.
//
// The position is 0-based (row, col). fycoder never opens files itself; this hook
// plus RevealRange is the whole navigation contract.
func (e *Editor) OnRequestNavigation(fn func(row, col int)) {
	e.onRequestNavigation = fn
}

// GoToDefinitionAtCursor fires [Editor.OnRequestNavigation] for the symbol at
// the current caret — the keyboard counterpart to Cmd/Ctrl+click, bound by
// default to F12 (see [KeyMap.GoToDefinition]). It mirrors the other keyboard
// entry points to the help surfaces, [Editor.ShowHoverAtCursor] and
// [Editor.ShowSignatureHelpAtCursor].
//
// It is a no-op when no navigation callback is installed. When a [LinkProvider]
// is set, navigation only fires from a navigable symbol and targets that
// symbol's start, so the caret's exact column within the identifier does not
// matter. With no provider it fires at the caret unconditionally, matching the
// provider-free Cmd/Ctrl+click gesture.
func (e *Editor) GoToDefinitionAtCursor() {
	if e.onRequestNavigation == nil {
		return
	}
	row, col := e.entry.CursorRow, e.entry.CursorColumn
	if e.link != nil && e.link.provider != nil {
		rng, ok := e.link.provider.IsNavigable(row, col)
		if !ok || rng.IsEmpty() {
			return
		}
		row, col = rng.StartRow, rng.StartCol
	}
	e.onRequestNavigation(row, col)
}

// WordLinkProvider is a simple [LinkProvider] that treats the identifier under
// the pointer as navigable when it appears in a set — handy for demos and tests.
type WordLinkProvider struct {
	// Navigable is the set of identifiers considered navigable.
	Navigable map[string]bool
	// Text returns the current document text, used to find the word.
	Text func() string
}

// IsNavigable reports whether the identifier under (row, col) is in the set,
// returning that identifier's range for the link underline.
func (p *WordLinkProvider) IsNavigable(row, col int) (TextRange, bool) {
	if p.Text == nil {
		return TextRange{}, false
	}
	word, rng := wordAt(p.Text(), row, col)
	if word == "" || !p.Navigable[word] {
		return TextRange{}, false
	}
	return rng, true
}

// linkController owns the Cmd/Ctrl-hover link affordance: it tracks the modifier
// key, queries the [LinkProvider] under the pointer, and drives an internal
// solid-underline decoration plus the pointer cursor.
type linkController struct {
	e        *Editor
	provider LinkProvider

	// modifierSource reports the window-wide modifier state, or ok=false when
	// the running driver cannot provide it. Defaults to driverKeyModifiers;
	// tests inject a fake.
	modifierSource func() (event.KeyModifier, bool)

	modDown bool         // platform shortcut modifier (Cmd/Ctrl) currently held
	lastPos geo.Position // last pointer position over the text
	hasPos  bool         // lastPos is valid
	linked  bool         // a link decoration is currently shown
}

func newLinkController(e *Editor) *linkController {
	return &linkController{e: e, modifierSource: driverKeyModifiers}
}

// driverKeyModifiers reports the driver's window-wide modifier state —
// tracked from the window's key callbacks (and pointer events on the web
// target), so it is current regardless of which widget (if any) has focus.
// The GLFW driver implements [desktop.KeyModifierDriver] on desktop AND wasm
// (pre-rebrand tracker #460, since v1.0). ok is false when the running driver has no such
// state (mobile, tests), in which case the caller keeps the Entry-focused
// KeyDown/Up tracking.
func driverKeyModifiers() (event.KeyModifier, bool) {
	app := bellui.CurrentApp()
	if app == nil {
		return 0, false
	}
	if d, ok := app.Driver().(desktop.KeyModifierDriver); ok {
		return d.CurrentKeyModifiers(), true
	}
	return 0, false
}

// watchModifiers subscribes to the driver's modifier-change notifications
// (pre-rebrand tracker #155, pre-rebrand tracker #460): pressing or releasing Cmd/Ctrl re-evaluates the link at
// the pointer's resting position the moment it happens — no hover-move
// polling lag — regardless of focus, in the browser too. It returns the
// listener's remove func, or nil when the driver has no modifier tracking
// (mobile, tests); the Entry KeyDown/Up path and the hover-move state sync
// then stand alone. The editor registers it when shown and removes it when
// its renderer is destroyed.
func (l *linkController) watchModifiers() (remove func()) {
	app := bellui.CurrentApp()
	if app == nil {
		return nil
	}
	d, ok := app.Driver().(desktop.KeyModifierDriver)
	if !ok {
		return nil
	}
	return d.AddKeyModifierListener(l.onModifiersChanged)
}

// onModifiersChanged is the driver modifier-change callback (main goroutine):
// it refreshes the tracked state and re-evaluates the link at the last known
// pointer position, so the underline and pointer cursor react instantly to a
// modifier press or release while the pointer rests still — with or without
// Entry focus.
func (l *linkController) onModifiersChanged(mods event.KeyModifier) {
	down := mods&event.KeyModifierShortcutDefault != 0
	if down == l.modDown {
		return
	}
	l.modDown = down
	if l.hasPos {
		l.evaluate(l.lastPos)
	}
}

// onMouseMoved is called by the hover-capture layer for each pointer move.
func (l *linkController) onMouseMoved(pos geo.Position) {
	l.lastPos = pos
	l.hasPos = true
	l.syncModifierState()
	l.evaluate(pos)
}

// syncModifierState refreshes modDown from the driver's window-wide modifier
// state (pre-rebrand tracker #132), so a Cmd/Ctrl-hover lights links while the editor is UNFOCUSED
// — the Entry KeyDown/Up tracking only observes the modifier with focus. When
// the driver cannot report modifiers (mobile/tests) the tracked state stands.
// With the modifier-change listener registered (watchModifiers, pre-rebrand tracker #155) this
// per-move sync is a cheap belt-and-braces re-read of the same state — the
// listener already reacts to presses/releases the instant they happen.
func (l *linkController) syncModifierState() {
	if l.modifierSource == nil {
		return
	}
	mods, ok := l.modifierSource()
	if !ok {
		return
	}
	l.modDown = mods&event.KeyModifierShortcutDefault != 0
}

// onMouseOut is called when the pointer leaves the text area.
func (l *linkController) onMouseOut() {
	l.hasPos = false
	l.clear()
}

// onKeyEvent tracks the platform shortcut modifier (Cmd on macOS, Ctrl
// elsewhere). Pressing or releasing it re-evaluates the link at the last known
// pointer position, so the underline appears/disappears without moving the mouse.
func (l *linkController) onKeyEvent(name event.KeyName, down bool) {
	if !isShortcutModifierKey(name) || l.modDown == down {
		return
	}
	l.modDown = down
	if l.hasPos {
		l.evaluate(l.lastPos)
	}
}

// onMouseDown fires OnRequestNavigation when a primary click carries the
// platform shortcut modifier. Navigability is the host's call — fycoder reports
// the position and lets the host resolve (and possibly find nothing).
func (l *linkController) onMouseDown(m *desktop.MouseEvent) {
	if l.e.onRequestNavigation == nil ||
		m.Button != desktop.MouseButtonPrimary ||
		m.Modifier&event.KeyModifierShortcutDefault == 0 {
		return
	}
	if row, col, ok := l.e.positionAtPoint(m.Position); ok {
		l.e.onRequestNavigation(row, col)
	}
}

// evaluate shows or clears the link decoration for the pointer at pos.
func (l *linkController) evaluate(pos geo.Position) {
	if l.provider == nil || !l.modDown {
		l.clear()
		return
	}
	row, col, ok := l.e.positionAtPoint(pos)
	if !ok {
		l.clear()
		return
	}
	rng, ok := l.provider.IsNavigable(row, col)
	if !ok || rng.IsEmpty() {
		l.clear()
		return
	}
	l.setLink(rng)
}

func (l *linkController) setLink(rng TextRange) {
	if l.e.linkDeco == nil {
		return
	}
	// Skip the refresh when the same range is already underlined.
	if l.linked && len(l.e.linkDeco.SpansList) == 1 && l.e.linkDeco.SpansList[0].Range == rng {
		return
	}
	l.e.linkDeco.SpansList = []DecoratedSpan{{
		Range:          rng,
		Kind:           SpanUnderline,
		UnderlineStyle: UnderlineSolid,
		Color:          themeColor(theme.ColorNamePrimary),
	}}
	l.linked = true
	l.e.RefreshDecorators()
}

func (l *linkController) clear() {
	if !l.linked {
		return
	}
	l.linked = false
	if l.e.linkDeco != nil {
		l.e.linkDeco.SpansList = nil
		l.e.RefreshDecorators()
	}
}

// active reports whether a link decoration is currently shown — the hover-capture
// layer uses it to switch the cursor to a pointer.
func (l *linkController) active() bool {
	return l.linked
}

// isShortcutModifierKey reports whether name is the platform's shortcut modifier
// key (Cmd on macOS, Ctrl elsewhere), matching event.KeyModifierShortcutDefault.
func isShortcutModifierKey(name event.KeyName) bool {
	if event.KeyModifierShortcutDefault == event.KeyModifierSuper {
		return name == desktop.KeySuperLeft || name == desktop.KeySuperRight
	}
	return name == desktop.KeyControlLeft || name == desktop.KeyControlRight
}
