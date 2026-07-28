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
		h.detached()
		return
	}

	// AncestorsOfObject walks the *visible* tree, so a false ok covers the
	// host being hidden, an ancestor being hidden, and the host having been
	// removed from the tree — all of which must hide the native view. These
	// are the only states the ticker stops in: each of them is reached, and
	// left again, through a widget-level event (Show, Move, Refresh) that
	// re-enters sync and re-arms it.
	if _, ok := driver.AncestorsOfObject(c, h); !ok {
		h.detached()
		return
	}

	// In the visible tree: from here the ticker must run, so that a state the
	// host is never told about — a parent scrolling it back into view, or the
	// owning window becoming resolvable a frame later — is still noticed.
	h.startSync()

	if !h.attach(c) {
		return
	}

	rect := geo.Rectangle{Pos: d.AbsolutePositionForObject(h), Size: h.Size()}
	clip, clipped := driver.ClipBoundsForObject(c, h)
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
	h.view.SetVisible(shown)
}

// detached hides the native view and stops the ticker, for the states in
// which nothing can change until a widget-level event re-enters sync: no
// canvas, or not in the visible tree.
func (h *NativeViewHost) detached() {
	h.setShown(false)
	h.stopSync()
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
