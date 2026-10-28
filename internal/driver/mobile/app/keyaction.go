package app

import "github.com/belluikit/bellui/internal/driver/mobile/event/key"

// The NDK's AKEY_EVENT_ACTION_* values, which AKeyEvent_getAction returns.
// Mirrored here so the conversion builds, and is tested, off Android.
const (
	keyActionDown     = 0 // AKEY_EVENT_ACTION_DOWN
	keyActionUp       = 1 // AKEY_EVENT_ACTION_UP
	keyActionMultiple = 2 // AKEY_EVENT_ACTION_MULTIPLE
)

// nativeKeyDirection converts a native key event's action and repeat count
// into the direction bellui delivers, or false to drop the event.
//
// The action used to be compared with the AKEY_STATE_* constants, whose UP is
// 0 and DOWN is 1, the reverse of the actions: every key acted when it was
// released, and a soft Backspace's UP read as a second press (#793).
//
// A held key repeats as further DOWNs with a repeat count above zero. Each is
// a press, so held keys type again, as on the desktop driver. Escape is the
// exception, as on desktop: its repeats are dropped, so holding it closes one
// pop-up rather than every pop-up in turn.
func nativeKeyDirection(action, repeatCount int32, code key.Code) (key.Direction, bool) {
	switch action {
	case keyActionDown:
		if repeatCount > 0 && code == key.CodeEscape {
			return key.DirNone, false
		}
		return key.DirPress, true
	case keyActionUp:
		return key.DirRelease, true
	default:
		// ACTION_MULTIPLE carries a run of characters, not a press or a
		// release. The driver acts on neither, as before.
		return key.DirNone, true
	}
}
