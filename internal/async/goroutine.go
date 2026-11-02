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
	pc := make([]uintptr, 64)
	pc = pc[:runtime.Callers(skip, pc)]
	frames := runtime.CallersFrames(pc)
	var stack []runtime.Frame
	for {
		frame, more := frames.Next()
		stack = append(stack, frame)
		if !more {
			break
		}
	}
	frame := callerOutsideBellui(stack)
	log.Printf("  From: %s:%d", frame.File, frame.Line)
}

// callerOutsideBellui picks the frame to blame from a stack: the first one
// outside this module, or the last one if the whole stack is inside it. It
// reads the function name and not the file path, because a module-cache path
// says bellui@v1.3.0, a worktree can be called anything, and -trimpath drops it.
func callerOutsideBellui(stack []runtime.Frame) runtime.Frame {
	var last runtime.Frame
	for _, frame := range stack {
		if strings.HasPrefix(frame.Function, "runtime.") {
			break
		}
		last = frame
		if !isBelluiFunc(frame.Function) {
			break
		}
	}
	return last
}

func isBelluiFunc(name string) bool {
	const module = "github.com/belluikit/bellui"
	rest, ok := strings.CutPrefix(name, module)
	return ok && (rest == "" || rest[0] == '/' || rest[0] == '.')
}
