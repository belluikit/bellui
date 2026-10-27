//go:build !wasm

package mobile

import (
	"errors"
	"io"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/driver/mobile/app"
	"github.com/belluikit/bellui/storage"
)

type fileOpen struct {
	io.ReadCloser
	uri  bellui.URI
	done func()
}

func (f *fileOpen) URI() bellui.URI {
	return f.uri
}

// fileReaderForURI opens a native reader for u. done, which may be nil, is the
// platform resource closer for a picker-vended URI (on iOS it ends the URL's
// security scope); it must be handed over here, before nativeFileOpen builds
// the platform reader, because the reader captures it at construction — a
// later assignment to file.done never reaches the reader's Close (#255).
func fileReaderForURI(u bellui.URI, done func()) (bellui.URIReadCloser, error) {
	file := &fileOpen{uri: u, done: done}
	read, err := nativeFileOpen(file)
	if read == nil {
		if done != nil {
			done()
		}
		return nil, err
	}
	return withSeek(file, read), err
}

// withSeek finishes file around read, and lets it seek when read can, as a
// picked file on the desktop does (#848). Callers discover seeking by type
// assertion, so a reader that cannot seek must not offer it.
func withSeek(file *fileOpen, read io.ReadCloser) bellui.URIReadCloser {
	file.ReadCloser = read
	if s, ok := read.(io.Seeker); ok {
		return &seekableFileOpen{fileOpen: file, Seeker: s}
	}
	return file
}

type seekableFileOpen struct {
	*fileOpen
	io.Seeker
}

// seekWithin answers where a seek lands in a document of size bytes held in
// memory, from cur, clamping past the end as a file read would.
func seekWithin(cur, size, offset int64, whence int) (int64, error) {
	var base int64
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		base = cur
	case io.SeekEnd:
		base = size
	default:
		return cur, errors.New("seek: invalid whence")
	}
	pos := base + offset
	if pos < 0 {
		return cur, errors.New("seek: negative position")
	}
	return min(pos, size), nil
}

func mobileFilter(filter storage.FileFilter) *app.FileFilter {
	mobile := &app.FileFilter{}

	if f, ok := filter.(*storage.MimeTypeFileFilter); ok {
		mobile.MimeTypes = f.MimeTypes
	} else if f, ok := filter.(*storage.ExtensionFileFilter); ok {
		mobile.Extensions = f.Extensions
	} else if filter != nil {
		bellui.LogError("Custom filter types not supported on mobile", nil)
	}

	return mobile
}

type hasOpenPicker interface {
	ShowFileOpenPicker(func(string, func()), *app.FileFilter)
}

// ShowFileOpenPicker loads the native file open dialog and returns the chosen
// file path via the callback func. It reports false — and the callback never
// fires — when the platform app provides no picker, so callers can fall back
// to the canvas dialog instead of silently dropping the request.
func ShowFileOpenPicker(callback func(bellui.URIReadCloser, error), filter storage.FileFilter) bool {
	drv := bellui.CurrentApp().Driver().(*driver)
	a, ok := drv.app.(hasOpenPicker)
	if !ok {
		return false
	}
	a.ShowFileOpenPicker(func(uri string, closer func()) {
		if uri == "" {
			callback(nil, nil)
			return
		}
		callback(fileReaderForURI(nativeURI(uri), closer))
	}, mobileFilter(filter))
	return true
}

type hasMultiOpenPicker interface {
	ShowFileOpenMultiplePicker(func([]string, []func()), *app.FileFilter)
}

// ShowFileOpenMultiplePicker loads the native file open dialog in multi-select
// mode and returns the chosen files via the callback func. It reports false —
// and the callback never fires — when the platform app provides no
// multi-select picker, so callers can fall back to the canvas dialog instead
// of silently dropping the request. A cancelled picker calls back (nil, nil);
// a failure to open any chosen file reports the error rather than a partial
// set, matching the desktop native backends.
func ShowFileOpenMultiplePicker(callback func([]bellui.URIReadCloser, error), filter storage.FileFilter) bool {
	drv := bellui.CurrentApp().Driver().(*driver)
	a, ok := drv.app.(hasMultiOpenPicker)
	if !ok {
		return false
	}
	a.ShowFileOpenMultiplePicker(func(uris []string, closers []func()) {
		if len(uris) == 0 {
			callback(nil, nil)
			return
		}
		callback(multiReadersForURIs(uris, closers, func(uri string, closer func()) (bellui.URIReadCloser, error) {
			return fileReaderForURI(nativeURI(uri), closer)
		}))
	}, mobileFilter(filter))
	return true
}

// multiReadersForURIs opens a reader per picked URI. open follows
// fileReaderForURI's contract: it owns the per-URI closer (which may be nil),
// running it itself when it produces no reader.
func multiReadersForURIs(uris []string, closers []func(), open func(uri string, closer func()) (bellui.URIReadCloser, error)) ([]bellui.URIReadCloser, error) {
	readers := make([]bellui.URIReadCloser, 0, len(uris))
	for i, uri := range uris {
		var closer func()
		if i < len(closers) {
			closer = closers[i]
		}
		f, err := open(uri, closer)
		if f == nil {
			// Stop at the first failure: release everything already opened
			// and everything not yet reached, then report the error rather
			// than a partial set.
			for _, r := range readers {
				r.Close()
			}
			for j := i + 1; j < len(closers); j++ {
				if closers[j] != nil {
					closers[j]()
				}
			}
			return nil, err
		}
		readers = append(readers, f)
	}
	return readers, nil
}

// ShowFolderOpenPicker loads the native folder open dialog and calls back the
// chosen directory path as a ListableURI. It reports false — and the callback
// never fires — when the platform app provides no picker, so callers can fall
// back to the canvas dialog instead of silently dropping the request.
func ShowFolderOpenPicker(callback func(bellui.ListableURI, error)) bool {
	filter := storage.NewMimeTypeFileFilter([]string{"application/x-directory"})
	drv := bellui.CurrentApp().Driver().(*driver)
	a, ok := drv.app.(hasOpenPicker)
	if !ok {
		return false
	}
	a.ShowFileOpenPicker(func(path string, _ func()) {
		if path == "" {
			callback(nil, nil)
			return
		}

		if _, err := storage.ParseURI(path); err != nil {
			callback(nil, err)
			return
		}
		// nativeURI, not the parsed URI: on Android that wrapper is what
		// carries the content:// Name()/Extension() overrides, and without it
		// Name() answers the percent-encoded document ID — "primary%3ADocs"
		// where the user expects "Docs" (#315).
		uri := nativeURI(path)

		callback(listerForURI(uri))
	}, mobileFilter(filter))
	return true
}

type fileSave struct {
	io.WriteCloser
	uri  bellui.URI
	done func()
}

func (f *fileSave) URI() bellui.URI {
	return f.uri
}

// fileWriterForURI opens a native writer for u. done, which may be nil, is
// the platform resource closer for a picker-vended URI; as with
// fileReaderForURI it must arrive before nativeFileSave builds the writer,
// which captures it at construction — and, also like the reader, it runs
// here when no writer takes ownership of it (#255).
func fileWriterForURI(u bellui.URI, truncate bool, done func()) (bellui.URIWriteCloser, error) {
	file := &fileSave{uri: u, done: done}
	write, err := nativeFileSave(file, truncate)
	if write == nil {
		if done != nil {
			done()
		}
		return nil, err
	}
	file.WriteCloser = write
	return file, err
}

type hasSavePicker interface {
	ShowFileSavePicker(func(string, func()), *app.FileFilter, string)
}

// ShowFileSavePicker loads the native file save dialog and returns the chosen
// file path via the callback func. It reports false — and the callback never
// fires — when the platform app provides no picker, so callers can fall back
// to the canvas dialog instead of silently dropping the request.
func ShowFileSavePicker(callback func(bellui.URIWriteCloser, error), filter storage.FileFilter, filename string) bool {
	drv := bellui.CurrentApp().Driver().(*driver)
	a, ok := drv.app.(hasSavePicker)
	if !ok {
		return false
	}
	a.ShowFileSavePicker(func(path string, closer func()) {
		if path == "" {
			callback(nil, nil)
			return
		}

		if _, err := storage.ParseURI(path); err != nil {
			callback(nil, err)
			return
		}
		// nativeURI, not the parsed URI: on Android that wrapper is what
		// carries the content:// Name()/Extension() overrides, and without it
		// Name() answers the percent-encoded document ID — "primary%3ADocs"
		// where the user expects "Docs" (#315).
		uri := nativeURI(path)

		// TODO: does the save dialog want to truncate by default?
		callback(fileWriterForURI(uri, true, closer))
	}, mobileFilter(filter), filename)
	return true
}
