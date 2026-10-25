//go:build !wasm

package mobile

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

type window struct {
	title              string
	visible            bool
	onClosed           func()
	onCloseIntercepted func()
	isChild            bool

	driver *driver // the driver that created and owns this window
	canvas *canvas
	icon   bellui.Resource
	menu   *bellui.MainMenu
	handle uintptr // the window handle - currently just Android

	// surfaceStale marks that handle changed under an initialized canvas, so the
	// painter's native surface must be rebuilt before the next paint. Deferred
	// rather than rebuilt on the spot because the replacement window may already
	// be dead by the time the queued paint event carrying it is drained (#283).
	surfaceStale bool
}

func (w *window) Title() string {
	return w.title
}

func (w *window) SetTitle(title string) {
	w.title = title
}

func (w *window) FullScreen() bool {
	return true
}

func (w *window) SetFullScreen(bool) {
	// no-op
}

func (w *window) Resize(size geo.Size) {
	w.Canvas().(*canvas).Resize(size)
}

func (w *window) RequestFocus() {
	// no-op - we cannot change which window is focused
}

func (w *window) FixedSize() bool {
	return true
}

func (w *window) SetFixedSize(bool) {
	// no-op - all windows are fixed size
}

func (w *window) CenterOnScreen() {
	// no-op
}

func (w *window) Padded() bool {
	return w.canvas.padded
}

func (w *window) SetPadded(padded bool) {
	w.canvas.padded = padded
}

func (w *window) SelectableMode() bellui.SelectableMode {
	return w.canvas.selectableMode
}

func (w *window) SetSelectableMode(mode bellui.SelectableMode) {
	w.canvas.SetSelectableMode(mode)
}

func (w *window) Icon() bellui.Resource {
	if w.icon == nil {
		return bellui.CurrentApp().Icon()
	}

	return w.icon
}

func (w *window) SetIcon(icon bellui.Resource) {
	w.icon = icon
}

func (w *window) SetMaster() {
	// no-op on mobile
}

func (w *window) MainMenu() *bellui.MainMenu {
	return w.menu
}

func (w *window) SetMainMenu(menu *bellui.MainMenu) {
	w.menu = menu
}

func (w *window) SetOnClosed(callback func()) {
	w.onClosed = callback
}

func (w *window) SetCloseIntercept(callback func()) {
	w.onCloseIntercepted = callback
}

func (w *window) SetOnDropped(dropped func(geo.Position, []bellui.URI)) {
	// not implemented yet
}

func (w *window) Show() {
	menu := bellui.CurrentApp().Driver().(*driver).findMenu(w)
	menuButton := w.newMenuButton(menu)
	if menu == nil {
		menuButton.Hide()
	}

	if w.isChild {
		exit := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
			w.tryClose()
		})
		title := widget.NewLabel(w.title)
		title.Alignment = bellui.TextAlignCenter
		w.canvas.setWindowHead(container.NewHBox(menuButton,
			layout.NewSpacer(), title, layout.NewSpacer(), exit))
		w.canvas.Resize(w.canvas.size)
	} else {
		w.canvas.setWindowHead(container.NewHBox(menuButton))
	}
	w.visible = true
	w.initAccessibilityForWindow()

	// Re-showing a previously-hidden window makes it current again: un-hide its
	// native surface layer and bring it back to the front so it covers the window
	// that was revealed beneath it while hidden (pre-rebrand tracker #742). A no-op on first show (the
	// layer is created lazily on the first paint).
	w.driver.setWindowSurfaceHidden(w, false)

	if w.Content() != nil {
		w.Content().Refresh()
		w.Content().Show()
	}
}

func (w *window) Hide() {
	w.visible = false

	if w.Content() != nil {
		w.Content().Hide()
	}

	// Hiding the visible (top) window reveals the one beneath it in the stack.
	// Toggle this window's native surface layer hidden (keeping it attached, so a
	// later Show can re-reveal it — unlike Close's detach) so it stops covering
	// the revealed window, and mark the newly-current window dirty (the paint loop
	// only repaints a dirty canvas) so it is presented rather than this window's
	// frozen last frame (pre-rebrand tracker #742, pre-rebrand tracker #645).
	w.driver.setWindowSurfaceHidden(w, true)
	w.driver.refreshCurrentWindow()
}

func (w *window) tryClose() {
	if w.onCloseIntercepted != nil {
		w.onCloseIntercepted()
		return
	}

	w.Close()
}

func (w *window) Close() {
	// Abandon any double-tap still waiting on this window's canvas: its timer
	// would otherwise fire up to TapDoubleDelay from now, into widgets whose
	// renderers this close destroys (#310).
	if c, ok := w.Canvas().(interface{ CancelPendingTap() }); ok {
		c.CancelPendingTap()
	}

	d := w.driver
	d.cancelWindowTouches(w)
	d.removeWindow(w)

	w.cleanupAccessibilityForWindow()

	cache.RangeTexturesFor(w.canvas, w.canvas.Painter().Free)
	if r, ok := w.canvas.Painter().(interface{ ReleaseCaches() }); ok {
		r.ReleaseCaches()
	}

	w.canvas.WalkTrees(nil, func(node *common.RenderCacheNode, _ geo.Position) {
		if wid, ok := node.Obj().(core.Widget); ok {
			cache.DestroyRenderer(wid)
		}
	})
	cache.CleanCanvas(w.canvas)

	if w.onClosed != nil {
		w.onClosed()
	}

	// Closing the top window pops back to the one beneath it. Detach this
	// window's native surface layer (else it keeps covering the revealed window)
	// and mark the newly-current window dirty (the paint loop only repaints a
	// dirty canvas), so the revealed window is presented rather than this one's
	// frozen last frame (pre-rebrand tracker #645).
	d.detachWindowSurface(w)
	// A closed window can never own the shared native surface; clear the
	// ownership here in the platform-neutral path (the per-window surface release
	// itself is platform-specific, in detachWindowSurface) so the revealed window
	// reclaims it on its next paint (pre-rebrand tracker #753). No-op where surface ownership is
	// unused (iOS per-window layers, GL).
	if d.surfaceOwner == w {
		d.surfaceOwner = nil
	}
	d.refreshCurrentWindow()
}

func (w *window) ShowAndRun() {
	w.Show()
	bellui.CurrentApp().Run()
}

func (w *window) Content() core.CanvasObject {
	return w.canvas.Content()
}

func (w *window) SetContent(content core.CanvasObject) {
	w.canvas.SetContent(content)
}

func (w *window) Canvas() core.Canvas {
	return w.canvas
}

func (w *window) Clipboard() bellui.Clipboard {
	return NewClipboard()
}

func (w *window) RunWithContext(f func()) {
	//	ctx, _ = e.DrawContext.(gl.Context)

	f()
}

func (w *window) RescaleContext() {
	// TODO
}

func (w *window) Context() any {
	return bellui.CurrentApp().Driver().(*driver).glctx
}
