//go:build windows

package glfw

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	internalclipboard "github.com/belluikit/bellui/internal/clipboard"
)

// This is the Windows drag-out backend for pre-rebrand tracker #837 (pre-rebrand tracker #515 Part B — see
// docs/design/515-drag-out.md): it starts a native OLE drag-and-drop out of the
// app via DoDragDrop, carrying whichever flavors the DragData holds (file list,
// plain text, image). It is cgo-free, driving COM purely through x/sys/windows
// syscalls exactly like the native file dialog (pre-rebrand tracker #514); the COM objects it hands
// to the OS live in dragsource_windows_com.go. Runtime verification needs a real
// Windows session (drop into Explorer / Notepad / a browser); per fork convention
// COM/native code is CI- and on-box-verified.

// dropFilesHeaderSize is sizeof(DROPFILES); the CF_HDROP payload is this header
// followed by the double-NUL-terminated wide file list.
const dropFilesHeaderSize = 20

// dragSession keeps the in-flight drag's COM objects (and any enumerator clones
// the shell requests) alive and pinned for the lifetime of the blocking
// DoDragDrop call. Go's GC is non-moving today, but the OS holds raw pointers to
// these objects, so we pin them defensively (mirroring the file dialog's
// pinned-pointer hygiene, pre-rebrand tracker #514). Only one drag runs at a time — the shared layer
// holds a single OnCompleted callback — so a package-level pointer suffices; it is
// set and cleared on the dedicated drag goroutine.
type dragSession struct {
	pinner runtime.Pinner
	keep   []any // strong refs so clones created in callbacks are not collected
}

func (s *dragSession) pin(obj any) {
	s.pinner.Pin(obj)
	s.keep = append(s.keep, obj)
}

var activeDrag *dragSession

// pngFormatID is the registered "PNG" clipboard format id, matching the clipboard
// flavor writer (pre-rebrand tracker #513) so image-aware targets (browsers, editors) can pick up the
// lossless PNG in preference to CF_DIB. Resolved once, lazily.
var (
	pngFormatOnce sync.Once
	pngFormatID   uint16
)

func registeredPNGFormat() uint16 {
	pngFormatOnce.Do(func() {
		name, err := windows.UTF16PtrFromString("PNG")
		if err != nil {
			return
		}
		id, _, _ := procRegisterClipboardFmt.Call(uintptr(unsafe.Pointer(name)))
		pngFormatID = uint16(id)
	})
	return pngFormatID
}

// startNativeDrag begins an OLE drag-and-drop carrying data out of the app. It
// returns true when a drag was launched (payload present + drag goroutine
// started), false when there is nothing to drag (the caller then keeps its
// in-app behavior).
// The cursorImg drag visual is not yet applied on Windows (the custom drag image
// via IDragSourceHelper is deferred, pre-rebrand tracker #850); the image *data* flavor still comes
// from an explicit DragData.Image in buildDragEntries.
func startNativeDrag(data desktop.DragData, _ image.Image) bool {
	formats, payloads := buildDragEntries(data)
	if len(formats) == 0 {
		return false
	}

	// DoDragDrop is modal and drives the drag from the *calling thread's* mouse
	// capture — the capture established by the button-down that began this
	// gesture. That capture lives on the main/UI goroutine (bellui dispatches
	// Dragged there), so the drag MUST run there: on any other thread DoDragDrop
	// has no capture and silently does nothing (no drag, cursor unchanged, no
	// drop). It blocks until the drop completes but pumps its own modal message
	// loop, so the window keeps repainting during the drag.
	runDragDrop(formats, payloads, data.OnCompleted)
	return true
}

// runDragDrop runs the OLE drag synchronously on the calling (main/UI) goroutine
// — see startNativeDrag for why it cannot be offloaded. When DoDragDrop returns,
// the outcome (onCompleted) is delivered on the main goroutine via bellui.Do.
func runDragDrop(formats []formatEtc, payloads [][]byte, onCompleted func(accepted bool)) {
	sess := &dragSession{}
	activeDrag = sess
	defer func() {
		activeDrag = nil
		sess.pinner.Unpin()
		runtime.KeepAlive(sess)
	}()

	// The main UI goroutine is locked to the process's initial OS thread, which
	// GLFW runs as an STA apartment. Ensure OLE is initialized on it, but do NOT
	// balance it with OleUninitialize — this is the shared main thread whose COM
	// lifetime GLFW owns; tearing it down would break the driver.
	procOleInitialize.Call(0)

	dataObj := newDataObject(formats, payloads)
	dropSrc := newDropSource()

	var effect uint32
	procDoDragDrop.Call(
		uintptr(unsafe.Pointer(dataObj)),
		uintptr(unsafe.Pointer(dropSrc)),
		dropEffectCopy|dropEffectMove,
		uintptr(unsafe.Pointer(&effect)),
	)
	// Keep the COM objects alive across the whole blocking call.
	runtime.KeepAlive(dataObj)
	runtime.KeepAlive(dropSrc)

	// A non-NONE effect means a target accepted the drop (a copy or move).
	// Deliver the outcome on the main goroutine via bellui.Do (single-shot).
	if onCompleted != nil {
		accepted := uintptr(effect) != dropEffectNone
		bellui.Do(func() { onCompleted(accepted) })
	}
}

// buildDragEntries assembles the parallel format/payload lists for every flavor
// present in data, in target-preference order: file list first (so file managers
// see a drop), then the image (lossless PNG before CF_DIB), then plain text.
func buildDragEntries(data desktop.DragData) (formats []formatEtc, payloads [][]byte) {
	add := func(cf uint16, payload []byte) {
		if cf == 0 || len(payload) == 0 {
			return
		}
		formats = append(formats, formatEtc{
			cfFormat: cf,
			ptd:      0,
			dwAspect: dvAspectContent,
			lindex:   -1,
			tymed:    tymedHGlobal,
		})
		payloads = append(payloads, payload)
	}

	if paths := filePaths(data.URIs); len(paths) > 0 {
		add(cfHDrop, buildDropFiles(paths))
	}
	if data.Image != nil {
		add(registeredPNGFormat(), encodePNG(data.Image))
		add(cfDIB, internalclipboard.EncodeDIB(data.Image))
	}
	if data.Text != "" {
		add(cfUnicodeText, utf16Bytes(data.Text))
	}
	return formats, payloads
}

// buildDropFiles constructs a CF_HDROP payload: a DROPFILES header (Unicode,
// origin at 0,0) followed by the wide, double-NUL-terminated file list. Forward
// slashes from URI.Path() are converted to native backslashes so Explorer
// resolves the paths.
func buildDropFiles(paths []string) []byte {
	var body []byte
	for _, p := range paths {
		u, err := windows.UTF16FromString(windowsPath(p))
		if err != nil {
			continue
		}
		body = appendUTF16(body, u) // each entry keeps its own terminating NUL
	}
	if len(body) == 0 {
		return nil
	}
	body = append(body, 0, 0) // extra NUL terminates the whole list

	buf := make([]byte, dropFilesHeaderSize+len(body))
	binary.LittleEndian.PutUint32(buf[0:], dropFilesHeaderSize) // pFiles: list offset
	// pt (x,y at offsets 4,8) and fNC (12) stay zero.
	binary.LittleEndian.PutUint32(buf[16:], 1) // fWide = TRUE (Unicode paths)
	copy(buf[dropFilesHeaderSize:], body)
	return buf
}

// windowsPath converts a URI-style forward-slash path (e.g. "C:/Users/x.txt") to
// a native backslash path.
func windowsPath(p string) string {
	b := []byte(p)
	for i := range b {
		if b[i] == '/' {
			b[i] = '\\'
		}
	}
	return string(b)
}

// utf16Bytes encodes s as a NUL-terminated UTF-16LE byte buffer for the
// CF_UNICODETEXT flavor.
func utf16Bytes(s string) []byte {
	u, err := windows.UTF16FromString(s) // includes the terminating NUL
	if err != nil {
		return nil
	}
	return appendUTF16(nil, u)
}

// appendUTF16 appends the little-endian bytes of a UTF-16 code-unit slice.
func appendUTF16(dst []byte, u []uint16) []byte {
	var tmp [2]byte
	for _, v := range u {
		binary.LittleEndian.PutUint16(tmp[:], v)
		dst = append(dst, tmp[0], tmp[1])
	}
	return dst
}

// encodePNG renders an image to PNG bytes for the registered "PNG" flavor, or nil.
func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

// Drag-image follow-up (pre-rebrand tracker #837): DoDragDrop currently shows only the default drag
// cursors — the custom drag image (data.Image / DragImageObject) is not yet handed
// to the OS. Supplying it needs a further COM interface (IDragSourceHelper::
// InitializeFromBitmap) plus an HBITMAP built from the image; that surface is
// deferred so the load-bearing payload flavors land first. The macOS backend
// already supplies a drag image, so this is a Windows-only parity gap.
