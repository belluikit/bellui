package glfw

import (
	"os"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/belluikit/bellui"
)

// taskbarDebug, when the BELLUI_TASKBAR_DEBUG env var is set, logs every COM
// HRESULT so the live-verify harness can confirm the calls succeeded.
var taskbarDebug = os.Getenv("BELLUI_TASKBAR_DEBUG") != ""

// Windows taskbar progress backend for desktop.ProgressWindow (pre-rebrand tracker #517). It drives
// ITaskbarList3::SetProgressState / SetProgressValue purely through
// x/sys/windows COM syscalls (no cgo), mirroring the native file-dialog style
// in dialog/file_windows_native.go. All calls run on the main goroutine (that
// is where the window's HWND and the COM apartment live), so the created
// ITaskbarList3 pointer is cached for the process lifetime without locking.

var (
	// CLSID_TaskbarList and IID_ITaskbarList3.
	clsidTaskbarList = windows.GUID{Data1: 0x56FDF344, Data2: 0xFD6D, Data3: 0x11D0, Data4: [8]byte{0x95, 0x8A, 0x00, 0x60, 0x97, 0xC9, 0xA0, 0x90}}
	iidTaskbarList3  = windows.GUID{Data1: 0xEA1AFB91, Data2: 0x9E28, Data3: 0x4B86, Data4: [8]byte{0x90, 0xE9, 0x9E, 0x9F, 0x8A, 0x5E, 0xEF, 0xAF}}

	modole32taskbar      = windows.NewLazySystemDLL("ole32.dll")
	procCoInitializeExTB = modole32taskbar.NewProc("CoInitializeEx")
	procCoCreateInstance = modole32taskbar.NewProc("CoCreateInstance")
)

const (
	clsctxInprocServer      = 0x1
	coinitApartmentThreaded = 0x2

	// ITaskbarList3 vtable slots: IUnknown -> ITaskbarList -> ITaskbarList2 ->
	// ITaskbarList3.
	vtblTBRelease          = 2  // IUnknown::Release
	vtblTBHrInit           = 3  // ITaskbarList::HrInit
	vtblTBSetProgressValue = 9  // ITaskbarList3::SetProgressValue
	vtblTBSetProgressState = 10 // ITaskbarList3::SetProgressState

	// TBPFLAG progress states.
	tbpfNoProgress    = 0x0
	tbpfIndeterminate = 0x1
	tbpfNormal        = 0x2

	// progressScale is the fixed denominator handed to SetProgressValue; the
	// numerator is value*progressScale, giving ~0.1% granularity.
	progressScale = 1000
)

// tbComObject is the shared COM interface layout: a pointer to a vtable of
// function pointers, indexed by slot.
type tbComObject struct {
	vtbl *[64]uintptr
}

var (
	taskbarOnce sync.Once
	taskbarList *tbComObject // nil if the taskbar interface is unavailable
)

// taskbar returns the process-wide ITaskbarList3 instance, creating it on first
// use. Must be called on the main goroutine (the COM apartment is thread-affine
// and every caller — SetProgress via async.EnsureMain and applyProgress from
// Show — already runs there).
func taskbar() *tbComObject {
	taskbarOnce.Do(func() {
		// Ensure this thread has a COM apartment. If GLFW already initialised a
		// different concurrency model, CoInitializeEx returns RPC_E_CHANGED_MODE
		// and we proceed regardless — ITaskbarList is registered "Both", so it
		// works in either apartment. We deliberately never CoUninitialize: the
		// instance lives for the process lifetime.
		procCoInitializeExTB.Call(0, coinitApartmentThreaded)

		var tb *tbComObject
		if hr, _, _ := procCoCreateInstance.Call(
			uintptr(unsafe.Pointer(&clsidTaskbarList)), 0, clsctxInprocServer,
			uintptr(unsafe.Pointer(&iidTaskbarList3)), uintptr(unsafe.Pointer(&tb)),
		); int32(hr) < 0 || tb == nil {
			bellui.LogError("taskbar progress: CoCreateInstance(ITaskbarList3) failed", syscall.Errno(uint32(hr)))
			return
		}
		// HrInit must succeed before any other method is called.
		if hr := tbCall(tb, vtblTBHrInit); hr < 0 {
			bellui.LogError("taskbar progress: ITaskbarList3::HrInit failed", syscall.Errno(uint32(hr)))
			tbCall(tb, vtblTBRelease)
			return
		}
		if taskbarDebug {
			bellui.LogError("taskbar progress: ITaskbarList3 initialised (HrInit S_OK)", nil)
		}
		taskbarList = tb
	})
	return taskbarList
}

// tbCall invokes a COM method by vtable slot with the interface pointer as the
// implicit `this` first argument.
func tbCall(obj *tbComObject, slot int, args ...uintptr) int32 {
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, uintptr(unsafe.Pointer(obj)))
	all = append(all, args...)
	hr, _, _ := syscall.SyscallN(obj.vtbl[slot], all...)
	return int32(hr)
}

// applyProgressNative drives the taskbar button for this window. It runs on the
// main goroutine. Values outside 0..1 are clamped; a negative value clears the
// indicator; indeterminate wins over any value.
func (w *window) applyProgressNative(value float64, indeterminate bool) {
	view := w.view()
	if view == nil {
		return
	}
	hwnd := uintptr(unsafe.Pointer(view.GetWin32Window()))
	if hwnd == 0 {
		return
	}
	tb := taskbar()
	if tb == nil {
		return
	}

	switch {
	case indeterminate:
		hrState := tbCall(tb, vtblTBSetProgressState, hwnd, tbpfIndeterminate)
		taskbarLog("SetProgressState(INDETERMINATE)", hrState)
	case value < 0:
		hrState := tbCall(tb, vtblTBSetProgressState, hwnd, tbpfNoProgress)
		taskbarLog("SetProgressState(NOPROGRESS)", hrState)
	default:
		if value > 1 {
			value = 1
		}
		hrState := tbCall(tb, vtblTBSetProgressState, hwnd, tbpfNormal)
		hrVal := tbCall(tb, vtblTBSetProgressValue, hwnd,
			uintptr(uint64(value*progressScale)), uintptr(uint64(progressScale)))
		taskbarLog("SetProgressState(NORMAL)", hrState)
		taskbarLog("SetProgressValue", hrVal)
	}
}

// taskbarLog reports a failed COM call always, and a successful one only under
// BELLUI_TASKBAR_DEBUG (used by the live-verify harness).
func taskbarLog(what string, hr int32) {
	if hr < 0 {
		bellui.LogError("taskbar progress: "+what+" failed", syscall.Errno(uint32(hr)))
	} else if taskbarDebug {
		bellui.LogError("taskbar progress: "+what+" S_OK (hwnd ok)", nil)
	}
}
