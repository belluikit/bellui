//go:build windows

package glfw

import (
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// This file implements the OLE COM objects the Windows drag-out backend hands to
// the OS (pre-rebrand tracker #837, pre-rebrand tracker #515 Part B — see docs/design/515-drag-out.md). Unlike the native
// file dialog (pre-rebrand tracker #514, dialog/file_windows_native.go), where bellui CALLS COM
// interfaces the shell implements, here we IMPLEMENT the interfaces DoDragDrop
// consumes: an IDataObject (with its IEnumFORMATETC) carrying the drag flavors and
// a minimal IDropSource driving the drag loop. Everything is cgo-free — the
// vtables are Go structs of function pointers produced by syscall.NewCallback and
// each interface "this"/out pointer is declared unsafe.Pointer so the callbacks
// reinterpret them without a uintptr round-trip (which keeps `go vet`'s unsafeptr
// analyzer happy). A COM object's first field is its vtable pointer, matching the
// C COM memory layout, so the "this" pointer casts straight back to the Go struct.
//
// The COM objects, their payloads and any enumerator clones are pinned for the
// duration of DoDragDrop (see dragSession in dragsource_windows.go) so the GC can
// neither move nor collect memory the OS holds a raw pointer to.
//
// Runtime verification needs a real Windows session (drop into Explorer / Notepad
// / a browser); per fork convention COM/native code is CI- and on-box-verified.

// HRESULT and OLE status codes returned from the COM methods.
const (
	sOK    uintptr = 0x00000000
	sFalse uintptr = 0x00000001

	eNotImpl      uintptr = 0x80004001
	eNoInterface  uintptr = 0x80004002
	eOutOfMemory  uintptr = 0x8007000E
	eInvalidArg   uintptr = 0x80070057
	dvETymed      uintptr = 0x80040069
	dvEFormatEtc  uintptr = 0x80040064
	dataSSameFmt  uintptr = 0x00040130 // DATA_S_SAMEFORMATETC
	oleEAdviseNot uintptr = 0x80040003 // OLE_E_ADVISENOTSUPPORTED

	// IDropSource / DoDragDrop return codes.
	dragDropSDrop              uintptr = 0x00040100
	dragDropSCancel            uintptr = 0x00040101
	dragDropSUseDefaultCursors uintptr = 0x00040102
)

// Clipboard formats, TYMED and aspect constants (wingdi.h / objidl.h).
const (
	cfDIB         uint16 = 8
	cfUnicodeText uint16 = 13
	cfHDrop       uint16 = 15

	tymedHGlobal    uint32 = 1
	dvAspectContent uint32 = 1

	dataDirGet uint32 = 1 // DATADIR_GET

	mkLButton uintptr = 0x0001 // MK_LBUTTON in grfKeyState

	// GlobalAlloc flags: moveable + zero-initialised (GHND).
	gmemMoveable uintptr = 0x0002
	gmemZeroInit uintptr = 0x0040
)

// DROPEFFECT bits negotiated with DoDragDrop.
const (
	dropEffectNone uintptr = 0
	dropEffectCopy uintptr = 1
	dropEffectMove uintptr = 2
)

// Interface IIDs. IUnknown plus the three we implement.
var (
	iidIUnknown       = windows.GUID{Data1: 0x00000000, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIDataObject    = windows.GUID{Data1: 0x0000010E, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIDropSource    = windows.GUID{Data1: 0x00000121, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIEnumFORMATETC = windows.GUID{Data1: 0x00000103, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
)

var (
	modole32Drag  = windows.NewLazySystemDLL("ole32.dll")
	modkernel32   = windows.NewLazySystemDLL("kernel32.dll")
	moduser32Drag = windows.NewLazySystemDLL("user32.dll")

	procOleInitialize        = modole32Drag.NewProc("OleInitialize")
	procDoDragDrop           = modole32Drag.NewProc("DoDragDrop")
	procGlobalAlloc          = modkernel32.NewProc("GlobalAlloc")
	procGlobalLock           = modkernel32.NewProc("GlobalLock")
	procGlobalUnlock         = modkernel32.NewProc("GlobalUnlock")
	procGlobalFree           = modkernel32.NewProc("GlobalFree")
	procRtlMoveMemory        = modkernel32.NewProc("RtlMoveMemory")
	procRegisterClipboardFmt = moduser32Drag.NewProc("RegisterClipboardFormatW")
)

// formatEtc mirrors the C FORMATETC. The Go field order and types reproduce the
// exact 64-bit ABI layout (cfFormat at 0, the 8-byte-aligned ptd at 8, then the
// three 32-bit fields), so the struct can be shared verbatim with the OS.
type formatEtc struct {
	cfFormat uint16
	ptd      uintptr
	dwAspect uint32
	lindex   int32
	tymed    uint32
}

// stgMedium mirrors the C STGMEDIUM (tymed at 0, the 8-byte-aligned union handle
// at 8, pUnkForRelease at 16). We only ever populate the HGLOBAL union member.
type stgMedium struct {
	tymed          uint32
	hGlobal        uintptr
	pUnkForRelease uintptr
}

// dataObjectVtbl is the IDataObject vtable — one function pointer per method, in
// interface-declaration order (IUnknown first).
type dataObjectVtbl struct {
	QueryInterface        uintptr
	AddRef                uintptr
	Release               uintptr
	GetData               uintptr
	GetDataHere           uintptr
	QueryGetData          uintptr
	GetCanonicalFormatEtc uintptr
	SetData               uintptr
	EnumFormatEtc         uintptr
	DAdvise               uintptr
	DUnadvise             uintptr
	EnumDAdvise           uintptr
}

type dropSourceVtbl struct {
	QueryInterface    uintptr
	AddRef            uintptr
	Release           uintptr
	QueryContinueDrag uintptr
	GiveFeedback      uintptr
}

type enumFormatEtcVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	Next           uintptr
	Skip           uintptr
	Reset          uintptr
	Clone          uintptr
}

// comDataObject is our IDataObject. The first field MUST stay the vtable pointer
// so the "this" pointer passed to each callback casts straight back to it.
type comDataObject struct {
	lpVtbl   *dataObjectVtbl
	refs     int32
	formats  []formatEtc
	payloads [][]byte // parallel to formats — the HGLOBAL bytes for each flavor
}

type comDropSource struct {
	lpVtbl *dropSourceVtbl
	refs   int32
}

type comEnumFormatEtc struct {
	lpVtbl  *enumFormatEtcVtbl
	refs    int32
	formats []formatEtc
	index   int
}

// Vtable singletons. Package-level so the syscall trampolines are created once
// and the vtables never move; every instance points its lpVtbl at these. They are
// populated in init() rather than as var initializers to break the static
// initialization cycle (a vtable holds enumClone, which calls newEnumFormatEtc,
// which reads enumFormatEtcVtblInst).
var (
	dataObjectVtblInst    dataObjectVtbl
	dropSourceVtblInst    dropSourceVtbl
	enumFormatEtcVtblInst enumFormatEtcVtbl
)

func init() {
	dataObjectVtblInst = dataObjectVtbl{
		QueryInterface:        syscall.NewCallback(dataObjectQueryInterface),
		AddRef:                syscall.NewCallback(dataObjectAddRef),
		Release:               syscall.NewCallback(dataObjectRelease),
		GetData:               syscall.NewCallback(dataObjectGetData),
		GetDataHere:           syscall.NewCallback(dataObjectGetDataHere),
		QueryGetData:          syscall.NewCallback(dataObjectQueryGetData),
		GetCanonicalFormatEtc: syscall.NewCallback(dataObjectGetCanonicalFormatEtc),
		SetData:               syscall.NewCallback(dataObjectSetData),
		EnumFormatEtc:         syscall.NewCallback(dataObjectEnumFormatEtc),
		DAdvise:               syscall.NewCallback(dataObjectDAdvise),
		DUnadvise:             syscall.NewCallback(dataObjectDUnadvise),
		EnumDAdvise:           syscall.NewCallback(dataObjectEnumDAdvise),
	}

	dropSourceVtblInst = dropSourceVtbl{
		QueryInterface:    syscall.NewCallback(dropSourceQueryInterface),
		AddRef:            syscall.NewCallback(dropSourceAddRef),
		Release:           syscall.NewCallback(dropSourceRelease),
		QueryContinueDrag: syscall.NewCallback(dropSourceQueryContinueDrag),
		GiveFeedback:      syscall.NewCallback(dropSourceGiveFeedback),
	}

	enumFormatEtcVtblInst = enumFormatEtcVtbl{
		QueryInterface: syscall.NewCallback(enumQueryInterface),
		AddRef:         syscall.NewCallback(enumAddRef),
		Release:        syscall.NewCallback(enumRelease),
		Next:           syscall.NewCallback(enumNext),
		Skip:           syscall.NewCallback(enumSkip),
		Reset:          syscall.NewCallback(enumReset),
		Clone:          syscall.NewCallback(enumClone),
	}
}

// newDataObject builds an IDataObject over the flavor list and pins it. formats
// and payloads must stay index-parallel.
func newDataObject(formats []formatEtc, payloads [][]byte) *comDataObject {
	o := &comDataObject{lpVtbl: &dataObjectVtblInst, refs: 1, formats: formats, payloads: payloads}
	if activeDrag != nil {
		activeDrag.pin(o)
	}
	return o
}

func newDropSource() *comDropSource {
	s := &comDropSource{lpVtbl: &dropSourceVtblInst, refs: 1}
	if activeDrag != nil {
		activeDrag.pin(s)
	}
	return s
}

func newEnumFormatEtc(formats []formatEtc, index int) *comEnumFormatEtc {
	e := &comEnumFormatEtc{lpVtbl: &enumFormatEtcVtblInst, refs: 1, formats: formats, index: index}
	if activeDrag != nil {
		activeDrag.pin(e)
	}
	return e
}

// guidEqual reports whether two interface IDs are identical.
func guidEqual(a, b *windows.GUID) bool {
	return a.Data1 == b.Data1 && a.Data2 == b.Data2 && a.Data3 == b.Data3 && a.Data4 == b.Data4
}

// globalFromBytes allocates a moveable HGLOBAL and copies b into it, returning
// the handle. The receiver of the STGMEDIUM owns and frees it via
// ReleaseStgMedium, so GetData hands out a fresh copy per call. The copy is done
// via RtlMoveMemory so the OS-owned destination stays a uintptr (no
// uintptr→unsafe.Pointer conversion to trip `go vet`).
func globalFromBytes(b []byte) (uintptr, bool) {
	h, _, _ := procGlobalAlloc.Call(gmemMoveable|gmemZeroInit, uintptr(len(b)))
	if h == 0 {
		return 0, false
	}
	if len(b) == 0 {
		return h, true
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		procGlobalFree.Call(h)
		return 0, false
	}
	procRtlMoveMemory.Call(p, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
	runtime.KeepAlive(b)
	procGlobalUnlock.Call(h)
	return h, true
}

// --- IDataObject ---

func dataObjectQueryInterface(this, riid, ppv unsafe.Pointer) uintptr {
	if ppv == nil {
		return eInvalidArg
	}
	g := (*windows.GUID)(riid)
	if guidEqual(g, &iidIUnknown) || guidEqual(g, &iidIDataObject) {
		obj := (*comDataObject)(this)
		atomic.AddInt32(&obj.refs, 1)
		*(*uintptr)(ppv) = uintptr(this)
		return sOK
	}
	*(*uintptr)(ppv) = 0
	return eNoInterface
}

func dataObjectAddRef(this unsafe.Pointer) uintptr {
	obj := (*comDataObject)(this)
	return uintptr(atomic.AddInt32(&obj.refs, 1))
}

func dataObjectRelease(this unsafe.Pointer) uintptr {
	obj := (*comDataObject)(this)
	// Lifetime is owned by the dragSession pin (freed after DoDragDrop returns),
	// so we only track the count; reaching zero is a no-op.
	return uintptr(atomic.AddInt32(&obj.refs, -1))
}

func dataObjectGetData(this, pformatetc, pmedium unsafe.Pointer) uintptr {
	if pformatetc == nil || pmedium == nil {
		return eInvalidArg
	}
	obj := (*comDataObject)(this)
	fe := (*formatEtc)(pformatetc)
	med := (*stgMedium)(pmedium)
	*med = stgMedium{}

	if fe.tymed&tymedHGlobal == 0 {
		return dvETymed
	}
	for i := range obj.formats {
		f := &obj.formats[i]
		if f.cfFormat != fe.cfFormat || f.dwAspect != fe.dwAspect {
			continue
		}
		h, ok := globalFromBytes(obj.payloads[i])
		if !ok {
			return eOutOfMemory
		}
		med.tymed = tymedHGlobal
		med.hGlobal = h
		med.pUnkForRelease = 0
		return sOK
	}
	return dvEFormatEtc
}

func dataObjectGetDataHere(this, pformatetc, pmedium unsafe.Pointer) uintptr {
	return eNotImpl // caller-allocated medium not supported (HGLOBAL flavors only)
}

func dataObjectQueryGetData(this, pformatetc unsafe.Pointer) uintptr {
	if pformatetc == nil {
		return eInvalidArg
	}
	obj := (*comDataObject)(this)
	fe := (*formatEtc)(pformatetc)
	if fe.tymed&tymedHGlobal == 0 {
		return dvETymed
	}
	for i := range obj.formats {
		if obj.formats[i].cfFormat == fe.cfFormat && obj.formats[i].dwAspect == fe.dwAspect {
			return sOK
		}
	}
	return dvEFormatEtc
}

func dataObjectGetCanonicalFormatEtc(this, pformatetcIn, pformatetcOut unsafe.Pointer) uintptr {
	if pformatetcOut != nil {
		out := (*formatEtc)(pformatetcOut)
		out.ptd = 0
	}
	return dataSSameFmt
}

func dataObjectSetData(this, pformatetc, pmedium unsafe.Pointer, fRelease uintptr) uintptr {
	return eNotImpl // read-only data source
}

func dataObjectEnumFormatEtc(this unsafe.Pointer, dwDirection uintptr, ppenum unsafe.Pointer) uintptr {
	if ppenum == nil {
		return eInvalidArg
	}
	if uint32(dwDirection) != dataDirGet {
		*(*uintptr)(ppenum) = 0
		return eNotImpl // only the GET direction is meaningful for a drag source
	}
	obj := (*comDataObject)(this)
	e := newEnumFormatEtc(obj.formats, 0)
	*(*uintptr)(ppenum) = uintptr(unsafe.Pointer(e))
	return sOK
}

func dataObjectDAdvise(this, pformatetc unsafe.Pointer, advf uintptr, pAdvSink, pdwConnection unsafe.Pointer) uintptr {
	if pdwConnection != nil {
		*(*uint32)(pdwConnection) = 0
	}
	return oleEAdviseNot
}

func dataObjectDUnadvise(this unsafe.Pointer, dwConnection uintptr) uintptr {
	return oleEAdviseNot
}

func dataObjectEnumDAdvise(this, ppenumAdvise unsafe.Pointer) uintptr {
	if ppenumAdvise != nil {
		*(*uintptr)(ppenumAdvise) = 0
	}
	return oleEAdviseNot
}

// --- IEnumFORMATETC ---

func enumQueryInterface(this, riid, ppv unsafe.Pointer) uintptr {
	if ppv == nil {
		return eInvalidArg
	}
	g := (*windows.GUID)(riid)
	if guidEqual(g, &iidIUnknown) || guidEqual(g, &iidIEnumFORMATETC) {
		obj := (*comEnumFormatEtc)(this)
		atomic.AddInt32(&obj.refs, 1)
		*(*uintptr)(ppv) = uintptr(this)
		return sOK
	}
	*(*uintptr)(ppv) = 0
	return eNoInterface
}

func enumAddRef(this unsafe.Pointer) uintptr {
	obj := (*comEnumFormatEtc)(this)
	return uintptr(atomic.AddInt32(&obj.refs, 1))
}

func enumRelease(this unsafe.Pointer) uintptr {
	obj := (*comEnumFormatEtc)(this)
	return uintptr(atomic.AddInt32(&obj.refs, -1))
}

func enumNext(this unsafe.Pointer, celt uintptr, rgelt, pceltFetched unsafe.Pointer) uintptr {
	obj := (*comEnumFormatEtc)(this)
	n := int(celt)
	if n <= 0 || rgelt == nil {
		if pceltFetched != nil {
			*(*uint32)(pceltFetched) = 0
		}
		return eInvalidArg
	}
	dst := unsafe.Slice((*formatEtc)(rgelt), n)
	fetched := 0
	for fetched < n && obj.index < len(obj.formats) {
		dst[fetched] = obj.formats[obj.index]
		dst[fetched].ptd = 0
		obj.index++
		fetched++
	}
	if pceltFetched != nil {
		*(*uint32)(pceltFetched) = uint32(fetched)
	}
	if fetched == n {
		return sOK
	}
	return sFalse
}

func enumSkip(this unsafe.Pointer, celt uintptr) uintptr {
	obj := (*comEnumFormatEtc)(this)
	remaining := len(obj.formats) - obj.index
	if int(celt) > remaining {
		obj.index = len(obj.formats)
		return sFalse
	}
	obj.index += int(celt)
	return sOK
}

func enumReset(this unsafe.Pointer) uintptr {
	obj := (*comEnumFormatEtc)(this)
	obj.index = 0
	return sOK
}

func enumClone(this, ppenum unsafe.Pointer) uintptr {
	if ppenum == nil {
		return eInvalidArg
	}
	obj := (*comEnumFormatEtc)(this)
	clone := newEnumFormatEtc(obj.formats, obj.index)
	*(*uintptr)(ppenum) = uintptr(unsafe.Pointer(clone))
	return sOK
}

// --- IDropSource ---

func dropSourceQueryInterface(this, riid, ppv unsafe.Pointer) uintptr {
	if ppv == nil {
		return eInvalidArg
	}
	g := (*windows.GUID)(riid)
	if guidEqual(g, &iidIUnknown) || guidEqual(g, &iidIDropSource) {
		obj := (*comDropSource)(this)
		atomic.AddInt32(&obj.refs, 1)
		*(*uintptr)(ppv) = uintptr(this)
		return sOK
	}
	*(*uintptr)(ppv) = 0
	return eNoInterface
}

func dropSourceAddRef(this unsafe.Pointer) uintptr {
	obj := (*comDropSource)(this)
	return uintptr(atomic.AddInt32(&obj.refs, 1))
}

func dropSourceRelease(this unsafe.Pointer) uintptr {
	obj := (*comDropSource)(this)
	return uintptr(atomic.AddInt32(&obj.refs, -1))
}

// dropSourceQueryContinueDrag drives the drag: Escape cancels, releasing the left
// mouse button (which started the drag) drops, otherwise the drag continues. This
// is the canonical IDropSource loop.
func dropSourceQueryContinueDrag(this unsafe.Pointer, fEscapePressed, grfKeyState uintptr) uintptr {
	if fEscapePressed != 0 {
		return dragDropSCancel
	}
	if grfKeyState&mkLButton == 0 {
		return dragDropSDrop
	}
	return sOK
}

// dropSourceGiveFeedback defers to the OS default drag cursors.
func dropSourceGiveFeedback(this unsafe.Pointer, dwEffect uintptr) uintptr {
	return dragDropSUseDefaultCursors
}
