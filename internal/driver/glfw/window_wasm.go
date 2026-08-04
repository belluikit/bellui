//go:build wasm

package glfw

import (
	"context"
	_ "image/png" // for the icon
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/scale"
	internalWidget "github.com/belluikit/bellui/internal/widget"

	"github.com/belluikit/glfw-js"
)

type Cursor struct {
	JSName string
}

const defaultTitle = "Bellui Application"

// Input modes.
const (
	CursorMode             glfw.InputMode = glfw.CursorMode
	StickyKeysMode         glfw.InputMode = glfw.StickyKeysMode
	StickyMouseButtonsMode glfw.InputMode = glfw.StickyMouseButtonsMode
	LockKeyMods            glfw.InputMode = glfw.LockKeyMods
	RawMouseMotion         glfw.InputMode = glfw.RawMouseMotion
)

// Cursor mode values.
const (
	CursorNormal   int = glfw.CursorNormal
	CursorHidden   int = glfw.CursorHidden
	CursorDisabled int = glfw.CursorDisabled
)

// Declare conformity to Window interface
var _ bellui.Window = (*window)(nil)

type window struct {
	viewport  *glfw.Window
	created   bool
	decorate  bool
	closing   bool
	fixedSize bool

	cursor   desktop.Cursor
	canvas   *glCanvas
	driver   *gLDriver
	title    string
	icon     bellui.Resource
	mainmenu *bellui.MainMenu

	master              bool
	fullScreen          bool
	fullScreenSecondary bool
	centered            bool
	visible             bool

	mousePos             geo.Position
	mouseDragged         event.Draggable
	mouseDraggedObjStart geo.Position
	mouseDraggedOffset   geo.Position
	mouseDragPos         geo.Position
	mouseDragStarted     bool
	dragGhost            *internalWidget.DragGhost // ghost overlay following the pointer during a drag (#819)
	dndDrag              bool                      // the active drag opted into in-canvas DnD (#819)
	dragPayload          any                       // source-declared payload carried to DragTargets
	dragTarget           core.DragTarget           // current DragTarget under the pointer
	mouseButton          desktop.MouseButton
	mouseOver            event.Hoverable
	tooltipObj           core.CanvasObject // current driver-level tooltip target under the pointer (#800)
	mouseLastClick       core.CanvasObject
	mousePressed         core.CanvasObject
	mouseClickCount      int
	mouseCancelFunc      context.CancelFunc

	// Press-and-hold long-press dispatch (#266); see window_desktop.go.
	longPressSeq   int
	longPressPos   geo.Position
	longPressFired core.CanvasObject

	onClosed           func()
	onCloseIntercepted func()

	menuTogglePending       event.KeyName
	menuDeactivationPending event.KeyName

	xpos, ypos                      int
	width, height                   int
	requestedWidth, requestedHeight int
	shouldWidth, shouldHeight       int
	shouldExpand                    bool

	pending []func()

	lastWalkedTime time.Time
}

func (w *window) SetFullScreen(full bool) {
	w.fullScreen = true
}

// applyParent is a no-op on the single-window web target; the shared Show path
// calls it after a window maps. Web windows do not implement desktop.ChildWindow.
func (w *window) applyParent() {}

// applyModal and releaseModal are no-ops on the single-window web target; the
// shared Show/destroyWindow paths call them. Web windows do not implement
// desktop.ModalWindow, so native modality never engages here.
func (w *window) applyModal()   {}
func (w *window) releaseModal() {}

// applyAttention is a no-op on the web target; the shared Show path calls it.
// Web windows do not implement desktop.AttentionWindow.
func (w *window) applyAttention() {}

// applyProgress is a no-op on the web target; the shared Show path calls it.
// Web windows do not implement desktop.ProgressWindow.
func (w *window) applyProgress() {}

// centerOnScreen handles the logic for centering a window
func (w *window) CenterOnScreen() {
	// FIXME: not supported with WebGL
	w.centered = true
}

func (w *window) SetOnDropped(dropped func(pos geo.Position, items []bellui.URI)) {
	// FIXME: not implemented yet
}

func (w *window) doCenterOnScreen() {
	// FIXME: no meaning for defining center on screen in WebGL
}

func (w *window) RequestFocus() {
	// FIXME: no meaning for defining focus in WebGL
}

func (w *window) SetIcon(icon bellui.Resource) {
	// FIXME: no support for SetIcon yet
}

func (w *window) SetMaster() {
	// FIXME: there could really only be one window
}

func (w *window) fitContent() {
	w.shouldWidth, w.shouldHeight = w.requestedWidth, w.requestedHeight
}

type monitor = glfw.Monitor

func (w *window) getMonitorForWindow() *glfw.Monitor {
	return glfw.GetPrimaryMonitor()
}

func (w *window) getSecondaryMonitor() *glfw.Monitor {
	return glfw.GetPrimaryMonitor()
}

func scaleForDpi(xdpi int) float32 {
	switch {
	case xdpi > 1000:
		// assume that this is a mistake and bail
		return float32(1.0)
	case xdpi > 192:
		return float32(1.5)
	case xdpi > 144:
		return float32(1.35)
	case xdpi > 120:
		return float32(1.2)
	default:
		return float32(1.0)
	}
}

func (w *window) detectScale() float32 {
	return scaleForDpi(int(96))
}

func (w *window) moved(_ *glfw.Window, x, y int) {
	runOnMainAsync(func() {
		w.processMoved(x, y)
	})
}

func (w *window) resized(_ *glfw.Window, width, height int) {
	runOnMainAsync(func() {
		w.canvas.scale = w.calculatedScale()
		w.processResized(width, height)
	})
}

func (w *window) frameSized(_ *glfw.Window, width, height int) {
	runOnMainAsync(func() {
		w.processFrameSized(width, height)
	})
}

func (w *window) refresh(_ *glfw.Window) {
	runOnMainAsync(w.processRefresh)
}

func (w *window) closed(viewport *glfw.Window) {
	runOnMainAsync(func() {
		viewport.SetShouldClose(false) // reset the closed flag until we check the veto in processClosed

		w.processClosed()
	})
}

func belluiToNativeCursor(cursor desktop.Cursor) (*Cursor, bool) {
	if _, ok := cursor.(desktop.StandardCursor); !ok {
		return nil, false // Custom cursors not implemented yet.
	}

	name := "default"
	switch cursor {
	case desktop.TextCursor:
		name = "text"
	case desktop.CrosshairCursor:
		name = "crosshair"
	case desktop.DefaultCursor:
		name = "default"
	case desktop.PointerCursor:
		name = "pointer"
	case desktop.HResizeCursor:
		name = "ew-resize"
	case desktop.VResizeCursor:
		name = "ns-resize"
	case desktop.HiddenCursor:
		name = "none"
	}

	return &Cursor{JSName: name}, false
}

func (w *window) SetCursor(cursor *Cursor) {
	setCursor(cursor.JSName)
}

func (w *window) setCustomCursor(rawCursor *Cursor, isCustomCursor bool) {
}

func (w *window) mouseMoved(viewport *glfw.Window, xpos, ypos float64) {
	// DOM pointer events carry the modifier state, so a move refreshes the
	// window-wide modifiers too — covering modifiers that changed while the
	// canvas lacked key focus (e.g. a key held before the pointer entered).
	current := wasmCurrentKeyModifiers(viewport.GetCurrentMods())
	runOnMainAsync(func() {
		w.driver.setCurrentKeyModifiers(current)
		w.processMouseMoved(w.scaleInput(xpos), w.scaleInput(ypos))
	})
}

func (w *window) mouseClicked(viewport *glfw.Window, btn glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
	current := wasmCurrentKeyModifiers(mods)
	runOnMainAsync(func() {
		w.driver.setCurrentKeyModifiers(current)
		button, modifiers := convertMouseButton(btn, mods)
		mouseAction := convertAction(action)

		w.processMouseClicked(button, mouseAction, modifiers)
	})
}

func (w *window) mouseScrolled(viewport *glfw.Window, xoff, yoff float64) {
	runOnMainAsync(func() {
		if xoff == 0 &&
			(viewport.GetKey(glfw.KeyLeftShift) == glfw.Press ||
				viewport.GetKey(glfw.KeyRightShift) == glfw.Press) {
			xoff, yoff = yoff, xoff
		}

		w.processMouseScrolled(xoff, yoff)
	})
}

func convertMouseButton(btn glfw.MouseButton, mods glfw.ModifierKey) (desktop.MouseButton, event.KeyModifier) {
	modifier := desktopModifier(mods)
	rightClick := false
	if isMacOSRuntime() {
		if modifier&event.KeyModifierControl != 0 {
			rightClick = true
			modifier &^= event.KeyModifierControl
		}
		if modifier&event.KeyModifierSuper != 0 {
			modifier |= event.KeyModifierControl
			modifier &^= event.KeyModifierSuper
		}
	}

	switch btn {
	case glfw.MouseButton1:
		if rightClick {
			return desktop.MouseButtonSecondary, modifier
		}
		return desktop.MouseButtonPrimary, modifier
	case glfw.MouseButton2:
		return desktop.MouseButtonSecondary, modifier
	case glfw.MouseButton3:
		return desktop.MouseButtonTertiary, modifier
	default:
		return 0, modifier
	}
}

//gocyclo:ignore
func glfwKeyToKeyName(key glfw.Key) event.KeyName {
	switch key {
	// numbers - lookup by code to avoid AZERTY using the symbol name instead of number
	case glfw.Key0, glfw.KeyKP0:
		return event.Key0
	case glfw.Key1, glfw.KeyKP1:
		return event.Key1
	case glfw.Key2, glfw.KeyKP2:
		return event.Key2
	case glfw.Key3, glfw.KeyKP3:
		return event.Key3
	case glfw.Key4, glfw.KeyKP4:
		return event.Key4
	case glfw.Key5, glfw.KeyKP5:
		return event.Key5
	case glfw.Key6, glfw.KeyKP6:
		return event.Key6
	case glfw.Key7, glfw.KeyKP7:
		return event.Key7
	case glfw.Key8, glfw.KeyKP8:
		return event.Key8
	case glfw.Key9, glfw.KeyKP9:
		return event.Key9

	// non-printable
	case glfw.KeyEscape:
		return event.KeyEscape
	case glfw.KeyEnter:
		return event.KeyReturn
	case glfw.KeyTab:
		return event.KeyTab
	case glfw.KeyBackspace:
		return event.KeyBackspace
	case glfw.KeyInsert:
		return event.KeyInsert
	case glfw.KeyDelete:
		return event.KeyDelete
	case glfw.KeyRight:
		return event.KeyRight
	case glfw.KeyLeft:
		return event.KeyLeft
	case glfw.KeyDown:
		return event.KeyDown
	case glfw.KeyUp:
		return event.KeyUp
	case glfw.KeyPageUp:
		return event.KeyPageUp
	case glfw.KeyPageDown:
		return event.KeyPageDown
	case glfw.KeyHome:
		return event.KeyHome
	case glfw.KeyEnd:
		return event.KeyEnd

	case glfw.KeySpace:
		return event.KeySpace
	case glfw.KeyKPEnter:
		return event.KeyEnter

	// desktop
	case glfw.KeyLeftShift:
		return desktop.KeyShiftLeft
	case glfw.KeyRightShift:
		return desktop.KeyShiftRight
	case glfw.KeyLeftControl:
		return desktop.KeyControlLeft
	case glfw.KeyRightControl:
		return desktop.KeyControlRight
	case glfw.KeyLeftAlt:
		return desktop.KeyAltLeft
	case glfw.KeyRightAlt:
		return desktop.KeyAltRight
	case glfw.KeyLeftSuper:
		return desktop.KeySuperLeft
	case glfw.KeyRightSuper:
		return desktop.KeySuperRight
	case glfw.KeyMenu:
		return desktop.KeyMenu
	case glfw.KeyPrintScreen:
		return desktop.KeyPrintScreen
	case glfw.KeyCapsLock:
		return desktop.KeyCapsLock

	// functions
	case glfw.KeyF1:
		return event.KeyF1
	case glfw.KeyF2:
		return event.KeyF2
	case glfw.KeyF3:
		return event.KeyF3
	case glfw.KeyF4:
		return event.KeyF4
	case glfw.KeyF5:
		return event.KeyF5
	case glfw.KeyF6:
		return event.KeyF6
	case glfw.KeyF7:
		return event.KeyF7
	case glfw.KeyF8:
		return event.KeyF8
	case glfw.KeyF9:
		return event.KeyF9
	case glfw.KeyF10:
		return event.KeyF10
	case glfw.KeyF11:
		return event.KeyF11
	case glfw.KeyF12:
		return event.KeyF12
	}

	return event.KeyUnknown
}

func keyCodeToKeyName(code string) event.KeyName {
	if len(code) != 1 {
		return event.KeyUnknown
	}

	char := code[0]
	if char >= 'a' && char <= 'z' {
		// Our alphabetical keys are all upper case characters.
		return event.KeyName('A' + char - 'a')
	}

	switch char {
	case '[':
		return event.KeyLeftBracket
	case '\\':
		return event.KeyBackslash
	case ']':
		return event.KeyRightBracket
	case '\'':
		return event.KeyApostrophe
	case ',':
		return event.KeyComma
	case '-':
		return event.KeyMinus
	case '.':
		return event.KeyPeriod
	case '/':
		return event.KeySlash
	case '*':
		return event.KeyAsterisk
	case '`':
		return event.KeyBackTick
	case ';':
		return event.KeySemicolon
	case '+':
		return event.KeyPlus
	case '=':
		return event.KeyEqual
	}

	return event.KeyUnknown
}

func keyToName(code glfw.Key, scancode int) event.KeyName {
	ret := glfwKeyToKeyName(code)
	if ret != event.KeyUnknown {
		return ret
	}

	//	keyName := glfw.GetKeyName(code, scancode)
	//	return keyCodeToKeyName(keyName)
	return event.KeyUnknown
}

func convertAction(action glfw.Action) action {
	switch action {
	case glfw.Press:
		return press
	case glfw.Release:
		return release
	case glfw.Repeat:
		return repeat
	}
	// An unexpected action value (driver/version drift) must not panic the input
	// loop; treat it as a release (the safe no-op-ish default) and log it.
	bellui.LogError("unexpected glfw.Action; treating as release", nil)
	return release
}

func convertASCII(key glfw.Key) event.KeyName {
	if key < glfw.KeyA || key > glfw.KeyZ {
		return event.KeyUnknown
	}

	return event.KeyName(rune(key))
}

func (w *window) keyPressed(viewport *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	keyName := keyToName(key, scancode)
	keyDesktopModifier := desktopModifier(mods)
	// DOM key events already include the pressed modifier on keydown and
	// exclude it on keyup, so no desktopModifierCorrected-style fixup is
	// needed here (the X11 quirk it works around does not exist in browsers).
	current := wasmCurrentKeyModifiers(mods)
	keyAction := convertAction(action)
	keyASCII := convertASCII(key)

	runOnMainAsync(func() {
		w.driver.setCurrentKeyModifiers(current)
		w.processKeyPressed(keyName, keyASCII, scancode, keyAction, keyDesktopModifier)
	})
}

// wasmCurrentKeyModifiers converts browser modifier bits into the window-wide
// modifier state. On a macOS browser the Command key reports as Super; it is
// swapped with Control so state checks against the platform shortcut modifier
// (Control on the web target) match the key macOS users actually hold —
// mirroring the swap convertMouseButton applies on the mouse path.
func wasmCurrentKeyModifiers(mods glfw.ModifierKey) event.KeyModifier {
	m := desktopModifier(mods)
	if isMacOSRuntime() {
		super := m&event.KeyModifierSuper != 0
		control := m&event.KeyModifierControl != 0
		m &^= event.KeyModifierSuper | event.KeyModifierControl
		if super {
			m |= event.KeyModifierControl
		}
		if control {
			m |= event.KeyModifierSuper
		}
	}
	return m
}

func desktopModifier(mods glfw.ModifierKey) event.KeyModifier {
	var m event.KeyModifier
	if (mods & glfw.ModShift) != 0 {
		m |= event.KeyModifierShift
	}
	if (mods & glfw.ModControl) != 0 {
		m |= event.KeyModifierControl
	}
	if (mods & glfw.ModAlt) != 0 {
		m |= event.KeyModifierAlt
	}
	if (mods & glfw.ModSuper) != 0 {
		m |= event.KeyModifierSuper
	}
	return m
}

// charInput defines the character with modifiers callback which is called when a
// Unicode character is input regardless of what modifier keys are used.
//
// Characters do not map 1:1 to physical keys, as a key may produce zero, one or more characters.
func (w *window) charInput(viewport *glfw.Window, char rune) {
	// Composed text arrives via the IME composition events, not as plain runes;
	// drop char events while an input method is composing to avoid double input.
	if imeComposing {
		return
	}
	runOnMainAsync(func() {
		w.processCharInput(char)
	})
}

// processComposeCommit forwards a committed IME composition to the focused
// object when it supports composition. A no-op otherwise. (On the web a
// compositionend with empty data commits "", which acts as a cancel — so no
// separate cancel is wired on this path.)
func (w *window) processComposeCommit(text string) {
	if c, ok := w.canvas.Focused().(event.Composable); ok {
		c.ComposeCommit(text)
	}
}

func (w *window) focused(_ *glfw.Window, focused bool) {
	runOnMainAsync(func() {
		w.processFocused(focused)
	})
}

func (w *window) DetachCurrentContext() {
	glfw.DetachCurrentContext()
}

func (w *window) RescaleContext() {
	if w.viewport == nil {
		return
	}

	w.width, w.height = w.viewport.GetSize()
	scaledFull := geo.NewSize(
		scale.ToBelluiCoordinate(w.canvas, w.width),
		scale.ToBelluiCoordinate(w.canvas, w.height),
	)
	w.canvas.Resize(scaledFull)

	// Ensure textures re-rasterize at the new scale
	cache.DeleteTextTexturesFor(w.canvas)
	w.canvas.content.Refresh()
}

func (w *window) create() {
	// we can't hide the window in webgl, so there might be some artifact
	initWindowHints()
	applyBackendWindowHints() // WebGPU requests a context-less (NoAPI) canvas; no-op for GL

	pixWidth, pixHeight := w.screenSize(w.canvas.size)
	pixWidth = int(geo.Max(float32(pixWidth), float32(w.width)))
	if pixWidth == 0 {
		pixWidth = 10
	}
	pixHeight = int(geo.Max(float32(pixHeight), float32(w.height)))
	if pixHeight == 0 {
		pixHeight = 10
	}

	win, err := glfw.CreateWindow(pixWidth, pixHeight, w.title, nil, nil)
	if err != nil {
		w.driver.initFailed("window creation error", err)
		return
	}

	w.viewport = win

	if w.view() == nil { // something went wrong above, it will have been logged
		return
	}

	// run the GL init on the draw thread
	w.RunWithContext(func() {
		w.canvas.SetPainter(newCanvasPainter(w.canvas, w))
		w.canvas.Painter().Init()
	})

	w.setDarkMode()

	win.SetCloseCallback(w.closed)
	win.SetPosCallback(w.moved)
	win.SetSizeCallback(w.resized)
	win.SetFramebufferSizeCallback(w.frameSized)
	win.SetRefreshCallback(w.refresh)
	win.SetCursorPosCallback(w.mouseMoved)
	win.SetMouseButtonCallback(w.mouseClicked)
	win.SetScrollCallback(w.mouseScrolled)
	win.SetKeyCallback(w.keyPressed)
	win.SetCharCallback(w.charInput)
	win.SetFocusCallback(w.focused)

	installNativeDragOut(w) // HTML5 drag-and-drop out of the canvas (#857/#874)

	w.canvas.detectedScale = w.detectScale()
	w.canvas.scale = w.calculatedScale()
	w.canvas.texScale = w.detectTextureScale()
	// update window size now we have scaled detected
	w.fitContent()

	w.drainPendingEvents()

	w.requestedWidth, w.requestedHeight = w.width, w.height

	width, height := win.GetSize()
	w.processFrameSized(width, height)
	w.processResized(width, height)
}

func (w *window) view() *glfw.Window {
	if w.closing {
		return nil
	}
	return w.viewport
}

// assert we are implementing driver.NativeWindow
var _ driver.NativeWindow = (*window)(nil)

// RunNative provides the browser-hosted window's native context: the <canvas>
// element rendering it. Implementing driver.NativeWindow is also the web
// driver's identity seam (#257): the wasm file-dialog overrides assert it on
// the dialog's parent before putting up a real browser picker, which keeps the
// test driver's windows on the canvas dialog. An emulated extra window
// (wrapInner) delegates here and so reports the root's canvas; see
// wrapInner.RunNative.
func (w *window) RunNative(f func(any)) {
	context := driver.BrowserWindowContext{}
	if v := w.view(); v != nil {
		context.Canvas = v.GetCanvas()
	}

	f(context)
}

// wrapInner represents a window that is provided by an InnerWindow container in the canvas.
type wrapInner struct {
	bellui.Window
	inner *container.InnerWindow
	d     *gLDriver

	centered bool
	onClosed func()
}

func wrapInnerWindow(w *container.InnerWindow, root bellui.Window, d *gLDriver) bellui.Window {
	wrapped := &wrapInner{inner: w, d: d}
	wrapped.Window = root
	w.CloseIntercept = wrapped.doClose
	return wrapped
}

func (w *wrapInner) CenterOnScreen() {
	w.centered = true

	w.doCenter()
}

func (w *wrapInner) Close() {
	w.inner.Close()
}

func (w *wrapInner) Hide() {
	w.inner.Hide()
	w.updateVisibility()
}

func (w *wrapInner) Move(p geo.Position) {
	w.inner.Move(p)
}

func (w *wrapInner) Resize(s geo.Size) {
	w.inner.Resize(s)
}

// assert the emulated extra window keeps the web driver's identity seam
var _ driver.NativeWindow = (*wrapInner)(nil)

// RunNative delegates to the root window this emulated extra window embeds,
// so it reports the root's canvas — correct on both counts: the inner window
// is rendered on that canvas, and the browser surfaces the file-dialog
// overrides put up (the <input type="file"> picker, the download) are
// page-global anyway (#257). The delegation must be explicit: wrapInner
// embeds the bellui.Window interface, which promotes only that interface's
// methods — RunNative, an extension method on the concrete window, is not
// among them, so without this method the extra window would fail the
// driver.NativeWindow assertion and lose the browser picker.
func (w *wrapInner) RunNative(f func(any)) {
	if native, ok := w.Window.(driver.NativeWindow); ok {
		native.RunNative(f)
		return
	}
	// The root is always the web driver's own window today; degrade visibly
	// to a canvas-less browser context rather than dropping the seam.
	f(driver.BrowserWindowContext{})
}

func (w *wrapInner) SetContent(o core.CanvasObject) {
	w.inner.SetContent(o)
}

func (w *wrapInner) SetOnClosed(fn func()) {
	w.onClosed = fn
}

func (w *wrapInner) Show() {
	c := w.Window.Canvas().(*glCanvas)
	multi := c.webExtraWindows
	multi.Show()
	w.inner.Show()

	c.Overlays().Add(multi)

	if w.centered {
		w.doCenter()
	}
}

func (w *wrapInner) doCenter() {
	c := w.Window.Canvas().(*glCanvas)
	multi := c.webExtraWindows

	min := w.inner.MinSize()
	min = min.Max(w.inner.Size())

	x := (multi.Size().Width - min.Width) / 2
	y := (multi.Size().Height - min.Height) / 2

	w.inner.Move(geo.NewPos(x, y))
}

func (w *wrapInner) doClose() {
	c := w.Window.Canvas().(*glCanvas)
	multi := c.webExtraWindows

	pos := -1
	for i, child := range multi.Windows {
		if child == w.inner {
			pos = i
			w.inner.Hide()
			break
		}
	}
	if pos != -1 {
		count := len(multi.Windows)
		copy(multi.Windows[pos:], multi.Windows[pos+1:])
		multi.Windows[count-1] = nil
		multi.Windows = multi.Windows[:count-1]
	}

	if w.onClosed != nil {
		w.onClosed()
	}
	w.updateVisibility()
}

func (w *wrapInner) updateVisibility() {
	c := w.Window.Canvas().(*glCanvas)
	multi := c.webExtraWindows

	visible := 0
	for _, win := range multi.Windows {
		if win.Visible() {
			visible++
		}
	}

	if visible > 0 {
		multi.Refresh()
	} else {
		multi.Hide()
		c.Overlays().Remove(multi)
	}
}

func (w *window) scaleInput(in float64) float64 {
	return in
}
