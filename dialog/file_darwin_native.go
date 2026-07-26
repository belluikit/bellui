//go:build darwin && !ios && !wasm && !js

package dialog

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework AppKit -framework UniformTypeIdentifiers
#include <stdlib.h>
#include "file_darwin_native.h"
*/
import "C"

import (
	"mime"
	"strings"
	"sync"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/storage"
)

// panelKind records which native panel was shown so the response handler can
// invoke the correctly-typed callback stored on the FileDialog.
type panelKind int

const (
	panelOpenFile panelKind = iota
	panelOpenFolder
	panelSaveFile
)

// pendingPanel keeps the Go-side state alive while the native NSOpenPanel /
// NSSavePanel is on screen. It is keyed by an integer handle passed to the
// ObjC bridge and looked up again from the completion handler.
type pendingPanel struct {
	dialog *FileDialog
	kind   panelKind
}

var (
	panelMu      sync.Mutex
	panelNext    int
	panelPending = map[int]*pendingPanel{}
)

func registerPanel(p *pendingPanel) int {
	panelMu.Lock()
	defer panelMu.Unlock()
	panelNext++
	handle := panelNext
	panelPending[handle] = p
	return handle
}

func takePanel(handle int) *pendingPanel {
	panelMu.Lock()
	defer panelMu.Unlock()
	p := panelPending[handle]
	delete(panelPending, handle)
	return p
}

func fileOpenOSOverride(d *FileDialog) bool {
	if !nativeFileDialogsEnabled() {
		return false
	}

	kind := panelOpenFile
	if d.isDirectory() {
		kind = panelOpenFolder
	}
	// Multi-select applies to file open only; a folder picker stays single.
	showNativePanel(d, kind, d.multiSelect && kind == panelOpenFile)
	return true
}

func fileSaveOSOverride(d *FileDialog) bool {
	if !nativeFileDialogsEnabled() {
		return false
	}

	showNativePanel(d, panelSaveFile, false)
	return true
}

// showNativePanel builds the C parameters from the dialog state and hands off
// to the ObjC bridge. The panel is presented and driven entirely on the main
// thread; the completion handler calls back into belluiFileDialogResponse.
func showNativePanel(d *FileDialog, kind panelKind, multi bool) {
	handle := registerPanel(&pendingPanel{dialog: d, kind: kind})

	var nsWindow uintptr
	if native, ok := d.parent.(driver.NativeWindow); ok {
		native.RunNative(func(context any) {
			if mac, ok := context.(driver.MacWindowContext); ok {
				nsWindow = mac.NSWindow
			}
		})
	}

	title := C.CString(dialogTitle(d, kind))
	defer C.free(unsafe.Pointer(title))

	prompt := C.CString(d.confirmText)
	defer C.free(unsafe.Pointer(prompt))

	initialDir := C.CString(startingDirPath(d))
	defer C.free(unsafe.Pointer(initialDir))

	initialName := C.CString(d.initialFileName)
	defer C.free(unsafe.Pointer(initialName))

	exts, mimes := filterForNativePanel(d.filter)
	cExts, extCount := newCStringArray(exts)
	defer freeCStringArray(cExts, extCount)
	cMimes, mimeCount := newCStringArray(mimes)
	defer freeCStringArray(cMimes, mimeCount)

	// On macOS < 11 the panel can only restrict by bare extension, so MIME
	// filters are pre-mapped to extensions here (best-effort, mirroring the
	// Windows backend) and combined with the extension list for the legacy
	// setAllowedFileTypes: path (#632). The 11+ path ignores this and uses the
	// exts/mimes UTType lists above.
	legacyExts := append(append([]string(nil), exts...), legacyExtsFromMimes(mimes)...)
	cLegacy, legacyCount := newCStringArray(legacyExts)
	defer freeCStringArray(cLegacy, legacyCount)

	C.belluiShowFilePanel(
		C.uintptr_t(nsWindow),
		C.int(boolToC(kind == panelSaveFile)),
		C.int(boolToC(kind == panelOpenFolder)),
		C.int(boolToC(multi)),
		title,
		prompt,
		initialDir,
		initialName,
		cExts, C.int(extCount),
		cMimes, C.int(mimeCount),
		cLegacy, C.int(legacyCount),
		C.int(handle),
	)
}

// dialogTitle mirrors the canvas dialog's title composition (file.go makeUI)
// so the native panel shows the same localized heading: a custom titleText
// wins, otherwise "<confirm> File" / "<confirm> Folder".
func dialogTitle(d *FileDialog, kind panelKind) string {
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

func startingDirPath(d *FileDialog) string {
	if d.startingLocation != nil && d.startingLocation.Scheme() == "file" {
		return d.startingLocation.Path()
	}
	return ""
}

// filterForNativePanel splits a bellui file filter into the extension list
// (NSOpenPanel allowedFileTypes) and the MIME list (converted to UTType by the
// ObjC side). The folder filter and unknown filters yield no restriction.
func filterForNativePanel(f storage.FileFilter) (exts, mimes []string) {
	if f == nil || f == folderFilter {
		return nil, nil
	}

	switch filter := f.(type) {
	case *storage.ExtensionFileFilter:
		for _, ext := range filter.Extensions {
			exts = append(exts, strings.TrimPrefix(ext, "."))
		}
	case *storage.MimeTypeFileFilter:
		mimes = append(mimes, filter.MimeTypes...)
	}
	return exts, mimes
}

// legacyExtsFromMimes maps MIME types to their known filename extensions for
// the pre-macOS-11 fallback, which cannot express MIME filters directly.
// Unmappable MIME types contribute nothing (the panel stays unrestricted for
// them), matching the Windows backend's graceful degradation.
func legacyExtsFromMimes(mimes []string) []string {
	var exts []string
	for _, m := range mimes {
		mapped, err := mime.ExtensionsByType(m)
		if err != nil {
			continue
		}
		for _, ext := range mapped {
			exts = append(exts, strings.TrimPrefix(ext, "."))
		}
	}
	return exts
}

//export belluiFileDialogResponse
func belluiFileDialogResponse(handle C.int, cPaths **C.char, count C.int, cancelled C.int) {
	p := takePanel(int(handle))
	if p == nil {
		return
	}

	var paths []string
	if n := int(count); n > 0 && cPaths != nil {
		slice := unsafe.Slice(cPaths, n)
		paths = make([]string, 0, n)
		for _, cs := range slice {
			if cs != nil {
				paths = append(paths, C.GoString(cs))
			}
		}
	}

	deliverPanelResult(p, paths, cancelled != 0)
}

// deliverPanelResult wraps the chosen path(s) in the appropriate storage handle
// and invokes the dialog's typed callback on the main goroutine. A multi-select
// open dialog delivers the whole slice to the []URIReadCloser callback (#716);
// every other panel uses the first (and only) path.
func deliverPanelResult(p *pendingPanel, paths []string, cancelled bool) {
	d := p.dialog

	first := ""
	if len(paths) > 0 {
		first = paths[0]
	}

	switch {
	case p.kind == panelOpenFolder:
		cb := d.callback.(func(bellui.ListableURI, error))
		if cancelled || first == "" {
			bellui.Do(func() { cb(nil, nil) })
			return
		}
		lister, err := storage.ListerForURI(storage.NewFileURI(first))
		bellui.Do(func() { cb(lister, err) })

	case p.kind == panelSaveFile:
		cb := d.callback.(func(bellui.URIWriteCloser, error))
		if cancelled || first == "" {
			bellui.Do(func() { cb(nil, nil) })
			return
		}
		writer, err := storage.Writer(storage.NewFileURI(first))
		bellui.Do(func() { cb(writer, err) })

	case d.multiSelect:
		cb := d.callback.(func([]bellui.URIReadCloser, error))
		if cancelled || len(paths) == 0 {
			bellui.Do(func() { cb(nil, nil) })
			return
		}
		readers, err := readersForPaths(paths)
		bellui.Do(func() { cb(readers, err) })

	default:
		cb := d.callback.(func(bellui.URIReadCloser, error))
		if cancelled || first == "" {
			bellui.Do(func() { cb(nil, nil) })
			return
		}
		reader, err := storage.Reader(storage.NewFileURI(first))
		bellui.Do(func() { cb(reader, err) })
	}
}

// readersForPaths opens a reader for each chosen file, stopping at the first
// error so the callback reports it rather than a partial set.
func readersForPaths(paths []string) ([]bellui.URIReadCloser, error) {
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

func boolToC(b bool) int {
	if b {
		return 1
	}
	return 0
}

func newCStringArray(values []string) (**C.char, int) {
	if len(values) == 0 {
		return nil, 0
	}

	size := C.size_t(len(values)) * C.size_t(unsafe.Sizeof(uintptr(0)))
	arr := (**C.char)(C.malloc(size))
	slice := unsafe.Slice(arr, len(values))
	for i, v := range values {
		slice[i] = C.CString(v)
	}
	return arr, len(values)
}

func freeCStringArray(arr **C.char, count int) {
	if arr == nil {
		return
	}
	slice := unsafe.Slice(arr, count)
	for _, s := range slice {
		C.free(unsafe.Pointer(s))
	}
	C.free(unsafe.Pointer(arr))
}
