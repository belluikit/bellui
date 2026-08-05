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
	if !nativeFileDialogsEnabled() {
		// UseNativeFileDialogs(false) forces the canvas dialog on mobile too
		// (#267): the doc promises "everywhere", and the canvas dialog
		// browses something real here (app-scoped storage) — unlike wasm,
		// which stays picker-always. The cost is the app's to weigh: the
		// platform pickers are the only surfaces that reach outside the
		// sandbox on modern Android/iOS.
		return false
	}
	if f.multiSelect {
		// iOS allowsMultipleSelection / Android EXTRA_ALLOW_MULTIPLE (#255);
		// an app without the multi picker falls back to the canvas dialog.
		cb := f.callback.(func([]bellui.URIReadCloser, error))
		return mobile.ShowFileOpenMultiplePicker(func(readers []bellui.URIReadCloser, err error) {
			bellui.Do(func() { f.notifyClosed(readers != nil || err != nil); cb(readers, err) })
		}, f.filter)
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
	if !nativeFileDialogsEnabled() {
		return false // see fileOpenOSOverride (#267)
	}
	cb := f.callback.(func(bellui.URIWriteCloser, error))
	return mobile.ShowFileSavePicker(func(writer bellui.URIWriteCloser, err error) {
		bellui.Do(func() { f.notifyClosed(writer != nil || err != nil); cb(writer, err) })
	}, f.filter, f.initialFileName)
}

func getFavoriteLocation(homeURI bellui.URI, name string) (bellui.URI, error) {
	return storage.Child(homeURI, name)
}
