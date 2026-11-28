package app

import "github.com/belluikit/bellui/internal/driver/mobile/event/scroll"

// Android motion events that are not fingers on the screen (#1510). They used
// to all become touch moves: hovering a mouse dragged whatever had last been
// touched, a wheel notch was a move that went nowhere, and a touchpad's
// two-finger scroll dragged the widget under the pointer as a finger would.
//
// The values are mirrored here so the conversion builds, and is tested, off
// Android. android.go checks the NDK's against them at compile time, except
// for the Android 14 touchpad values, which NDK r26's headers do not have:
// those come from the Java MotionEvent constants of API 34.
const (
	motionActionMask          = 0xff // AMOTION_EVENT_ACTION_MASK
	motionActionDown          = 0    // AMOTION_EVENT_ACTION_DOWN
	motionActionUp            = 1    // AMOTION_EVENT_ACTION_UP
	motionActionMove          = 2    // AMOTION_EVENT_ACTION_MOVE
	motionActionCancel        = 3    // AMOTION_EVENT_ACTION_CANCEL
	motionActionHoverMove     = 7    // AMOTION_EVENT_ACTION_HOVER_MOVE
	motionActionScroll        = 8    // AMOTION_EVENT_ACTION_SCROLL
	motionActionHoverEnter    = 9    // AMOTION_EVENT_ACTION_HOVER_ENTER
	motionActionHoverExit     = 10   // AMOTION_EVENT_ACTION_HOVER_EXIT
	motionActionButtonPress   = 11   // AMOTION_EVENT_ACTION_BUTTON_PRESS
	motionActionButtonRelease = 12   // AMOTION_EVENT_ACTION_BUTTON_RELEASE

	// MotionEvent.CLASSIFICATION_TWO_FINGER_SWIPE (API 34): a touchpad
	// two-finger scroll, sent as one fake finger moving from the pointer.
	motionClassTwoFingerSwipe = 3
	// MotionEvent.AXIS_GESTURE_SCROLL_X_DISTANCE and _Y_DISTANCE (API 34):
	// how far a two-finger swipe moved since its last event, in pixels. A
	// swipe whose fingers move down reports a negative Y (measured on the
	// pixel143 emulator with a uinput touchpad).
	motionAxisGestureScrollX = 50
	motionAxisGestureScrollY = 51
)

// motionKind is what processEvent makes of an Android motion event.
type motionKind int

const (
	motionTouch    motionKind = iota // fingers on the screen: touch events
	motionIgnore                     // a pointer hovering, or a button changing
	motionWheel                      // a mouse wheel or a pre-14 touchpad scroll
	motionTouchpad                   // a step of a touchpad two-finger swipe
)

// classifyMotion sorts a motion event by its action (masked or not) and its
// classification.
//
// A hover is a mouse or stylus moving with nothing pressed, which a finger
// cannot do, and a button press or release is reported again by the DOWN or
// UP it comes with. Under TalkBack's explore-by-touch, a finger's explore
// hovers arrive the same way; they were already dropped as moves that never
// began.
func classifyMotion(action, classification int32) motionKind {
	switch action & motionActionMask {
	case motionActionHoverMove, motionActionHoverEnter, motionActionHoverExit,
		motionActionButtonPress, motionActionButtonRelease:
		return motionIgnore
	case motionActionScroll:
		return motionWheel
	}
	if classification == motionClassTwoFingerSwipe {
		return motionTouchpad
	}
	return motionTouch
}

// wheelScroll converts an ACTION_SCROLL event's axes into a scroll event.
// Android's AXIS_VSCROLL is positive for a wheel turned away from the user, as
// bellui's DY is, and its AXIS_HSCROLL is positive toward the right, the
// opposite of bellui's DX. A notch is 1 on each axis, and stepV and stepH are
// the pixels one notch scrolls.
func wheelScroll(x, y, vscroll, hscroll, stepV, stepH float32) scroll.Event {
	return scroll.Event{X: x, Y: y, DX: -hscroll * stepH, DY: vscroll * stepV}
}

// touchpadScroll converts one event of a touchpad two-finger swipe. Its
// distances are the fingers' movement in pixels, negative for fingers moving
// down or right, and the content follows the fingers. The DOWN that starts
// the swipe has not moved yet, and its UP or CANCEL ends it.
func touchpadScroll(action int32, x, y, distX, distY float32) scroll.Event {
	e := scroll.Event{X: x, Y: y, Touchpad: true}
	switch action & motionActionMask {
	case motionActionDown:
		e.Phase = scroll.PhaseBegan
	case motionActionUp, motionActionCancel:
		e.Phase = scroll.PhaseEnded
	default:
		e.Phase = scroll.PhaseChanged
		e.DX, e.DY = -distX, -distY
	}
	return e
}
