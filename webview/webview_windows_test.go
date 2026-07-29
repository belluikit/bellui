//go:build windows

package webview

import (
	"path/filepath"
	"runtime"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"golang.org/x/sys/windows"

	"github.com/belluikit/bellui/geo"
)

// The COM object this package hands the engine is a Go struct, and what makes
// it a valid interface pointer is that its first word is the vtable pointer.
// Nothing in the language enforces that, so a field added above it would be a
// silent memory-corrupting change rather than a compile error.
func TestComHandler_VtableIsTheFirstWord(t *testing.T) {
	h := newHandler(handlerWebMessage, &iidWebMessageReceived, 1, 0)
	defer h.release()

	firstWord := *(*uintptr)(unsafe.Pointer(h))
	assert.Equal(t, uintptr(unsafe.Pointer(&handlerVtbl)), firstWord)
	assert.Equal(t, uintptr(0), unsafe.Offsetof(h.vtbl))
}

// A handler lives exactly as long as its COM reference count says, since that
// count is the only thing keeping the object out of the garbage collector while
// the engine still holds the address.
func TestComHandler_ReferenceCountGovernsLifetime(t *testing.T) {
	h := newHandler(handlerNavStarting, &iidNavigationStarting, 1, 0)
	this := h.ptr()

	require.Same(t, h, comHandlerAt(this))

	assert.Equal(t, uintptr(2), comHandlerAddRef(this))
	assert.Equal(t, uintptr(1), comHandlerRelease(this))
	assert.Same(t, h, comHandlerAt(this), "still referenced")

	assert.Equal(t, uintptr(0), comHandlerRelease(this))
	assert.Nil(t, comHandlerAt(this), "last reference gone")
}

// QueryInterface must answer for IUnknown and for the one interface the handler
// was created as, and refuse everything else: handing the engine this object
// under an interface it does not implement would have it call through a vtable
// that is not there.
func TestComHandler_QueryInterface(t *testing.T) {
	h := newHandler(handlerNavCompleted, &iidNavigationCompleted, 1, 0)
	defer h.release()

	query := func(iid windows.GUID) (uintptr, uintptr) {
		var out uintptr
		hr := comHandlerQueryInterface(h.ptr(), uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&out)))
		return hr, out
	}

	hr, out := query(iidIUnknown)
	assert.Equal(t, uintptr(sOK), hr)
	assert.Equal(t, h.ptr(), out)
	comHandlerRelease(h.ptr()) // the reference QueryInterface just handed out

	hr, out = query(iidNavigationCompleted)
	assert.Equal(t, uintptr(sOK), hr)
	assert.Equal(t, h.ptr(), out)
	comHandlerRelease(h.ptr())

	hr, out = query(iidWebMessageReceived)
	assert.Equal(t, uintptr(hrNoInterface), hr)
	assert.Equal(t, uintptr(0), out, "a refused query must not leave a pointer behind")
}

// Callbacks can arrive after the object they name is gone — the engine holds
// raw addresses — so every entry point has to survive one that no longer
// resolves rather than reading through it.
func TestComHandler_StaleThisIsHarmless(t *testing.T) {
	const stale = uintptr(0xDEAD0000)

	assert.Nil(t, comHandlerAt(stale))
	assert.Equal(t, uintptr(0), comHandlerAddRef(stale))
	assert.Equal(t, uintptr(0), comHandlerRelease(stale))
	assert.Equal(t, uintptr(sOK), comHandlerInvoke(stale, 0, 0))
}

// An Invoke for a view that has been destroyed must be dropped, not delivered:
// the view's platform state is gone and its callbacks have already been failed.
func TestComHandler_InvokeAfterViewDestroyed(t *testing.T) {
	w := New("")
	h := newHandler(handlerGotFocus, &iidFocusChanged, w.token, 0)
	defer h.release()

	w.destroyRaw()
	assert.Equal(t, uintptr(sOK), comHandlerInvoke(h.ptr(), 0, 0))
	assert.False(t, w.HasFocus(), "a dropped callback changed state anyway")
}

// The runtime is only usable if its API is at least the one this backend calls,
// and the version is read out of the install path, so a shape it does not
// recognise has to be a refusal rather than a guess.
func TestWebView2VersionOK(t *testing.T) {
	for _, tt := range []struct {
		version string
		want    bool
	}{
		{"120.0.2210.91", true},
		{"1.0.1150.38", true},  // the SDK release this backend is written against
		{"1.0.1149.99", false}, // one below it
		{"94.0.992.31", false},
		{"1.0", false},
		{"", false},
		{"1.0.abc.4", false},
	} {
		assert.Equal(t, tt.want, webview2VersionOK(tt.version), "version %q", tt.version)
	}
}

// Only the client built for this process's architecture can be loaded into it,
// so the path has to name the right one.
func TestWebView2ClientDLL(t *testing.T) {
	dll := webview2ClientDLL(`C:\Program Files (x86)\Microsoft\EdgeWebView\Application\120.0.2210.91`)

	assert.Equal(t, "EmbeddedBrowserWebView.dll", filepath.Base(dll))
	assert.Contains(t, dll, `EBWebView\`)

	switch runtime.GOARCH {
	case "amd64":
		assert.Contains(t, dll, `\x64\`)
	case "arm64":
		assert.Contains(t, dll, `\arm64\`)
	case "386":
		assert.Contains(t, dll, `\x86\`)
	}
}

// Bellui works in device-independent points and Win32 in physical pixels, so
// every coordinate crossing this backend is scaled — and rounded, since a
// truncated edge leaves a one-pixel seam at the hole.
func TestPixelConversion(t *testing.T) {
	assert.Equal(t, int32(200), px(100, 2))
	assert.Equal(t, int32(150), px(100, 1.5))
	assert.Equal(t, int32(151), px(100.4, 1.5)) // 150.6 rounds up, not down
	assert.Equal(t, int32(0), px(0, 2))
	assert.Equal(t, int32(-10), px(-5, 2))
}

// A view with no engine must still be safe to drive. The host places and shows
// its view on the same pass it attaches it, long before the controller exists,
// and a failed attach leaves it in this state for good.
func TestWebView_UnattachedIsSafeToDrive(t *testing.T) {
	w := New("")
	defer w.Destroy()

	assert.NotPanics(t, func() {
		w.SetVisible(true)
		w.SetGeometry(geo.NewRectangle(10, 20, 300, 200), geo.NewRectangle(10, 20, 100, 50), 2)
	})
	assert.False(t, w.TakeFocus(), "there is no engine to give the focus to")
	assert.True(t, w.ResignFocus(), "a view that holds no focus has nothing to give back")
	assert.False(t, w.HasFocus())
}
