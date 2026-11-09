package bellui

import (
	"time"

	"github.com/belluikit/bellui/internal/activity"
)

// LastUserActivity returns when the user last gave the app input, in any of
// its windows, or the zero time if they have not since it started. It is for
// idle timers: return to a home screen, dim, lock or pause something costly
// after a while untouched. A timer reads it when it fires and compares it
// with time.Now.
//
// What counts is deliberate input, as the driver receives it and before any
// widget does, so a scroll inside a list counts though the list reports
// nothing of it:
//   - a mouse button pressed or released, and the mouse moved while a button
//     is held (a drag);
//   - a touch going down, moving or lifting, including a screen reader's
//     explore-by-touch on Android;
//   - a scroll from a wheel or a trackpad;
//   - a key pressed, repeated or released, typed text, and an input method's
//     composition or edit, which is how a soft keyboard types;
//   - files dropped into a window.
//
// Moving the mouse with no button held does not count, since a mouse at rest
// can jitter. Nor does anything the app does itself: an animation, a timer,
// a touch scroll coasting after the finger has lifted, or a value set from
// code. A trackpad's own momentum scrolling does count where the system
// delivers it as scroll events, as macOS does. Actions a screen reader performs through the accessibility API, such
// as VoiceOver activating a button, are not counted yet (#1109).
//
// The time carries a monotonic reading, so time.Since is not fooled by the
// wall clock being changed. It may be called from any goroutine.
//
// Since: 1.3
func LastUserActivity() time.Time {
	return activity.Last()
}
