package test

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

type window struct {
	title              string
	fullScreen         bool
	fixedSize          bool
	focused            bool
	onClosed           func()
	onCloseIntercepted func()

	canvas *canvas
	driver *driver
	menu   *bellui.MainMenu
}

// NewWindow creates and registers a new window for test purposes
func NewWindow(content core.CanvasObject) bellui.Window {
	window := bellui.CurrentApp().NewWindow("")
	window.SetContent(content)
	return window
}

func (w *window) Canvas() core.Canvas {
	return w.canvas
}

func (w *window) CenterOnScreen() {
	// no-op
}

func (w *window) Clipboard() bellui.Clipboard {
	return NewClipboard()
}

func (w *window) Close() {
	if w.onClosed != nil {
		w.onClosed()
	}
	w.focused = false
	w.driver.removeWindow(w)
}

func (w *window) Content() core.CanvasObject {
	return w.Canvas().Content()
}

func (w *window) FixedSize() bool {
	return w.fixedSize
}

func (w *window) FullScreen() bool {
	return w.fullScreen
}

func (w *window) Hide() {
	w.focused = false
}

func (w *window) Icon() bellui.Resource {
	return bellui.CurrentApp().Icon()
}

func (w *window) MainMenu() *bellui.MainMenu {
	return w.menu
}

func (w *window) Padded() bool {
	return w.canvas.Padded()
}

func (w *window) RequestFocus() {
	for _, win := range w.driver.AllWindows() {
		win.(*window).focused = false
	}

	w.focused = true
}

func (w *window) Resize(size geo.Size) {
	w.canvas.Resize(size)
}

func (w *window) SetContent(obj core.CanvasObject) {
	w.Canvas().SetContent(obj)
}

func (w *window) SetFixedSize(fixed bool) {
	w.fixedSize = fixed
}

func (w *window) SetIcon(_ bellui.Resource) {
	// no-op
}

func (w *window) SetFullScreen(fullScreen bool) {
	w.fullScreen = fullScreen
}

func (w *window) SetMainMenu(menu *bellui.MainMenu) {
	w.menu = menu
}

func (w *window) SetMaster() {
	// no-op
}

func (w *window) SetOnClosed(closed func()) {
	w.onClosed = closed
}

func (w *window) SetCloseIntercept(callback func()) {
	w.onCloseIntercepted = callback
}

func (w *window) SetOnDropped(dropped func(geo.Position, []bellui.URI)) {
}

func (w *window) SetPadded(padded bool) {
	w.canvas.SetPadded(padded)
}

func (w *window) SelectableMode() bellui.SelectableMode {
	return w.canvas.SelectableMode()
}

func (w *window) SetSelectableMode(mode bellui.SelectableMode) {
	w.canvas.SetSelectableMode(mode)
}

func (w *window) SetTitle(title string) {
	w.title = title
}

func (w *window) Show() {
	w.RequestFocus()
}

func (w *window) ShowAndRun() {
	w.Show()
}

func (w *window) Title() string {
	return w.title
}
