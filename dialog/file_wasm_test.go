//go:build wasm || js

package dialog

import (
	"syscall/js"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
)

// browserWindowStub satisfies the driver.NativeWindow identity check the wasm
// overrides gate on (#257), standing in for a real web-driver window so the
// override mechanics below can still be exercised from a test binary. Only the
// type assertion is used; no other bellui.Window method is called.
type browserWindowStub struct {
	bellui.Window
}

func (*browserWindowStub) RunNative(f func(any)) {
	f(driver.BrowserWindowContext{})
}

// The overrides gate on the parent being a real browser-hosted window: under
// the test driver they must decline, so a js/wasm test binary is served by the
// canvas dialog instead of clicking a real <input type="file"> or triggering a
// real download in the harness page (#257).
func TestFileOSOverridesDeclineWithoutBrowserWindow(t *testing.T) {
	test.NewTempApp(t)

	open := NewFileOpen(func(bellui.URIReadCloser, error) {}, nil)
	assert.False(t, fileOpenOSOverride(open), "open must fall back to the canvas dialog under the test driver")

	save := NewFileSave(func(bellui.URIWriteCloser, error) {}, nil)
	assert.False(t, fileSaveOSOverride(save), "save must fall back to the canvas dialog under the test driver")
}

// The wasm save override has no picker-close moment: the writer is handed out
// immediately and the "dialog" is the download offered when it is closed.
// Closing the writer is the closest thing to confirming the dialog, so that is
// when SetOnClosed must report — after the result callback, unlike the
// picker-backed paths, and never at all while the writer stays open (#233).
func TestFileSaveOSOverrideReportsCloseAtWriterClose(t *testing.T) {
	test.NewTempApp(t)

	var writer bellui.URIWriteCloser
	d := NewFileSave(func(w bellui.URIWriteCloser, err error) {
		writer = w
		assert.NoError(t, err)
	}, &browserWindowStub{})

	closed := 0
	d.SetOnClosed(func() { closed++ })

	assert.True(t, fileSaveOSOverride(d))
	assert.NotNil(t, writer, "the override hands the writer out immediately")
	assert.Zero(t, closed, "no close report while the writer is still open")

	_, err := writer.Write([]byte("payload"))
	assert.NoError(t, err)
	assert.NoError(t, writer.Close())
	assert.Equal(t, 1, closed, "closing the writer reports the close once")

	assert.NoError(t, writer.Close())
	assert.Equal(t, 1, closed, "a second Close — a deferred one alongside an explicit one — does not re-report")
}

// A dismissed browser open picker reports through SetOnClosed immediately
// before the (nil, nil) result callback, like every picker-backed path (#233).
// The programmatic click cannot open a real picker (no user activation), so
// the injected <input> stays in the DOM and the browser's cancel event is
// synthesized on it directly.
func TestFileOpenOSOverrideReportsCancel(t *testing.T) {
	test.NewTempApp(t)

	calls := 0
	d := NewFileOpen(func(r bellui.URIReadCloser, err error) {
		calls++
		assert.Nil(t, r, "a dismissed picker delivers (nil, nil)")
		assert.NoError(t, err)
	}, &browserWindowStub{})

	closed := 0
	d.SetOnClosed(func() {
		closed++
		assert.Zero(t, calls, "the close reports before the result callback")
	})

	assert.True(t, fileOpenOSOverride(d))

	input := js.Global().Get("document").Call("querySelector", `input[type="file"]`)
	if !assert.True(t, input.Truthy(), "the override injects a hidden file input") {
		return
	}
	input.Call("dispatchEvent", js.Global().Get("Event").New("cancel"))

	assert.Equal(t, 1, calls, "the result callback reports the dismissal")
	assert.Equal(t, 1, closed, "the close reports exactly once")

	// The dismissal fallback can finish while the picker is still open, so a
	// change event may arrive after finish has run. finish detaches the
	// listeners, so the late event finds nothing — it must neither deliver a
	// result nor re-report the close.
	input.Call("dispatchEvent", js.Global().Get("Event").New("change"))
	assert.Equal(t, 1, calls, "a change event after finish is dropped, not delivered")
	assert.Equal(t, 1, closed, "a change event after finish does not re-report the close")
}
