package embedded

import (
	"image"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/embedded"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/async"
	"github.com/belluikit/bellui/internal/cache"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/internal/painter"
)

type noosDriver struct {
	events chan embedded.Event
	queue  chan funcData
	render func(image.Image)
	run    func(func())
	size   func() geo.Size
	done   bool

	wins    []bellui.Window
	coast   common.DragCoast // a released drag coasting on; the next touch stops it (#751)
	current int
	device  noosDevice
}

func (n *noosDriver) CreateWindow(_ string) bellui.Window {
	w := newWindow(n)
	n.wins = append(n.wins, w)
	n.current = len(n.wins) - 1

	if f := n.size; f != nil {
		w.Resize(f())
	}
	return w
}

func (n *noosDriver) AllWindows() []bellui.Window {
	return n.wins
}

func (n *noosDriver) RenderedTextSize(text string, fontSize float32, style bellui.TextStyle, source bellui.Resource) (size geo.Size, baseline float32) {
	return painter.RenderedTextSize(text, fontSize, style, source)
}

func (n *noosDriver) CanvasForObject(obj core.CanvasObject) core.Canvas {
	return cache.GetCanvasForObject(obj)
}

func (n *noosDriver) AbsolutePositionForObject(o core.CanvasObject) geo.Position {
	c := n.CanvasForObject(o)
	if c == nil {
		return geo.NewPos(0, 0)
	}

	pos := intdriver.AbsolutePositionForObject(o, []core.CanvasObject{c.Content()})
	inset, _ := c.InteractiveArea()
	return pos.Subtract(inset)
}

func (n *noosDriver) Device() bellui.Device {
	return &n.device
}

func (n *noosDriver) Run() {
	n.run(n.doRun)
}

func (n *noosDriver) doRun() {
	for _, w := range n.wins {
		n.renderWindow(w)
	}

	for !n.done {
		select {
		case fn := <-n.queue:
			fn.f()
			if fn.done != nil {
				fn.done <- struct{}{}
			}
		case e := <-n.events:
			if e == nil {
				// closing
				n.Quit()
				continue
			}

			w := n.wins[n.current].(*noosWindow)

			switch t := e.(type) {
			case *embedded.CharacterEvent:
				w.tabEscape.Disarm()
				if focused := w.c.Focused(); focused != nil {
					focused.TypedRune(t.Rune)
				} else if tr := w.c.OnTypedRune(); tr != nil {
					tr(t.Rune)
				}

				n.renderWindow(n.wins[n.current])
			case *embedded.KeyEvent:
				keyEvent := &event.KeyEvent{Name: t.Name, Modifier: t.Modifiers}

				if t.Direction == embedded.KeyReleased {
					// No desktop events so key/up down not reported
					continue // ignore key up in other core events
				}

				if w.tabEscape.MoveTabFocus(w.Canvas(), t.Name, t.Modifiers) {
					n.renderWindow(n.wins[n.current])
					continue
				}

				// Escape closes the innermost pop-up, menu or dialog unless the
				// focused widget is using it for a state of its own.
				if keyEvent.Name == event.KeyEscape {
					if intdriver.HandleEscape(w.c) {
						n.renderWindow(n.wins[n.current])
						continue
					}
					w.tabEscape.EscapeNotConsumed(w.c.Focused())
				}

				// No shortcut detected, pass down to TypedKey
				focused := w.c.Focused()
				if focused != nil {
					// Keyboard-activation parity with taps (pre-rebrand tracker #700).
					if act, ok := focused.(event.Activatable); ok && act.ActivatedByKey(keyEvent) {
						if co, ok := focused.(core.CanvasObject); ok {
							core.NotifyActivation(co, true)
						}
					}
					focused.TypedKey(keyEvent)
				} else if tk := w.c.OnTypedKey(); tk != nil {
					tk(keyEvent)
				}

				n.renderWindow(n.wins[n.current])
			case *embedded.TouchDownEvent:
				n.handleTouchDown(t, n.wins[n.current].(*noosWindow))
			case *embedded.TouchMoveEvent:
				n.handleTouchMove(t, n.wins[n.current].(*noosWindow))
			case *embedded.TouchUpEvent:
				n.handleTouchUp(t, n.wins[n.current].(*noosWindow))
			}
		}
	}
}

func (n *noosDriver) handleTouchDown(ev *embedded.TouchDownEvent, w *noosWindow) {
	if n.coast.Stop() { // a new touch stops a coasting drag dead (#751)
		w.c.CatchCoast(ev.ID) // and only stops it (#792)
	}
	w.c.TapDown(ev.Position, ev.ID)
	n.renderWindow(w)
}

func (n *noosDriver) handleTouchMove(ev *embedded.TouchMoveEvent, w *noosWindow) {
	w.c.TapMove(ev.Position, ev.ID, func(wid event.Draggable, ev *event.DragEvent) {
		wid.Dragged(ev)
	})
	n.renderWindow(w)
}

func (n *noosDriver) handleTouchUp(ev *embedded.TouchUpEvent, w *noosWindow) {
	w.c.TapUp(ev.Position, ev.ID, func(wid event.Tappable, ev *event.PointEvent) {
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
		n.coast.Release(wid, ev, func(f func()) { n.DoFromGoroutine(f, false) })
	})
	n.renderWindow(w)
}

func (n *noosDriver) Quit() {
	n.done = true

	go func() {
		n.queue <- funcData{f: func() {}}
	}()
}

func (n *noosDriver) StartAnimation(*bellui.Animation) {
	// no animations on embedded
}

func (n *noosDriver) StopAnimation(*bellui.Animation) {
	// no animations on embedded
}

func (n *noosDriver) DoubleTapDelay() time.Duration {
	return common.TapDoubleDelay
}

func (n *noosDriver) SetDisableScreenBlanking(bool) {}

func (n *noosDriver) DoFromGoroutine(fn func(), wait bool) {
	if wait {
		async.EnsureNotMain(func() {
			done := make(chan struct{})

			n.queue <- funcData{f: fn, done: done}
			<-done
		})
	} else {
		n.queue <- funcData{f: fn}
	}
}

// TODO add some caching to stop allocating images...
func (n *noosDriver) renderWindow(w bellui.Window) {
	img := w.Canvas().Capture()

	n.render(img)
}

func NewNoOSDriver(render func(img image.Image), run func(func()), events chan embedded.Event, size func() geo.Size) bellui.Driver {
	return &noosDriver{
		events: events, queue: make(chan funcData), size: size,
		render: render, run: run, wins: make([]bellui.Window, 0),
	}
}

type funcData struct {
	f    func()
	done chan struct{} // Zero allocation signalling channel
}
