//go:build windows

package webview

import (
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// This file is the COM half of the WebView2 backend: the interfaces we *call*,
// and the handler interfaces we *implement*.
//
// It follows the repo's cgo-free Windows convention — dialog/file_windows_native.go
// proves COM calling through x/sys/windows syscalls. WebView2 needs the other
// direction as well: it is callback-driven, so every result and event arrives
// through a COM object that this package has to supply. A COM object is only a
// pointer to a table of function pointers, and syscall.NewCallback turns a Go
// function into one, so an "implementation" here is a Go struct whose first
// word points at a vtable of four such callbacks.
//
// All eleven handler kinds share ONE vtable. Invoke has the same arity in every
// WebView2 handler interface — (this, a, b) — so the kind stored in the object
// is enough to tell a finished eval from an arriving message, and the process
// spends four syscall.NewCallback slots in total rather than four per kind.
//
// Vtable slot numbers and interface IIDs are ABI facts taken from the WebView2
// SDK headers (WebView2.h) and cross-checked against two independent
// third-party bindings. They are not guessable and a wrong one crashes, so each
// block below lists the methods it skips rather than only the ones it uses.
//
// `go vet -unsafeptr` reports the handful of uintptr-to-pointer conversions
// below. They are inherent rather than accidental: an address that arrived from
// the engine — an interface pointer, an out-parameter a caller supplied, a
// string owned by the browser — is a uintptr with no Go type behind it, and
// reading it is the entire job of a COM binding. Each one is confined to a
// single helper for that reason, and handler objects are resolved through the
// registry rather than by casting, so a stale callback finds nothing instead of
// reading freed memory.
//
// They are also compatible with -race, whose checkptr only rejects a computed
// pointer that lands *inside the Go heap* without a tracked original — which an
// engine-owned address never is. What checkptr does refuse is a Go pointer
// handed to one of these uintptr parameters and converted back, so the
// decisions worth testing are separated from the ABI shims that receive them
// (see comHandlerQueryInterface).

// HRESULTs. A COM method reports one as a signed 32-bit code and a failure is
// simply a negative one, which is why E_POINTER appears twice: once as the
// familiar hex value a callback returns, and once as the signed value a call
// reports.
const (
	sOK           = 0
	hrNoInterface = 0x80004002 // E_NOINTERFACE
	hrPointer     = 0x80004003 // E_POINTER

	ePointer int32 = -2147467261 // E_POINTER
)

// IUnknown slots, shared by every COM interface. 0 is QueryInterface, which
// this package never calls — it only answers it.
const (
	slotAddRef  = 1
	slotRelease = 2
)

// ICoreWebView2Environment slots, after IUnknown.
const (
	envCreateController = 3 // CreateCoreWebView2Controller
	// 4 CreateWebResourceResponse, 5 get_BrowserVersionString,
	// 6/7 add/remove_NewBrowserVersionAvailable — unused.
)

// ICoreWebView2Controller slots, after IUnknown. get/put pairs are adjacent.
const (
	// 3 get_IsVisible, 5 get_Bounds — the reads are never needed, since this
	// package is the only thing that writes them.
	ctrlPutIsVisible = 4
	ctrlPutBounds    = 6
	// 7/8 get/put_ZoomFactor, 9/10 add/remove_ZoomFactorChanged,
	// 11 SetBoundsAndZoomFactor — unused.
	ctrlMoveFocus = 12
	// 13/14 add/remove_MoveFocusRequested — unused.
	ctrlAddGotFocus = 15
	// 16 remove_GotFocus.
	ctrlAddLostFocus = 17
	// 18 remove_LostFocus, 19/20 add/remove_AcceleratorKeyPressed,
	// 21/22 get/put_ParentWindow.
	ctrlNotifyParentWindowPositionChanged = 23
	ctrlClose                             = 24
	ctrlGetCoreWebView2                   = 25
)

// ICoreWebView2 slots, after IUnknown.
const (
	coreGetSettings      = 3
	coreGetSource        = 4
	coreNavigate         = 5
	coreNavigateToString = 6

	coreAddNavigationStarting = 7
	// 8 remove_NavigationStarting, 9/10 add/remove_ContentLoading,
	// 11/12 add/remove_SourceChanged, 13/14 add/remove_HistoryChanged.
	coreAddNavigationCompleted     = 15
	coreAddFrameNavigationStarting = 17
	// 18 remove_FrameNavigationStarting, 19/20 add/remove_FrameNavigationCompleted,
	// 21/22 add/remove_ScriptDialogOpening, 23/24 add/remove_PermissionRequested,
	// 25/26 add/remove_ProcessFailed.
	coreAddScriptToExecuteOnDocumentCreated = 27
	// 28 RemoveScriptToExecuteOnDocumentCreated.
	coreExecuteScript = 29
	// 30 CapturePreview.
	coreReload = 31
	// 32 PostWebMessageAsJson, 33 PostWebMessageAsString.
	coreAddWebMessageReceived = 34
	// 35 remove_WebMessageReceived, 36 CallDevToolsProtocolMethod,
	// 37 get_BrowserProcessId.
	coreGetCanGoBack    = 38
	coreGetCanGoForward = 39
	coreGoBack          = 40
	coreGoForward       = 41
	// 42 GetDevToolsProtocolEventReceiver.
	coreStop = 43
	// 44 add_NewWindowRequested — see below; 45..60 unused.
	coreAddNewWindowRequested = 44
)

// ICoreWebView2Settings slots, after IUnknown.
const (
	// 3/4 get/put_IsScriptEnabled.
	settingsPutIsWebMessageEnabled = 6
	// 7..14 dialogs, status bar, dev tools, context menus.
	settingsPutAreHostObjectsAllowed = 16
)

// Event-argument interface slots, after IUnknown.
const (
	// ICoreWebView2WebMessageReceivedEventArgs.
	msgArgsGetSource                = 3
	msgArgsTryGetWebMessageAsString = 5

	// ICoreWebView2NavigationStartingEventArgs.
	navStartGetURI             = 3
	navStartGetIsUserInitiated = 4
	navStartPutCancel          = 8
	navStartGetNavigationID    = 9

	// ICoreWebView2NavigationCompletedEventArgs.
	navDoneGetIsSuccess    = 3
	navDoneGetErrorStatus  = 4
	navDoneGetNavigationID = 5

	// ICoreWebView2NewWindowRequestedEventArgs.
	newWinGetURI             = 3
	newWinPutHandled         = 6
	newWinGetIsUserInitiated = 8
)

// COREWEBVIEW2_MOVE_FOCUS_REASON.
const moveFocusProgrammatic = 0

// COREWEBVIEW2_WEB_ERROR_STATUS value this backend has to recognise: a
// navigation the engine reports as cancelled is not a network failure, and
// telling them apart is what keeps a stopped load from being reported twice.
const webErrorOperationCanceled = 14

// Interface IIDs. Only the handler interfaces need one — a handler answers
// QueryInterface for IUnknown and for itself, and refuses everything else, so
// the engine never receives this object under an interface it does not
// implement.
var (
	iidIUnknown = windows.GUID{Data1: 0x00000000, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}

	iidEnvironmentCompleted = windows.GUID{Data1: 0x4E8A3389, Data2: 0xC9D8, Data3: 0x4BD2, Data4: [8]byte{0xB6, 0xB5, 0x12, 0x4F, 0xEE, 0x6C, 0xC1, 0x4D}}
	iidControllerCompleted  = windows.GUID{Data1: 0x6C4819F3, Data2: 0xC9B7, Data3: 0x4260, Data4: [8]byte{0x81, 0x27, 0xC9, 0xF5, 0xBD, 0xE7, 0xF6, 0x8C}}
	iidWebMessageReceived   = windows.GUID{Data1: 0x57213F19, Data2: 0x00E6, Data3: 0x49FA, Data4: [8]byte{0x8E, 0x07, 0x89, 0x8E, 0xA0, 0x1E, 0xCB, 0xD2}}
	iidNavigationStarting   = windows.GUID{Data1: 0x9ADBE429, Data2: 0xF36D, Data3: 0x432B, Data4: [8]byte{0x9D, 0xDC, 0xF8, 0x88, 0x1F, 0xBD, 0x76, 0xE3}}
	iidNavigationCompleted  = windows.GUID{Data1: 0xD33A35BF, Data2: 0x1C49, Data3: 0x4F98, Data4: [8]byte{0x93, 0xAB, 0x00, 0x6E, 0x05, 0x33, 0xFE, 0x1C}}
	iidNewWindowRequested   = windows.GUID{Data1: 0xD4C185FE, Data2: 0xC81C, Data3: 0x4989, Data4: [8]byte{0x97, 0xAF, 0x2D, 0x3F, 0xA7, 0xAB, 0x56, 0x51}}
	iidFocusChanged         = windows.GUID{Data1: 0x05EA24BD, Data2: 0x6452, Data3: 0x4926, Data4: [8]byte{0x90, 0x14, 0x4B, 0x82, 0xB4, 0x98, 0x13, 0x5D}}
	iidExecuteScriptDone    = windows.GUID{Data1: 0x49511172, Data2: 0xCC67, Data3: 0x4BCA, Data4: [8]byte{0x99, 0x23, 0x13, 0x71, 0x12, 0xF4, 0xC4, 0xCC}}
	iidAddScriptDone        = windows.GUID{Data1: 0xB99369F3, Data2: 0x9B11, Data3: 0x47B5, Data4: [8]byte{0xBC, 0x6F, 0x8E, 0x78, 0x95, 0xFC, 0xEA, 0x17}}
)

// ---- calling COM ----

// comObject is a COM interface pointer: the address of an object whose first
// word points at a vtable of function pointers, indexed by slot. ICoreWebView2
// is the widest interface used here, at 61 slots.
//
// It is a uintptr rather than a Go pointer type on purpose. The object belongs
// to the engine, not to Go, so nothing about it should look to the reader — or
// to the garbage collector — like memory this package owns; and keeping it
// numeric confines reading through a foreign address to the one place that
// genuinely has to, rather than spreading the conversion over every accessor.
type comObject uintptr

// call invokes a method by vtable slot, with the interface pointer as the
// implicit `this` first argument, and reports the HRESULT.
//
// Like dialog/file_windows_native.go's comCall, stashing the arguments in a
// slice forfeits the syscall package's pointer-liveness guarantee (which only
// holds when the unsafe.Pointer conversion is written inside the call
// expression). Go's GC does not move heap objects, so an address stays valid;
// callers that pass a Go pointer must keep the object itself alive across the
// call, which every caller here does by holding it in a local variable.
func (o comObject) call(slot int, args ...uintptr) int32 {
	if o == 0 {
		return ePointer
	}
	// The one deliberate look through a raw address in this package: the
	// interface pointer came from the engine, and its vtable is the only way
	// to reach the methods. Two hops, not one — the interface pointer
	// addresses the object, whose FIRST WORD holds the vtable's address; the
	// slots live behind that word. Collapsing them reads the object's own
	// fields as function addresses and jumps into the engine's data (#82
	// found this on the first live run: dialog's comCall gets the second hop
	// for free from its typed struct, and flattening comObject to a uintptr
	// here silently dropped it).
	vtbl := (*[64]uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(o))))

	all := make([]uintptr, 0, len(args)+1)
	all = append(all, uintptr(o))
	all = append(all, args...)
	hr, _, _ := syscall.SyscallN(vtbl[slot], all...)
	return int32(hr)
}

func (o comObject) release() {
	o.call(slotRelease)
}

func (o comObject) addRef() {
	o.call(slotAddRef)
}

// getBool reads a BOOL out-parameter property.
func (o comObject) getBool(slot int) bool {
	var v int32
	hr := o.call(slot, uintptr(unsafe.Pointer(&v)))
	return hr >= 0 && v != 0
}

// getUint64 reads a UINT64 out-parameter property.
func (o comObject) getUint64(slot int) uint64 {
	var v uint64
	o.call(slot, uintptr(unsafe.Pointer(&v)))
	return v
}

// getInt32 reads an enum out-parameter property.
func (o comObject) getInt32(slot int) int32 {
	var v int32
	o.call(slot, uintptr(unsafe.Pointer(&v)))
	return v
}

// getString reads an LPWSTR out-parameter property and frees it. COM
// out-strings are allocated by the callee with CoTaskMemAlloc and owned by us.
func (o comObject) getString(slot int) string {
	var p *uint16
	if hr := o.call(slot, uintptr(unsafe.Pointer(&p))); hr < 0 || p == nil {
		return ""
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(p))
	return windows.UTF16PtrToString(p)
}

// getObject reads an interface out-parameter property. The result carries a
// reference the caller owns and must release.
func (o comObject) getObject(slot int) comObject {
	var obj comObject
	if hr := o.call(slot, uintptr(unsafe.Pointer(&obj))); hr < 0 {
		return 0
	}
	return obj
}

// utf16AtToString reads a NUL-terminated UTF-16 string the engine passed by
// address, as an event argument rather than an out-parameter. The string is the
// engine's for the duration of the call, so it is copied and never freed.
func utf16AtToString(addr uintptr) string {
	if addr == 0 {
		return ""
	}
	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(addr)))
}

// putBool writes a BOOL property.
func (o comObject) putBool(slot int, v bool) int32 {
	var b uintptr
	if v {
		b = 1
	}
	return o.call(slot, b)
}

// ---- implementing COM ----

// handlerKind names which of the WebView2 callback interfaces a handler object
// is standing in for, so that one Invoke callback can serve them all.
type handlerKind int

const (
	handlerEnvCompleted handlerKind = iota
	handlerControllerCompleted
	handlerWebMessage
	handlerNavStarting
	handlerFrameNavStarting
	handlerNavCompleted
	handlerNewWindow
	handlerGotFocus
	handlerLostFocus
	handlerExecuteScriptDone
	handlerAddScriptDone
)

// comHandlerVtbl is the four-slot vtable every handler object shares. It holds
// C function addresses, not Go pointers, so it is safe to hand to the engine.
type comHandlerVtbl struct {
	queryInterface uintptr
	addRef         uintptr
	release        uintptr
	invoke         uintptr
}

// comHandler is one WebView2 callback object. Its first word is the vtable
// pointer, which is what makes the Go struct a valid COM interface pointer.
//
// The engine holds this pointer for as long as it holds a reference, so the
// object must not be collected: every live handler is kept in the package-level
// registry below, and leaves it only when its COM reference count reaches zero.
type comHandler struct {
	vtbl *comHandlerVtbl
	iid  *windows.GUID
	kind handlerKind
	view uint64 // registry token of the owning WebView
	data uint64 // eval call id, for handlerExecuteScriptDone
	refs int32
}

// handlerVtbl is filled in init rather than at declaration: a handler's Invoke
// reaches code that allocates further handlers, so the vtable would otherwise
// depend on itself.
var handlerVtbl comHandlerVtbl

func init() {
	handlerVtbl = comHandlerVtbl{
		queryInterface: syscall.NewCallback(comHandlerQueryInterface),
		addRef:         syscall.NewCallback(comHandlerAddRef),
		release:        syscall.NewCallback(comHandlerRelease),
		invoke:         syscall.NewCallback(comHandlerInvoke),
	}
}

// liveHandlers keeps every handler reachable from Go for as long as the engine
// holds a reference to it, and is also how an incoming callback finds the Go
// object behind the `this` pointer it was called with. Keying it by address
// serves both: the map is the only thing standing between a Go-allocated COM
// object and the garbage collector, and a stale pointer resolves to nothing
// rather than into memory that has been reused.
var (
	liveHandlersMu sync.Mutex
	liveHandlers   = map[uintptr]*comHandler{}
)

// newHandler allocates a callback object with one reference, the caller's.
// Pass it to the engine — which takes its own reference — and then release it.
func newHandler(kind handlerKind, iid *windows.GUID, view, data uint64) *comHandler {
	h := &comHandler{vtbl: &handlerVtbl, iid: iid, kind: kind, view: view, data: data, refs: 1}

	liveHandlersMu.Lock()
	liveHandlers[h.ptr()] = h
	liveHandlersMu.Unlock()

	return h
}

func (h *comHandler) ptr() uintptr {
	return uintptr(unsafe.Pointer(h))
}

// release drops the caller's own reference, mirroring what a COM Release does.
func (h *comHandler) release() {
	comHandlerRelease(h.ptr())
}

func comHandlerAt(this uintptr) *comHandler {
	liveHandlersMu.Lock()
	defer liveHandlersMu.Unlock()

	return liveHandlers[this]
}

// comHandlerQueryInterface is the ABI shim: it gives the addresses the engine
// passed their real types and hands the decision to queryInterface.
//
// The split is what makes the decision testable. Handing a Go pointer to a
// uintptr parameter and converting it back inside is a round trip the race
// detector's checkptr refuses — correctly, since it cannot see that the two
// ends are the same allocation — so a test that drove this signature directly
// would abort under -race even though the production caller (whose addresses
// are the engine's, outside the Go heap) is fine.
func comHandlerQueryInterface(this, riid, ppv uintptr) uintptr {
	if ppv == 0 {
		return hrPointer
	}
	var id *windows.GUID
	if riid != 0 {
		id = (*windows.GUID)(unsafe.Pointer(riid))
	}
	return queryInterface(comHandlerAt(this), this, id, (*uintptr)(unsafe.Pointer(ppv)))
}

// queryInterface answers for IUnknown and for the one interface this handler
// was created as, and refuses everything else — so the engine never receives
// the object under an interface it does not implement.
func queryInterface(h *comHandler, this uintptr, riid *windows.GUID, ppv *uintptr) uintptr {
	if ppv == nil {
		return hrPointer
	}
	if h == nil {
		*ppv = 0
		return hrPointer
	}
	if riid != nil && (*riid == iidIUnknown || *riid == *h.iid) {
		atomic.AddInt32(&h.refs, 1)
		*ppv = this
		return sOK
	}
	*ppv = 0
	return hrNoInterface
}

func comHandlerAddRef(this uintptr) uintptr {
	h := comHandlerAt(this)
	if h == nil {
		return 0
	}
	return uintptr(atomic.AddInt32(&h.refs, 1))
}

func comHandlerRelease(this uintptr) uintptr {
	h := comHandlerAt(this)
	if h == nil {
		return 0
	}
	refs := atomic.AddInt32(&h.refs, -1)
	if refs > 0 {
		return uintptr(refs)
	}

	// Last reference gone: let the object be collected. Anything the engine
	// still held would have kept a reference of its own.
	liveHandlersMu.Lock()
	delete(liveHandlers, this)
	liveHandlersMu.Unlock()

	return 0
}

// comHandlerInvoke is the single entry point for every WebView2 callback. It
// runs on the thread that created the environment — bellui's main goroutine,
// re-entered while the driver pumps the message loop — which is where this
// package contracts every callback to run, so the delivery functions are called
// directly rather than posted.
func comHandlerInvoke(this, a, b uintptr) uintptr {
	h := comHandlerAt(this)
	if h == nil {
		return sOK
	}
	w := lookupView(h.view)
	if w == nil {
		return sOK // the view was destroyed; nothing to deliver to
	}

	switch h.kind {
	case handlerEnvCompleted:
		w.onEnvironmentCreated(int32(a), comObject(b))
	case handlerControllerCompleted:
		w.onControllerCreated(int32(a), comObject(b))
	case handlerWebMessage:
		w.onWebMessage(comObject(b))
	case handlerNavStarting:
		w.onNavigationStarting(comObject(b), true)
	case handlerFrameNavStarting:
		w.onNavigationStarting(comObject(b), false)
	case handlerNavCompleted:
		w.onNavigationCompleted(comObject(b))
	case handlerNewWindow:
		w.onNewWindowRequested(comObject(b))
	case handlerGotFocus:
		w.onFocusChanged(true)
	case handlerLostFocus:
		w.onFocusChanged(false)
	case handlerExecuteScriptDone:
		w.onExecuteScriptDone(h.data, int32(a), b)
	case handlerAddScriptDone:
		// Nothing to report: the bridge script either registered or it did
		// not, and a page that never got it simply has no window.bellui.
	}
	return sOK
}
