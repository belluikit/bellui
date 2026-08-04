//go:build wasm || js

package glfw

import (
	"syscall/js"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/driver"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubNativeRoot stands in for the web driver's root window. Only the
// driver.NativeWindow delegation is exercised; no other bellui.Window method
// is called.
type stubNativeRoot struct {
	bellui.Window
	ctx driver.BrowserWindowContext
}

func (r *stubNativeRoot) RunNative(f func(any)) {
	f(r.ctx)
}

// An emulated extra window must keep the web driver's identity seam (#257):
// embedding the bellui.Window interface promotes only that interface's
// methods, so without wrapInner's explicit RunNative the extra window would
// fail the driver.NativeWindow assertion and lose the browser picker. This
// locks in both the assertion and the delegation to the embedded root.
func TestWrapInner_RunNativeDelegatesToRoot(t *testing.T) {
	sentinel := js.ValueOf("wrapinner-canvas-sentinel")
	root := &stubNativeRoot{ctx: driver.BrowserWindowContext{Canvas: sentinel}}

	w := wrapInnerWindow(container.NewInnerWindow("t", canvas.NewRectangle(nil)), root, nil)

	native, ok := w.(driver.NativeWindow)
	require.True(t, ok, "an emulated extra window must satisfy driver.NativeWindow")

	var got any
	native.RunNative(func(ctx any) { got = ctx })

	bctx, ok := got.(driver.BrowserWindowContext)
	require.True(t, ok, "the delegated context must be the root's BrowserWindowContext")
	assert.True(t, bctx.Canvas.Equal(sentinel), "the extra window must report the root window's canvas")
}
