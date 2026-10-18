package embedded

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
)

type noosWindow struct {
	c *touchCanvas
	d *noosDriver

	// tabEscape is the Escape-then-Tab way out of a widget that keeps Tab (#357).
	tabEscape intdriver.TabEscape

	title string
}

func (w *noosWindow) Title() string {
	return w.title
}

func (w *noosWindow) SetTitle(s string) {
	w.title = s
}

func (w *noosWindow) FullScreen() bool {
	return true
}

func (w *noosWindow) SetFullScreen(_ bool) {
}

func (w *noosWindow) Resize(s geo.Size) {
	w.c.Resize(s)
}

func (w *noosWindow) RequestFocus() {
	// Make this window the driver's current (focused) one so input and
	// rendering target it, then bring it to the front.
	for i, win := range w.d.wins {
		if win == w {
			w.d.current = i
			w.d.renderWindow(w)
			return
		}
	}
}

func (w *noosWindow) FixedSize() bool {
	return true
}

func (w *noosWindow) SetFixedSize(bool) {}

func (w *noosWindow) CenterOnScreen() {}

func (w *noosWindow) Padded() bool {
	return w.c.Padded()
}

func (w *noosWindow) SetPadded(pad bool) {
	w.c.SetPadded(pad)
}

func (w *noosWindow) SelectableMode() bellui.SelectableMode {
	return w.c.SelectableMode()
}

func (w *noosWindow) SetSelectableMode(mode bellui.SelectableMode) {
	w.c.SetSelectableMode(mode)
}

func (w *noosWindow) Icon() bellui.Resource {
	// TODO implement me
	return nil
}

func (w *noosWindow) SetIcon(bellui.Resource) {
	// TODO implement me
}

func (w *noosWindow) SetMaster() {
	// TODO implement me
}

func (w *noosWindow) MainMenu() *bellui.MainMenu {
	// TODO implement me
	return nil
}

func (w *noosWindow) SetMainMenu(menu *bellui.MainMenu) {
	// TODO implement me
}

func (w *noosWindow) SetOnClosed(f func()) {
	// TODO implement me
}

func (w *noosWindow) SetCloseIntercept(f func()) {
	// TODO implement me
}

func (w *noosWindow) SetOnDropped(func(geo.Position, []bellui.URI)) {}

func (w *noosWindow) Show() {
	w.d.renderWindow(w)
}

func (w *noosWindow) Hide() {}

func (w *noosWindow) Close() {
	i := -1
	for _, win := range w.d.wins {
		if win == w {
			break
		}
		i++
	}
	if i == -1 {
		return
	}

	copy(w.d.wins[i:], w.d.wins[i+1:])
	w.d.wins[len(w.d.wins)-1] = nil // Allow the garbage collector to reclaim the memory.
	w.d.wins = w.d.wins[:len(w.d.wins)-1]

	if w.d.current > 0 {
		w.d.current--
	}
}

func (w *noosWindow) ShowAndRun() {
	w.Show()
	w.d.Run()
}

func (w *noosWindow) Content() core.CanvasObject {
	return w.c.Content()
}

func (w *noosWindow) SetContent(object core.CanvasObject) {
	w.c.SetContent(object)
}

func (w *noosWindow) Canvas() core.Canvas {
	return w.c
}

func (w *noosWindow) Clipboard() bellui.Clipboard {
	// TODO implement me
	return nil
}

func newWindow(d *noosDriver) bellui.Window {
	return &noosWindow{c: newTouchCanvas(), d: d}
}
