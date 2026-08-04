//go:build ios || android

package dialog

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/driver/mobile"
	"github.com/belluikit/bellui/storage"
)

func (f *fileDialog) getPlaces() []favoriteItem {
	return []favoriteItem{}
}

func hideFile(filename string) error {
	return nil
}

// The pickers report through the dialog's SetOnClosed callback exactly like
// the desktop native backends (#233): notifyClosed immediately before the
// result callback, true whenever the user confirmed a choice — a cancelled
// picker delivers (nil, nil), so anything else, including a confirm whose
// resource then failed to open, is a confirmation. The picker result arrives
// on a platform thread (a JNI upcall on Android, the UIKit main thread on
// iOS), so both are queued to the main goroutine in one hop, per the fork's
// main-goroutine discipline. A platform app without a picker makes the
// mobile.Show* call report false, and the override falls back to the canvas
// dialog rather than silently dropping the request.

func fileOpenOSOverride(f *FileDialog) bool {
	if f.multiSelect {
		return false // native multi-select not wired for mobile pickers yet (#716)
	}
	if f.isDirectory() {
		cb := f.callback.(func(bellui.ListableURI, error))
		return mobile.ShowFolderOpenPicker(func(list bellui.ListableURI, err error) {
			bellui.Do(func() { f.notifyClosed(list != nil || err != nil); cb(list, err) })
		})
	}
	cb := f.callback.(func(bellui.URIReadCloser, error))
	return mobile.ShowFileOpenPicker(func(reader bellui.URIReadCloser, err error) {
		bellui.Do(func() { f.notifyClosed(reader != nil || err != nil); cb(reader, err) })
	}, f.filter)
}

func fileSaveOSOverride(f *FileDialog) bool {
	cb := f.callback.(func(bellui.URIWriteCloser, error))
	return mobile.ShowFileSavePicker(func(writer bellui.URIWriteCloser, err error) {
		bellui.Do(func() { f.notifyClosed(writer != nil || err != nil); cb(writer, err) })
	}, f.filter, f.initialFileName)
}

func getFavoriteLocation(homeURI bellui.URI, name string) (bellui.URI, error) {
	return storage.Child(homeURI, name)
}
