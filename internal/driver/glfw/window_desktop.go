//go:build !wasm

package glfw

import (
	"bytes"
	"context"
	"image"
	_ "image/png" // for the icon
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/async"
	"github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/internal/scale"
	"github.com/belluikit/bellui/internal/svg"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/storage"

	"github.com/belluikit/glfw"
)

const (
	defaultTitle              = "Bellui Application"
	disableDPIDetectionEnvKey = "BELLUI_DISABLE_DPI_DETECTION"
)

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

var cursors [desktop.GrabbingCursor + 1]*glfw.Cursor

func initCursors() {
	cursors = [desktop.GrabbingCursor + 1]*glfw.Cursor{
		desktop.DefaultCursor:   glfw.CreateStandardCursor(glfw.ArrowCursor),
		desktop.TextCursor:      glfw.CreateStandardCursor(glfw.IBeamCursor),
		desktop.CrosshairCursor: glfw.CreateStandardCursor(glfw.CrosshairCursor),
		desktop.PointerCursor:   glfw.CreateStandardCursor(glfw.HandCursor),
		desktop.HResizeCursor:   glfw.CreateStandardCursor(glfw.HResizeCursor),
		desktop.VResizeCursor:   glfw.CreateStandardCursor(glfw.VResizeCursor),
		desktop.HiddenCursor:    nil,
		// glfw 3.4 has no dedicated grab/grabbing cursor; approximate with the
		// hand (grabbable) and the four-way move cursor (actively dragging).
		desktop.GrabCursor:     glfw.CreateStandardCursor(glfw.HandCursor),
		desktop.GrabbingCursor: glfw.CreateStandardCursor(glfw.ResizeAllCursor),
	}
}

// Declare conformity to Window interface and the desktop extensions
var (
	_ bellui.Window              = (*window)(nil)
	_ desktop.Window             = (*window)(nil)
	_ desktop.PositionableWindow = (*window)(nil)
	_ desktop.AttentionWindow    = (*window)(nil)
	_ desktop.ProgressWindow     = (*window)(nil)
)

type window struct {
	viewport  *glfw.Window
	created   bool
	decorate  bool
	closing   bool
	fixedSize bool

	cursor       desktop.Cursor
	customCursor *glfw.Cursor
	canvas       *glCanvas
	driver       *gLDriver
	title        string
	icon         bellui.Resource
	mainmenu     *bellui.MainMenu

	master              bool
	fullScreen          bool
	fullScreenSecondary bool
	centered            bool
	visible             bool
	onTop               bool
	wantAttention       bool

	wantProgress          bool
	progressValue         float64
	progressIndeterminate bool

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

	// Press-and-hold long-press dispatch (#266). All three are main-goroutine
	// state: the hold timer hops back via runOnMain before touching them
	// (the waitForDoubleTap pattern).
	longPressSeq   int          // arm generation; bumping cancels a pending fire
	longPressPos   geo.Position // where the armed press went down (slop origin)
	longPressFired bool         // this primary press hold-fired, so its release is not a tap

	onClosed           func()
	onCloseIntercepted func()
	onMoved            func(x, y int)
	onDropped          func(pos geo.Position, items []bellui.URI)
	dragHoverTarget    bellui.DropHoverable // object currently receiving external drag-hover feedback (#708)
	parent             bellui.Window        // owner window for a native child/owner relationship (#598)
	modal              bool                 // native window-modality against parent (#498)
	modalChildren      int                  // count of modal children blocking this window as a parent (#682)
	modalBlockApplied  bool                 // whether this window currently contributes to its parent's modal block (#682)

	menuTogglePending       event.KeyName
	menuDeactivationPending event.KeyName

	// imeModeOn/imeModeSet track the GLFW IME input mode last applied to the
	// view so imeFocusChanged only toggles it on an actual change. Re-applying
	// the mode resets the system input source to a Roman/US layout on macOS, so
	// a redundant toggle (focus callbacks can repeat) kicks the user out of their
	// CJK input method mid-typing (#352).
	imeModeOn  bool
	imeModeSet bool

	// preeditActive reports whether the last preedit delivered by GLFW was
	// non-empty. Some backends re-deliver an empty preedit repeatedly (the
	// Wayland text-input done handler acknowledged every commit this way,
	// #414), so empty→empty transitions are dropped instead of forwarding a
	// redundant ComposeCancel to the focused widget every frame.
	preeditActive bool

	xpos, ypos                      int
	width, height                   int
	requestedWidth, requestedHeight int
	shouldWidth, shouldHeight       int
	shouldExpand                    bool

	pending []func()

	lastWalkedTime time.Time
}

func (w *window) SetFullScreen(full bool) {
	w.fullScreen = full
	w.fullScreenSecondary = false

	if w.view() != nil {
		async.EnsureMain(func() {
			w.doSetFullScreen(full)
		})
	}
}

// assert desktop windows expose the native parent/owner relationship (#598)
var _ desktop.ChildWindow = (*window)(nil)

func (w *window) RequestAlwaysOnTop() {
	w.onTop = true
}

func (w *window) RequestFullScreenSecondary() {
	w.fullScreenSecondary = true
	w.fullScreen = true

	if w.view() != nil {
		async.EnsureMain(func() {
			w.doSetFullScreen2(true)
		})
	}
}

func (w *window) RequestPosition(x, y int) {
	w.xpos = x
	w.ypos = y

	if build.IsWayland || w.view() == nil {
		return
	}

	async.EnsureMain(func() {
		if view := w.view(); view != nil {
			view.SetPos(x, y)
		}
	})
}

func (w *window) RequestAttention() {
	// Remember the request so a call before the window is mapped is honoured at
	// creation; apply immediately when the view already exists.
	w.wantAttention = true
	if w.view() == nil {
		return
	}

	async.EnsureMain(func() {
		if view := w.view(); view != nil {
			view.RequestAttention()
		}
	})
}

// applyAttention honours a RequestAttention made before the window mapped. The
// shared Show path calls it once a native window exists (mirrors applyParent /
// applyModal); it is a no-op on the web target.
func (w *window) applyAttention() {
	if w.wantAttention {
		if view := w.view(); view != nil {
			view.RequestAttention()
		}
	}
}

// SetProgress implements desktop.ProgressWindow. The platform work is done by
// applyProgressNative (real on Windows, a no-op elsewhere); the last requested
// state is remembered so a call before the window maps is honoured at Show.
func (w *window) SetProgress(value float64, indeterminate bool) {
	w.wantProgress = true
	w.progressValue = value
	w.progressIndeterminate = indeterminate
	if w.view() == nil {
		return
	}

	async.EnsureMain(func() {
		w.applyProgressNative(value, indeterminate)
	})
}

// applyProgress re-applies a progress request once a native window exists,
// mirroring applyAttention. Called from the shared Show path.
func (w *window) applyProgress() {
	if w.wantProgress {
		w.applyProgressNative(w.progressValue, w.progressIndeterminate)
	}
}

func (w *window) Position() (x, y int) {
	return w.xpos, w.ypos
}

// SetParent implements desktop.ChildWindow: it records parent as this window's
// owner and, when both windows already exist, applies the native relationship
// immediately. Otherwise it is applied once this window is shown (applyParent).
func (w *window) SetParent(parent bellui.Window) {
	w.parent = parent
	if w.view() == nil {
		return
	}
	async.EnsureMain(w.applyParent)
}

// parentWindow resolves w.parent to its glfw *window when both it and its
// native view already exist, else nil (a not-yet-shown or foreign parent).
func (w *window) parentWindow() *window {
	if w.parent == nil {
		return nil
	}
	if pw, ok := w.parent.(*window); ok && pw.view() != nil {
		return pw
	}
	return nil
}

// blockParentModal ref-counts modal children on parent so a shared native
// parent block (Windows EnableWindow, macOS key suppression) is applied on the
// first modal child and lifted only when the last one clears — keeping the
// parent blocked while any window-hosted modal over it is open (#682). It
// reports whether this call is the 0<->1 transition that should toggle the
// native block, and is idempotent per child via w.modalBlockApplied (so
// repeated SetModal(true) or a stray unblock cannot mis-count). X11 and Wayland
// set per-child protocol state instead and do not use this.
func (w *window) blockParentModal(parent *window, block bool) bool {
	if block == w.modalBlockApplied {
		return false
	}
	w.modalBlockApplied = block
	if block {
		parent.modalChildren++
		return parent.modalChildren == 1
	}
	parent.modalChildren--
	return parent.modalChildren == 0
}

// applyParent establishes the native parent/owner relationship on the main
// goroutine, once this window's view exists. It resolves the parent to its
// glfw window and defers to the per-platform doSetParentNative. A nil or
// not-yet-shown parent clears the relationship.
func (w *window) applyParent() {
	if w.view() == nil {
		return
	}
	w.doSetParentNative(w.parentWindow())
}

// SetModal implements desktop.ModalWindow: it records the modal state and, when
// this window already exists, applies the native window-modality against its
// parent immediately. Otherwise it is applied once this window is shown
// (applyModal). Disabling it restores input to the parent.
func (w *window) SetModal(modal bool) {
	w.modal = modal
	if w.view() == nil {
		return
	}
	async.EnsureMain(w.applyModal)
}

// applyModal enforces (or lifts) native window-modality on the main goroutine,
// once this window's view exists. Modality is expressed against the parent, so
// with no resolved parent it is a no-op — the pure-Go scrim still blocks it.
func (w *window) applyModal() {
	if w.view() == nil {
		return
	}
	parentWin := w.parentWindow()
	if parentWin == nil {
		return
	}
	w.doSetModalNative(parentWin, w.modal)
}

// releaseModal lifts native window-modality before this window is torn down, so
// a disabled parent (Windows EnableWindow(false)) is never left orphaned when
// the child is destroyed. Called from destroyWindow for every teardown path.
func (w *window) releaseModal() {
	if !w.modal {
		return
	}
	w.modal = false
	if parentWin := w.parentWindow(); parentWin != nil {
		w.doSetModalNative(parentWin, false)
	}
}

func (w *window) SetOnMoved(moved func(x, y int)) {
	w.onMoved = moved
}

func (w *window) CenterOnScreen() {
	if build.IsWayland {
		return
	}

	w.centered = true

	w.runOnMainWhenCreated(w.doCenterOnScreen)
}

func (w *window) SetOnDropped(dropped func(pos geo.Position, items []bellui.URI)) {
	// The GLFW drop callback is installed once at window creation (see
	// create); routing to per-widget Droppable targets and this window-wide
	// fallback both read w.onDropped / the live tree from there.
	w.onDropped = dropped
}

// processDropped is the GLFW drop callback. It routes a file drop to the
// deepest bellui.Droppable under the pointer, falling back to the window-wide
// SetOnDropped callback when no widget claims the drop.
func (w *window) processDropped(_ *glfw.Window, names []string) {
	uris := make([]bellui.URI, len(names))
	for i, name := range names {
		uris[i] = storage.NewFileURI(name)
	}

	pos := w.mousePos
	obj, objPos, _ := w.findObjectAtPositionMatching(w.canvas, pos, func(object core.CanvasObject) bool {
		_, ok := object.(bellui.Droppable)
		return ok
	})

	if target, ok := obj.(bellui.Droppable); ok {
		target.Dropped(&bellui.DropEvent{
			AbsolutePosition: pos,
			Position:         objPos,
			URIs:             uris,
		})
		// The drop ends the drag; the backends report no drag-leave on drop, so
		// clear any hover highlight here (fires DragLeave on the tracked target).
		w.clearDragHover()
		return
	}

	w.clearDragHover()
	if w.onDropped != nil {
		w.onDropped(pos, uris)
	}
}

// processDragMotion is the GLFW drag-motion callback (belluikit/glfw #708). It
// routes external drag hover feedback to the deepest bellui.DropHoverable under the
// pointer: DragEnter/DragOver as it moves within, DragLeave when it moves off the
// object, leaves the window, or the drag ends. The final drop still arrives via
// processDropped.
func (w *window) processDragMotion(_ *glfw.Window, phase glfw.DragPhase, xpos, ypos float64) {
	if phase == glfw.DragLeave {
		w.clearDragHover()
		return
	}

	pos := geo.NewPos(scale.ToBelluiCoordinate(w.canvas, int(xpos)), scale.ToBelluiCoordinate(w.canvas, int(ypos)))
	obj, objPos, _ := w.findObjectAtPositionMatching(w.canvas, pos, func(object core.CanvasObject) bool {
		_, ok := object.(bellui.DropHoverable)
		return ok
	})
	target, _ := obj.(bellui.DropHoverable)

	if target != w.dragHoverTarget {
		// Moved onto a different (or no) target: leave the old, enter the new.
		w.clearDragHover()
		w.dragHoverTarget = target
		if target != nil {
			target.DragEnter(&bellui.DragHoverEvent{AbsolutePosition: pos, Position: objPos})
		}
		return
	}

	if target != nil {
		target.DragOver(&bellui.DragHoverEvent{AbsolutePosition: pos, Position: objPos})
	}
}

// clearDragHover ends any in-progress external drag hover, notifying the tracked
// target via DragLeave so it can clear its highlight.
func (w *window) clearDragHover() {
	if w.dragHoverTarget != nil {
		w.dragHoverTarget.DragLeave()
		w.dragHoverTarget = nil
	}
}

func (w *window) doCenterOnScreen() {
	viewWidth, viewHeight := w.screenSize(w.canvas.size)
	if w.width > viewWidth { // in case our window has not called back to canvas size yet
		viewWidth = w.width
	}
	if w.height > viewHeight {
		viewHeight = w.height
	}

	// get window dimensions in pixels
	monitor := w.getMonitorForWindow()
	monMode := monitor.GetVideoMode()
	if monMode == nil {
		return // display-settings query failed (RDP/virtual display); leave the window where it is
	}

	// these come into play when dealing with multiple monitors
	monX, monY := monitor.GetPos()

	// math them to the middle
	newX := (monMode.Width-viewWidth)/2 + monX
	newY := (monMode.Height-viewHeight)/2 + monY

	// set new window coordinates
	w.viewport.SetPos(newX, newY)
}

func (w *window) RequestFocus() {
	if build.IsWayland || w.view() == nil {
		return
	}

	w.runOnMainWhenCreated(w.viewport.Focus)
}

func (w *window) SetIcon(icon bellui.Resource) {
	w.icon = icon
	if build.IsWayland {
		return
	}

	if icon == nil {
		appIcon := bellui.CurrentApp().Icon()
		if appIcon != nil {
			w.SetIcon(appIcon)
		}
		return
	}

	w.runOnMainWhenCreated(func() {
		if w.icon == nil {
			w.viewport.SetIcon(nil)
			return
		}

		var img image.Image
		if svg.IsResourceSVG(w.icon) {
			img = painter.PaintImage(&canvas.Image{Resource: w.icon}, nil, windowIconSize, windowIconSize)
		} else {
			pix, _, err := image.Decode(bytes.NewReader(w.icon.Content()))
			if err != nil {
				bellui.LogError("Failed to decode image for window icon", err)
				return
			}
			img = pix
		}

		w.viewport.SetIcon([]image.Image{img})
	})
}

func (w *window) SetMaster() {
	w.master = true
}

func (w *window) fitContent() {
	if w.canvas.Content() == nil || (w.fullScreen && w.visible) {
		return
	}

	if w.isClosing() {
		return
	}

	minWidth, minHeight := w.minSizeOnScreen()
	view := w.viewport
	w.shouldWidth, w.shouldHeight = w.width, w.height
	if w.width < minWidth || w.height < minHeight {
		if w.width < minWidth {
			w.shouldWidth = minWidth
		}
		if w.height < minHeight {
			w.shouldHeight = minHeight
		}
		w.shouldExpand = true // queue the resize to happen on main
	}
	if w.fixedSize {
		if w.shouldWidth > w.requestedWidth {
			w.requestedWidth = w.shouldWidth
		}
		if w.shouldHeight > w.requestedHeight {
			w.requestedHeight = w.shouldHeight
		}
		view.SetSizeLimits(w.requestedWidth, w.requestedHeight, w.requestedWidth, w.requestedHeight)
	} else {
		view.SetSizeLimits(minWidth, minHeight, glfw.DontCare, glfw.DontCare)
	}
}

// getMonitorScale returns the scale factor for a given monitor, handling platform-specific cases
func getMonitorScale(monitor *glfw.Monitor) float32 {
	widthMm, heightMm := monitor.GetPhysicalSize()
	if runtime.GOOS == "linux" && widthMm == 60 && heightMm == 60 { // Steam Deck incorrectly reports 6cm square!
		return 1.0
	}
	mode := monitor.GetVideoMode()
	if mode == nil {
		// GetVideoMode returns nil when the platform display-settings query
		// fails — e.g. a Windows RDP/virtual display where EnumDisplaySettings
		// errors ("Failed to query display settings"), or a monitor hot-removed
		// mid-session. Default to unscaled rather than dereferencing nil.
		return 1.0
	}
	widthPx := mode.Width
	return calculateDetectedScale(widthMm, widthPx)
}

// getScaledMonitorSize returns the monitor dimensions adjusted for scaling
func getScaledMonitorSize(monitor *glfw.Monitor) geo.Size {
	videoMode := monitor.GetVideoMode()
	if videoMode == nil {
		// Display-settings query failed (e.g. RDP/virtual display); report a
		// zero size so the caller's monitor-containment test skips this monitor
		// and falls back, rather than dereferencing nil.
		return geo.Size{}
	}
	scale := getMonitorScale(monitor)

	scaledWidth := float32(videoMode.Width) / scale
	scaledHeight := float32(videoMode.Height) / scale
	return geo.NewSize(scaledWidth, scaledHeight)
}

func (w *window) getMonitorForWindow() *glfw.Monitor {
	if !build.IsWayland {
		x, y := w.xpos, w.ypos
		if w.fullScreen {
			x, y = w.viewport.GetPos()
		}
		xOff := x + (w.width / 2)
		yOff := y + (w.height / 2)

		for _, monitor := range glfw.GetMonitors() {
			x, y := monitor.GetPos()

			if x > xOff || y > yOff {
				continue
			}

			scaledSize := getScaledMonitorSize(monitor)
			if x+int(scaledSize.Width) <= xOff || y+int(scaledSize.Height) <= yOff {
				continue
			}

			return monitor
		}
	}

	// try built-in function to detect monitor if above logic didn't succeed
	// if it doesn't work then return primary monitor as default
	monitor := w.viewport.GetMonitor()
	if monitor == nil {
		monitor = glfw.GetPrimaryMonitor()
	}
	return monitor
}

func (w *window) findSiblingMonitor() *glfw.Monitor {
	for _, other := range w.driver.windowList() {
		ow, ok := other.(*window)
		if !ok || ow == w || !ow.visible || ow.viewport == nil {
			continue
		}
		return ow.getMonitorForWindow()
	}
	return nil
}

func (w *window) detectScale() float32 {
	if build.IsWayland { // Wayland controls scale through content scaling
		return 1
	}

	// check if DPI detection is disabled
	env := os.Getenv(disableDPIDetectionEnvKey)
	if strings.EqualFold(env, "true") || strings.EqualFold(env, "t") || env == "1" {
		return 1
	}

	monitor := w.getMonitorForWindow()
	if monitor == nil {
		return 1
	}

	return getMonitorScale(monitor)
}

func (w *window) moved(_ *glfw.Window, x, y int) {
	w.processMoved(x, y)

	// entering fullscreen re-seats the window at the monitor origin; that is
	// not a move of the window the user placed, so don't report it
	if !w.fullScreen && !w.fullScreenSecondary && w.onMoved != nil {
		w.onMoved(x, y)
	}
}

func (w *window) resized(_ *glfw.Window, width, height int) {
	w.processResized(width, height)
}

func (w *window) scaled(_ *glfw.Window, x float32, y float32) {
	if !build.IsWayland { // other platforms handle this using older APIs
		return
	}

	w.canvas.texScale = x
	w.canvas.Refresh(w.canvas.content)
}

func (w *window) frameSized(_ *glfw.Window, width, height int) {
	w.processFrameSized(width, height)
}

func (w *window) refresh(_ *glfw.Window) {
	w.processRefresh()
}

func (w *window) closed(viewport *glfw.Window) {
	if viewport != nil {
		viewport.SetShouldClose(false) // reset the closed flag until we check the veto in processClosed
	}

	w.processClosed()
}

func belluiToNativeCursor(cursor desktop.Cursor) (*glfw.Cursor, bool) {
	cursorType, standard := cursor.(desktop.StandardCursor)
	if !standard {
		img, x, y := cursor.Image()
		if img == nil {
			return nil, true
		}
		return glfw.CreateCursor(img, x, y), true
	}

	if cursorType < 0 || cursorType >= desktop.StandardCursor(len(cursors)) {
		return cursors[desktop.DefaultCursor], false
	}

	return cursors[cursorType], false
}

func (w *window) SetCursor(cursor *glfw.Cursor) {
	async.EnsureMain(func() {
		w.viewport.SetCursor(cursor)
	})
}

func (w *window) setCustomCursor(rawCursor *glfw.Cursor, isCustomCursor bool) {
	if w.customCursor != nil {
		w.customCursor.Destroy()
		w.customCursor = nil
	}
	if isCustomCursor {
		w.customCursor = rawCursor
	}
}

func (w *window) mouseMoved(_ *glfw.Window, xpos, ypos float64) {
	w.processMouseMoved(xpos, ypos)
}

func (w *window) mouseClicked(_ *glfw.Window, btn glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
	button, modifiers := convertMouseButton(btn, mods)
	mouseAction := convertAction(action)

	w.processMouseClicked(button, mouseAction, modifiers)
}

func (w *window) mouseScrolled(viewport *glfw.Window, xoff float64, yoff float64) {
	if runtime.GOOS != "darwin" && xoff == 0 &&
		(viewport.GetKey(glfw.KeyLeftShift) == glfw.Press ||
			viewport.GetKey(glfw.KeyRightShift) == glfw.Press) {
		xoff, yoff = yoff, xoff
	}

	w.processMouseScrolled(xoff, yoff)
}

func convertMouseButton(btn glfw.MouseButton, mods glfw.ModifierKey) (desktop.MouseButton, event.KeyModifier) {
	modifier := desktopModifier(mods)
	rightClick := false
	if runtime.GOOS == "darwin" {
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
		return event.KeyName(char ^ ('a' - 'A')) // Corresponding KeyName is uppercase. Convert with simple bit flip.
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

	keyName := glfw.GetKeyName(code, scancode)
	return keyCodeToKeyName(keyName)
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

func (w *window) keyPressed(_ *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	keyName := keyToName(key, scancode)
	keyDesktopModifier := desktopModifier(mods)
	w.driver.setCurrentKeyModifiers(desktopModifierCorrected(mods, key, action))
	keyAction := convertAction(action)
	keyASCII := convertASCII(key)

	w.processKeyPressed(keyName, keyASCII, scancode, keyAction, keyDesktopModifier)
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

func desktopModifierCorrected(mods glfw.ModifierKey, key glfw.Key, action glfw.Action) event.KeyModifier {
	// On X11, pressing/releasing modifier keys does not include newly pressed/released keys in 'mod' mask.
	// https://github.com/glfw/glfw/issues/1630
	if action == glfw.Press {
		mods |= glfwKeyToModifier(key)
	} else {
		mods &= ^glfwKeyToModifier(key)
	}
	return desktopModifier(mods)
}

func glfwKeyToModifier(key glfw.Key) glfw.ModifierKey {
	switch key {
	case glfw.KeyLeftControl, glfw.KeyRightControl:
		return glfw.ModControl
	case glfw.KeyLeftAlt, glfw.KeyRightAlt:
		return glfw.ModAlt
	case glfw.KeyLeftShift, glfw.KeyRightShift:
		return glfw.ModShift
	case glfw.KeyLeftSuper, glfw.KeyRightSuper:
		return glfw.ModSuper
	default:
		return 0
	}
}

// charInput defines the character with modifiers callback which is called when a
// Unicode character is input.
//
// Characters do not map 1:1 to physical keys, as a key may produce zero, one or more characters.
func (w *window) charInput(viewport *glfw.Window, char rune) {
	w.processCharInput(char)
}

// preeditInput is the GLFW IME preedit (composition) callback. While an input
// method is composing, it delivers the in-progress preedit text; the committed
// text arrives separately through the normal char callback (charInput →
// TypedRune), so on-the-spot commit needs no special handling here. An empty
// preedit clears any visible composition.
func (w *window) preeditInput(_ *glfw.Window, preedit []rune, _ []int, _ int, caret int) {
	if len(preedit) == 0 {
		if !w.preeditActive {
			return
		}
		w.preeditActive = false
		w.processComposeCancel()
		return
	}
	w.preeditActive = true
	w.processCompose(&event.PreeditEvent{Text: string(preedit), CursorPos: caret})
	w.updatePreeditCursorRect()
}

// processComposeCancel discards any in-progress IME composition on the focused
// object when it supports composition. A no-op otherwise.
func (w *window) processComposeCancel() {
	if c, ok := w.canvas.Focused().(event.Composable); ok {
		c.ComposeCancel()
	}
}

// imeFocusChanged enables the GLFW IME input mode only while a Composable widget
// is focused, so input methods (and their candidate window) activate for text
// fields and stay off elsewhere. obj is nil when focus is cleared.
func (w *window) imeFocusChanged(obj event.Focusable) {
	v := w.view()
	if v == nil {
		return
	}
	_, composable := obj.(event.Composable)
	// Only toggle when the desired mode actually changes: re-applying it resets
	// the system input source to US/Roman on macOS, and this callback can fire
	// redundantly (focus is re-asserted on interactions), which would drop the
	// user out of their CJK input method on every such event (#352).
	if w.imeModeSet && w.imeModeOn == composable {
		return
	}
	if composable {
		v.SetInputMode(glfw.IMEMode, glfw.True)
	} else {
		v.SetInputMode(glfw.IMEMode, glfw.False)
	}
	w.imeModeOn = composable
	w.imeModeSet = true
}

// updatePreeditCursorRect tells the IME where the caret is, in window pixels, so
// the candidate window tracks the text cursor. A no-op unless the focused object
// reports a caret rectangle via event.PreeditPositioner.
func (w *window) updatePreeditCursorRect() {
	focused := w.canvas.Focused()
	pp, ok := focused.(event.PreeditPositioner)
	if !ok {
		return
	}
	co, ok := focused.(core.CanvasObject)
	if !ok {
		return
	}
	v := w.view()
	if v == nil {
		return
	}

	rect := pp.PreeditCursorRectangle()
	abs := w.driver.AbsolutePositionForObject(co)
	x := scale.ToScreenCoordinate(w.canvas, abs.X+rect.Pos.X)
	y := scale.ToScreenCoordinate(w.canvas, abs.Y+rect.Pos.Y)
	width := scale.ToScreenCoordinate(w.canvas, rect.Size.Width)
	height := scale.ToScreenCoordinate(w.canvas, rect.Size.Height)
	v.SetPreeditCursorRectangle(x, y, width, height)
}

func (w *window) focused(viewport *glfw.Window, focused bool) {
	if !focused {
		// Discard any composition the OS input method is still holding. The
		// focused widget cancels its inline preedit on focus-out (#354), but
		// without this the IME keeps its own marked text and commits it as
		// char events when the window is refocused, resurrecting text the
		// user saw cancelled (#413). Cocoa discards the marked text, Windows
		// sends IMM CPS_CANCEL; a no-op when nothing is composing.
		viewport.ResetPreeditText()
	}
	w.processFocused(focused)
}

func (w *window) DetachCurrentContext() {
	glfw.DetachCurrentContext()
}

func (w *window) RescaleContext() {
	if w.isClosing() {
		return
	}
	w.fitContent()

	if w.fullScreen {
		w.width, w.height = w.viewport.GetSize()
		scaledFull := geo.NewSize(
			scale.ToBelluiCoordinate(w.canvas, w.width),
			scale.ToBelluiCoordinate(w.canvas, w.height),
		)
		w.canvas.Resize(scaledFull)
		return
	}

	size := w.canvas.size.Max(w.canvas.MinSize())
	newWidth, newHeight := w.screenSize(size)
	w.viewport.SetSize(newWidth, newHeight)

	// Ensure textures re-rasterize at the new scale
	cache.DeleteTextTexturesFor(w.canvas)
	w.canvas.content.Refresh()
}

func (w *window) create() {
	if !build.IsWayland {
		// make the window hidden, we will set it up and then show it later
		glfw.WindowHint(glfw.Visible, glfw.False)
	}
	if w.decorate {
		glfw.WindowHint(glfw.Decorated, glfw.True)
	} else {
		glfw.WindowHint(glfw.Decorated, glfw.False)
	}
	if w.fixedSize {
		glfw.WindowHint(glfw.Resizable, glfw.False)
	} else {
		glfw.WindowHint(glfw.Resizable, glfw.True)
	}
	if w.onTop {
		glfw.WindowHint(glfw.Floating, glfw.True)
	} else {
		glfw.WindowHint(glfw.Floating, glfw.False)
	}
	glfw.WindowHint(glfw.AutoIconify, glfw.False)
	initWindowHints()
	applyBackendWindowHints() // backend override (e.g. NoAPI for WebGPU)

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

	// macOS 26 places new windows on a different screen than existing app windows;
	// default new windows onto the same monitor as a visible sibling when no position was set.
	if runtime.GOOS == "darwin" && !build.IsWayland && w.xpos == 0 && w.ypos == 0 {
		if monitor := w.findSiblingMonitor(); monitor != nil {
			if monMode := monitor.GetVideoMode(); monMode != nil {
				monX, monY := monitor.GetPos()
				w.xpos = monX + (monMode.Width-pixWidth)/2
				w.ypos = monY + (monMode.Height-pixHeight)/2
			}
		}
	}

	// Windows leaves unpositioned windows to the OS default placement
	// (CW_USEDEFAULT), whose cascade pins to the work-area corner on an
	// otherwise-empty desktop, while GLFW centers them on macOS. Center the
	// app's first window when no position was requested, so first-run
	// placement matches across platforms (#955); later windows (a visible
	// sibling exists) keep the OS cascade, which anchors near that centered
	// window — the Windows counterpart of Cocoa's cascade. X11/Wayland are
	// untouched: window managers own placement policy there.
	if runtime.GOOS == "windows" && w.xpos == 0 && w.ypos == 0 && w.findSiblingMonitor() == nil {
		if monitor := glfw.GetPrimaryMonitor(); monitor != nil {
			if monMode := monitor.GetVideoMode(); monMode != nil {
				monX, monY := monitor.GetPos()
				w.xpos = monX + (monMode.Width-pixWidth)/2
				w.ypos = monY + (monMode.Height-pixHeight)/2
			}
		}
	}

	if (w.xpos != 0 || w.ypos != 0) && !build.IsWayland {
		win.SetPos(w.xpos, w.ypos)
	}

	// run the painter init on the draw thread
	w.RunWithContext(func() {
		w.canvas.SetPainter(newCanvasPainter(w.canvas, w))
		w.canvas.Painter().Init()
	})

	w.setDarkMode()

	win.SetCloseCallback(w.closed)
	win.SetDragCallback(w.processDragMotion)
	win.SetPosCallback(w.moved)
	win.SetSizeCallback(w.resized)
	win.SetFramebufferSizeCallback(w.frameSized)
	win.SetRefreshCallback(w.refresh)
	win.SetContentScaleCallback(w.scaled)
	win.SetCursorPosCallback(w.mouseMoved)
	win.SetMouseButtonCallback(w.mouseClicked)
	win.SetScrollCallback(w.mouseScrolled)
	win.SetDropCallback(w.processDropped)
	win.SetKeyCallback(w.keyPressed)
	win.SetCharCallback(w.charInput)
	win.SetFocusCallback(w.focused)
	win.SetPreeditCallback(w.preeditInput)
	w.canvas.OnFocus = w.imeFocusChanged

	w.canvas.detectedScale = w.detectScale()
	w.canvas.scale = w.calculatedScale()
	w.canvas.texScale = w.detectTextureScale()
	// update window size now we have scaled detected
	w.fitContent()

	w.drainPendingEvents()

	if w.FixedSize() && (w.requestedWidth == 0 || w.requestedHeight == 0) {
		bigEnough := w.canvas.canvasSize(w.canvas.Content().MinSize())
		w.width, w.height = scale.ToScreenCoordinate(w.canvas, bigEnough.Width), scale.ToScreenCoordinate(w.canvas, bigEnough.Height)
		w.shouldWidth, w.shouldHeight = w.width, w.height
	}

	w.requestedWidth, w.requestedHeight = w.width, w.height
	// order of operation matters so we do these last items in order
	w.viewport.SetSize(w.shouldWidth, w.shouldHeight) // ensure we requested latest size
}

func (w *window) view() *glfw.Window {
	if w.closing {
		return nil
	}
	return w.viewport
}

// wrapInnerWindow is a no-op to match what the web driver provides
func wrapInnerWindow(*container.InnerWindow, bellui.Window, *gLDriver) bellui.Window {
	return nil
}
