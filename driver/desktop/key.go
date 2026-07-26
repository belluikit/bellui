package desktop

import "github.com/belluikit/bellui/event"

const (
	// KeyNone represents no key
	KeyNone event.KeyName = ""
	// KeyShiftLeft represents the left shift key
	KeyShiftLeft event.KeyName = "LeftShift"
	// KeyShiftRight represents the right shift key
	KeyShiftRight event.KeyName = "RightShift"
	// KeyControlLeft represents the left control key
	KeyControlLeft event.KeyName = "LeftControl"
	// KeyControlRight represents the right control key
	KeyControlRight event.KeyName = "RightControl"
	// KeyAltLeft represents the left alt key
	KeyAltLeft event.KeyName = "LeftAlt"
	// KeyAltRight represents the right alt key
	KeyAltRight event.KeyName = "RightAlt"
	// KeySuperLeft represents the left "Windows" key (or "Command" key on macOS)
	KeySuperLeft event.KeyName = "LeftSuper"
	// KeySuperRight represents the right "Windows" key (or "Command" key on macOS)
	KeySuperRight event.KeyName = "RightSuper"
	// KeyMenu represents the left or right menu / application key
	KeyMenu event.KeyName = "Menu"
	// KeyPrintScreen represents the key used to cause a screen capture
	KeyPrintScreen event.KeyName = "PrintScreen"

	// KeyCapsLock represents the caps lock key, tapping once is the down event then again is the up
	KeyCapsLock event.KeyName = "CapsLock"
)

// Keyable describes any focusable canvas object that can accept desktop key events.
// This is the traditional key down and up event that is not applicable to all devices.
type Keyable interface {
	event.Focusable

	KeyDown(*event.KeyEvent)
	KeyUp(*event.KeyEvent)
}
