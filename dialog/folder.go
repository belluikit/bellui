package dialog

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
)

var folderFilter = storage.NewMimeTypeFileFilter([]string{"application/x-directory"})

// NewFolderOpen creates a file dialog allowing the user to choose a folder to
// open. The callback function will run when the dialog closes. The URI will be
// nil when the user cancels or when nothing is selected.
//
// The dialog will appear over the window specified when Show() is called.
//
// Since: 1.0
func NewFolderOpen(callback func(bellui.ListableURI, error), parent bellui.Window) *FileDialog {
	dialog := &FileDialog{}
	dialog.callback = callback
	dialog.parent = parent
	dialog.filter = folderFilter
	dialog.windowed = true
	return dialog
}

// ShowFolderOpen creates and shows a file dialog allowing the user to choose a
// folder to open. The callback function will run when the dialog closes. The
// URI will be nil when the user cancels or when nothing is selected.
//
// The dialog will appear over the window specified.
//
// Since: 1.0
func ShowFolderOpen(callback func(bellui.ListableURI, error), parent bellui.Window) {
	dialog := NewFolderOpen(callback, parent)
	if fileOpenOSOverride(dialog) {
		return
	}
	dialog.Show()
}

func (f *FileDialog) isDirectory() bool {
	return f.filter == folderFilter
}
