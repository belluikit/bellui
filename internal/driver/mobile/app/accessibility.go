package app

import "C"

import (
	"sync/atomic"
)

// The Java accessibility overlay (GoNativeActivity) reports TalkBack actions
// through the accessibilityAction native method, which lands here (#488).
// The handler is registered by the driver's Android accessibility backend;
// without the accessibility build tag nothing registers and the up-call is a
// no-op, so the always-compiled Java/JNI side stays safe to ship.

var accessibilityActionHandler atomic.Pointer[func(id, kind int32)]

// SetAccessibilityActionHandler installs the callback invoked for every
// accessibility action the Java side dispatches. The callback runs on the
// Android UI thread; it must hand work to the main goroutine itself.
func SetAccessibilityActionHandler(fn func(id, kind int32)) {
	accessibilityActionHandler.Store(&fn)
}

//export onAccessibilityAction
func onAccessibilityAction(id C.int, kind C.int) {
	if fn := accessibilityActionHandler.Load(); fn != nil {
		(*fn)(int32(id), int32(kind))
	}
}

// The Java overlay reports which element the screen reader has accessibility
// focus on through the accessibilityFocusChanged native method, so Bellui can
// paint its own in-scene focus ring (#759) — the OS ring cannot reach the flat
// GPU surface. Registered by the Android accessibility backend; a no-op without
// the accessibility build tag.
var accessibilityFocusHandler atomic.Pointer[func(id int32, focused bool)]

// SetAccessibilityFocusHandler installs the callback invoked when the screen
// reader moves (or clears) accessibility focus. The callback runs on the
// Android UI thread; it must hand work to the main goroutine itself.
func SetAccessibilityFocusHandler(fn func(id int32, focused bool)) {
	accessibilityFocusHandler.Store(&fn)
}

//export onAccessibilityFocusChanged
func onAccessibilityFocusChanged(id C.int, focused C.int) {
	if fn := accessibilityFocusHandler.Load(); fn != nil {
		(*fn)(int32(id), focused != 0)
	}
}

var touchExplorationEnabled atomic.Bool

//export onSetTouchExploration
func onSetTouchExploration(enabled C.int) {
	touchExplorationEnabled.Store(enabled != 0)
}

// TouchExplorationEnabled reports whether an explore-by-touch accessibility
// service (e.g. TalkBack) is active. Bellui consumes touch through the native
// AInputQueue, which bypasses the framework's touch-exploration filter, so
// while this is true the driver suppresses raw tap activation — activation then
// comes only via the double-tap -> ACTION_CLICK accessibility path. Always
// compiled (no accessibility tag) so the driver can gate taps unconditionally.
func TouchExplorationEnabled() bool {
	return touchExplorationEnabled.Load()
}

// The Java overlay routes the platform virtualized-collection bring-into-view
// request (AccessibilityAction.ACTION_SCROLL_TO_POSITION on a grid proxy)
// through the accessibilityActionScrollTo native method (#817). Registered by
// the Android accessibility backend; a no-op without the accessibility tag.
var accessibilityScrollToHandler atomic.Pointer[func(id, row, col int32)]

// SetAccessibilityScrollToHandler installs the callback invoked when an
// accessibility service asks a virtualized grid to scroll a (row, col) cell
// into view. The callback runs on the Android UI thread; it must hand work to
// the main goroutine itself.
func SetAccessibilityScrollToHandler(fn func(id, row, col int32)) {
	accessibilityScrollToHandler.Store(&fn)
}

//export onAccessibilityScrollTo
func onAccessibilityScrollTo(id, row, col C.int) {
	if fn := accessibilityScrollToHandler.Load(); fn != nil {
		(*fn)(int32(id), int32(row), int32(col))
	}
}

// The Java overlay routes editable-text node actions (#974 option D) — set
// selection, granularity caret moves, set text, cut/paste — through the
// accessibilityTextAction native method after resolving granularity segments
// and converting UTF-16 offsets to rune offsets. Registered by the Android
// accessibility backend; a no-op without the accessibility tag.
var accessibilityTextActionHandler atomic.Pointer[func(id, kind, a, b int32, text string)]

// SetAccessibilityTextActionHandler installs the callback invoked for every
// editable-text action the Java side dispatches. Offsets a/b are rune indices
// into the node's document; text carries the replacement for edit kinds. The
// callback runs on the Android UI thread; it must hand work to the main
// goroutine itself.
func SetAccessibilityTextActionHandler(fn func(id, kind, a, b int32, text string)) {
	accessibilityTextActionHandler.Store(&fn)
}

//export onAccessibilityTextAction
func onAccessibilityTextAction(id, kind, a, b C.int, text *C.char) {
	if fn := accessibilityTextActionHandler.Load(); fn != nil {
		(*fn)(int32(id), int32(kind), int32(a), int32(b), C.GoString(text))
	}
}
