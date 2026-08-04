package mobile

import (
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
	file.ReadCloser = read
	return file, err
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
		readers := make([]bellui.URIReadCloser, 0, len(uris))
		for i, uri := range uris {
			var closer func()
			if i < len(closers) {
				closer = closers[i]
			}
			f, err := fileReaderForURI(nativeURI(uri), closer)
			if f == nil {
				// Stop at the first failure: release everything already
				// opened and everything not yet reached, then report the
				// error rather than a partial set.
				for _, r := range readers {
					r.Close()
				}
				for j := i + 1; j < len(closers); j++ {
					if closers[j] != nil {
						closers[j]()
					}
				}
				callback(nil, err)
				return
			}
			readers = append(readers, f)
		}
		callback(readers, nil)
	}, mobileFilter(filter))
	return true
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

		uri, err := storage.ParseURI(path)
		if err != nil {
			callback(nil, err)
			return
		}

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
// which captures it at construction (#255). nativeFileSave runs it itself on
// its own error path.
func fileWriterForURI(u bellui.URI, truncate bool, done func()) (bellui.URIWriteCloser, error) {
	file := &fileSave{uri: u, done: done}
	write, err := nativeFileSave(file, truncate)
	if write == nil {
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

		uri, err := storage.ParseURI(path)
		if err != nil {
			callback(nil, err)
			return
		}

		// TODO: does the save dialog want to truncate by default?
		callback(fileWriterForURI(uri, true, closer))
	}, mobileFilter(filter), filename)
	return true
}
