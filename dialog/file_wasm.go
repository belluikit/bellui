//go:build wasm || js

package dialog

import (
	"bytes"
	"fmt"
	"strings"
	"syscall/js"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/storage"
)

// pickerDismissGrace is how long after the window regains focus (the picker
// closing) showBrowserOpenPicker waits for a change or cancel event before
// treating the silence as a dismissal — the fallback for browsers that never
// fire "cancel" on <input type="file">. Change events can trail the refocus,
// so this errs generous.
const pickerDismissGrace = time.Second

func (f *fileDialog) loadPlaces() []core.CanvasObject {
	return nil
}

// fileOpenOSOverride presents a browser file picker (<input type="file">) for
// the open case. Directory selection has no portable <input> equivalent, so it
// falls back to the canvas dialog (returns false) rather than doing nothing.
func fileOpenOSOverride(f *FileDialog) bool {
	if f.isDirectory() {
		return false
	}
	if f.multiSelect {
		return false // multi-select falls back to the canvas dialog on web (#716)
	}

	cb, ok := f.callback.(func(bellui.URIReadCloser, error))
	if !ok {
		return false
	}
	// Report the close through SetOnClosed exactly like the desktop native
	// backends (#233): immediately before the result callback, true whenever
	// the user confirmed a choice — a dismissed picker delivers (nil, nil), so
	// anything else, including a confirm whose file then failed to read, is a
	// confirmation. showBrowserOpenPicker already delivers on the main
	// goroutine.
	showBrowserOpenPicker(acceptForFilter(f.filter), func(reader bellui.URIReadCloser, err error) {
		f.notifyClosed(reader != nil || err != nil)
		cb(reader, err)
	})
	return true
}

// fileSaveOSOverride hands the app a writer whose contents are offered to the
// browser as a download when it is closed. The File System Access API's
// showSaveFilePicker is not universally available (absent in Firefox/Safari),
// so the download path is used for portability across Chrome + Firefox.
func fileSaveOSOverride(f *FileDialog) bool {
	cb, ok := f.callback.(func(bellui.URIWriteCloser, error))
	if !ok {
		return false
	}

	name := f.initialFileName
	if name == "" {
		name = "download"
	}
	// There is no picker-close moment here: the writer is handed out
	// immediately and the "dialog" is the download offered when it is closed.
	// Closing the writer is therefore the closest thing to confirming the
	// dialog, so that is when SetOnClosed reports (#233) — after the result
	// callback, unlike every picker-backed path, and never at all if the app
	// abandons the writer. The hop keeps the report on the main goroutine
	// whatever goroutine the app closes the writer from.
	w := newDownloadWriteCloser(name)
	w.onClose = func() {
		bellui.Do(func() { f.notifyClosed(true) })
	}
	cb(w, nil)
	return true
}

func (f *fileDialog) getPlaces() []favoriteItem {
	return []favoriteItem{}
}

func getFavoriteLocation(homeURI bellui.URI, name string) (bellui.URI, error) {
	return nil, nil
}

// acceptForFilter renders a bellui file filter as an HTML input "accept"
// attribute value. Extension filters keep their leading dot; MIME filters pass
// through verbatim. Other filters yield "" (accept anything).
func acceptForFilter(f storage.FileFilter) string {
	switch filter := f.(type) {
	case *storage.ExtensionFileFilter:
		parts := make([]string, 0, len(filter.Extensions))
		for _, e := range filter.Extensions {
			if !strings.HasPrefix(e, ".") {
				e = "." + e
			}
			parts = append(parts, e)
		}
		return strings.Join(parts, ",")
	case *storage.MimeTypeFileFilter:
		return strings.Join(filter.MimeTypes, ",")
	}
	return ""
}

// showBrowserOpenPicker creates a hidden <input type="file">, wires change and
// cancel handlers, and clicks it. The click must happen within the user
// activation of the gesture that opened the dialog for the browser to allow it.
func showBrowserOpenPicker(accept string, cb func(bellui.URIReadCloser, error)) {
	doc := js.Global().Get("document")
	input := doc.Call("createElement", "input")
	input.Set("type", "file")
	if accept != "" {
		input.Set("accept", accept)
	}
	input.Get("style").Set("display", "none")

	var changeFn, cancelFn, focusFn js.Func
	done := false
	chosen := false
	finish := func(r bellui.URIReadCloser, err error) {
		if done {
			return
		}
		done = true
		// Detach every listener before releasing its func: the dismissal
		// fallback can finish while the picker is still open, and a change or
		// cancel event arriving after that must find no listener — reaching a
		// released js.Func is also dropped, but logs a "call to released
		// function" error on the console.
		js.Global().Call("removeEventListener", "focus", focusFn)
		input.Call("removeEventListener", "change", changeFn)
		input.Call("removeEventListener", "cancel", cancelFn)
		changeFn.Release()
		cancelFn.Release()
		focusFn.Release()
		if body := doc.Get("body"); body.Truthy() {
			body.Call("removeChild", input)
		}
		bellui.Do(func() { cb(r, err) })
	}

	changeFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		chosen = true // a choice arrived; the dismissal fallback must stand down
		files := input.Get("files")
		if !files.Truthy() || files.Get("length").Int() == 0 {
			finish(nil, nil)
			return nil
		}
		file := files.Index(0)
		readBrowserFile(file, func(data []byte, err error) {
			if err != nil {
				finish(nil, err)
				return
			}
			finish(newBytesReadCloser(file.Get("name").String(), data), nil)
		})
		return nil
	})
	// Newer browsers fire "cancel" when the picker is dismissed with no file.
	cancelFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		finish(nil, nil)
		return nil
	})
	// Older browsers fire nothing at all on dismissal, so the window regaining
	// focus — the picker closing — stands in: if no change or cancel event has
	// arrived within the grace period, report the dismissal. chosen covers the
	// async read that follows a change event; done covers losing the race to a
	// real cancel event. Known costs, both only on browsers taking this
	// fallback: a picker that is not modal to the window reports a dismissal
	// if the user refocuses the page for over the grace period mid-pick, and a
	// choice whose change event trails the refocus by more than the grace
	// period is dropped — the dismissal has already been reported and finish
	// has detached the listeners.
	focusFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		time.AfterFunc(pickerDismissGrace, func() {
			if !chosen {
				finish(nil, nil)
			}
		})
		return nil
	})

	input.Call("addEventListener", "change", changeFn)
	input.Call("addEventListener", "cancel", cancelFn)
	js.Global().Call("addEventListener", "focus", focusFn, map[string]any{"once": true})
	doc.Get("body").Call("appendChild", input)
	input.Call("click")
}

// readBrowserFile reads a JS File object's bytes via its arrayBuffer() promise
// and delivers them (or an error) to cb.
func readBrowserFile(file js.Value, cb func([]byte, error)) {
	var thenFn, catchFn js.Func
	release := func() {
		thenFn.Release()
		catchFn.Release()
	}
	thenFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		arr := js.Global().Get("Uint8Array").New(args[0])
		data := make([]byte, arr.Get("length").Int())
		js.CopyBytesToGo(data, arr)
		release()
		cb(data, nil)
		return nil
	})
	catchFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		release()
		cb(nil, fmt.Errorf("failed to read file: %s", args[0].Call("toString").String()))
		return nil
	})
	file.Call("arrayBuffer").Call("then", thenFn).Call("catch", catchFn)
}

// bytesReadCloser is an in-memory bellui.URIReadCloser over a selected file's
// bytes. The URI is synthetic (the file name) — web sandboxing does not expose
// a real path — and is only an identifier; the data is already in hand.
type bytesReadCloser struct {
	*bytes.Reader
	uri bellui.URI
}

func newBytesReadCloser(name string, data []byte) *bytesReadCloser {
	return &bytesReadCloser{Reader: bytes.NewReader(data), uri: storage.NewFileURI(name)}
}

func (b *bytesReadCloser) Close() error    { return nil }
func (b *bytesReadCloser) URI() bellui.URI { return b.uri }

// downloadWriteCloser buffers writes and, on Close, offers the contents to the
// browser as a download named after the requested file. Close is idempotent:
// a deferred Close alongside an explicit one must not offer the download — or
// report the dialog close — twice.
type downloadWriteCloser struct {
	buf     bytes.Buffer
	name    string
	uri     bellui.URI
	closed  bool
	onClose func() // fired after the download is offered; reports the dialog close
}

func newDownloadWriteCloser(name string) *downloadWriteCloser {
	return &downloadWriteCloser{name: name, uri: storage.NewFileURI(name)}
}

func (d *downloadWriteCloser) Write(p []byte) (int, error) { return d.buf.Write(p) }
func (d *downloadWriteCloser) URI() bellui.URI             { return d.uri }

func (d *downloadWriteCloser) Close() error {
	if d.closed {
		return nil
	}
	d.closed = true
	triggerBrowserDownload(d.name, d.buf.Bytes())
	if d.onClose != nil {
		d.onClose()
	}
	return nil
}

// triggerBrowserDownload creates a Blob from data and clicks a transient anchor
// with a download attribute, then revokes the object URL.
func triggerBrowserDownload(name string, data []byte) {
	arr := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(arr, data)
	blob := js.Global().Get("Blob").New([]any{arr},
		map[string]any{"type": "application/octet-stream"})
	url := js.Global().Get("URL").Call("createObjectURL", blob)

	doc := js.Global().Get("document")
	a := doc.Call("createElement", "a")
	a.Set("href", url)
	a.Set("download", name)
	a.Get("style").Set("display", "none")
	doc.Get("body").Call("appendChild", a)
	a.Call("click")
	doc.Get("body").Call("removeChild", a)
	js.Global().Get("URL").Call("revokeObjectURL", url)
}
