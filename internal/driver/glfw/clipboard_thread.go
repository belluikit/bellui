// Only platforms with a clipboard-flavor bridge need the helper.
//go:build (darwin || windows || linux) && !wasm

package glfw

import "github.com/belluikit/bellui/internal/async"

// runOnClipboardThread runs f on the main thread and waits for it, running
// inline when already there. The native clipboard bridges need this: the
// Win32 clipboard belongs to the thread owning the message pump and AppKit
// prefers the main thread. runOnMainWithWait(wait=true) cannot be used
// directly because calling it from the running main goroutine would queue f
// behind the very loop iteration that is blocked waiting on it.
func runOnClipboardThread(f func()) {
	if async.IsMainGoroutine() {
		f()
		return
	}
	runOnMainWithWait(f, true)
}
