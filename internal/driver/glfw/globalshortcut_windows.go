//go:build windows

package glfw

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
)

// This is the Windows global-hotkey backend for #518. It drives the Win32
// RegisterHotKey / UnregisterHotKey API purely through x/sys/windows syscalls
// (no cgo), mirroring the native file-dialog style. WM_HOTKEY is a thread
// message delivered to whichever thread called RegisterHotKey, so registration
// and the message pump both live on one dedicated, OS-thread-locked goroutine;
// GLFW's own pump on the main thread would silently drop these NULL-hwnd
// messages (DispatchMessage has no window to route them to).

var _ desktop.GlobalShortcutApp = (*gLDriver)(nil)

var (
	moduser32win           = windows.NewLazySystemDLL("user32.dll")
	procRegisterHotKey     = moduser32win.NewProc("RegisterHotKey")
	procUnregisterHotKey   = moduser32win.NewProc("UnregisterHotKey")
	procGetMessageW        = moduser32win.NewProc("GetMessageW")
	procPeekMessageW       = moduser32win.NewProc("PeekMessageW")
	procPostThreadMessageW = moduser32win.NewProc("PostThreadMessageW")
)

const (
	// RegisterHotKey fsModifiers flags (winuser.h).
	modAlt      = 0x0001 // MOD_ALT
	modControl  = 0x0002 // MOD_CONTROL
	modShift    = 0x0004 // MOD_SHIFT
	modWin      = 0x0008 // MOD_WIN
	modNoRepeat = 0x4000 // MOD_NOREPEAT — one message per press (Win7+)

	wmHotKey    = 0x0312 // WM_HOTKEY (wParam == hotkey id)
	wmHotKeyCmd = 0x8000 // WM_APP: wake the pump to drain queued register/unregister commands
)

// win32Msg mirrors the Win32 MSG struct; GetMessageW fills it. The nested POINT
// (two LONGs) lands at offset 36 on amd64, matching the C layout exactly.
type win32Msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

type winHotKey struct {
	id uint32
	fn func()
}

var (
	hotkeyMu      sync.Mutex
	hotkeysByName = map[string]*winHotKey{} // shortcut name -> entry
	hotkeysByID   = map[uint32]*winHotKey{} // hotkey id -> entry
	hotkeyNextID  uint32

	hotkeyPumpOnce sync.Once
	hotkeyThreadID uint32 // GetCurrentThreadId of the pump goroutine

	// hotkeyCmdCh carries thread-affine work (Register/UnregisterHotKey) to the
	// pump goroutine. Buffered so a submitter can enqueue *before* posting the
	// wake message without deadlocking against the pump.
	hotkeyCmdCh = make(chan hotkeyCmd, 64)
)

type hotkeyCmd struct {
	fn   func()
	done chan struct{}
}

// RegisterGlobalShortcut implements desktop.GlobalShortcutApp using Win32
// RegisterHotKey. The shortcut fires system-wide (even when no app window has
// focus); the handler is delivered on the main goroutine via bellui.Do.
func (d *gLDriver) RegisterGlobalShortcut(shortcut *desktop.CustomShortcut, handler func()) error {
	if shortcut == nil || handler == nil {
		return fmt.Errorf("global shortcut: nil shortcut or handler")
	}
	vk, ok := winVirtualKeyCodes[shortcut.KeyName]
	if !ok {
		return fmt.Errorf("global shortcut: key %q cannot be mapped to a hotkey", shortcut.KeyName)
	}
	mods := winHotKeyModifiers(shortcut.Modifier)
	name := shortcut.ShortcutName()
	ensureHotkeyPump()

	// Replace an existing binding for the same shortcut: drop it from the maps
	// and unregister it on the pump thread. The maps are only touched while the
	// lock is held, never across the blocking cross-thread call.
	hotkeyMu.Lock()
	if existing := hotkeysByName[name]; existing != nil {
		oldID := existing.id
		delete(hotkeysByID, oldID)
		delete(hotkeysByName, name)
		hotkeyMu.Unlock()
		runOnHotkeyThread(func() { procUnregisterHotKey.Call(0, uintptr(oldID)) })
		hotkeyMu.Lock()
	}
	hotkeyNextID++
	id := hotkeyNextID
	hotkeyMu.Unlock()

	var regErr error
	runOnHotkeyThread(func() {
		// hwnd == 0: WM_HOTKEY is posted to this (pump) thread's message queue.
		r, _, e := procRegisterHotKey.Call(0, uintptr(id), uintptr(mods|modNoRepeat), uintptr(vk))
		if r == 0 {
			regErr = fmt.Errorf("global shortcut %q could not be registered (already in use?): %w", name, e)
		}
	})
	if regErr != nil {
		return regErr
	}

	entry := &winHotKey{id: id, fn: handler}
	hotkeyMu.Lock()
	hotkeysByName[name] = entry
	hotkeysByID[id] = entry
	hotkeyMu.Unlock()
	return nil
}

// UnregisterGlobalShortcut implements desktop.GlobalShortcutApp.
func (d *gLDriver) UnregisterGlobalShortcut(shortcut *desktop.CustomShortcut) {
	if shortcut == nil {
		return
	}
	name := shortcut.ShortcutName()

	hotkeyMu.Lock()
	entry := hotkeysByName[name]
	if entry != nil {
		delete(hotkeysByID, entry.id)
		delete(hotkeysByName, name)
	}
	hotkeyMu.Unlock()
	if entry != nil {
		runOnHotkeyThread(func() { procUnregisterHotKey.Call(0, uintptr(entry.id)) })
	}
}

// ensureHotkeyPump starts the dedicated pump goroutine once and blocks until it
// has reported its thread id (needed for PostThreadMessageW).
func ensureHotkeyPump() {
	hotkeyPumpOnce.Do(func() {
		ready := make(chan uint32)
		go hotkeyPumpLoop(ready)
		hotkeyThreadID = <-ready
	})
}

// hotkeyPumpLoop runs on a locked OS thread: it owns every RegisterHotKey call
// and blocks in GetMessageW dispatching WM_HOTKEY to the app handlers.
func hotkeyPumpLoop(ready chan<- uint32) {
	// Lock (and never unlock) this goroutine's OS thread so the hotkey
	// registrations — which are thread-affine — live for the process lifetime.
	runtime.LockOSThread()

	// Force the thread message queue into existence *before* announcing our
	// thread id: PostThreadMessageW fails (ERROR_INVALID_THREAD_ID) against a
	// thread that has not yet created a queue, and it is only created on the
	// first message call. Without this, a register request racing ahead of the
	// GetMessageW below would lose its wake message and deadlock the caller.
	const pmNoRemove = 0
	var probe win32Msg
	procPeekMessageW.Call(uintptr(unsafe.Pointer(&probe)), 0, 0, 0, pmNoRemove)

	ready <- windows.GetCurrentThreadId()

	var m win32Msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		switch int32(r) {
		case -1: // GetMessage error — keep pumping rather than spin out
			continue
		case 0: // WM_QUIT
			return
		}
		switch m.message {
		case wmHotKeyCmd:
			drainHotkeyCommands()
		case wmHotKey:
			dispatchHotkey(uint32(m.wParam))
		}
	}
}

// runOnHotkeyThread executes fn synchronously on the pump goroutine (the only
// thread allowed to touch the hotkey registrations) and waits for completion.
func runOnHotkeyThread(fn func()) {
	c := hotkeyCmd{fn: fn, done: make(chan struct{})}
	hotkeyCmdCh <- c // buffered: enqueue before posting the wake message
	procPostThreadMessageW.Call(uintptr(hotkeyThreadID), wmHotKeyCmd, 0, 0)
	<-c.done
}

// drainHotkeyCommands runs every queued command on the pump thread.
func drainHotkeyCommands() {
	for {
		select {
		case c := <-hotkeyCmdCh:
			c.fn()
			close(c.done)
		default:
			return
		}
	}
}

// dispatchHotkey looks up the fired hotkey and delivers its handler on the main
// goroutine. The map read is brief and never overlaps the cross-thread call, so
// the pump thread cannot deadlock against a registering goroutine.
func dispatchHotkey(id uint32) {
	hotkeyMu.Lock()
	entry := hotkeysByID[id]
	hotkeyMu.Unlock()
	if entry != nil && entry.fn != nil {
		bellui.Do(entry.fn)
	}
}

// winHotKeyModifiers maps bellui key modifiers to RegisterHotKey MOD_* flags.
func winHotKeyModifiers(mod event.KeyModifier) uint32 {
	var out uint32
	if mod&event.KeyModifierShift != 0 {
		out |= modShift
	}
	if mod&event.KeyModifierControl != 0 {
		out |= modControl
	}
	if mod&event.KeyModifierAlt != 0 {
		out |= modAlt
	}
	if mod&event.KeyModifierSuper != 0 {
		out |= modWin
	}
	return out
}

// winVirtualKeyCodes maps bellui key names to Win32 virtual-key codes (VK_*).
var winVirtualKeyCodes = map[event.KeyName]uint32{
	event.KeyA: 0x41, event.KeyB: 0x42, event.KeyC: 0x43, event.KeyD: 0x44,
	event.KeyE: 0x45, event.KeyF: 0x46, event.KeyG: 0x47, event.KeyH: 0x48,
	event.KeyI: 0x49, event.KeyJ: 0x4A, event.KeyK: 0x4B, event.KeyL: 0x4C,
	event.KeyM: 0x4D, event.KeyN: 0x4E, event.KeyO: 0x4F, event.KeyP: 0x50,
	event.KeyQ: 0x51, event.KeyR: 0x52, event.KeyS: 0x53, event.KeyT: 0x54,
	event.KeyU: 0x55, event.KeyV: 0x56, event.KeyW: 0x57, event.KeyX: 0x58,
	event.KeyY: 0x59, event.KeyZ: 0x5A,

	event.Key0: 0x30, event.Key1: 0x31, event.Key2: 0x32, event.Key3: 0x33,
	event.Key4: 0x34, event.Key5: 0x35, event.Key6: 0x36, event.Key7: 0x37,
	event.Key8: 0x38, event.Key9: 0x39,

	event.KeyF1: 0x70, event.KeyF2: 0x71, event.KeyF3: 0x72, event.KeyF4: 0x73,
	event.KeyF5: 0x74, event.KeyF6: 0x75, event.KeyF7: 0x76, event.KeyF8: 0x77,
	event.KeyF9: 0x78, event.KeyF10: 0x79, event.KeyF11: 0x7A, event.KeyF12: 0x7B,

	event.KeySpace:     0x20, // VK_SPACE
	event.KeyReturn:    0x0D, // VK_RETURN
	event.KeyEnter:     0x0D, // no distinct keypad-enter hotkey code
	event.KeyTab:       0x09, // VK_TAB
	event.KeyEscape:    0x1B, // VK_ESCAPE
	event.KeyBackspace: 0x08, // VK_BACK
	event.KeyDelete:    0x2E, // VK_DELETE
	event.KeyLeft:      0x25, // VK_LEFT
	event.KeyUp:        0x26, // VK_UP
	event.KeyRight:     0x27, // VK_RIGHT
	event.KeyDown:      0x28, // VK_DOWN
	event.KeyHome:      0x24, // VK_HOME
	event.KeyEnd:       0x23, // VK_END
	event.KeyPageUp:    0x21, // VK_PRIOR
	event.KeyPageDown:  0x22, // VK_NEXT
}
