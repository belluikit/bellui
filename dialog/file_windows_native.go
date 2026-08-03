//go:build windows

package dialog

import (
	"mime"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/storage"
)

// This is the Windows native file-dialog backend for #514: it drives the modern
// Common Item Dialog (IFileOpenDialog / IFileSaveDialog) purely through
// x/sys/windows COM syscalls (no cgo). It plugs into the shared
// fileOpenOSOverride / fileSaveOSOverride seam; returning false anywhere means
// the canvas-drawn dialog is used instead, so the feature is never lost.

// COM CLSIDs / IIDs for the Common Item Dialog.
var (
	clsidFileOpenDialog = windows.GUID{Data1: 0xDC1C5A9C, Data2: 0xE88A, Data3: 0x4DDE, Data4: [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	clsidFileSaveDialog = windows.GUID{Data1: 0xC0B4E2F3, Data2: 0xBA21, Data3: 0x4773, Data4: [8]byte{0x8D, 0xBA, 0x33, 0x5E, 0xC9, 0x46, 0xEB, 0x8B}}
	iidFileOpenDialog   = windows.GUID{Data1: 0xD57C7288, Data2: 0xD4AD, Data3: 0x4768, Data4: [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
	iidFileSaveDialog   = windows.GUID{Data1: 0x84BCCD23, Data2: 0x5FDE, Data3: 0x4CDB, Data4: [8]byte{0xAE, 0xA4, 0xAF, 0x64, 0xB8, 0x3D, 0x78, 0xAB}}
	iidShellItem        = windows.GUID{Data1: 0x43826D1E, Data2: 0xE718, Data3: 0x42EE, Data4: [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}
)

const (
	clsctxInprocServer      = 0x1
	coinitApartmentThreaded = 0x2

	// SIGDN_FILESYSPATH: request the item's absolute file-system path.
	sigdnFilesysPath = 0x80058000

	// FILEOPENDIALOGOPTIONS bits.
	fosOverwritePrompt  = 0x2
	fosPickFolders      = 0x20
	fosForceFilesystem  = 0x40
	fosAllowMultiSelect = 0x200
	fosPathMustExist    = 0x800
	fosFileMustExist    = 0x1000

	// HRESULT_FROM_WIN32(ERROR_CANCELLED): the user dismissed the dialog.
	hresultCancelled = 0x800704C7

	// RPC_E_CHANGED_MODE: CoInitializeEx was already called on this thread with
	// a different concurrency model; we then must not balance it with an
	// unmatched CoUninitialize.
	rpcEChangedMode = 0x80010106
)

var (
	modole32   = windows.NewLazySystemDLL("ole32.dll")
	modshell32 = windows.NewLazySystemDLL("shell32.dll")
	moduser32  = windows.NewLazySystemDLL("user32.dll")

	procCoInitializeEx              = modole32.NewProc("CoInitializeEx")
	procCoUninitialize              = modole32.NewProc("CoUninitialize")
	procCoCreateInstance            = modole32.NewProc("CoCreateInstance")
	procCoTaskMemFree               = modole32.NewProc("CoTaskMemFree")
	procSHCreateItemFromParsingName = modshell32.NewProc("SHCreateItemFromParsingName")
	procEnableWindow                = moduser32.NewProc("EnableWindow")
)

// comObject is the memory layout every COM interface pointer shares: a pointer
// to its vtable of function pointers. We index the vtable by offset.
type comObject struct {
	vtbl *[64]uintptr
}

// IFileDialog (the common base of IFileOpenDialog / IFileSaveDialog) vtable
// slots we use. Order follows IUnknown -> IModalWindow -> IFileDialog.
const (
	vtblRelease      = 2  // IUnknown::Release
	vtblShow         = 3  // IModalWindow::Show
	vtblSetFileTypes = 4  // IFileDialog::SetFileTypes
	vtblSetOptions   = 9  // IFileDialog::SetOptions
	vtblGetOptions   = 10 // IFileDialog::GetOptions
	vtblSetFolder    = 12 // IFileDialog::SetFolder
	vtblSetFileName  = 15 // IFileDialog::SetFileName
	vtblSetTitle     = 17 // IFileDialog::SetTitle
	vtblSetOkLabel   = 18 // IFileDialog::SetOkButtonLabel
	vtblGetResult    = 20 // IFileDialog::GetResult
	vtblSetDefExt    = 22 // IFileDialog::SetDefaultExtension
	vtblGetResults   = 27 // IFileOpenDialog::GetResults (multi-select, #716)
)

// IShellItem vtable slots.
const (
	vtblShellItemRelease        = 2
	vtblShellItemGetDisplayName = 5
)

// IShellItemArray vtable slots (the multi-select result, #716).
const (
	vtblShellArrayRelease  = 2
	vtblShellArrayGetCount = 7
	vtblShellArrayGetItem  = 8
)

// comdlgFilterSpec mirrors the C COMDLG_FILTERSPEC { LPCWSTR pszName, pszSpec }.
type comdlgFilterSpec struct {
	pszName *uint16
	pszSpec *uint16
}

func fileOpenOSOverride(d *FileDialog) bool {
	if !nativeFileDialogsFor(d) {
		return false
	}

	kind := panelOpenFile
	if d.isDirectory() {
		kind = panelOpenFolder
	}
	// Multi-select applies to file open only; a folder picker stays single (#716).
	showWindowsDialog(d, kind, d.multiSelect && kind == panelOpenFile)
	return true
}

func fileSaveOSOverride(d *FileDialog) bool {
	if !nativeFileDialogsFor(d) {
		return false
	}

	showWindowsDialog(d, panelSaveFile, false)
	return true
}

// panelKind records which native dialog to present so the result can be wrapped
// in the correctly-typed storage handle and delivered to the right callback.
type panelKind int

const (
	panelOpenFile panelKind = iota
	panelOpenFolder
	panelSaveFile
)

// showWindowsDialog captures the parent HWND on the calling goroutine, then
// runs the modal COM dialog on a dedicated, OS-thread-locked goroutine (COM
// apartments are thread-affine and Show pumps its own modal message loop). The
// result is delivered back on the main goroutine via bellui.Do, mirroring the
// portal and macOS backends.
func showWindowsDialog(d *FileDialog, kind panelKind, multi bool) {
	var hwnd uintptr
	if native, ok := d.parent.(driver.NativeWindow); ok {
		native.RunNative(func(context any) {
			if win, ok := context.(driver.WindowsWindowContext); ok {
				hwnd = win.HWND
			}
		})
	}

	title := dialogTitleWindows(d, kind)
	prompt := d.confirmText
	initialDir := startingDirPathWindows(d)
	initialName := d.initialFileName
	exts := filterExtensionsWindows(d.filter)

	// The dialog's Show runs on the dedicated COM thread below, so Windows does
	// not auto-disable the owner window (owner-modality needs same-thread
	// ownership). Disable it manually here — on the parent's own (main)
	// goroutine — for true modality, and re-enable once Show returns (#634).
	// EnableWindow is skipped when unparented (hwnd == 0).
	enableParentWindow(hwnd, false)

	go func() {
		// Lock (and never unlock) this goroutine's OS thread so the COM
		// apartment lives and dies with the goroutine.
		runtime.LockOSThread()

		paths, cancelled, err := runWindowsDialog(kind, multi, hwnd, title, prompt, initialDir, initialName, exts)
		if err != nil {
			bellui.LogError("Native file dialog failed", err)
		}
		// Re-enable the parent on the main goroutine before delivering the
		// result, so it is interactive again the instant the callback fires.
		bellui.Do(func() { enableParentWindow(hwnd, true) })
		deliverWindowsResult(d, kind, paths, cancelled || err != nil)
	}()
}

// enableParentWindow toggles input to the owner window via EnableWindow. A zero
// handle (unparented dialog) is a no-op.
func enableParentWindow(hwnd uintptr, enable bool) {
	if hwnd == 0 {
		return
	}
	var flag uintptr
	if enable {
		flag = 1
	}
	procEnableWindow.Call(hwnd, flag)
}

// runWindowsDialog performs the full COM sequence on the current (locked)
// thread and returns the chosen file-system path.
func runWindowsDialog(kind panelKind, multi bool, hwnd uintptr, title, prompt, initialDir, initialName string, exts []string) (paths []string, cancelled bool, err error) {
	hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded)
	// S_OK (0) and S_FALSE (1) each need a matching CoUninitialize;
	// RPC_E_CHANGED_MODE means the apartment already existed - proceed but
	// leave its lifetime to whoever created it.
	if uint32(hr) != rpcEChangedMode {
		defer procCoUninitialize.Call()
	}

	clsid, iid := &clsidFileOpenDialog, &iidFileOpenDialog
	if kind == panelSaveFile {
		clsid, iid = &clsidFileSaveDialog, &iidFileSaveDialog
	}

	var dialog *comObject
	if hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(clsid)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&dialog)),
	); int32(hr) < 0 {
		return nil, false, syscall.Errno(hr)
	}
	defer comCall(dialog, vtblRelease)

	// comCall stashes pointer arguments into a []uintptr, which defeats the
	// syscall pointer-liveness guarantee (that only fires when the
	// unsafe.Pointer->uintptr conversion is written in the call expression
	// itself). Go's GC is non-moving so the addresses stay valid, but the
	// backing objects could still be reclaimed mid-call - so every Go pointer
	// handed to a COM method is pinned in this slice, kept alive to end of
	// function.
	var pins []any
	defer func() { runtime.KeepAlive(pins) }()
	pinnedUTF16 := func(s string) *uint16 {
		p, e := windows.UTF16PtrFromString(s)
		if e != nil {
			return nil
		}
		pins = append(pins, p)
		return p
	}

	applyWindowsOptions(dialog, kind, multi)
	if title != "" {
		if p := pinnedUTF16(title); p != nil {
			comCall(dialog, vtblSetTitle, uintptr(unsafe.Pointer(p)))
		}
	}
	if prompt != "" {
		if p := pinnedUTF16(prompt); p != nil {
			comCall(dialog, vtblSetOkLabel, uintptr(unsafe.Pointer(p)))
		}
	}
	if initialName != "" {
		if p := pinnedUTF16(initialName); p != nil {
			comCall(dialog, vtblSetFileName, uintptr(unsafe.Pointer(p)))
		}
	}
	setWindowsFolder(dialog, initialDir)
	if specs := setWindowsFileTypes(dialog, kind, exts); specs != nil {
		pins = append(pins, specs)
	}

	if hr, _, _ := syscall.SyscallN(dialog.vtbl[vtblShow], uintptr(unsafe.Pointer(dialog)), hwnd); uint32(hr) == hresultCancelled {
		return nil, true, nil
	} else if int32(hr) < 0 {
		return nil, false, syscall.Errno(hr)
	}

	if multi {
		return windowsDialogResultMulti(dialog)
	}
	path, cancelled, err := windowsDialogResult(dialog)
	if err != nil || cancelled || path == "" {
		return nil, cancelled, err
	}
	return []string{path}, false, nil
}

// applyWindowsOptions reads the current option flags and OR-s in the ones this
// dialog kind needs (the documented get-then-set pattern).
func applyWindowsOptions(dialog *comObject, kind panelKind, multi bool) {
	var opts uint32
	comCall(dialog, vtblGetOptions, uintptr(unsafe.Pointer(&opts)))

	opts |= fosForceFilesystem | fosPathMustExist
	switch kind {
	case panelOpenFolder:
		opts |= fosPickFolders
	case panelSaveFile:
		opts |= fosOverwritePrompt
	default:
		opts |= fosFileMustExist
	}
	if multi {
		opts |= fosAllowMultiSelect
	}
	comCall(dialog, vtblSetOptions, uintptr(opts))
}

// setWindowsFolder points the dialog at the requested starting directory, if a
// shell item can be created for it. A failure is non-fatal - the dialog just
// opens at its default location.
func setWindowsFolder(dialog *comObject, dir string) {
	if dir == "" {
		return
	}
	pathPtr, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return
	}

	var item *comObject
	if hr, _, _ := procSHCreateItemFromParsingName.Call(
		uintptr(unsafe.Pointer(pathPtr)), 0,
		uintptr(unsafe.Pointer(&iidShellItem)), uintptr(unsafe.Pointer(&item)),
	); int32(hr) < 0 || item == nil {
		return
	}
	defer comCall(item, vtblShellItemRelease)

	comCall(dialog, vtblSetFolder, uintptr(unsafe.Pointer(item)))
}

// setWindowsFileTypes installs the extension filter (folder dialogs skip it) and
// returns the backing spec slice so its strings survive until after Show.
func setWindowsFileTypes(dialog *comObject, kind panelKind, exts []string) []comdlgFilterSpec {
	if kind == panelOpenFolder || len(exts) == 0 {
		return nil
	}

	patterns := make([]string, len(exts))
	for i, ext := range exts {
		patterns[i] = "*." + ext
	}
	name := lang.L("Supported Files") + " (" + strings.Join(patterns, ";") + ")"

	namePtr, err1 := windows.UTF16PtrFromString(name)
	specPtr, err2 := windows.UTF16PtrFromString(strings.Join(patterns, ";"))
	if err1 != nil || err2 != nil {
		return nil
	}
	specs := []comdlgFilterSpec{{pszName: namePtr, pszSpec: specPtr}}

	comCall(dialog, vtblSetFileTypes, uintptr(len(specs)), uintptr(unsafe.Pointer(&specs[0])))
	if defExt, e := windows.UTF16PtrFromString(exts[0]); e == nil {
		comCall(dialog, vtblSetDefExt, uintptr(unsafe.Pointer(defExt)))
		runtime.KeepAlive(defExt)
	}
	return specs
}

// windowsDialogResult pulls the selected item's file-system path out of the
// dialog after a successful Show.
func windowsDialogResult(dialog *comObject) (path string, cancelled bool, err error) {
	var item *comObject
	if hr, _, _ := syscall.SyscallN(dialog.vtbl[vtblGetResult], uintptr(unsafe.Pointer(dialog)), uintptr(unsafe.Pointer(&item))); int32(hr) < 0 || item == nil {
		return "", false, syscall.Errno(hr)
	}
	defer comCall(item, vtblShellItemRelease)

	var namePtr *uint16
	if hr, _, _ := syscall.SyscallN(item.vtbl[vtblShellItemGetDisplayName], uintptr(unsafe.Pointer(item)), sigdnFilesysPath, uintptr(unsafe.Pointer(&namePtr))); int32(hr) < 0 || namePtr == nil {
		return "", false, syscall.Errno(hr)
	}
	defer procCoTaskMemFree.Call(uintptr(unsafe.Pointer(namePtr)))

	return windows.UTF16PtrToString(namePtr), false, nil
}

// windowsDialogResultMulti pulls every chosen file-system path out of the
// IShellItemArray returned by IFileOpenDialog::GetResults after a multi-select
// Show (#716).
func windowsDialogResultMulti(dialog *comObject) (paths []string, cancelled bool, err error) {
	var arr *comObject
	if hr, _, _ := syscall.SyscallN(dialog.vtbl[vtblGetResults], uintptr(unsafe.Pointer(dialog)), uintptr(unsafe.Pointer(&arr))); int32(hr) < 0 || arr == nil {
		return nil, false, syscall.Errno(hr)
	}
	defer comCall(arr, vtblShellArrayRelease)

	var count uint32
	if hr := comCall(arr, vtblShellArrayGetCount, uintptr(unsafe.Pointer(&count))); hr < 0 {
		return nil, false, syscall.Errno(uintptr(uint32(hr)))
	}

	paths = make([]string, 0, count)
	for i := uint32(0); i < count; i++ {
		var item *comObject
		if hr := comCall(arr, vtblShellArrayGetItem, uintptr(i), uintptr(unsafe.Pointer(&item))); hr < 0 || item == nil {
			continue
		}
		var namePtr *uint16
		hr, _, _ := syscall.SyscallN(item.vtbl[vtblShellItemGetDisplayName], uintptr(unsafe.Pointer(item)), sigdnFilesysPath, uintptr(unsafe.Pointer(&namePtr)))
		if int32(hr) >= 0 && namePtr != nil {
			paths = append(paths, windows.UTF16PtrToString(namePtr))
			procCoTaskMemFree.Call(uintptr(unsafe.Pointer(namePtr)))
		}
		comCall(item, vtblShellItemRelease)
	}
	return paths, false, nil
}

// comCall invokes a COM method by vtable slot with the interface pointer as the
// implicit `this` first argument.
func comCall(obj *comObject, slot int, args ...uintptr) int32 {
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, uintptr(unsafe.Pointer(obj)))
	all = append(all, args...)
	hr, _, _ := syscall.SyscallN(obj.vtbl[slot], all...)
	return int32(hr)
}

// deliverWindowsResult wraps the chosen path in the appropriate storage handle
// and invokes the dialog's typed callback on the main goroutine.
func deliverWindowsResult(d *FileDialog, kind panelKind, paths []string, cancelled bool) {
	first := ""
	if len(paths) > 0 {
		first = paths[0]
	}

	switch {
	case kind == panelOpenFolder:
		cb := d.callback.(func(bellui.ListableURI, error))
		if cancelled || first == "" {
			bellui.Do(func() { d.notifyClosed(false); cb(nil, nil) })
			return
		}
		lister, err := storage.ListerForURI(storage.NewFileURI(first))
		bellui.Do(func() { d.notifyClosed(true); cb(lister, err) })

	case kind == panelSaveFile:
		cb := d.callback.(func(bellui.URIWriteCloser, error))
		if cancelled || first == "" {
			bellui.Do(func() { d.notifyClosed(false); cb(nil, nil) })
			return
		}
		writer, err := storage.Writer(storage.NewFileURI(first))
		bellui.Do(func() { d.notifyClosed(true); cb(writer, err) })

	case d.multiSelect:
		cb := d.callback.(func([]bellui.URIReadCloser, error))
		if cancelled || len(paths) == 0 {
			bellui.Do(func() { d.notifyClosed(false); cb(nil, nil) })
			return
		}
		readers, err := windowsReadersForPaths(paths)
		bellui.Do(func() { d.notifyClosed(true); cb(readers, err) })

	default:
		cb := d.callback.(func(bellui.URIReadCloser, error))
		if cancelled || first == "" {
			bellui.Do(func() { d.notifyClosed(false); cb(nil, nil) })
			return
		}
		reader, err := storage.Reader(storage.NewFileURI(first))
		bellui.Do(func() { d.notifyClosed(true); cb(reader, err) })
	}
}

// windowsReadersForPaths opens a reader per chosen file, stopping at the first
// error so the multi-select callback reports it rather than a partial set.
func windowsReadersForPaths(paths []string) ([]bellui.URIReadCloser, error) {
	readers := make([]bellui.URIReadCloser, 0, len(paths))
	for _, path := range paths {
		reader, err := storage.Reader(storage.NewFileURI(path))
		if err != nil {
			return nil, err
		}
		readers = append(readers, reader)
	}
	return readers, nil
}

// dialogTitleWindows mirrors the canvas dialog's localized heading composition
// (file.go makeUI): a custom titleText wins, else "<confirm> File/Folder".
func dialogTitleWindows(d *FileDialog, kind panelKind) string {
	if d.titleText != "" {
		return d.titleText
	}

	label := d.confirmText
	if label == "" {
		if kind == panelSaveFile {
			label = lang.L("Save")
		} else {
			label = lang.L("Open")
		}
	}

	if kind == panelOpenFolder {
		return label + " " + lang.L("Folder")
	}
	return label + " " + lang.L("File")
}

func startingDirPathWindows(d *FileDialog) string {
	if d.startingLocation != nil && d.startingLocation.Scheme() == "file" {
		// URI.Path() yields forward slashes (e.g. "C:/Users");
		// SHCreateItemFromParsingName wants a native backslash path.
		return filepath.FromSlash(d.startingLocation.Path())
	}
	return ""
}

// filterExtensionsWindows reduces a bellui filter to a list of bare extensions
// (no leading dot). The Common Item Dialog has no MIME concept, so MIME filters
// are best-effort mapped to their registered extensions; an unmappable filter
// yields no restriction (all files), a graceful degradation.
func filterExtensionsWindows(f storage.FileFilter) []string {
	if f == nil || f == folderFilter {
		return nil
	}

	var exts []string
	switch filter := f.(type) {
	case *storage.ExtensionFileFilter:
		for _, ext := range filter.Extensions {
			exts = append(exts, strings.TrimPrefix(ext, "."))
		}
	case *storage.MimeTypeFileFilter:
		for _, m := range filter.MimeTypes {
			mapped, err := mime.ExtensionsByType(m)
			if err != nil {
				continue
			}
			for _, ext := range mapped {
				exts = append(exts, strings.TrimPrefix(ext, "."))
			}
		}
	}
	return exts
}
