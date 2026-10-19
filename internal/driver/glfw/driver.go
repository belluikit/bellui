// Package glfw provides a full Bellui desktop driver that uses the system OpenGL libraries.
// This supports Windows, Mac OS X and Linux using the gl and glfw packages from go-gl.
package glfw

import (
	"bytes"
	"image"
	"os"
	"runtime"
	"time"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/async"
	"github.com/belluikit/bellui/internal/ico"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/belluikit/bellui/internal/animation"
	intapp "github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/internal/painter"
	intRepo "github.com/belluikit/bellui/internal/repository"
	"github.com/belluikit/bellui/storage/repository"
)

var curWindow *window

// Declare conformity with Driver — including the desktop extensions on the
// web target, so browser apps can assert desktop.Driver /
// desktop.KeyModifierDriver for the window-wide modifier state (pre-rebrand tracker #460).
var (
	_ bellui.Driver             = (*gLDriver)(nil)
	_ desktop.Driver            = (*gLDriver)(nil)
	_ desktop.KeyModifierDriver = (*gLDriver)(nil)
)

type gLDriver struct {
	windows     []bellui.Window
	initialized bool
	done        chan struct{}

	animation animation.Runner

	currentKeyModifiers event.KeyModifier // window-wide modifier state (desktop + web)

	// modListeners holds the AddKeyModifierListener subscribers in
	// registration order; ids survive removals in the middle. Main goroutine
	// only, like the key callbacks that fire them.
	modListeners      []keyModifierListener
	nextModListenerID int

	trayStart, trayStop func()        // shut down the system tray, if used
	systrayMenu         *bellui.Menu  // cache the menu set so we know when to refresh
	systrayClickDone    chan struct{} // closed before each rebuild to stop the prior menu's click listeners

	// frameDur is the frame duration the desktop loop paces to, last
	// computed at frameChecked (loop_desktop.go, #480).
	frameDur     time.Duration
	frameChecked time.Time
}

type keyModifierListener struct {
	id int
	fn func(event.KeyModifier)
}

// CurrentKeyModifiers returns the set of key modifiers that are currently
// active for the app's windows, tracked window-wide from every key event (and
// on the web target from pointer events too), regardless of widget focus.
func (d *gLDriver) CurrentKeyModifiers() event.KeyModifier {
	return d.currentKeyModifiers
}

// AddKeyModifierListener registers fn to be called on the main goroutine
// whenever the set of active key modifiers changes, regardless of widget
// focus. It returns a remove function; multiple listeners may be registered.
// Must be called on the main goroutine.
func (d *gLDriver) AddKeyModifierListener(fn func(event.KeyModifier)) (remove func()) {
	id := d.nextModListenerID
	d.nextModListenerID++
	d.modListeners = append(d.modListeners, keyModifierListener{id: id, fn: fn})
	return func() {
		for i, l := range d.modListeners {
			if l.id == id {
				d.modListeners = append(d.modListeners[:i], d.modListeners[i+1:]...)
				return
			}
		}
	}
}

// setCurrentKeyModifiers stores the window-wide modifier state and notifies
// the modifier listeners when it actually changed. Main goroutine only.
func (d *gLDriver) setCurrentKeyModifiers(mods event.KeyModifier) {
	if mods == d.currentKeyModifiers {
		return
	}
	d.currentKeyModifiers = mods
	for _, l := range d.modListeners {
		l.fn(mods)
	}
}

func (d *gLDriver) init() {
	if !d.initialized {
		d.initialized = true
		d.initGLFW()
	}
}

func toOSIcon(icon []byte) ([]byte, error) {
	if runtime.GOOS != "windows" {
		return icon, nil
	}

	img, _, err := image.Decode(bytes.NewReader(icon))
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	err = ico.Encode(buf, img)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (d *gLDriver) DoFromGoroutine(f func(), wait bool) {
	if wait {
		async.EnsureNotMain(func() {
			runOnMainWithWait(f, true)
		})
	} else {
		runOnMainWithWait(f, false)
	}
}

func (d *gLDriver) RenderedTextSize(text string, textSize float32, style bellui.TextStyle, source bellui.Resource) (size geo.Size, baseline float32) {
	return painter.RenderedTextSize(text, textSize, style, source)
}

func (d *gLDriver) CanvasForObject(obj core.CanvasObject) core.Canvas {
	return common.CanvasForObject(obj)
}

func (d *gLDriver) AbsolutePositionForObject(co core.CanvasObject) geo.Position {
	c := d.CanvasForObject(co)
	if c == nil {
		return geo.NewPos(0, 0)
	}

	glc := c.(*glCanvas)
	return driver.AbsolutePositionForObject(co, glc.ObjectTrees())
}

func (d *gLDriver) Device() bellui.Device {
	return &glDevice{}
}

// AnnounceToScreenReader implements the optional bellui.AccessibilityAnnouncer
// interface by fanning the message out to the registered accessibility
// backends (pre-rebrand tracker #525). A no-op without the accessibility build tag or while no
// backend listener is registered.
func (d *gLDriver) AnnounceToScreenReader(text string, assertive bool) {
	accessibility.Announce(text, assertive)
}

func (d *gLDriver) Quit() {
	if curWindow != nil {
		if f := bellui.CurrentApp().Lifecycle().(*intapp.Lifecycle).OnExitedForeground(); f != nil {
			f()
		}
		curWindow = nil
		if d.trayStop != nil {
			d.trayStop()
		}
		if d.systrayClickDone != nil {
			close(d.systrayClickDone)
			d.systrayClickDone = nil
		}
	}

	// Only call close once to avoid panic.
	if running.CompareAndSwap(true, false) {
		close(d.done)
	}
}

func (d *gLDriver) addWindow(w *window) {
	d.windows = append(d.windows, w)
}

// a trivial implementation of "focus previous" - return to the most recently opened, or master if set.
// This may not do the right thing if your app has 3 or more windows open, but it was agreed this was not much
// of an issue, and the added complexity to track focus was not needed at this time.
func (d *gLDriver) focusPreviousWindow() {
	var chosen *window
	for _, w := range d.windows {
		win := w.(*window)
		if !win.visible {
			continue
		}
		chosen = win
		if win.master {
			break
		}
	}

	if chosen == nil || chosen.view() == nil {
		return
	}
	chosen.RequestFocus()
}

func (d *gLDriver) windowList() []bellui.Window {
	return d.windows
}

func (d *gLDriver) initFailed(msg string, err error) {
	bellui.LogError(msg, err)

	if !running.Load() {
		d.Quit()
	} else {
		os.Exit(1)
	}
}

func (d *gLDriver) Run() {
	if !async.IsMainGoroutine() {
		panic("Run() or ShowAndRun() must be called from main goroutine")
	}

	go d.catchTerm()
	d.runGL()

	// Ensure lifecycle events run to completion before the app exits
	l := bellui.CurrentApp().Lifecycle().(*intapp.Lifecycle)
	l.WaitForEvents()
	l.DestroyEventQueue()
}

func (d *gLDriver) SetDisableScreenBlanking(disable bool) {
	setDisableScreenBlank(disable)
}

// NewGLDriver sets up a new Driver instance implemented using the GLFW Go library and OpenGL bindings.
func NewGLDriver() *gLDriver {
	repository.Register("file", intRepo.NewFileRepository())

	return &gLDriver{
		done: make(chan struct{}),
	}
}
