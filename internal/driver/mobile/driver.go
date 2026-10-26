//go:build !wasm

package mobile

import (
	"image/color"
	"math"
	"runtime"
	"strconv"
	"sync"
	"time"
	"unicode"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal"
	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/belluikit/bellui/internal/animation"
	intapp "github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/internal/async"
	"github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/bellui/internal/cache"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/internal/driver/mobile/app"
	"github.com/belluikit/bellui/internal/driver/mobile/event/composition"
	"github.com/belluikit/bellui/internal/driver/mobile/event/config"
	"github.com/belluikit/bellui/internal/driver/mobile/event/key"
	"github.com/belluikit/bellui/internal/driver/mobile/event/lifecycle"
	"github.com/belluikit/bellui/internal/driver/mobile/event/paint"
	"github.com/belluikit/bellui/internal/driver/mobile/event/size"
	"github.com/belluikit/bellui/internal/driver/mobile/event/textedit"
	"github.com/belluikit/bellui/internal/driver/mobile/event/touch"
	"github.com/belluikit/bellui/internal/driver/mobile/event/undoredo"
	"github.com/belluikit/bellui/internal/driver/mobile/gl"
	"github.com/belluikit/bellui/internal/gpu"
	"github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/internal/scale"
	"github.com/belluikit/bellui/internal/textinput"
	"github.com/belluikit/bellui/internal/uidir"
	"github.com/belluikit/bellui/theme"
)

// Tap constants are now in common.TapMoveDecay, common.TapDoubleDelay, etc.

// Configuration is the system information about the current device
type Configuration struct {
	SystemTheme bellui.ThemeVariant
}

// ConfiguredDriver is a simple type that allows packages to hook into configuration changes of this driver.
type ConfiguredDriver interface {
	SetOnConfigurationChanged(func(*Configuration))
}

type driver struct {
	// touchWindows is the window each touch that is down began in (#392).
	touchWindows map[touch.Sequence]*window

	// coast carries a released drag on; the next touch stops it (#751).
	coast common.DragCoast

	app   app.App
	glctx gl.Context

	// windows is guarded by windowsMu: a window may be opened or closed off
	// the main goroutine while the main goroutine reads the list (a Refresh
	// looking up its direction walks AllWindows).
	windows     []bellui.Window
	windowsMu   sync.RWMutex
	device      device
	animation   animation.Runner
	currentSize size.Event

	theme           bellui.ThemeVariant
	onConfigChanged func(*Configuration)
	painting        bool
	running         bool

	// surfaceOwner is the window whose WebGPU painter currently holds the
	// configured surface on the shared Android ANativeWindow. Only one window may
	// own it at a time (two Vulkan swapchains on one native window can't both
	// present, pre-rebrand tracker #753), so claimNativeSurface hands it over on a current-window
	// change. Android-only; nil/unused on iOS (per-window CAMetalLayers) and GL.
	surfaceOwner *window

	// nativeWindow is the shared Android ANativeWindow handle, captured from the
	// last hardware paint event. All windows share it, but a window opened over
	// another only ever receives handle-less draw-ticker paint events, so it
	// inherits this handle to become renderable (pre-rebrand tracker #753). Android-only.
	nativeWindow uintptr

	// textInput is the neutral text-input mirror (pre-rebrand tracker #768/#770). It binds the
	// focused core.AccessibleTexter on focus change and pushes its (text,
	// selection) into the native IME shadow via keyboardStateBackend — the
	// seam-driven successor to the bespoke pushKeyboardState cache (pre-rebrand tracker #572). It
	// also routes IME composition back to the editor's event.Composable.
	textInput   *textinput.Session
	queuedFuncs *async.UnboundedChan[func()]

	// keyboardType is the mobile.KeyboardType of the field the soft keyboard was
	// last shown for, tracked so the one-writer IME reroute (pre-rebrand tracker #1061) can tell a
	// prose field (Gboard composes and commits hardware keys through the
	// InputConnection) from a password/number field (no composition — hardware
	// keys arrive only on the native queue and must not be suppressed). Set by
	// showVirtualKeyboard on the event/UI goroutine, read at key dispatch.
	keyboardType mobile.KeyboardType

	// tbPass gates raw touch streams while an explore-by-touch screen reader
	// owns the touchscreen — see exploreTouch (pre-rebrand tracker #1104).
	tbPass tbPassthrough

	// keyMods is the hardware-keyboard modifier set carried by the most recent
	// key event, exposed via CurrentKeyModifiers so Entry can extend its
	// selection on Shift+Arrow (pre-rebrand tracker #1083) — the mobile driver satisfies
	// desktop.KeyModifierDriver. keyModListeners are notified on change. Both are
	// touched only on the event goroutine at key dispatch.
	keyMods         event.KeyModifier
	keyModListeners []func(event.KeyModifier)

	// lastCanUndo/lastCanRedo/undoStatePushed dedupe the per-repaint iOS
	// undo-state mirror (pre-rebrand tracker #1134): SetKeyboardUndoState is called only when the
	// focused editor's declared undo/redo availability changes. Event-goroutine
	// only, like the keyboard-state sync that drives it.
	lastCanUndo, lastCanRedo bool
	undoStatePushed          bool

	// a11yRing / a11yRingCore are the reusable stroke-only rectangles painted
	// over the element a screen reader has accessibility-focused (pre-rebrand tracker #759). The
	// OS-drawn focus ring cannot reach Bellui's flat GPU surface, so the driver
	// paints its own after the content walk, as a high-contrast double ring (a
	// light outer halo + a dark inner core) so it stays legible over content of
	// any colour. Nil until first needed; only ever touched on the main
	// goroutine at paint time.
	a11yRing     *belluicanvas.Rectangle
	a11yRingCore *belluicanvas.Rectangle
	// a11yRingPainted is where the ring was last painted, and whether it was.
	// Under partial repaint it marks that area dirty on the next paint, so a
	// ring that moved or went away does not leave itself behind (#551).
	a11yRingPainted    geo.Rectangle
	a11yRingWasPainted bool

	// surfaceWasHidden records that the window's surface has gone away once
	// (the app went to the background), for surfaceShown.
	surfaceWasHidden bool

	// surfaceVisible is whether the app is on screen: true from the visible
	// lifecycle crossing on, false from the crossing off. Backends with no
	// other signal for it gate painting on it (iOS WebGPU, #673): iOS may
	// kill an app that submits GPU work while backgrounded.
	surfaceVisible bool
}

// Declare conformity with Driver
var (
	_ bellui.Driver    = (*driver)(nil)
	_ ConfiguredDriver = (*driver)(nil)
)

func init() {
	runtime.LockOSThread()
}

func (d *driver) DoFromGoroutine(fn func(), wait bool) {
	caller := func() {
		if d.queuedFuncs == nil {
			fn() // before the app actually starts
			return
		}
		var done chan struct{}
		if wait {
			done = common.DonePool.Get()
			defer common.DonePool.Put(done)
		}

		d.queuedFuncs.In() <- func() {
			fn()
			if wait {
				done <- struct{}{}
			}
		}

		if wait {
			<-done
		}
	}

	if wait {
		async.EnsureNotMain(caller)
	} else {
		caller()
	}
}

func (d *driver) CreateWindow(title string) bellui.Window {
	c := newCanvas(bellui.CurrentDevice()).(*canvas) // silence lint
	ret := &window{title: title, driver: d, canvas: c, isChild: len(d.AllWindows()) > 0}
	// Set up before it is listed, and without the lock: setting content may
	// refresh, and a Refresh can look up the window list (AllWindows).
	c.setContent(&belluicanvas.Rectangle{FillColor: theme.Color(theme.ColorNameBackground)})
	c.SetPainter(newCanvasPainter(c, ret))
	d.windowsMu.Lock()
	d.windows = append(d.windows, ret)
	d.windowsMu.Unlock()
	return ret
}

// AllWindows returns the open windows, oldest first. The slice is a copy, so
// a window closed meanwhile does not change it under the caller.
func (d *driver) AllWindows() []bellui.Window {
	d.windowsMu.RLock()
	defer d.windowsMu.RUnlock()
	return append([]bellui.Window(nil), d.windows...)
}

// removeWindow takes w out of the window list, if it is in it.
func (d *driver) removeWindow(w *window) {
	d.windowsMu.Lock()
	defer d.windowsMu.Unlock()
	for i, win := range d.windows {
		if win == w {
			d.windows = append(d.windows[:i:i], d.windows[i+1:]...)
			return
		}
	}
}

// currentWindow returns the most recently opened window - we can only show one at a time.
func (d *driver) currentWindow() *window {
	windows := d.AllWindows()
	var last *window
	for i := len(windows) - 1; i >= 0; i-- {
		last = windows[i].(*window)
		if last.visible {
			return last
		}
	}

	return last
}

// refreshCurrentWindow marks the current (top) window's content dirty so the
// next paint tick repaints it. Called after Hide/Close reveals the window
// beneath in the stack, since handlePaint only repaints a dirty canvas and would
// otherwise keep presenting the hidden/closed window's last frame (pre-rebrand tracker #645).
func (d *driver) refreshCurrentWindow() {
	cur := d.currentWindow()
	if cur == nil {
		return
	}
	// Mark the canvas dirty directly (not via Content().Refresh, which depends on
	// CanvasForObject resolving the just-revealed window's content) so the next
	// paint tick's CheckDirtyAndClear repaints it.
	cur.canvas.SetDirty()
}

func (d *driver) Clipboard() bellui.Clipboard {
	return NewClipboard()
}

func (d *driver) RenderedTextSize(text string, textSize float32, style bellui.TextStyle, source bellui.Resource) (size geo.Size, baseline float32) {
	return painter.RenderedTextSize(text, textSize, style, source)
}

func (d *driver) CanvasForObject(obj core.CanvasObject) core.Canvas {
	// TODO figure out how we handle multiple windows...
	if w := d.currentWindow(); w != nil {
		return w.Canvas()
	}
	return nil
}

func (d *driver) AbsolutePositionForObject(co core.CanvasObject) geo.Position {
	c := d.CanvasForObject(co)
	if c == nil {
		return geo.NewPos(0, 0)
	}

	mc := c.(*canvas)
	pos := intdriver.AbsolutePositionForObject(co, mc.ObjectTrees())
	inset, _ := c.InteractiveArea()
	return pos.Subtract(inset)
}

func (d *driver) GoBack() {
	app.GoBack()
}

func (d *driver) Quit() {
	// Android and iOS guidelines say this should not be allowed!
}

func (d *driver) Run() {
	if d.running {
		return // Run was called twice.
	}
	d.running = true

	app.Main(func(a app.App) {
		async.SetMainGoroutine()
		d.app = a
		d.queuedFuncs = async.NewUnboundedChan[func()]()

		bellui.CurrentApp().Settings().AddListener(func(s bellui.Settings) {
			painter.ClearFontCache()
			cache.ResetThemeCaches()
			intapp.ApplySettingsWithCallback(s, bellui.CurrentApp(), func(w bellui.Window) {
				c, ok := w.Canvas().(*canvas)
				if !ok {
					return
				}
				c.applyThemeOutOfTreeObjects()
				// Cached RepaintBoundary FBOs hold pixels rendered with the
				// old theme; invalidate so they re-render.
				if bp, ok := c.Painter().(gpu.BoundaryPainter); ok {
					bp.InvalidateAllBoundaries()
				}
			})
		})

		draw := time.NewTicker(time.Second / 60)
		defer func() {
			l := bellui.CurrentApp().Lifecycle().(*intapp.Lifecycle)

			// exhaust the event queue
			go func() {
				l.WaitForEvents()
				d.queuedFuncs.Close()
			}()
			for fn := range d.queuedFuncs.Out() {
				fn()
			}

			l.DestroyEventQueue()
		}()

		for {
			select {
			case <-draw.C:
				d.sendPaintEvent()
			case fn := <-d.queuedFuncs.Out():
				fn()
			case e, ok := <-a.Events():
				if !ok {
					return // events channel closed, app done
				}
				current := d.currentWindow()
				if current == nil {
					continue
				}
				c := current.Canvas().(*canvas)

				switch e := a.Filter(e).(type) {
				case lifecycle.Event:
					d.handleLifecycle(e, current)
				case config.Event:
					// An in-place configuration change (Android delivers
					// manifest-declared changes as onConfigurationChanged —
					// orientation|keyboardHidden|uiMode|locale|layoutDirection
					// since #287, plus screenLayout since #299). Re-resolve
					// the system locale so an Android 13+ per-app language
					// change applies with no lifecycle crossing for the
					// StageFocused seam to catch (#288).
					// Override-guarded, and an unchanged locale skips the
					// relayout, so the changes that are not locale changes
					// (rotation above all, and since #299 the multi-window
					// screen-layout transitions) cost only the GetLocales
					// lookup — which go-locale runs on its own attached JNI
					// thread, so it cannot deadlock against the Java main
					// thread waiting in the redraw handshake behind this
					// event.
					resolveSystemLocale()
				case size.Event:
					if e.WidthPx <= 0 {
						continue
					}
					d.currentSize = e
					currentOrientation = e.Orientation
					currentDPI = e.PixelsPerPt * 72
					d.setTheme(e.DarkMode)

					dev := &d.device
					insetChange := dev.safeTop != e.InsetTopPx || dev.safeBottom != e.InsetBottomPx ||
						dev.safeLeft != e.InsetLeftPx || dev.safeRight != e.InsetRightPx
					dev.safeTop = e.InsetTopPx
					dev.safeLeft = e.InsetLeftPx
					dev.safeBottom = e.InsetBottomPx
					dev.safeRight = e.InsetRightPx
					c.scale = bellui.CurrentDevice().SystemScaleForWindow(nil)
					c.Painter().SetFrameBufferScale(1.0)

					if insetChange {
						current.canvas.sizeContent(current.canvas.size) // even if size didn't change we invalidate
					}
					// make sure that we paint on the next frame
					c.Content().Refresh()
				case paint.Event:
					d.handlePaint(e, current)
				case touch.Event:
					// Under an explore-by-touch screen reader (TalkBack), raw
					// touches still reach the native input queue, but they are a
					// mix the framework never disambiguates for us: explore
					// hovers (which must not focus or activate anything — the
					// pre-rebrand tracker #572 device finding) and genuine PASSTHROUGH interactions
					// the app is expected to handle natively — the two-finger
					// scroll gesture and everything after double-tap-and-hold
					// (pre-rebrand tracker #1104). exploreTouch suppresses the former and routes
					// qualified passthrough streams into the normal pipeline.
					d.routeTouch(current, e, app.TouchExplorationEnabled())
				case key.Event:
					d.dispatchKeyEvent(c, e)
				case composition.Event:
					d.handleComposition(e) // routes through the mirror, which auto-syncs
				case textedit.Event:
					d.handleTextEdit(c, e) // committed native edit → seam Replace (auto-syncs)
				case undoredo.Event:
					d.handleUndoRedo(e) // native undo/redo key/gesture → editor's accessible action (pre-rebrand tracker #1134)
				}
			}
		}
	})
}

// dispatchKeyEvent is the event-loop entry for a key event. It applies the
// one-writer IME reroute (pre-rebrand tracker #1061), then dispatches and syncs the native
// keyboard state.
func (d *driver) dispatchKeyEvent(c *canvas, e key.Event) {
	if d.suppressNativeTextKey(e) {
		return
	}
	d.handleKeyEvent(c, e)
	d.syncKeyboardState()
}

// suppressNativeTextKey reports whether a raw native-queue key event is the
// redundant echo of a text edit the Android IME has already delivered through
// its InputConnection, so bellui must ignore it (pre-rebrand tracker #1061, the one-writer reroute).
//
// When a text editor is focused the IME is the single writer: Gboard applies
// every printable key and Backspace to the bellui-owned IME buffer
// (commitText / setComposingText / replaceText / deleteSurroundingText /
// sendKeyEvent) and those edits reach the editor through the IC channel
// (keyboardReplace / keyboardComposing / keyboardComposeCommit / keyboardDelete
// → handleTextEdit / handleComposition, or the non-native keyboardDelete /
// keyboardTyped key events). The same hardware key still traverses the
// NativeActivity input queue a few ms later tagged Native; applying it a second
// time was the dual write that doubled characters (pre-rebrand tracker #1057) and left a spurious
// space after autocorrected words, and it also dropped word-boundary
// autocorrections because the two writers had to be reconciled by guesswork.
// Ignoring the native echo makes the IC the sole writer, so autocorrect /
// auto-capitalization simply land as the committed edits Gboard already sends.
//
// Only bare printable keys and Backspace are suppressed. Non-text keys (arrows,
// Enter, Tab, Esc, function keys) and any Ctrl/Alt/Super combination (hardware
// shortcuts) are left for bellui — the IME does not consume those, so the native
// queue remains their only delivery. Non-native key events (the IME's own
// keyboardDelete / keyboardTyped / keyboardArrowKey) are never touched.
func (d *driver) suppressNativeTextKey(e key.Event) bool {
	// Android-only: iOS delivers text through its input view as textedit
	// events, never as Native key events, so there is nothing to reconcile there.
	return runtime.GOOS == "android" && d.imeOwnsNativeKey(e)
}

// imeOwnsNativeKey is the platform-independent core of suppressNativeTextKey,
// split out so it can be unit-tested on the host.
func (d *driver) imeOwnsNativeKey(e key.Event) bool {
	if !e.Native {
		return false
	}
	if d.textInput == nil || !d.textInput.Bound() {
		return false // no focused editor → no IC writer, native key is the only delivery
	}
	// A shortcut combination (Ctrl/Alt/Meta held) is forwarded to the focused
	// editor by the IME through sendKeyEvent → keyboardShortcut (pre-rebrand tracker #1077), in every
	// field type, so the raw native key is a redundant echo — suppress it here or
	// the shortcut dispatches twice (device-verified: Ctrl+V pasted the clipboard
	// twice, since the paste is not idempotent). The forwarded (non-native) key
	// carries the shortcut to bellui.
	if e.Modifiers&(key.ModControl|key.ModAlt|key.ModMeta) != 0 {
		return true
	}
	// Plain text keys: only prose fields put Gboard in a composing/committing
	// mode where it writes every hardware key through the InputConnection.
	// Password and number fields disable suggestions and composition, so Gboard
	// passes their hardware keys straight through the native queue — there the
	// native key is the sole delivery and suppressing it would swallow all typing
	// (device-verified: the external keyboard went dead in the password field).
	// Only suppress text where the IC is known to be the writer.
	switch d.keyboardType {
	case mobile.DefaultKeyboard, mobile.SingleLineKeyboard:
	default:
		return false
	}
	return e.Code == key.CodeDeleteBackspace || unicode.IsPrint(e.Rune)
}

func (d *driver) handleKeyEvent(c *canvas, e key.Event) {
	if e.Direction == key.DirPress {
		d.typeDownCanvas(c, e.Rune, e.Code, e.Modifiers)
	} else if e.Direction == key.DirRelease {
		d.typeUpCanvas(c, e.Rune, e.Code, e.Modifiers)
	}
}

// handleComposition dispatches an IME composition event to the focused editor
// through the text-input mirror's compose ops (pre-rebrand tracker #770). Session.Compose/Commit
// route to the bound event.Composable — a no-op when the focus does not support
// composition — and each auto-syncs the native shadow afterwards.
//
// The Android one-writer reroute (pre-rebrand tracker #1061) suppresses the raw-key echoes of
// hardware typing (suppressNativeTextKey), so the IME's InputConnection is the
// sole writer here. Gboard's hardware-keyboard latin editing arrives as
// committed edits (commitText / setComposingRegion → handleTextEdit), and a
// genuine composition (CJK, or an IME that composes inline) arrives as the
// preedit/commit pair below — no dual-writer reconciliation is needed.
func (d *driver) handleComposition(e composition.Event) {
	if d.textInput == nil {
		return
	}
	if e.Commit {
		d.textInput.Commit(e.Text)
		return
	}
	d.textInput.Compose(e.Text, e.CursorPos)
}

// handleTextEdit applies a committed native keyboard/IME edit — a replacement of
// the [Start, End) rune range with Text — to the focused editor through the
// text-input mirror's Replace op (pre-rebrand tracker #771). Reporting the full range (rather than a
// caret-relative delta) lets typing over a selection replace it in one gated,
// undo-recorded edit instead of appending (pre-rebrand tracker #748). Session.Replace auto-syncs the
// native document afterwards. iOS drives this from its input view.
//
// When the focus is not an editor, nothing takes the edit, except a newline:
// the soft keyboard's Return key, which an Android IME commits as the text
// "\n" on the default keyboard (IME_FLAG_NO_ENTER_ACTION). It reaches the
// focused widget on c as a Return key press, so a widget that takes keys and
// finishes on Return (the chart's Text tool) sees it (#666). Other text is not
// forwarded: a non-editor gets app.KeysKeyboard, on which the IME sends key
// presses rather than text, so its typing arrives as key events. The newline
// stays for an IME that commits it anyway.
func (d *driver) handleTextEdit(c *canvas, e textedit.Event) {
	if d.textInput == nil || !d.textInput.Bound() {
		if !e.Selection && c != nil {
			for _, r := range e.Text {
				if r == '\n' {
					d.dispatchKeyEvent(c, key.Event{Rune: r, Code: key.CodeReturnEnter, Direction: key.DirPress})
					d.dispatchKeyEvent(c, key.Event{Rune: r, Code: key.CodeReturnEnter, Direction: key.DirRelease})
				}
			}
		}
		return
	}
	if e.Selection {
		d.textInput.SetSelection(e.Start, e.End)
		return
	}
	d.textInput.Replace(e.Start, e.End, e.Text)
}

// keyboardStateBackend mirrors the focused editor's (text, selection) into the
// native input surface via app.SetKeyboardState — the iOS input-view document
// (pre-rebrand tracker #771) and the Android bellui-owned IME buffer (pre-rebrand tracker #1006 Phase 4) — so the native
// IME reads real content and the selection round-trips. It is a no-op on desktop
// (build-tagged stub).
type keyboardStateBackend struct{}

func (keyboardStateBackend) SetState(s textinput.Snapshot) {
	app.SetKeyboardState(s.Text, s.SelStart, s.SelEnd)
}

// syncKeyboardState pushes the bound editor's current text + selection into the
// native input surface via the text-input mirror (pre-rebrand tracker #770/#771 seam), skipping
// redundant native pushes. Active on the two platforms with a real backend: iOS
// (the input-view document) and, since pre-rebrand tracker #1006 Phase 4, Android (the bellui-owned
// IME buffer the soft keyboard's InputConnection reads). The read is elided on
// desktop/host where the backend is a no-op anyway. Both platforms ignore the
// push during an active IME composition (so it never clobbers the composing
// region), and the buffer's own IME-side edits make an echoing push a strict
// no-op.
func (d *driver) syncKeyboardState() {
	if d.textInput == nil {
		return
	}
	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		d.textInput.Sync()
	}
	if runtime.GOOS == "ios" {
		d.syncKeyboardUndoState()
	}
}

// syncKeyboardUndoState mirrors the focused editor's undo/redo availability
// into the iOS input view's NSUndoManager (pre-rebrand tracker #1134), so the iPad keyboard's
// undo/redo bar keys (and the system undo gestures) reflect bellui's history.
// Availability is read from the editor's declared accessible actions —
// Entry/RichEntry declare ActionUndo/ActionRedo gated on history/read-only/
// password exactly as assistive tech sees them (pre-rebrand tracker #1025) — and pushed only on
// change. A native invocation comes back as an undoredo.Event.
func (d *driver) syncKeyboardUndoState() {
	var focused any
	if cur := d.currentWindow(); cur != nil {
		focused = cur.canvas.Focused()
	}
	canUndo, canRedo := undoActionsState(focused)
	if d.undoStatePushed && canUndo == d.lastCanUndo && canRedo == d.lastCanRedo {
		return
	}
	d.undoStatePushed = true
	d.lastCanUndo, d.lastCanRedo = canUndo, canRedo
	app.SetKeyboardUndoState(canUndo, canRedo)
}

// undoActionsState derives undo/redo availability from an object's declared
// accessible actions; (false, false) when it declares none or is not an
// AccessibleActioner.
func undoActionsState(obj any) (canUndo, canRedo bool) {
	actioner, ok := obj.(core.AccessibleActioner)
	if !ok {
		return false, false
	}
	for _, a := range actioner.AccessibleActions() {
		switch a.Name {
		case core.ActionUndo:
			canUndo = true
		case core.ActionRedo:
			canRedo = true
		}
	}
	return canUndo, canRedo
}

// handleUndoRedo applies a native undo/redo invocation (the iPad keyboard's
// undo/redo keys or system undo gestures, fired through the proxy actions in
// the input view's NSUndoManager, pre-rebrand tracker #1134) to the focused editor via its
// accessible undo/redo action — the same gated path assistive tech uses.
func (d *driver) handleUndoRedo(e undoredo.Event) {
	cur := d.currentWindow()
	if cur == nil {
		return
	}
	actioner, ok := cur.canvas.Focused().(core.AccessibleActioner)
	if !ok {
		return
	}
	name := core.ActionUndo
	if e.Redo {
		name = core.ActionRedo
	}
	actioner.PerformAccessibleAction(name)
	// An invocation always consumed a native proxy action, so the manager's
	// stacks no longer mirror bellui's even when the {canUndo, canRedo} flags are
	// unchanged (an undo mid-history stays true→true) — without a forced
	// rebuild the native undo stack runs dry after a tap and the keyboard key
	// dies, while stale opposite-direction proxies accumulate (device finding:
	// "the undo stack seems to have 3 slots"). Flutter's framework likewise
	// pushes state unconditionally after every undo/redo execution.
	d.undoStatePushed = false
	d.syncKeyboardState()
}

func (*driver) SetDisableScreenBlanking(disable bool) {
	setDisableScreenBlank(disable)
}

// surfaceShown is the canvas FBO's half of the surface coming up. The
// persistent partial-repaint canvas FBO stops retaining its contents across
// frames after a mobile background→resume (the off-screen texture is no longer
// load-stable on tiled GPUs, even when reallocated), so dirty-region frames
// would lose everything they don't redraw. After a resume, fall back to full
// repaint for the rest of the session — correct, at the cost of the
// optimization (pre-rebrand tracker #342). The first time the surface comes up
// is no resume: marking then turned partial repaint off for every session
// within a frame of launch (#591). No-op when not using the FBO path.
func (d *driver) surfaceShown(c *canvas) {
	if !d.surfaceWasHidden {
		return
	}
	if m, ok := c.Painter().(interface{ MarkCanvasFBORetentionUnreliable() }); ok {
		m.MarkCanvasFBORetentionUnreliable()
	}
}

func (d *driver) handleLifecycle(e lifecycle.Event, w *window) {
	c := w.Canvas().(*canvas)
	switch e.Crosses(lifecycle.StageAlive) {
	case lifecycle.CrossOn:
		d.onStart()
	case lifecycle.CrossOff:
		d.onStop()
	}
	switch e.Crosses(lifecycle.StageVisible) {
	case lifecycle.CrossOn:
		d.glctx, _ = e.DrawContext.(gl.Context)
		d.surfaceVisible = true
		d.surfaceShown(c)
		// this is a fix for some android phone to prevent the app from being drawn as a blank screen after being pushed in the background
		c.Content().Refresh()

		d.sendPaintEvent()
	case lifecycle.CrossOff:
		d.glctx = nil
		d.surfaceVisible = false
		d.surfaceWasHidden = true
	}
	switch e.Crosses(lifecycle.StageFocused) {
	case lifecycle.CrossOn: // foregrounding
		// Re-resolve a system locale changed while backgrounded (Android only;
		// #282) before the app's foreground hook, so that hook already sees
		// the new locale.
		resolveSystemLocale()
		if f := bellui.CurrentApp().Lifecycle().(*intapp.Lifecycle).OnEnteredForeground(); f != nil {
			f()
		}
	case lifecycle.CrossOff: // will enter background
		if runtime.GOOS == "darwin" || runtime.GOOS == "ios" {
			if !d.renderReady(w) {
				return
			}

			s := geo.NewSize(float32(d.currentSize.WidthPx)/c.scale, float32(d.currentSize.HeightPx)/c.scale)
			d.paintWindow(w, s)
			d.presentPainter(c)
			d.app.Publish()
		}
		if f := bellui.CurrentApp().Lifecycle().(*intapp.Lifecycle).OnExitedForeground(); f != nil {
			f()
		}
	}
}

func (d *driver) handlePaint(e paint.Event, w *window) {
	c := w.Canvas().(*canvas)
	if e.Window != 0 { // not all paint events come from hardware
		// A changed native-window handle after init means the OS recreated the
		// window (e.g. Android delivers a fresh ANativeWindow when the app returns
		// from the background). The surface must be rebuilt or the painter presents
		// to the destroyed one — a black canvas and a main-thread stall/ANR (#310);
		// note it and let the liveness gate below pick the moment.
		d.nativeWindow = e.Window // the shared Android ANativeWindow
		if c.initialized && e.Window != w.handle {
			w.surfaceStale = true
		}
		w.handle = e.Window
	} else if w.handle == 0 {
		// A window opened over another (a child in the stack) becomes current
		// without ever receiving a hardware paint event — only handle-less
		// draw-ticker events — so it never learns the ANativeWindow and stays
		// unrenderable (renderReady is false). All Android windows share one
		// native window, so inherit it here so the newcomer can paint (pre-rebrand tracker #753).
		w.handle = d.nativeWindow
	}
	d.painting = false
	if !d.renderReady(w) {
		return
	}
	if w.surfaceStale {
		// Rebuild here rather than the moment the handle changed: renderReady is
		// the native-window liveness gate on Android (#283), and building a wgpu
		// surface on a destroyed ANativeWindow wedges the same GetCapabilities
		// call that presenting into one does. Paint events are queued, so a fresh
		// handle can be drained after the window it names has died — hold the
		// rebuild until a paint the window is alive for. Still ahead of the
		// External return so a resume rebuilds on the handle-carrying event
		// rather than waiting for the next draw tick (#312).
		w.surfaceStale = false
		d.recreateSurface(c, w)
	}
	if e.External {
		return
	}
	// Hand the shared Android native surface to the window about to paint,
	// releasing whatever window held it before (pre-rebrand tracker #753). No-op on iOS/GL.
	d.claimNativeSurface(c, w)
	if !c.initialized {
		// Retry until the native surface is ready (iOS builds it from the key
		// window, which may not exist on the first paint).
		if d.initCanvasPainter(c, w) {
			c.initialized = true
		}
	}

	d.animation.TickAnimations()
	canvasNeedRefresh := c.FreeDirtyTextures() > 0 || c.CheckDirtyAndClear()
	if canvasNeedRefresh {
		newSize := d.prepareFrame(w)
		d.paintWindow(w, newSize)
		d.presentPainter(c)
		d.app.Publish()
		// Sync the screen-reader tree with what was just presented (pre-rebrand tracker #488);
		// a no-op stub without the accessibility tag.
		w.updateAccessibility()
		// Mirror the focused editor's (text, selection) into the native IME
		// document each repaint (pre-rebrand tracker #771). Key-event syncs alone miss selection
		// changes that don't go through a key — e.g. Select All from the edit
		// menu, or a tap that moves the caret — which would leave the native
		// field's selection stale so a following keystroke replaces the wrong
		// range. The seam's redundant-push filter makes this a cheap no-op when
		// nothing changed, and it is skipped during composition.
		d.syncKeyboardState()
	}
	cache.Clean(canvasNeedRefresh)
}

// prepareFrame sizes the window's content to the screen before a paint, and
// returns the size to paint at.
func (d *driver) prepareFrame(w *window) geo.Size {
	c := w.canvas
	newSize := geo.NewSize(float32(d.currentSize.WidthPx)/c.scale, float32(d.currentSize.HeightPx)/c.scale)
	if c.EnsureMinSize() {
		c.sizeContent(newSize) // force resize of content
	} else { // if screen changed
		w.Resize(newSize)
	}
	return newSize
}

func (d *driver) onStart() {
	if f := bellui.CurrentApp().Lifecycle().(*intapp.Lifecycle).OnStarted(); f != nil {
		f()
	}
}

func (d *driver) onStop() {
	l := bellui.CurrentApp().Lifecycle().(*intapp.Lifecycle)
	if f := l.OnStopped(); f != nil {
		l.QueueEvent(f)
	}
}

// clearCanvasFBO prepares the bound canvas FBO for this frame and returns the
// dirty region to redraw. Partial repaint off (build.PartialRepaint), or the
// first frame: full clear, no region. On: clear just the changed region, or
// full-clear when it covers most of the
// canvas; an empty region (nothing changed) draws nothing and re-presents the
// retained FBO.
func clearCanvasFBO(c *canvas, drp gpu.DirtyRegionPainter, size geo.Size) (partial bool, drPos geo.Position, drSize geo.Size) {
	if build.PartialRepaint && !drp.CanvasFBONeedsFullClear() {
		pos, sz, set := c.PartialDirtyRegion()
		// Clamped as the desktop driver does: a region running off the canvas
		// would make a scissor the painter cannot set, so the clear would not be
		// confined to it.
		pos, sz = common.ClampRegionToCanvas(pos, sz, size)
		set = set && sz.Width > 0 && sz.Height > 0
		switch {
		case !set:
			return true, geo.Position{}, geo.Size{}
		case common.RegionCoversCanvas(sz, size):
			c.Painter().Clear()
			return false, geo.Position{}, geo.Size{}
		default:
			c.Painter().StartClipping(pos, sz)
			c.Painter().Clear()
			c.Painter().StopClipping()
			return true, pos, sz
		}
	}
	c.Painter().Clear()
	drp.MarkCanvasFBOCleared()
	c.MarkFullRepaint()
	return false, geo.Position{}, geo.Size{}
}

type mobileBoundaryEntry struct {
	obj                    core.CanvasObject
	state                  gpu.BoundaryFBO
	wasDirty               bool
	invWT                  geo.Matrix
	origin                 geo.Position         // the texture's origin: the boundary's position snapped to a device pixel
	phase                  common.BoundaryPhase // the sub-pixel phase the texture is rendered at
	size                   geo.Size
	canvasPixW, canvasPixH int
	outerClips             *internal.ClipStack // the clips outside a dirty boundary (#559)
}

// mobileTreeWalker bundles the mutable state shared by the draw/afterDraw
// callbacks of the paint walk. It mirrors the desktop driver's
// boundary FBO + dirty-region handling.
type mobileTreeWalker struct {
	c     *canvas
	clips *internal.ClipStack
	size  geo.Size

	bp          gpu.BoundaryPainter
	hasBoundary bool
	bStack      []mobileBoundaryEntry

	canvasPixW, canvasPixH int

	partialRepaint bool
	drPos          geo.Position
	drSize         geo.Size
}

// paint walks the content and overlay trees with draw/afterDraw, then rebuilds
// the hit-test index. Clip-owners scrolled out of view are visited without
// their subtrees (#464, #481), as in the desktop driver. A culled walk must not
// sweep the node map, which WalkTrees does: the objects it skips would lose
// their nodes every frame. The min-size pass's full walks sweep it instead.
func (w *mobileTreeWalker) paint() {
	w.c.WalkContentTreesCulled(w.cull, w.draw, w.afterDraw)
	w.c.WalkOverlayTrees(w.draw, w.afterDraw)
	w.c.RebuildSpatialIndex()
}

// paintClipped is paint with, under a partial repaint, the dirty region's clip
// on through the walk. Culling skips the objects outside the region, but one
// that overlaps it is drawn whole: a full-canvas background painted over the
// retained pixels of everything culled, so only what the region held stayed
// on screen (#591; the desktop driver's paint does the same). The region
// seeds the clip stack, so nested clips intersect with it and restore to it.
func (w *mobileTreeWalker) paintClipped() {
	dirtyClip := w.partialRepaint && w.drSize.Width > 0 && w.drSize.Height > 0
	if dirtyClip {
		w.clips.Push(w.drPos, w.drSize)
		w.applyClip()
	}
	w.paint()
	if dirtyClip {
		w.clips.Pop()
		w.applyClip()
	}
}

// cull is the paint walk's driver.CullFunc: common.CullOffClip, except while
// a boundary renders into its texture, as in the desktop driver (#564). The
// boundary's cache must hold its whole subtree, or content off screen when it
// rendered would stay missing once scrolled into view, so only the clips
// opened inside it cull: its own clip stack, in the texture's space, not the
// walk's clip, which includes the clips outside it (#566).
func (w *mobileTreeWalker) cull(obj core.CanvasObject, pos, clipPos geo.Position, clipSize geo.Size) bool {
	if ab := w.activeBoundary(); ab != nil && ab.wasDirty {
		top := w.clips.Top()
		if top == nil {
			return false
		}
		clipPos, clipSize = top.Rect()
		clipPos = clipPos.Add(ab.origin)
	}
	return w.c.CullOffClip(obj, pos, clipPos, clipSize)
}

func (w *mobileTreeWalker) insideCleanBoundary() bool {
	return len(w.bStack) > 0 && !w.bStack[len(w.bStack)-1].wasDirty
}

func (w *mobileTreeWalker) activeBoundary() *mobileBoundaryEntry {
	if len(w.bStack) == 0 {
		return nil
	}
	return &w.bStack[len(w.bStack)-1]
}

func (w *mobileTreeWalker) shouldSkipForDirtyRegion(pos geo.Position, objSize geo.Size) bool {
	return w.partialRepaint && !common.InDirtyRegion(pos, objSize, w.drPos, w.drSize)
}

// skipObjectForDirtyRegion is shouldSkipForDirtyRegion for obj at pos, by
// where it draws rather than its bounds (#602).
func (w *mobileTreeWalker) skipObjectForDirtyRegion(obj core.CanvasObject, pos geo.Position) bool {
	if !w.partialRepaint {
		return false
	}
	off, size := common.PaintExtent(obj)
	return w.shouldSkipForDirtyRegion(pos.Add(off), size)
}

// pushBoundary handles a RepaintBoundary node in the draw pass. It returns true
// when the node was a boundary (and thus fully handled here).
func (w *mobileTreeWalker) pushBoundary(node *common.RenderCacheNode, pos geo.Position) bool {
	if !w.hasBoundary || !node.IsBoundary() {
		return false
	}

	obj := node.Obj()
	// Snapped to a device pixel so the blit is a copy, pixel for pixel (#570).
	g := common.BoundaryGeometryFor(pos, obj.Size(), w.c.Scale())
	pixW, pixH, objSize := g.PixW, g.PixH, g.Size

	if pixW > 0 && pixH > 0 {
		if w.bp.IsBoundaryClean(obj) && node.BoundaryRenderedAt(g.Phase) {
			cleanState := w.bp.EnsureBoundaryFBO(obj, pixW, pixH, objSize)
			w.bStack = append(w.bStack, mobileBoundaryEntry{
				obj: obj, state: cleanState, wasDirty: false,
				origin: g.Origin, size: objSize,
			})
			return true
		}

		// The subtree is rendered in boundary-local space with a clip stack
		// of its own; the outer clips apply to the blit. The scissor stops
		// before the bind, whose clear GL would scissor too (#559).
		state := w.bp.EnsureBoundaryFBO(obj, pixW, pixH, objSize)
		outerClips := w.clips
		w.clips = &internal.ClipStack{}
		w.c.Painter().StopClipping()
		w.bp.BindBoundaryFBO(state)
		invWT, ok := node.WorldTransform().Inverse()
		if !ok {
			invWT = geo.Identity()
		}
		invWT = geo.NewTranslation(pos.X-g.Origin.X, pos.Y-g.Origin.Y).Multiply(invWT)
		w.bStack = append(w.bStack, mobileBoundaryEntry{
			obj: obj, state: state, wasDirty: true,
			invWT: invWT, origin: g.Origin, phase: g.Phase, size: objSize,
			canvasPixW: w.canvasPixW, canvasPixH: w.canvasPixH,
			outerClips: outerClips,
		})
	}
	return true
}

func (w *mobileTreeWalker) draw(node *common.RenderCacheNode, pos geo.Position) {
	obj := node.Obj()

	// Scope the subtree's paint to a container DirectionScope's direction, the
	// paint-time mirror of the EnsureMinSize layout hook. Done before any early
	// return so it always pairs with the pop in afterDraw.
	intdriver.PushDirectionScope(obj)

	// --- RepaintBoundary handling ---
	if w.pushBoundary(node, pos) {
		return
	}

	if w.insideCleanBoundary() {
		return
	}

	paintPos := pos
	paintFrame := w.size
	paintTransform := node.WorldTransform()
	if ab := w.activeBoundary(); ab != nil && ab.wasDirty {
		paintPos = geo.NewPos(pos.X-ab.origin.X, pos.Y-ab.origin.Y)
		paintFrame = ab.size
		paintTransform = ab.invWT.Multiply(node.WorldTransform())
	}

	// Establish this object's clips BEFORE the dirty-region cull check, so a culled
	// clip-owner (scroll/ClipToBounds) still constrains its descendants; otherwise
	// its children that extend into the dirty region draw unclipped, leaking content
	// past the viewport (pre-rebrand tracker #333). afterDraw tears these down regardless of culling.
	// A clip-owner the paint walk culled has no children to constrain, and
	// gets no clip (#481).
	clips := common.PaintClips(node)
	if clips.Rect {
		inner := w.clips.Push(paintPos, obj.Size())
		w.c.Painter().StartClipping(inner.Rect())
	}
	if clips.Rounded {
		w.c.Painter().StartRoundedClipping(paintPos, obj.Size(), clips.Radius)
	}
	if clips.Path != nil {
		if clipPath, is := clips.Path.(*belluicanvas.Path); is {
			w.c.Painter().StartPathClipping(clipPath, paintPos, obj.Size())
		}
	}

	if w.activeBoundary() == nil && w.skipObjectForDirtyRegion(obj, pos) {
		return
	}

	w.c.Painter().SetTransform(paintTransform)
	if paintFrame.Width <= 0 || paintFrame.Height <= 0 {
		return
	}
	w.c.Painter().Paint(obj, paintPos, paintFrame)
}

// applyClip sets the painter's scissor to the top of the clip stack, or
// clears it when the stack is empty.
func (w *mobileTreeWalker) applyClip() {
	if top := w.clips.Top(); top != nil {
		w.c.Painter().StartClipping(top.Rect())
	} else {
		w.c.Painter().StopClipping()
	}
}

func (w *mobileTreeWalker) afterDraw(node *common.RenderCacheNode, pos geo.Position) {
	obj := node.Obj()

	// Pop the DirectionScope pushed in draw (before any early return, to pair).
	intdriver.PopDirectionScope(obj)

	if w.hasBoundary && node.IsBoundary() && len(w.bStack) > 0 && w.bStack[len(w.bStack)-1].obj == obj {
		entry := w.bStack[len(w.bStack)-1]
		w.bStack = w.bStack[:len(w.bStack)-1]

		if entry.wasDirty && entry.state != nil {
			w.bp.UnbindBoundaryFBO(entry.canvasPixW, entry.canvasPixH)
			entry.state.SetDirty(false)
			node.SetBoundaryRenderedAt(entry.phase)
			w.clips = entry.outerClips
			w.applyClip()
		}
		if !w.shouldSkipForDirtyRegion(entry.origin, entry.size) {
			w.bp.BlitBoundaryTexture(entry.state, entry.origin, w.size)
		}
		return
	}

	if w.insideCleanBoundary() {
		return
	}

	culled := w.activeBoundary() == nil && w.skipObjectForDirtyRegion(node.Obj(), pos)

	// Tear down clips even when culled: draw establishes them unconditionally for
	// clip-owners so descendants stay clipped (pre-rebrand tracker #333), so push/pop must stay paired.
	clips := common.PaintClips(node)
	if clips.Rounded {
		w.c.Painter().StopRoundedClipping()
	}
	if clips.Path != nil {
		w.c.Painter().StopPathClipping()
	}
	if clips.Rect {
		w.clips.Pop()
		w.applyClip()
	}

	if !culled && build.Mode == bellui.BuildDebug {
		w.c.DrawDebugOverlay(obj, pos, w.size)
	}
}

func (d *driver) paintWindow(win bellui.Window, size geo.Size) {
	c := win.Canvas().(*canvas)

	// --- Canvas FBO setup ---
	drp, hasDRP := c.Painter().(gpu.DirtyRegionPainter)

	canvasPixW := scale.ToScreenCoordinate(c, size.Width)
	canvasPixH := scale.ToScreenCoordinate(c, size.Height)

	// Tell the painter its output pixel size. GL derives this per-paint from the
	// frame, but the WebGPU painter needs it to configure the surface and bring
	// the device up (its Init no-ops until the size is known).
	d.setPainterOutputSize(c, canvasPixW, canvasPixH)

	// partialRepaint and drPos/drSize drive shouldSkipForDirtyRegion below.
	partialRepaint := false
	var drPos geo.Position
	var drSize geo.Size

	ring, hasRing := win.(*window).a11yFocusRing()
	if build.PartialRepaint {
		d.markA11yRingDirty(c, ring, hasRing)
	}

	if hasDRP {
		drp.EnsureCanvasFBO(canvasPixW, canvasPixH)
		drp.BindCanvasFBO()
		// Partial repaint is off by default on mobile (build.PartialRepaint),
		// until it is validated on devices as on desktop (#496): every frame
		// is a full clear and redraw. The partialrepaint tag turns it on.
		partialRepaint, drPos, drSize = clearCanvasFBO(c, drp, size)
	} else {
		r, g, b, a := theme.Color(theme.ColorNameBackground).RGBA()
		max16bit := float32(255 * 255)
		d.glctx.ClearColor(float32(r)/max16bit, float32(g)/max16bit, float32(b)/max16bit, float32(a)/max16bit)
		d.glctx.Clear(gl.ColorBufferBit)
	}

	// Check if the painter supports boundary FBO caching.
	bp, hasBoundary := c.Painter().(gpu.BoundaryPainter)

	walk := &mobileTreeWalker{
		c:              c,
		clips:          &internal.ClipStack{},
		size:           size,
		bp:             bp,
		hasBoundary:    hasBoundary,
		canvasPixW:     canvasPixW,
		canvasPixH:     canvasPixH,
		partialRepaint: partialRepaint,
		drPos:          drPos,
		drSize:         drSize,
	}

	// Scope the whole paint pass to this canvas' direction so paint-time
	// alignment and text base direction match the layout pass, mirroring the
	// GLFW driver (a no-op for the default DirectionAuto, which defers to the
	// global setting). Container DirectionScopes nest inside via the per-node
	// push in draw/afterDraw; Restore truncates the stack back to its pre-walk
	// depth even if a paint panic unwinds past those per-node Pops.
	defer uidir.Restore(uidir.Mark())
	uidir.Push(c.Direction())
	defer uidir.Pop(c.Direction())

	walk.paintClipped()

	// In-scene screen-reader focus ring (pre-rebrand tracker #759): the OS-drawn highlight cannot
	// reach the flat GPU surface, so paint our own over the accessibility-focused
	// element, on top of the content. No-op (ok=false) without the accessibility
	// bridge or when nothing is focused.
	if hasRing {
		d.paintA11yFocusRing(c, ring, size)
	}
	d.a11yRingPainted, d.a11yRingWasPainted = ring, hasRing

	// --- Blit canvas FBO to the default framebuffer ---
	if hasDRP {
		drp.UnbindAndBlitCanvasFBO(canvasPixW, canvasPixH)
	}
}

// Stroke widths (device-independent units) of the in-scene screen-reader focus
// ring: a light outer halo with a narrower dark core drawn on top, so the ring
// shows a light/dark/light edge that stays visible over any background.
const (
	a11yFocusRingHalo = 5 // light outer stroke
	a11yFocusRingCore = 2 // dark inner stroke, centred inside the halo
)

// a11yFocusRingHaloColor / a11yFocusRingCoreColor are fixed (not theme-derived)
// high-contrast colours: the point of the double ring is to be legible over
// arbitrary content, so it must not blend into a primary-coloured widget.
var (
	a11yFocusRingHaloColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	a11yFocusRingCoreColor = color.NRGBA{R: 0x14, G: 0x14, B: 0x14, A: 0xFF}
)

// paintA11yFocusRing draws the high-contrast double ring over ring (in
// canvas-space device-independent units), on top of the just-painted content,
// as the in-scene screen-reader focus indicator (pre-rebrand tracker #759). A light halo with a
// dark core centred on the same bounds reads as a focus ring against content of
// any colour — one of the two edges always contrasts (the approach VoiceOver
// uses), and unlike a theme-coloured stroke it never vanishes into a
// same-coloured widget.
// markA11yRingDirty marks where the focus ring was last painted, and where it
// is painted now, dirty for the partial repaint about to be computed. The ring
// is no canvas object, so nothing else puts it in the region: where it left,
// its pixels stayed in the retained canvas texture, and where it stays, it was
// painted over itself every frame, its anti-aliased edges thickening (#551).
// Its halo is stroked across the rect's edge, so the rect is widened by it.
func (d *driver) markA11yRingDirty(c *canvas, ring geo.Rectangle, hasRing bool) {
	mark := func(r geo.Rectangle) {
		w := float32(a11yFocusRingHalo)
		c.MarkDirtyRect(r.Pos.SubtractXY(w, w), r.Size.AddWidthHeight(2*w, 2*w))
	}
	if d.a11yRingWasPainted {
		mark(d.a11yRingPainted)
	}
	if hasRing {
		mark(ring)
	}
}

func (d *driver) paintA11yFocusRing(c *canvas, ring geo.Rectangle, size geo.Size) {
	if size.Width <= 0 || size.Height <= 0 || ring.Size.Width <= 0 || ring.Size.Height <= 0 {
		return
	}
	if d.a11yRing == nil {
		d.a11yRing = &belluicanvas.Rectangle{}
		d.a11yRingCore = &belluicanvas.Rectangle{}
	}
	radius := theme.Size(theme.SizeNameShapeSmall)

	halo := d.a11yRing
	halo.FillColor = color.Transparent
	halo.StrokeColor = a11yFocusRingHaloColor
	halo.StrokeWidth = a11yFocusRingHalo
	halo.CornerRadius = radius
	halo.Resize(ring.Size)

	core := d.a11yRingCore
	core.FillColor = color.Transparent
	core.StrokeColor = a11yFocusRingCoreColor
	core.StrokeWidth = a11yFocusRingCore
	core.CornerRadius = radius
	core.Resize(ring.Size)

	// Reset the transform the content walk left set, then paint the halo and the
	// narrower core over it, both at the element's absolute canvas position.
	c.Painter().SetTransform(geo.Identity())
	c.Painter().Paint(halo, ring.Pos, size)
	c.Painter().SetTransform(geo.Identity())
	c.Painter().Paint(core, ring.Pos, size)
}

func (d *driver) sendPaintEvent() {
	if d.painting {
		return
	}
	d.app.Send(paint.Event{})
	d.painting = true
}

func (d *driver) setTheme(dark bool) {
	var mode bellui.ThemeVariant
	if dark {
		mode = theme.VariantDark
	} else {
		mode = theme.VariantLight
	}

	if d.theme != mode && d.onConfigChanged != nil {
		d.onConfigChanged(&Configuration{SystemTheme: mode})
	}
	d.theme = mode
}

// tbPassthrough is the raw-touch gate state while an explore-by-touch screen
// reader (TalkBack) owns the touchscreen (pre-rebrand tracker #1104). One stream is considered at
// a time: a Begin-led stream is buffered until it proves intentional, then
// flows to the canvas; everything else is suppressed.
type tbPassthrough struct {
	seq            touch.Sequence // the candidate / dispatched sequence
	tracking       bool           // Begin seen, stream not yet qualified
	dispatched     bool           // stream qualified; events flow to the canvas
	startX, startY float32
	lastX, lastY   float32
	win            *window
	gen            int // arm generation for the hold-qualify timer
}

// exploreTouch gates raw touches while an explore-by-touch screen reader owns
// the touchscreen. The native input queue then delivers a mix of streams the
// framework never labels for us:
//
//   - Explore hovers: bare Move events with no Begin (modern TalkBack with
//     service-side gesture detection). Dropped — exploration is served by the
//     accessibility tree, and an explore touch must never focus or activate.
//   - Quick stationary taps: older TalkBack/Android versions deliver a plain
//     explore tap as a full Begin/End stream, which used to activate widgets
//     (pre-rebrand tracker #572). Still suppressed: a stream that ends before qualifying is
//     swallowed whole.
//   - PASSTHROUGH interactions the app must handle natively (pre-rebrand tracker #1104): the
//     two-finger scroll gesture (TalkBack requests two-finger passthrough,
//     so the gesture arrives as a real single-pointer drag and the framework
//     expects the app to scroll) and everything after double-tap-and-hold
//     ("use standard gestures" — long-press, drag).
//
// A Begin-led stream qualifies as passthrough once it moves past the tap slop
// (scroll/drag intent) or stays down for the long-press delay (hold intent);
// it then enters the normal touch pipeline at its original Begin position, so
// drag deltas stay exact and bellui's own scroll, hold-fire long-press (pre-rebrand tracker #758)
// and drag semantics apply natively from there.
func (d *driver) exploreTouch(w *window, e touch.Event) {
	t := &d.tbPass
	switch e.Type {
	case touch.TypeBegin:
		if t.tracking || t.dispatched {
			return // one stream at a time; a second finger is ignored
		}
		t.tracking = true
		t.seq = e.Sequence
		t.startX, t.startY = e.X, e.Y
		t.lastX, t.lastY = e.X, e.Y
		t.win = w
		t.gen++
		gen := t.gen
		// Qualify a stationary hold even if no further events arrive.
		time.AfterFunc(common.TapSecondaryDelay, func() {
			bellui.Do(func() { d.tbHoldQualify(gen) })
		})
	// Past its Begin, a stream goes to the window it began in (t.win), like
	// any other touch (#392): the current window may have changed since.
	case touch.TypeMove:
		if t.dispatched && e.Sequence == t.seq {
			d.tapMoveCanvas(t.win, e.X, e.Y, e.Sequence)
			return
		}
		if !t.tracking || e.Sequence != t.seq {
			return // explore hover (never began) or a second finger
		}
		t.lastX, t.lastY = e.X, e.Y
		if !d.tbPastSlop(t.win, e.X, e.Y) {
			return
		}
		// Passthrough scroll/drag: enter the pipeline at the Begin point so
		// the accumulated delta is preserved, then continue live.
		t.tracking = false
		t.dispatched = true
		d.tapDownCanvas(t.win, t.startX, t.startY, t.seq)
		d.tapMoveCanvas(t.win, e.X, e.Y, e.Sequence)
	case touch.TypeEnd, touch.TypeCancel:
		if t.dispatched && e.Sequence == t.seq {
			t.dispatched = false
			if e.Type == touch.TypeCancel {
				d.tapCancelCanvas(t.win, e.X, e.Y, e.Sequence)
			} else {
				d.tapUpCanvas(t.win, e.X, e.Y, e.Sequence)
			}
			return
		}
		if t.tracking && e.Sequence == t.seq {
			t.tracking = false // unqualified quick tap: suppressed (pre-rebrand tracker #572)
		}
	}
}

// tbHoldQualify dispatches a still-held, still-stationary gated stream after
// the long-press delay — the double-tap-and-hold passthrough. Runs on the
// main goroutine via bellui.Do; gen guards against a stream that ended or
// qualified meanwhile.
func (d *driver) tbHoldQualify(gen int) {
	t := &d.tbPass
	if !t.tracking || t.gen != gen || d.currentWindow() != t.win {
		return
	}
	t.tracking = false
	t.dispatched = true
	d.tapDownCanvas(t.win, t.startX, t.startY, t.seq)
	if t.lastX != t.startX || t.lastY != t.startY {
		d.tapMoveCanvas(t.win, t.lastX, t.lastY, t.seq)
	}
}

// tbPastSlop reports whether the gated stream has moved past the tap slop,
// compared in dp like the touch handler's own drag threshold.
func (d *driver) tbPastSlop(w *window, x, y float32) bool {
	t := &d.tbPass
	dx := scale.ToBelluiCoordinate(w.canvas, int(x)) - scale.ToBelluiCoordinate(w.canvas, int(t.startX))
	dy := scale.ToBelluiCoordinate(w.canvas, int(y)) - scale.ToBelluiCoordinate(w.canvas, int(t.startY))
	return math.Abs(float64(dx)) >= common.TapMoveThreshold || math.Abs(float64(dy)) >= common.TapMoveThreshold
}

// routeTouch sends a touch event to exploreTouch while an explore-by-touch
// screen reader is on, and to handleTouch otherwise. The choice is made when
// a touch begins and holds for the rest of it: a screen reader switched on or
// off mid-touch would otherwise send the touch's release to the path that
// never saw it go down, which ignores it, and leave the touch latched as down
// (a drag, a scroll or a pending long press) in the path that did.
func (d *driver) routeTouch(current *window, e touch.Event, exploring bool) {
	if e.Type != touch.TypeBegin {
		if _, ok := d.touchWindows[e.Sequence]; ok {
			d.handleTouch(current, e)
			return
		}
		if t := &d.tbPass; (t.tracking || t.dispatched) && t.seq == e.Sequence {
			d.exploreTouch(current, e)
			return
		}
	}
	if exploring {
		d.exploreTouch(current, e)
		return
	}
	d.handleTouch(current, e)
}

// handleTouch delivers a touch event to the window it began in. A touch keeps
// its window for its whole sequence: if the current window changes while a
// finger is down, its moves and release still reach the canvas that saw it go
// down, whose touch handler is waiting for them (#392).
func (d *driver) handleTouch(current *window, e touch.Event) {
	if e.Type == touch.TypeBegin {
		if d.touchWindows == nil {
			d.touchWindows = make(map[touch.Sequence]*window)
		}
		d.touchWindows[e.Sequence] = current
		d.tapDownCanvas(current, e.X, e.Y, e.Sequence)
		return
	}

	w, ok := d.touchWindows[e.Sequence]
	if !ok {
		w = current
	}
	if e.Type != touch.TypeMove {
		delete(d.touchWindows, e.Sequence)
	}
	if w == nil {
		return // the window it began in closed, which cancelled the touch
	}
	switch e.Type {
	case touch.TypeMove:
		d.tapMoveCanvas(w, e.X, e.Y, e.Sequence)
	case touch.TypeEnd:
		d.tapUpCanvas(w, e.X, e.Y, e.Sequence)
	case touch.TypeCancel:
		d.tapCancelCanvas(w, e.X, e.Y, e.Sequence)
	}
}

// cancelWindowTouches ends every touch that began in w, which is closing: its
// canvas cancels them while its widgets still exist, and the rest of each
// sequence is dropped. Delivered there, a release would hit-test the closed
// window's content and could tap a button nobody can see; delivered to the
// current window, it would reach a handler that never saw the touch go down.
func (d *driver) cancelWindowTouches(w *window) {
	for seq, began := range d.touchWindows {
		if began == w {
			w.canvas.TapCancel(geo.Position{}, int(seq))
			d.touchWindows[seq] = nil
		}
	}

	t := &d.tbPass
	if t.win != w {
		return
	}
	if t.dispatched {
		w.canvas.TapCancel(geo.Position{}, int(t.seq))
	}
	// The rest of the stream is ignored like a second finger's.
	t.tracking, t.dispatched = false, false
	t.win = nil
}

// canvasPos converts a touch position in device pixels to a position on w's
// canvas.
func canvasPos(w *window, x, y float32) geo.Position {
	tapX := scale.ToBelluiCoordinate(w.canvas, int(x))
	tapY := scale.ToBelluiCoordinate(w.canvas, int(y))
	return geo.NewPos(tapX, tapY+tapYOffset)
}

// tapCancelCanvas ends a touch the system took away: no tap, long press or
// drag completes (#392).
func (d *driver) tapCancelCanvas(w *window, x, y float32, tapID touch.Sequence) {
	w.canvas.TapCancel(canvasPos(w, x, y), int(tapID))
}

func (d *driver) tapDownCanvas(w *window, x, y float32, tapID touch.Sequence) {
	d.coast.Stop() // a new touch stops a coasting drag dead (#751)
	w.canvas.TapDown(canvasPos(w, x, y), int(tapID))
}

func (d *driver) tapMoveCanvas(w *window, x, y float32, tapID touch.Sequence) {
	w.canvas.TapMove(canvasPos(w, x, y), int(tapID), func(wid event.Draggable, ev *event.DragEvent) {
		wid.Dragged(ev)
	})
}

func (d *driver) tapUpCanvas(w *window, x, y float32, tapID touch.Sequence) {
	w.canvas.TapUp(canvasPos(w, x, y), int(tapID), func(wid event.Tappable, ev *event.PointEvent) {
		wid.Tapped(ev)
		if co, ok := wid.(core.CanvasObject); ok {
			core.NotifyActivation(co, false)
		}
	}, func(wid event.SecondaryTappable, ev *event.PointEvent) {
		wid.TappedSecondary(ev)
	}, func(wid event.LongPressable, ev *event.PointEvent) {
		wid.LongPressed(ev)
	}, func(wid event.DoubleTappable, ev *event.PointEvent) {
		wid.DoubleTapped(ev)
	}, func(wid event.Draggable, ev *event.DragEvent) {
		d.coast.Release(wid, ev, func(f func()) { d.DoFromGoroutine(f, false) })
	})
}

var keyCodeMap = map[key.Code]event.KeyName{
	// non-printable
	key.CodeEscape:          event.KeyEscape,
	key.CodeReturnEnter:     event.KeyReturn,
	key.CodeTab:             event.KeyTab,
	key.CodeDeleteBackspace: event.KeyBackspace,
	key.CodeInsert:          event.KeyInsert,
	key.CodePageUp:          event.KeyPageUp,
	key.CodePageDown:        event.KeyPageDown,
	key.CodeHome:            event.KeyHome,
	key.CodeEnd:             event.KeyEnd,

	key.CodeLeftArrow:  event.KeyLeft,
	key.CodeRightArrow: event.KeyRight,
	key.CodeUpArrow:    event.KeyUp,
	key.CodeDownArrow:  event.KeyDown,

	key.CodeF1:  event.KeyF1,
	key.CodeF2:  event.KeyF2,
	key.CodeF3:  event.KeyF3,
	key.CodeF4:  event.KeyF4,
	key.CodeF5:  event.KeyF5,
	key.CodeF6:  event.KeyF6,
	key.CodeF7:  event.KeyF7,
	key.CodeF8:  event.KeyF8,
	key.CodeF9:  event.KeyF9,
	key.CodeF10: event.KeyF10,
	key.CodeF11: event.KeyF11,
	key.CodeF12: event.KeyF12,

	key.CodeKeypadEnter: event.KeyEnter,

	// printable
	key.CodeA:       event.KeyA,
	key.CodeB:       event.KeyB,
	key.CodeC:       event.KeyC,
	key.CodeD:       event.KeyD,
	key.CodeE:       event.KeyE,
	key.CodeF:       event.KeyF,
	key.CodeG:       event.KeyG,
	key.CodeH:       event.KeyH,
	key.CodeI:       event.KeyI,
	key.CodeJ:       event.KeyJ,
	key.CodeK:       event.KeyK,
	key.CodeL:       event.KeyL,
	key.CodeM:       event.KeyM,
	key.CodeN:       event.KeyN,
	key.CodeO:       event.KeyO,
	key.CodeP:       event.KeyP,
	key.CodeQ:       event.KeyQ,
	key.CodeR:       event.KeyR,
	key.CodeS:       event.KeyS,
	key.CodeT:       event.KeyT,
	key.CodeU:       event.KeyU,
	key.CodeV:       event.KeyV,
	key.CodeW:       event.KeyW,
	key.CodeX:       event.KeyX,
	key.CodeY:       event.KeyY,
	key.CodeZ:       event.KeyZ,
	key.Code0:       event.Key0,
	key.CodeKeypad0: event.Key0,
	key.Code1:       event.Key1,
	key.CodeKeypad1: event.Key1,
	key.Code2:       event.Key2,
	key.CodeKeypad2: event.Key2,
	key.Code3:       event.Key3,
	key.CodeKeypad3: event.Key3,
	key.Code4:       event.Key4,
	key.CodeKeypad4: event.Key4,
	key.Code5:       event.Key5,
	key.CodeKeypad5: event.Key5,
	key.Code6:       event.Key6,
	key.CodeKeypad6: event.Key6,
	key.Code7:       event.Key7,
	key.CodeKeypad7: event.Key7,
	key.Code8:       event.Key8,
	key.CodeKeypad8: event.Key8,
	key.Code9:       event.Key9,
	key.CodeKeypad9: event.Key9,

	key.CodeSemicolon: event.KeySemicolon,
	key.CodeEqualSign: event.KeyEqual,

	key.CodeSpacebar:           event.KeySpace,
	key.CodeApostrophe:         event.KeyApostrophe,
	key.CodeComma:              event.KeyComma,
	key.CodeHyphenMinus:        event.KeyMinus,
	key.CodeKeypadHyphenMinus:  event.KeyMinus,
	key.CodeFullStop:           event.KeyPeriod,
	key.CodeKeypadFullStop:     event.KeyPeriod,
	key.CodeSlash:              event.KeySlash,
	key.CodeLeftSquareBracket:  event.KeyLeftBracket,
	key.CodeBackslash:          event.KeyBackslash,
	key.CodeRightSquareBracket: event.KeyRightBracket,
	key.CodeGraveAccent:        event.KeyBackTick,

	key.CodeBackButton: mobile.KeyBack,
}

func keyToName(code key.Code) event.KeyName {
	ret, ok := keyCodeMap[code]
	if !ok {
		return ""
	}

	return ret
}

// keyEventModifier converts the gomobile modifier bitmask into the
// cross-platform event.KeyModifier carried on event.KeyEvent (pre-rebrand tracker #1042).
func keyEventModifier(mod key.Modifiers) event.KeyModifier {
	var m event.KeyModifier
	if mod&key.ModShift != 0 {
		m |= event.KeyModifierShift
	}
	if mod&key.ModControl != 0 {
		m |= event.KeyModifierControl
	}
	if mod&key.ModAlt != 0 {
		m |= event.KeyModifierAlt
	}
	if mod&key.ModMeta != 0 {
		m |= event.KeyModifierSuper
	}
	return m
}

func runeToPrintable(r rune) rune {
	if strconv.IsPrint(r) {
		return r
	}

	return 0
}

func (d *driver) typeDownCanvas(canvas *canvas, r rune, code key.Code, mod key.Modifiers) {
	d.setKeyMods(keyEventModifier(mod))
	keyName := keyToName(code)
	if canvas.tabEscape.MoveTabFocus(canvas, keyName, keyEventModifier(mod)) {
		return
	}

	// Hardware-keyboard editing shortcuts (Ctrl+A/C/V/X/Z/Y). On Android these
	// arrive via the IME's sendKeyEvent, forwarded through keyboardShortcut(...)
	// as a modified key event (pre-rebrand tracker #1077) — the mobile mirror of the desktop
	// driver's triggersShortcut. Fire only when the modifier is EXACTLY the
	// shortcut modifier (Control, or Meta/Cmd), never combined with Shift — so a
	// Shift+Ctrl+key combo still reaches TypedKey with its modifier (pre-rebrand tracker #1042).
	// Dispatch to the focused Shortcutable and stop, so it never also fires
	// TypedKey/TypedRune.
	if mod == key.ModControl || mod == key.ModMeta {
		if sc := common.BuiltinKeyboardShortcut(keyName, NewClipboard()); sc != nil {
			if focused, ok := canvas.Focused().(bellui.Shortcutable); ok {
				run := true
				type selectableText interface {
					core.Disableable
					SelectedText() string
				}
				if st, ok := focused.(selectableText); ok && st.Disabled() {
					run = sc.ShortcutName() == "Copy" // a disabled field only allows Copy
				}
				if run {
					focused.TypedShortcut(sc)
				}
			} else {
				canvas.TypedShortcut(sc)
			}
			return
		}
	}

	r = runeToPrintable(r)
	if r > 0 {
		canvas.tabEscape.Disarm() // the key may have no name, but it typed text
	}
	keyEvent := &event.KeyEvent{Name: keyName, Modifier: keyEventModifier(mod)}

	// Escape closes the innermost pop-up, menu or dialog — the same default the
	// desktop driver applies — unless the focused widget is using the key for a
	// state of its own. Android's Back button carries the same meaning (#259):
	// backing out of the innermost surface is what Back does everywhere else on
	// the platform, so it only falls through to KeyBack delivery / GoBack below
	// when there is nothing to dismiss.
	if keyName == event.KeyEscape || keyName == mobile.KeyBack {
		if intdriver.HandleEscape(canvas) {
			return
		}
		if keyName == event.KeyEscape {
			canvas.tabEscape.EscapeNotConsumed(canvas.Focused())
		}
		// The walk declines when the focused widget claims the Escape meaning
		// for a state of its own (event.EscapeHandler) — but such widgets act
		// on KeyEscape in TypedKey, so Back must arrive as the key they listen
		// for or it would go dead exactly when the widget has state to back
		// out of.
		if keyName == mobile.KeyBack {
			if h, ok := canvas.Focused().(event.EscapeHandler); ok && h.HandlesEscape() {
				keyEvent.Name = event.KeyEscape
			}
		}
	}

	if canvas.Focused() != nil {
		if keyName != "" {
			// Keyboard-activation parity with taps (pre-rebrand tracker #700).
			if act, ok := canvas.Focused().(event.Activatable); ok && act.ActivatedByKey(keyEvent) {
				if co, ok := canvas.Focused().(core.CanvasObject); ok {
					core.NotifyActivation(co, true)
				}
			}
			canvas.Focused().TypedKey(keyEvent)
		}
		if r > 0 {
			canvas.Focused().TypedRune(r)
		}
	} else {
		if keyName != "" {
			if canvas.onTypedKey != nil {
				canvas.onTypedKey(keyEvent)
			} else if keyName == mobile.KeyBack {
				d.GoBack()
			}
		}
		if r > 0 && canvas.onTypedRune != nil {
			canvas.onTypedRune(r)
		}
	}
}

func (d *driver) typeUpCanvas(_ *canvas, _ rune, _ key.Code, _ key.Modifiers) {
}

// CurrentKeyModifiers reports the hardware-keyboard modifier set carried by the
// most recent key event, so widgets can read the live Shift/Ctrl/... state
// (e.g. Entry's Shift+Arrow selection, pre-rebrand tracker #1083). Implements
// desktop.KeyModifierDriver on mobile.
func (d *driver) CurrentKeyModifiers() event.KeyModifier {
	return d.keyMods
}

// AddKeyModifierListener registers fn to be called when the active modifier set
// changes. Part of desktop.KeyModifierDriver.
func (d *driver) AddKeyModifierListener(fn func(event.KeyModifier)) (remove func()) {
	d.keyModListeners = append(d.keyModListeners, fn)
	idx := len(d.keyModListeners) - 1
	return func() { d.keyModListeners[idx] = nil }
}

func (d *driver) setKeyMods(m event.KeyModifier) {
	if m == d.keyMods {
		return
	}
	d.keyMods = m
	for _, fn := range d.keyModListeners {
		if fn != nil {
			fn(m)
		}
	}
}

func (d *driver) Device() bellui.Device {
	return &d.device
}

func (d *driver) SetOnConfigurationChanged(f func(*Configuration)) {
	d.onConfigChanged = f
}

func (d *driver) DoubleTapDelay() time.Duration {
	return common.TapDoubleDelay
}

// AnnounceToScreenReader implements the optional bellui.AccessibilityAnnouncer
// interface by fanning the message out to the registered accessibility
// backends (pre-rebrand tracker #525). With -tags accessibility the iOS and
// Android backends register a listener in their init (accessibility_ios.go,
// accessibility_android.go) that hands the text to VoiceOver / TalkBack;
// without that tag no listener is registered and this is a no-op.
func (d *driver) AnnounceToScreenReader(text string, assertive bool) {
	accessibility.Announce(text, assertive)
}

// NewGoMobileDriver sets up a new Driver instance implemented using the Go
// Mobile extension and OpenGL bindings.
func NewGoMobileDriver() bellui.Driver {
	// Register the main goroutine here, at app construction. gomobile runs the
	// user's main.main on a dedicated goroutine (callMain -> `go callfn.CallFn`)
	// and this constructor is the first bellui call on it (app.New, before any UI
	// is built), so registering now means every subsequent GUI mutation the app
	// makes during setup is recognised as on-main. Registering later (in Run's
	// loop) left the whole initial build tripping the bellui.Do guard (pre-rebrand tracker #193). This
	// mirrors the desktop driver, which registers in init(). The Run loop also
	// runs on this goroutine (see app.main under WebGPU), so it re-registers the
	// same goroutine harmlessly.
	async.SetMainGoroutine()

	d := &driver{
		theme:     bellui.ThemeVariant(2), // unspecified
		textInput: textinput.NewSession(keyboardStateBackend{}),
	}

	registerRepository(d)
	return d
}
