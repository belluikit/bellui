//go:build darwin

package glfw

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework AppKit

#import <stdbool.h>

void setFullScreen(bool full, void *window);
void setFullScreenSecondary(bool full, void *window);
void setWindowParent(void *child, void *parent);
void setWindowModalBlocked(void *parent, bool blocked);
bool isWindowModalBlocked(void *parent);
*/
import "C"

import (
	"runtime"
	"unsafe"

	"github.com/belluikit/bellui/driver"
)

// assert we are implementing driver.NativeWindow
var _ driver.NativeWindow = (*window)(nil)

func (w *window) RunNative(f func(any)) {
	context := driver.MacWindowContext{}
	if v := w.view(); v != nil {
		context.NSWindow = uintptr(v.GetCocoaWindow())
	}

	f(context)
}

// doSetParentNative establishes (or clears, when parent is nil) the native
// NSWindow owner relationship for this window. Must run on the main goroutine.
func (w *window) doSetParentNative(parent *window) {
	child := w.view().GetCocoaWindow()
	var parentPtr unsafe.Pointer
	if parent != nil {
		parentPtr = parent.view().GetCocoaWindow()
	}
	C.setWindowParent(child, parentPtr)
}

// doSetModalNative blocks (or restores) keyboard/main focus on the parent
// window while this dialog is modal (pre-rebrand tracker #676). macOS has no window-modal primitive
// for a free-floating window — a sheet is not free-floating and an NSApp modal
// session is app-modal (would block every window) — so instead of native
// modality the parent NSWindow is prevented from becoming key/main
// (setWindowModalBlocked), which stops the keyboard reaching it and greys its
// title bar. The addChildWindow ordering (SetParent) and the pure-Go scrim
// cover ordering and mouse input. Must run on the main goroutine. A nil parent
// is a no-op — the block is expressed against the owner.
func (w *window) doSetModalNative(parent *window, modal bool) {
	if parent == nil || !w.blockParentModal(parent, modal) {
		return // nil parent, or another modal child already blocks it (pre-rebrand tracker #682)
	}
	C.setWindowModalBlocked(parent.view().GetCocoaWindow(), C.bool(modal))
}

// modalBlocked reports whether this window is currently key/main-blocked as a
// modal dialog's parent (test introspection for pre-rebrand tracker #676).
func (w *window) modalBlocked() bool {
	return bool(C.isWindowModalBlocked(w.view().GetCocoaWindow()))
}

func (w *window) doSetFullScreen(full bool) {
	if runtime.GOOS == "darwin" {
		win := w.view().GetCocoaWindow()
		C.setFullScreen(C.bool(full), win)
		return
	}
}

func (w *window) doSetFullScreen2(full bool) {
	if runtime.GOOS == "darwin" {
		win := w.view().GetCocoaWindow()
		C.setFullScreenSecondary(C.bool(full), win)
		return
	}
}
