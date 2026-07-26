//go:build darwin

package glfw

/*
#cgo LDFLAGS: -framework Carbon

unsigned long registerHotKey(unsigned int id, unsigned int keyCode, unsigned int modifiers);
void unregisterHotKey(unsigned long ref);
*/
import "C"

import (
	"fmt"
	"sync"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
)

var _ desktop.GlobalShortcutApp = (*gLDriver)(nil)

type globalHotKey struct {
	id  uint32
	ref uint64 // Carbon EventHotKeyRef
	fn  func()
}

var (
	globalHotKeyMu   sync.Mutex
	globalHotKeys    = map[string]*globalHotKey{} // shortcut name -> entry
	globalHotKeyByID = map[uint32]*globalHotKey{} // Carbon id -> entry
	globalHotKeyNext uint32
)

// RegisterGlobalShortcut implements desktop.GlobalShortcutApp using Carbon's
// RegisterEventHotKey (the sanctioned system-wide hotkey API — no accessibility
// permission required, unlike a CGEventTap). The hotkey fires into the Carbon
// event handler installed on first use, which is pumped by the normal event
// loop, so no extra thread is needed.
func (d *gLDriver) RegisterGlobalShortcut(shortcut *desktop.CustomShortcut, handler func()) error {
	if shortcut == nil || handler == nil {
		return fmt.Errorf("global shortcut: nil shortcut or handler")
	}
	keyCode, ok := macVirtualKeyCodes[shortcut.KeyName]
	if !ok {
		return fmt.Errorf("global shortcut: key %q cannot be mapped to a hotkey", shortcut.KeyName)
	}
	mods := macCarbonModifiers(shortcut.Modifier)
	name := shortcut.ShortcutName()

	globalHotKeyMu.Lock()
	defer globalHotKeyMu.Unlock()

	// Registering the same shortcut again replaces its handler.
	if existing := globalHotKeys[name]; existing != nil {
		C.unregisterHotKey(C.ulong(existing.ref))
		delete(globalHotKeyByID, existing.id)
		delete(globalHotKeys, name)
	}

	globalHotKeyNext++
	id := globalHotKeyNext
	ref := uint64(C.registerHotKey(C.uint(id), C.uint(keyCode), C.uint(mods)))
	if ref == 0 {
		return fmt.Errorf("global shortcut %q could not be registered (already in use?)", name)
	}

	entry := &globalHotKey{id: id, ref: ref, fn: handler}
	globalHotKeys[name] = entry
	globalHotKeyByID[id] = entry
	return nil
}

// UnregisterGlobalShortcut implements desktop.GlobalShortcutApp.
func (d *gLDriver) UnregisterGlobalShortcut(shortcut *desktop.CustomShortcut) {
	if shortcut == nil {
		return
	}
	name := shortcut.ShortcutName()

	globalHotKeyMu.Lock()
	defer globalHotKeyMu.Unlock()
	if entry := globalHotKeys[name]; entry != nil {
		C.unregisterHotKey(C.ulong(entry.ref))
		delete(globalHotKeyByID, entry.id)
		delete(globalHotKeys, name)
	}
}

//export belluiGlobalHotKeyFired
func belluiGlobalHotKeyFired(id C.uint) {
	globalHotKeyMu.Lock()
	entry := globalHotKeyByID[uint32(id)]
	globalHotKeyMu.Unlock()
	if entry != nil && entry.fn != nil {
		// The Carbon handler runs on the main thread within the event loop;
		// bellui.Do delivers the app handler on the main goroutine.
		bellui.Do(entry.fn)
	}
}

// macCarbonModifiers maps bellui key modifiers to Carbon modifier flags.
func macCarbonModifiers(mod event.KeyModifier) uint32 {
	const (
		cmdKey     = 0x0100
		shiftKey   = 0x0200
		optionKey  = 0x0800
		controlKey = 0x1000
	)
	var out uint32
	if mod&event.KeyModifierShift != 0 {
		out |= shiftKey
	}
	if mod&event.KeyModifierControl != 0 {
		out |= controlKey
	}
	if mod&event.KeyModifierAlt != 0 {
		out |= optionKey
	}
	if mod&event.KeyModifierSuper != 0 {
		out |= cmdKey
	}
	return out
}

// macVirtualKeyCodes maps bellui key names to macOS virtual key codes (kVK_*).
var macVirtualKeyCodes = map[event.KeyName]uint32{
	event.KeyA: 0x00, event.KeyB: 0x0B, event.KeyC: 0x08, event.KeyD: 0x02,
	event.KeyE: 0x0E, event.KeyF: 0x03, event.KeyG: 0x05, event.KeyH: 0x04,
	event.KeyI: 0x22, event.KeyJ: 0x26, event.KeyK: 0x28, event.KeyL: 0x25,
	event.KeyM: 0x2E, event.KeyN: 0x2D, event.KeyO: 0x1F, event.KeyP: 0x23,
	event.KeyQ: 0x0C, event.KeyR: 0x0F, event.KeyS: 0x01, event.KeyT: 0x11,
	event.KeyU: 0x20, event.KeyV: 0x09, event.KeyW: 0x0D, event.KeyX: 0x07,
	event.KeyY: 0x10, event.KeyZ: 0x06,

	event.Key0: 0x1D, event.Key1: 0x12, event.Key2: 0x13, event.Key3: 0x14,
	event.Key4: 0x15, event.Key5: 0x17, event.Key6: 0x16, event.Key7: 0x1A,
	event.Key8: 0x1C, event.Key9: 0x19,

	event.KeyF1: 0x7A, event.KeyF2: 0x78, event.KeyF3: 0x63, event.KeyF4: 0x76,
	event.KeyF5: 0x60, event.KeyF6: 0x61, event.KeyF7: 0x62, event.KeyF8: 0x64,
	event.KeyF9: 0x65, event.KeyF10: 0x6D, event.KeyF11: 0x67, event.KeyF12: 0x6F,

	event.KeySpace:     0x31,
	event.KeyReturn:    0x24,
	event.KeyEnter:     0x4C, // keypad enter
	event.KeyTab:       0x30,
	event.KeyEscape:    0x35,
	event.KeyBackspace: 0x33, // kVK_Delete (backspace)
	event.KeyDelete:    0x75, // kVK_ForwardDelete
	event.KeyLeft:      0x7B,
	event.KeyRight:     0x7C,
	event.KeyDown:      0x7D,
	event.KeyUp:        0x7E,
	event.KeyHome:      0x73,
	event.KeyEnd:       0x77,
	event.KeyPageUp:    0x74,
	event.KeyPageDown:  0x79,
}
