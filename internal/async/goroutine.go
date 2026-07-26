package async

import (
	"log"
	"runtime"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/bellui/internal/thread"
)

// SetMainGoroutine delegates to internal/thread.
func SetMainGoroutine() {
	thread.SetMainGoroutine()
}

// IsMainGoroutine returns true if called from the main goroutine.
func IsMainGoroutine() bool {
	return thread.IsMainGoroutine()
}

// EnsureNotMain is part of our thread transition and makes sure that the passed function runs off main.
// If the context is running on a goroutine or the transition has been disabled this will blindly run.
// Otherwise, an error will be logged and the function will be called on a new goroutine.
//
// This will be removed later and should never be public
func EnsureNotMain(fn func()) {
	if build.MigratedToBelluiDo() || !IsMainGoroutine() {
		fn()
		return
	}

	log.Println("*** Error in GUI call thread, bellui.Do[AndWait] called from main goroutine ***")

	logStackTop(2)
	go fn()
}

// EnsureMain is part of our thread transition and makes sure that the passed function runs on main.
// If the context is main or the transition has been disabled this will blindly run.
// Otherwise, an error will be logged and the function will be called on the main goroutine.
//
// This will be removed later and should never be public
func EnsureMain(fn func()) {
	if build.MigratedToBelluiDo() || IsMainGoroutine() {
		fn()
		return
	}

	log.Println("*** Error in GUI call thread, this should have been called in bellui.Do[AndWait] ***")

	logStackTop(1)
	bellui.DoAndWait(fn)
}

func logStackTop(skip int) {
	pc := make([]uintptr, 16)
	_ = runtime.Callers(skip, pc)
	frames := runtime.CallersFrames(pc)
	frame, more := frames.Next()

	var nextFrame runtime.Frame
	for more {
		nextFrame, more = frames.Next()
		if nextFrame.File == "" || strings.Contains(nextFrame.File, "runtime") {
			break
		}

		frame = nextFrame
		if !strings.Contains(nextFrame.File, "/bellui/") {
			break
		}
	}
	log.Printf("  From: %s:%d", frame.File, frame.Line)
}
