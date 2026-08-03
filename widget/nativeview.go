package widget

import (
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/geo"
)

// NativeViewHost embeds an OS-owned view — a web view, a native video surface,
// a map or a camera preview — at a widget's position in the layout.
//
// # This is a hole punch, not a composited widget
//
// Bellui draws its whole scene into one GPU surface. An OS view has its own
// render pipeline and cannot be composited into that surface, so a
// NativeViewHost does not draw the native content at all: it draws nothing,
// and drives a real platform view positioned over the canvas at the host's
// screen rectangle. This is the same approach Flutter, Compose Multiplatform
// and Avalonia ship, and it carries limitations that are properties of the
// approach rather than bugs to be fixed later:
//
//   - Nothing bellui draws can appear above the native view. Every [PopUp],
//     dialog, menu, [Tooltip], [Toast], [Snackbar], [CompletionPopup] and the
//     mobile selection handles and loupe are occluded where they overlap it.
//   - [core.Canvas.Capture] sees a hole. The native content is not in the
//     surface Capture reads back, so screenshots, golden-image tests and the
//     hack/*probe -auto verification convention capture whatever bellui drew
//     underneath, never the native view.
//   - Render-pipeline features do not reach it: [container.RepaintBoundary]
//     FBO caching, dirty-region culling, SetGroupOpacity, world transforms and
//     [container.DirectionScope] mirroring all apply to bellui's surface only.
//   - Clipping is native. The host converts its bellui clip bounds into a
//     platform clip or mask (see [NativeView.SetGeometry]); a backend that
//     does not implement one will overflow an enclosing [container.Scroll].
//
// Use it when the embedded content genuinely has to be an OS view. When
// bellui can draw the content itself, draw it — a real widget composites,
// captures, clips and mirrors for free.
//
// # Keyboard focus
//
// Keyboard focus is arbitrated by the platform's own responder chain, which
// bellui's canvas focus knows nothing about, so a host whose backend
// implements [NativeViewFocuser] bridges the two (#59):
//
//   - When the platform view takes the OS focus — which is what a click
//     inside it does — the host clears the canvas focus, so no bellui widget
//     is left drawing a caret while the keystrokes go to the OS view.
//   - When bellui focuses one of its own widgets while the platform view
//     still holds the OS focus, the host makes the platform view resign it,
//     so the OS routes keys and IME composition back to bellui.
//
// The bridge rides the same per-frame sync as the geometry (see Lifecycle),
// polling the backend rather than requiring every platform to grow a
// focus-change notification; a backend that does not implement
// [NativeViewFocuser] simply has no bridge, and focus behaves as it did
// before.
//
// A NativeViewHost is deliberately *not* a Tab stop. Once the platform view
// holds the OS focus bellui never sees the Tab key, so it could not move the
// focus back out — putting the host in the Tab ring would create a keyboard
// trap (WCAG 2.1.2 No Keyboard Trap), which is worse than not being reachable
// by Tab at all. Focus reaches the platform view by clicking into it, or
// programmatically through [NativeViewHost.FocusNativeView].
//
// # Lifecycle
//
// The host attaches its [NativeView] once it is laid out on a canvas, then
// re-syncs the view's geometry and visibility every frame while it is shown,
// so the native view tracks scrolling, resizing and layout changes the widget
// itself is never told about. Hiding the host, scrolling it fully out of a
// clipping ancestor, or removing it from the tree hides the native view;
// destroying the renderer (the widget leaving the tree, or its window
// closing) destroys it.
//
// All methods must be called on the main goroutine.
//
// Since: 2.19
type NativeViewHost struct {
	BaseWidget

	view NativeView

	win      bellui.Window
	attached bool
	shown    bool
	anim     *bellui.Animation

	// nativeFocus is the host's view of the platform responder chain: true
	// while the OS keyboard focus is inside the platform view. It is the state
	// the focus bridge compares each frame's poll against, and the invariant
	// it maintains is that while it is true the canvas focus is nil.
	nativeFocus bool

	min                geo.Size
	lastRect, lastClip geo.Rectangle
	lastScale          float32

	// syncs counts every per-frame evaluation, applies counts those that
	// found changed geometry and called through to the backend. The ratio is
	// what makes the cost of the per-frame sync measurable (#45).
	syncs, applies atomic.Uint64
}

// NativeView is the platform-side view a [NativeViewHost] places, sizes,
// clips and tears down. Implementations are inherently platform-specific and
// belong in their own optional package, so an app that never embeds a native
// view never links the platform SDK.
//
// Every method is called on the main goroutine.
//
// Since: 2.19
type NativeView interface {
	// Attach creates the platform view inside win, whose native handle the
	// backend obtains through [driver.NativeWindow.RunNative]. It is called
	// once, when the host is first laid out on a canvas. Returning an error
	// leaves the host inert — it occupies its space and draws nothing.
	Attach(win bellui.Window) error

	// SetGeometry places the view. rect is the host's rectangle and clip the
	// region the view must be masked to, both in canvas-absolute bellui
	// coordinates (device-independent points); scale is the canvas scale,
	// which converts them to whatever unit the platform view uses. clip is
	// always contained in rect's canvas and never empty — the host hides the
	// view rather than reporting an empty clip.
	SetGeometry(rect, clip geo.Rectangle, scale float32)

	// SetVisible shows or hides the platform view. The host calls it with
	// false when it is hidden, detached from a canvas, or clipped entirely
	// out of view, and with true when it becomes visible again.
	SetVisible(visible bool)

	// Destroy removes the platform view and releases its resources. It must
	// be safe to call more than once, and safe to call without a preceding
	// successful Attach.
	Destroy()
}

// NativeViewFocuser is the optional capability of a [NativeView] whose
// platform view can hold the OS keyboard focus. A [NativeViewHost] whose
// backend implements it bridges the platform responder chain to canvas focus
// in both directions (#59); a backend without it — a video surface or a map
// that never wants the keyboard — is left alone.
//
// The three methods are the whole contract: the host polls HasFocus on the
// per-frame sync it already runs, so a backend needs no focus-change
// notification of its own. Every method is called on the main goroutine.
//
// Since: 2.19
type NativeViewFocuser interface {
	NativeView

	// TakeFocus asks the platform view to become the OS keyboard focus, as a
	// click inside it would, and reports whether the platform accepted.
	TakeFocus() bool

	// ResignFocus gives the OS keyboard focus back to bellui's own surface,
	// so that keys and input-method composition reach the canvas again. It
	// reports whether the platform accepted; a backend that does not hold the
	// focus should report success and do nothing.
	ResignFocus() bool

	// HasFocus reports whether the platform view currently holds the OS
	// keyboard focus, meaning bellui is receiving no key events. It is called
	// once per frame while the view is shown, so it must be cheap.
	HasFocus() bool
}

var _ core.Widget = (*NativeViewHost)(nil)

// NewNativeViewHost returns a host that embeds view at its position in the
// layout. The view is not created until the host is laid out on a canvas.
//
// Since: 2.19
func NewNativeViewHost(view NativeView) *NativeViewHost {
	h := &NativeViewHost{view: view}
	h.ExtendBaseWidget(h)
	return h
}

// CreateRenderer returns a renderer that draws nothing. The space the host
// occupies is a hole through which the native view is seen.
func (h *NativeViewHost) CreateRenderer() core.WidgetRenderer {
	h.ExtendBaseWidget(h)
	return &nativeViewHostRenderer{host: h}
}

// MinSize returns the minimum size of the host. A native view has no
// intrinsic size bellui can measure — the OS owns its content — so this
// defaults to 0x0 and the host takes whatever the layout gives it. Set a
// floor with [NativeViewHost.SetMinSize] when the layout would otherwise
// collapse it.
func (h *NativeViewHost) MinSize() geo.Size {
	return h.min
}

// SetMinSize sets the smallest size this host should be laid out at. Use
// device-independent points, not pixels, as with any other widget size.
func (h *NativeViewHost) SetMinSize(min geo.Size) {
	h.min = min
}

// Hide hides the host and its native view.
func (h *NativeViewHost) Hide() {
	h.BaseWidget.Hide()
	h.sync()
}

// Show shows the host and its native view.
func (h *NativeViewHost) Show() {
	h.BaseWidget.Show()
	h.sync()
}

// Resize resizes the host and re-places its native view.
func (h *NativeViewHost) Resize(size geo.Size) {
	h.BaseWidget.Resize(size)
	h.sync()
}

// Move moves the host and re-places its native view.
func (h *NativeViewHost) Move(pos geo.Position) {
	h.BaseWidget.Move(pos)
	h.sync()
}

// Refresh re-places the native view along with the widget.
func (h *NativeViewHost) Refresh() {
	h.BaseWidget.Refresh()
	h.sync()
}

// View returns the platform view this host drives.
func (h *NativeViewHost) View() NativeView {
	return h.view
}

// FocusNativeView hands the OS keyboard focus to the platform view, the way a
// click inside it would, and clears the canvas focus so no bellui widget is
// left drawing itself focused. It reports whether the platform accepted.
//
// It is the programmatic way into a native view: the host is deliberately not
// a Tab stop (see [NativeViewHost]), so an app that wants the keyboard to
// start in the embedded content calls this. It reports false for a backend
// that does not implement [NativeViewFocuser], or one that has not attached.
func (h *NativeViewHost) FocusNativeView() bool {
	f, ok := h.focuser()
	if !ok {
		return false
	}
	if !f.TakeFocus() {
		return false
	}
	h.nativeFocus = true
	if c := h.canvas(); c != nil {
		c.Unfocus()
	}
	return true
}

// NativeViewFocused reports whether the OS keyboard focus is inside the
// platform view, meaning keys and input-method composition are going to it
// rather than to bellui. It reports false for a backend that cannot take
// focus.
func (h *NativeViewHost) NativeViewFocused() bool {
	return h.nativeFocus
}

// SyncStats reports how many per-frame geometry evaluations have run and how
// many of them actually reached the backend. It exists to make the cost of
// the per-frame sync measurable; the counters are cumulative for the life of
// the host.
func (h *NativeViewHost) SyncStats() (syncs, applies uint64) {
	return h.syncs.Load(), h.applies.Load()
}

// sync is the single seam: it resolves the host's canvas-absolute rectangle
// and clip bounds, and pushes them to the backend when either changed. Every
// path that can move the native view — a widget mutation above, and the
// per-frame ticker for movement the widget is never told about (a parent
// scrolling, a window resize) — funnels through here.
func (h *NativeViewHost) sync() {
	if h.view == nil {
		return
	}
	h.syncs.Add(1)

	app := bellui.CurrentApp()
	if app == nil || app.Driver() == nil {
		return
	}
	d := app.Driver()
	c := d.CanvasForObject(h)
	if c == nil {
		h.unresolved()
		return
	}

	// GeometryForObject walks the *visible* tree, so a false ok covers the
	// host being hidden, an ancestor being hidden, and the host having been
	// removed from the tree — all of which must hide the native view. These
	// are the only states the ticker stops in: each of them is reached, and
	// left again, through a widget-level event (Show, Move, Refresh) that
	// re-enters sync and re-arms it.
	//
	// One walk answers all three questions this path used to ask separately
	// (in the tree? where? clipped by what?), which matters because it runs
	// every frame for every hosted view.
	pos, clip, clipped, ok := driver.GeometryForObject(c, h)
	if !ok {
		h.unresolved()
		return
	}

	// In the visible tree: from here the ticker must run, so that a state the
	// host is never told about — a parent scrolling it back into view, or the
	// owning window becoming resolvable a frame later — is still noticed.
	h.startSync()

	if !h.attach(c) {
		return
	}

	rect := geo.Rectangle{Pos: pos, Size: h.Size()}
	if !clipped {
		clip = geo.Rectangle{Size: c.Size()}
	}
	vis, ok := intersectRects(rect, clip)
	if !ok {
		// Scrolled entirely out of a clipping ancestor. The host is still in
		// the visible tree and nothing about it will change when the scroll
		// brings it back, so the ticker has to keep running — it is the only
		// thing that will notice.
		h.setShown(false)
		return
	}

	scale := c.Scale()
	if rect != h.lastRect || vis != h.lastClip || scale != h.lastScale {
		h.lastRect, h.lastClip, h.lastScale = rect, vis, scale
		h.applies.Add(1)
		h.view.SetGeometry(rect, vis, scale)
	}
	h.setShown(true)
	h.syncFocus(c)
}

// syncFocus bridges the platform responder chain to canvas focus, on the same
// per-frame pass as the geometry (#59). Polling is what makes the bridge
// portable: a backend answers one cheap question instead of every platform
// growing a focus-change notification of its own.
//
// The two directions are told apart by an invariant rather than by timing: a
// native gain always clears the canvas focus, so "the platform view holds the
// OS focus *and* bellui has a focused widget" can only mean bellui took the
// focus for itself since the last frame.
func (h *NativeViewHost) syncFocus(c core.Canvas) {
	f, ok := h.focuser()
	if !ok {
		return
	}

	has := f.HasFocus()
	if has != h.nativeFocus {
		h.nativeFocus = has
		if has {
			// Native -> bellui. The user clicked into the platform view (or it
			// claimed the focus itself); stop bellui drawing a focused widget
			// whose keystrokes are going somewhere else.
			c.Unfocus()
		}
		// A native loss the host did not ask for needs nothing: the canvas
		// focus is already nil, and whatever took the focus owns it now.
		return
	}
	if !has || c.Focused() == nil {
		return
	}

	// Bellui -> native. Hand the OS focus back, or the platform view would go
	// on receiving the keys the focused bellui widget is drawing itself ready
	// for.
	if f.ResignFocus() {
		h.nativeFocus = false
	}
}

// focuser reports the backend's focus capability, if it has one. It also
// gates on the view having attached: a platform view that does not exist can
// hold no focus, and asking it would reach a nil handle.
func (h *NativeViewHost) focuser() (NativeViewFocuser, bool) {
	if h.win == nil {
		return nil, false
	}
	f, ok := h.view.(NativeViewFocuser)
	return f, ok
}

// resignNativeFocus takes the OS focus back from the platform view for states
// in which it must not keep it — being hidden, or torn down — where leaving
// it there would send keystrokes to a view the user can no longer see.
func (h *NativeViewHost) resignNativeFocus() {
	if !h.nativeFocus {
		return
	}
	h.nativeFocus = false
	if f, ok := h.focuser(); ok {
		f.ResignFocus()
	}
}

// canvas resolves the canvas this host is laid out on, or nil.
func (h *NativeViewHost) canvas() core.Canvas {
	app := bellui.CurrentApp()
	if app == nil || app.Driver() == nil {
		return nil
	}
	return app.Driver().CanvasForObject(h)
}

// attach creates the platform view the first time the host reaches a canvas,
// resolving the bellui.Window that owns it — the backend needs the window to
// reach its native handle through driver.NativeWindow.RunNative.
func (h *NativeViewHost) attach(c core.Canvas) bool {
	if h.attached {
		return h.win != nil
	}

	// Resolving the window is retried every frame: on the first layout pass
	// the canvas may not be registered against a window yet, and giving up
	// permanently there would leave the host silently inert forever.
	var win bellui.Window
	for _, w := range bellui.CurrentApp().Driver().AllWindows() {
		if w.Canvas() == c {
			win = w
			break
		}
	}
	if win == nil {
		return false
	}

	// The backend, by contrast, gets exactly one Attach: a platform view that
	// failed to create will not succeed on the next frame, and retrying would
	// spam the log sixty times a second.
	h.attached = true
	if err := h.view.Attach(win); err != nil {
		bellui.LogError("Failed to attach native view", err)
		return false
	}
	h.win = win
	return true
}

// setShown toggles the native view's visibility. It deliberately does not
// touch the ticker: a view hidden because it scrolled out of a clipping
// ancestor has to keep being evaluated, since scrolling it back into view
// produces no event on the host itself.
func (h *NativeViewHost) setShown(shown bool) {
	if h.win == nil || h.shown == shown {
		return
	}
	h.shown = shown
	if !shown {
		// Give the OS focus up before disappearing, while the platform view is
		// still there to hand it over: a hidden view holding the keyboard focus
		// would swallow every keystroke with nothing on screen to show for it.
		h.resignNativeFocus()
	}
	h.view.SetVisible(shown)
}

// detached hides the native view and stops the ticker, for the states in
// which nothing can change until a widget-level event re-enters sync: no
// canvas, or not in the visible tree.
// unresolved handles the two very different states that look identical at the
// probes above: the host has no canvas, or it is not in the visible tree.
//
// Once the host has attached, that is a genuine detach — hidden, an ancestor
// hidden, removed from the tree — and the ticker stops, to be re-armed by the
// widget-level event that reverses it.
//
// Before the first attach it means the opposite: the driver has not yet
// painted (and cached) the tree, which is the ordinary state during the
// initial SetContent layout, when the widget events that drive sync fire.
// Stopping there left the host inert forever on a screen where nothing else
// moves — no later event arrives to re-enter sync, so the platform view never
// attached and the window stayed blank (#167). Arming the ticker instead lets
// the next frame resolve the canvas: the same per-frame retry attach() already
// does one level down for the window, and for the same reason — giving up
// permanently on a state that is merely early leaves the host silently inert.
//
// The ticker this arms is bounded by the widget lifecycle: destroy(), which
// the renderer calls when the host leaves the tree or its window closes,
// stops it.
func (h *NativeViewHost) unresolved() {
	h.setShown(false)
	if h.attached {
		h.stopSync()
		return
	}
	h.startSync()
}

// startSync arms the per-frame geometry ticker (the #417 prefill / #758
// selection-handle pattern). A native view is not in the widget tree, so no
// layout pass reaches it: only re-evaluating every frame keeps it glued to a
// host that a parent scroll, window resize or animation is moving.
func (h *NativeViewHost) startSync() {
	if h.anim != nil {
		return
	}
	if app := bellui.CurrentApp(); app == nil || app.Driver() == nil {
		return
	}
	anim := &bellui.Animation{
		Duration:    time.Second, // irrelevant: Tick fires every frame while running
		RepeatCount: bellui.AnimationRepeatForever,
		Curve:       bellui.AnimationLinear,
		Tick:        func(float32) { h.sync() },
	}
	h.anim = anim
	anim.Start()
}

func (h *NativeViewHost) stopSync() {
	if h.anim == nil {
		return
	}
	h.anim.Stop()
	h.anim = nil
}

// destroy tears the platform view down. It runs from the renderer's Destroy,
// which the toolkit calls when the host leaves the tree or its window closes.
func (h *NativeViewHost) destroy() {
	h.stopSync()
	h.resignNativeFocus()
	if h.view != nil {
		h.view.Destroy()
	}
	h.win = nil
	h.attached = false
	h.shown = false
	h.lastRect, h.lastClip, h.lastScale = geo.Rectangle{}, geo.Rectangle{}, 0
}

// intersectRects returns the overlap of a and b, and whether it is non-empty.
func intersectRects(a, b geo.Rectangle) (geo.Rectangle, bool) {
	x0 := geo.Max(a.Pos.X, b.Pos.X)
	y0 := geo.Max(a.Pos.Y, b.Pos.Y)
	x1 := geo.Min(a.Pos.X+a.Size.Width, b.Pos.X+b.Size.Width)
	y1 := geo.Min(a.Pos.Y+a.Size.Height, b.Pos.Y+b.Size.Height)
	if x1 <= x0 || y1 <= y0 {
		return geo.Rectangle{}, false
	}
	return geo.NewRectangle(x0, y0, x1-x0, y1-y0), true
}

var _ core.WidgetRenderer = (*nativeViewHostRenderer)(nil)

// nativeViewHostRenderer draws nothing: the host's rectangle is a hole in
// bellui's surface through which the OS view is seen.
type nativeViewHostRenderer struct {
	host *NativeViewHost
}

func (r *nativeViewHostRenderer) Layout(geo.Size)              { r.host.sync() }
func (r *nativeViewHostRenderer) MinSize() geo.Size            { return geo.Size{} }
func (r *nativeViewHostRenderer) Objects() []core.CanvasObject { return nil }
func (r *nativeViewHostRenderer) Refresh()                     { r.host.sync() }
func (r *nativeViewHostRenderer) Destroy()                     { r.host.destroy() }
