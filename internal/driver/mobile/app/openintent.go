package app

import "C"

import "sync/atomic"

// The Java activity (GoNativeActivity) reports an "open with" launch or new
// intent — a VIEW/EDIT data Uri from a file association or custom URL scheme /
// deep link — through the openIntent native method, which lands here (fyne#622).
//
// The handler is registered by the bellui app package's Android launch-open path.
// A launch intent is read in the Java onCreate, which may run before the app
// calls Run and registers the handler, so an intent that arrives handler-less is
// buffered and flushed once SetOpenIntentHandler is called. The always-compiled
// Java/JNI side thus stays safe to ship even when nothing registers.

var (
	openIntentHandler atomic.Pointer[func(uri string)]
	pendingOpenIntent atomic.Pointer[string]
)

// SetOpenIntentHandler installs the callback invoked for every "open with"
// intent the Java side delivers, and flushes any intent buffered before
// registration. The callback runs on the JNI up-call thread; it must hand work
// to the main goroutine itself.
func SetOpenIntentHandler(fn func(uri string)) {
	openIntentHandler.Store(&fn)
	if p := pendingOpenIntent.Swap(nil); p != nil {
		fn(*p)
	}
}

//export onOpenIntent
func onOpenIntent(uri *C.char) {
	s := C.GoString(uri)
	if fn := openIntentHandler.Load(); fn != nil {
		(*fn)(s)
		return
	}
	pendingOpenIntent.Store(&s)
}
