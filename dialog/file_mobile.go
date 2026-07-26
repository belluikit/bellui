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

func fileOpenOSOverride(f *FileDialog) bool {
	if f.multiSelect {
		return false // native multi-select not wired for mobile pickers yet (#716)
	}
	if f.isDirectory() {
		mobile.ShowFolderOpenPicker(f.callback.(func(bellui.ListableURI, error)))
	} else {
		mobile.ShowFileOpenPicker(f.callback.(func(bellui.URIReadCloser, error)), f.filter)
	}
	return true
}

func fileSaveOSOverride(f *FileDialog) bool {
	mobile.ShowFileSavePicker(f.callback.(func(bellui.URIWriteCloser, error)), f.filter, f.initialFileName)
	return true
}

func getFavoriteLocation(homeURI bellui.URI, name string) (bellui.URI, error) {
	return storage.Child(homeURI, name)
}
