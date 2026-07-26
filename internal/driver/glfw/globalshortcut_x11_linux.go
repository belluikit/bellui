//go:build linux && !android && !wasm && !wayland

package glfw

/*
#cgo LDFLAGS: -lX11

#include <X11/Xlib.h>
#include <X11/keysym.h>
#include <stdlib.h>
#include <unistd.h>
#include <sys/select.h>

// belluiX11HotkeyFired is the Go callback (see globalshortcut_x11_cb_linux.go),
// declared here so the pump can call it. Defined via //export elsewhere.
extern void belluiX11HotkeyFired(unsigned int keycode, unsigned int state);

// A grab is registered for all four combinations of the two lock modifiers
// (CapsLock = LockMask, NumLock = Mod2Mask) so it fires regardless of lock state.
static const unsigned int belluiLockMasks[4] = {0, LockMask, Mod2Mask, LockMask | Mod2Mask};

// Grab conflicts surface as an asynchronous BadAccess X error, so we install a
// temporary error handler around the grab and force delivery with XSync.
static int belluiGrabFailed = 0;
static XErrorHandler belluiPrevErrorHandler = 0;

static int belluiGrabErrorHandler(Display *d, XErrorEvent *e) {
    if (e->error_code == BadAccess) {
        belluiGrabFailed = 1;
        return 0;
    }
    if (belluiPrevErrorHandler)
        return belluiPrevErrorHandler(d, e);
    return 0;
}

// belluiX11GrabKey grabs keycode+mask (all lock permutations) on root. Returns -1
// if the combination is already owned by another client (BadAccess), else 0.
static int belluiX11GrabKey(Display *d, Window root, unsigned int keycode, unsigned int mask) {
    belluiGrabFailed = 0;
    belluiPrevErrorHandler = XSetErrorHandler(belluiGrabErrorHandler);
    for (int i = 0; i < 4; i++)
        XGrabKey(d, keycode, mask | belluiLockMasks[i], root, False, GrabModeAsync, GrabModeAsync);
    XSync(d, False);
    XSetErrorHandler(belluiPrevErrorHandler);
    return belluiGrabFailed ? -1 : 0;
}

static void belluiX11UngrabKey(Display *d, Window root, unsigned int keycode, unsigned int mask) {
    for (int i = 0; i < 4; i++)
        XUngrabKey(d, keycode, mask | belluiLockMasks[i], root);
    XSync(d, False);
}

static unsigned int belluiX11KeycodeFor(Display *d, KeySym sym) {
    return (unsigned int)XKeysymToKeycode(d, sym);
}

// belluiX11Wait blocks until the X connection has events or wakefd is readable
// (a queued command), draining the wake pipe. Returns the pending X event count.
// All X access happens on the caller (the pump goroutine), avoiding the need for
// XInitThreads (which GLFW has already precluded by opening its own connection).
static int belluiX11Wait(Display *d, int wakefd) {
    XFlush(d);
    int xfd = ConnectionNumber(d);
    fd_set fds;
    FD_ZERO(&fds);
    FD_SET(xfd, &fds);
    FD_SET(wakefd, &fds);
    int maxfd = xfd > wakefd ? xfd : wakefd;
    if (select(maxfd + 1, &fds, NULL, NULL, NULL) < 0)
        return 0;
    if (FD_ISSET(wakefd, &fds)) {
        char buf[64];
        while (read(wakefd, buf, sizeof buf) > 0) {
        }
    }
    return XPending(d);
}

// belluiX11HandleOne reads one queued X event; a KeyPress notifies Go.
static void belluiX11HandleOne(Display *d) {
    XEvent ev;
    XNextEvent(d, &ev);
    if (ev.type == KeyPress)
        belluiX11HotkeyFired(ev.xkey.keycode, ev.xkey.state);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
)

// errGlobalShortcutConflict reports that the key combination is already grabbed
// by another X client — the X11 analogue of the Windows "already registered".
var errGlobalShortcutConflict = errors.New("global shortcut is already registered by another application")

// x11GrabKey identifies a grab by its resolved keycode and modifier mask (lock
// bits excluded). It keys the handler map, which — like every other bit of X
// state here — is only ever touched on the pump goroutine.
type x11GrabKey struct {
	keycode C.uint
	mask    C.uint
}

// x11Command is a grab/ungrab request marshalled onto the pump goroutine (the
// sole owner of the X connection), replying with the outcome.
type x11Command struct {
	register bool
	keysym   C.KeySym
	mask     C.uint
	handler  func()
	reply    chan error
}

var (
	x11Once    sync.Once
	x11InitErr error
	x11Display *C.Display
	x11Root    C.Window
	x11WakeFd  int
	x11Cmds    chan x11Command
	x11Grabs   = map[x11GrabKey]func(){}
)

// x11ModifierBits are the modifier bits a grab is keyed on (the lock modifiers
// CapsLock/NumLock are excluded so a lock state does not defeat the match). Used
// by the //export callback in globalshortcut_x11_cb_linux.go.
var x11ModifierBits = C.uint(C.ShiftMask | C.ControlMask | C.Mod1Mask | C.Mod4Mask)

// ensureX11 opens a dedicated X connection and starts the event pump once.
func ensureX11() error {
	x11Once.Do(func() {
		disp := C.XOpenDisplay(nil)
		if disp == nil {
			x11InitErr = errors.New("global shortcuts: cannot open X display")
			return
		}
		// A raw pipe (not os.Pipe): the pump's C select() reads these fds directly,
		// so they must not be managed by Go's runtime poller — a poller-managed
		// os.Pipe write is invisible to a raw select and deadlocks the pump.
		var fds [2]int
		if err := syscall.Pipe2(fds[:], syscall.O_CLOEXEC|syscall.O_NONBLOCK); err != nil {
			C.XCloseDisplay(disp)
			x11InitErr = err
			return
		}

		x11Display = disp
		x11Root = C.XDefaultRootWindow(disp)
		x11WakeFd = fds[1]
		x11Cmds = make(chan x11Command, 16)
		go x11Pump(fds[0])
	})
	return x11InitErr
}

// x11Pump owns the X connection: it waits for events or queued commands, runs
// the commands (grab/ungrab, all X calls on this goroutine), then dispatches any
// key events. Modelled on the Windows WM_HOTKEY pump.
func x11Pump(wakeReadFd int) {
	runtime.LockOSThread()
	wakeFd := C.int(wakeReadFd)
	for {
		pending := C.belluiX11Wait(x11Display, wakeFd)

	drain:
		for {
			select {
			case cmd := <-x11Cmds:
				runX11Command(cmd)
			default:
				break drain
			}
		}

		for i := 0; i < int(pending); i++ {
			C.belluiX11HandleOne(x11Display)
		}
	}
}

func runX11Command(cmd x11Command) {
	keycode := C.belluiX11KeycodeFor(x11Display, cmd.keysym)
	if keycode == 0 {
		cmd.reply <- errors.New("global shortcut: key has no keycode on this keyboard")
		return
	}
	key := x11GrabKey{keycode: keycode, mask: cmd.mask}
	if cmd.register {
		if C.belluiX11GrabKey(x11Display, x11Root, keycode, cmd.mask) != 0 {
			cmd.reply <- errGlobalShortcutConflict
			return
		}
		x11Grabs[key] = cmd.handler
		cmd.reply <- nil
		return
	}
	C.belluiX11UngrabKey(x11Display, x11Root, keycode, cmd.mask)
	delete(x11Grabs, key)
	cmd.reply <- nil
}

func x11Wake() {
	if x11Cmds != nil {
		_, _ = syscall.Write(x11WakeFd, []byte{0})
	}
}

// registerX11GlobalShortcut is the X11 backend of desktop.GlobalShortcutApp.
func (d *gLDriver) registerX11GlobalShortcut(shortcut *desktop.CustomShortcut, handler func()) error {
	if err := ensureX11(); err != nil {
		return err
	}
	sym := x11KeysymForKey(shortcut.Key())
	if sym == 0 {
		return fmt.Errorf("global shortcut: unmappable key %q", shortcut.Key())
	}
	reply := make(chan error, 1)
	x11Cmds <- x11Command{register: true, keysym: sym, mask: x11ModMask(shortcut.Mod()), handler: handler, reply: reply}
	x11Wake()
	return <-reply
}

func (d *gLDriver) unregisterX11GlobalShortcut(shortcut *desktop.CustomShortcut) {
	if x11Cmds == nil {
		return
	}
	sym := x11KeysymForKey(shortcut.Key())
	if sym == 0 {
		return
	}
	reply := make(chan error, 1)
	x11Cmds <- x11Command{register: false, keysym: sym, mask: x11ModMask(shortcut.Mod()), reply: reply}
	x11Wake()
	<-reply
}

// x11ModMask maps bellui modifiers to the X11 modifier mask.
func x11ModMask(mods event.KeyModifier) C.uint {
	var m C.uint
	if mods&event.KeyModifierShift != 0 {
		m |= C.ShiftMask
	}
	if mods&event.KeyModifierControl != 0 {
		m |= C.ControlMask
	}
	if mods&event.KeyModifierAlt != 0 {
		m |= C.Mod1Mask
	}
	if mods&event.KeyModifierSuper != 0 {
		m |= C.Mod4Mask
	}
	return m
}

// x11KeysymForKey resolves a bellui key name to an X11 keysym via the (thread-safe,
// display-free) XStringToKeysym. Returns 0 when unmappable.
func x11KeysymForKey(key event.KeyName) C.KeySym {
	name := x11KeysymName(key)
	if name == "" {
		return 0
	}
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))
	return C.XStringToKeysym(cs)
}

// x11KeysymName maps a bellui key name to the X keysym string XStringToKeysym
// expects. Single ASCII letters lowercase to their keysym (bellui "A" → "a");
// digits and function keys pass through; a few named keys are translated.
func x11KeysymName(key event.KeyName) string {
	s := string(key)
	if len(s) == 1 {
		if s[0] >= 'A' && s[0] <= 'Z' {
			return strings.ToLower(s)
		}
		return s
	}
	switch key {
	case event.KeySpace:
		return "space"
	case event.KeyReturn, event.KeyEnter:
		return "Return"
	case event.KeyTab:
		return "Tab"
	case event.KeyEscape:
		return "Escape"
	case event.KeyBackspace:
		return "BackSpace"
	case event.KeyDelete:
		return "Delete"
	case event.KeyInsert:
		return "Insert"
	case event.KeyHome:
		return "Home"
	case event.KeyEnd:
		return "End"
	case event.KeyPageUp:
		return "Prior"
	case event.KeyPageDown:
		return "Next"
	case event.KeyUp:
		return "Up"
	case event.KeyDown:
		return "Down"
	case event.KeyLeft:
		return "Left"
	case event.KeyRight:
		return "Right"
	}
	// F1..F24 and any other name are passed through as-is (XStringToKeysym
	// returns NoSymbol for anything it does not recognise → treated as unmappable).
	return s
}
