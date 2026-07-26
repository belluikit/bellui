//go:build windows

package dialog

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/storage"
)

func TestFilterExtensionsWindows(t *testing.T) {
	// Nil and the folder sentinel impose no restriction.
	assert.Nil(t, filterExtensionsWindows(nil))
	assert.Nil(t, filterExtensionsWindows(folderFilter))

	// Extension filters lose their leading dot (COMDLG_FILTERSPEC patterns are
	// built as "*.<ext>").
	assert.Equal(t, []string{"png", "JPG"},
		filterExtensionsWindows(storage.NewExtensionFileFilter([]string{".png", ".JPG"})))

	// MIME filters are best-effort mapped to registered extensions; image/png
	// resolves to at least .png.
	exts := filterExtensionsWindows(storage.NewMimeTypeFileFilter([]string{"image/png"}))
	assert.Contains(t, exts, "png")
}

func TestDialogTitleWindows(t *testing.T) {
	// Default titles compose "<confirm> File" / "<confirm> Folder".
	d := &FileDialog{}
	assert.Equal(t, "Open File", dialogTitleWindows(d, panelOpenFile))
	assert.Equal(t, "Open Folder", dialogTitleWindows(d, panelOpenFolder))
	assert.Equal(t, "Save File", dialogTitleWindows(d, panelSaveFile))

	// A custom confirm label replaces the verb.
	d = &FileDialog{confirmText: "Choose"}
	assert.Equal(t, "Choose File", dialogTitleWindows(d, panelOpenFile))

	// A custom title wins outright.
	d = &FileDialog{titleText: "Pick an image", confirmText: "Choose"}
	assert.Equal(t, "Pick an image", dialogTitleWindows(d, panelOpenFile))
}

func TestStartingDirPathWindows(t *testing.T) {
	// No starting location yields an empty path (dialog opens at its default).
	assert.Empty(t, startingDirPathWindows(&FileDialog{}))

	// A file:// starting location yields its filesystem path with native
	// backslash separators (URI.Path() uses forward slashes).
	lister, err := storage.ListerForURI(storage.NewFileURI("C:\\Users"))
	assert.NoError(t, err)
	d := &FileDialog{startingLocation: lister}
	assert.Equal(t, "C:\\Users", startingDirPathWindows(d))
}
