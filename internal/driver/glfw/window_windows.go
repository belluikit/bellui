package glfw

import (
	"runtime"
	"syscall"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/scale"

	"golang.org/x/sys/windows/registry"
)

func (w *window) setDarkMode() {
	if runtime.GOOS == "windows" {
		hwnd := w.view().GetWin32Window()
		dark := isDark()
		// cannot use a go bool.
		var winBool int32
		if dark {
			winBool = 1
		}
		dwm := syscall.NewLazyDLL("dwmapi.dll")
		setAtt := dwm.NewProc("DwmSetWindowAttribute")
		ret, _, err := setAtt.Call(uintptr(unsafe.Pointer(hwnd)), // window handle
			20,                                // DWMWA_USE_IMMERSIVE_DARK_MODE
			uintptr(unsafe.Pointer(&winBool)), // on or off
			4)                                 // sizeof(bool for windows)

		if ret != 0 && ret != 0x80070057 { // err is always non-nil, we check return value (except erroneous code)
			bellui.LogError("Failed to set dark mode", err)
		}
	}
}

func isDark() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil { // older version of Windows will not have this key
		return false
	}
	defer k.Close()

	useLight, _, err := k.GetIntegerValue("AppsUseLightTheme")
	if err != nil { // older version of Windows will not have this value
		return false
	}

	return useLight == 0
}

func (w *window) computeCanvasSize(width, height int) geo.Size {
	if w.fixedSize {
		return geo.NewSize(scale.ToBelluiCoordinate(w.canvas, w.width), scale.ToBelluiCoordinate(w.canvas, w.height))
	}
	return geo.NewSize(scale.ToBelluiCoordinate(w.canvas, width), scale.ToBelluiCoordinate(w.canvas, height))
}

// doSetParentNative sets (or clears, when parent is nil) the native owner
// window via GWLP_HWNDPARENT, so Windows keeps this window above its owner and
// minimizes them together. Must run on the main goroutine.
func (w *window) doSetParentNative(parent *window) {
	child := uintptr(unsafe.Pointer(w.view().GetWin32Window()))
	var parentHWND uintptr
	if parent != nil {
		parentHWND = uintptr(unsafe.Pointer(parent.view().GetWin32Window()))
	}

	const gwlpHWNDParent = -8 // GWLP_HWNDPARENT (owner window)
	user32 := syscall.NewLazyDLL("user32.dll")
	setLongPtr := user32.NewProc("SetWindowLongPtrW")
	idx := gwlpHWNDParent
	setLongPtr.Call(child, uintptr(idx), parentHWND)
}

// doSetModalNative enforces (modal) or lifts native window-modality against the
// parent via EnableWindow: a disabled parent HWND rejects mouse and keyboard
// input and greys its title bar, so Windows blocks the owner while the dialog
// is up. The parent must be re-enabled before the child is destroyed (see
// releaseModal), else it is left permanently disabled. Must run on the main
// goroutine. A nil parent is a no-op — modality is expressed against the owner.
func (w *window) doSetModalNative(parent *window, modal bool) {
	if parent == nil || !w.blockParentModal(parent, modal) {
		return // nil parent, or another modal child already blocks it (#682)
	}
	parentHWND := uintptr(unsafe.Pointer(parent.view().GetWin32Window()))

	user32 := syscall.NewLazyDLL("user32.dll")
	enableWindow := user32.NewProc("EnableWindow")
	var enable uintptr // BOOL bEnable — 0 disables the parent, 1 re-enables it
	if !modal {
		enable = 1
	}
	enableWindow.Call(parentHWND, enable)
}

// assert we are implementing driver.NativeWindow
var _ driver.NativeWindow = (*window)(nil)

func (w *window) RunNative(f func(any)) {
	context := driver.WindowsWindowContext{}
	if v := w.view(); v != nil {
		context.HWND = uintptr(unsafe.Pointer(v.GetWin32Window()))
	}

	f(context)
}
