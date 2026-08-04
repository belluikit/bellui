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

func fileReaderForURI(u bellui.URI) (bellui.URIReadCloser, error) {
	file := &fileOpen{uri: u}
	read, err := nativeFileOpen(file)
	if read == nil {
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
		f, err := fileReaderForURI(nativeURI(uri))
		if f != nil {
			f.(*fileOpen).done = closer
		}
		callback(f, err)
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

func fileWriterForURI(u bellui.URI, truncate bool) (bellui.URIWriteCloser, error) {
	file := &fileSave{uri: u}
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
		f, err := fileWriterForURI(uri, true)
		if f != nil {
			f.(*fileSave).done = closer
		}
		callback(f, err)
	}, mobileFilter(filter), filename)
	return true
}
