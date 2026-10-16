//go:build darwin && !ios && !wasm && !js

package dialog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/test"
)

func TestFilterForNativePanel(t *testing.T) {
	// Nil and the folder sentinel impose no restriction.
	exts, mimes := filterForNativePanel(nil)
	assert.Nil(t, exts)
	assert.Nil(t, mimes)

	exts, mimes = filterForNativePanel(folderFilter)
	assert.Nil(t, exts)
	assert.Nil(t, mimes)

	// Extension filters lose their leading dot (NSOpenPanel wants bare
	// extensions / UTType filename extensions).
	exts, mimes = filterForNativePanel(storage.NewExtensionFileFilter([]string{".png", ".JPG"}))
	assert.Equal(t, []string{"png", "JPG"}, exts)
	assert.Nil(t, mimes)

	// MIME filters pass through verbatim for UTType conversion in ObjC.
	exts, mimes = filterForNativePanel(storage.NewMimeTypeFileFilter([]string{"image/png", "text/plain"}))
	assert.Nil(t, exts)
	assert.Equal(t, []string{"image/png", "text/plain"}, mimes)
}

func TestLegacyExtsFromMimes(t *testing.T) {
	// Unmappable / empty input contributes nothing.
	assert.Nil(t, legacyExtsFromMimes(nil))
	assert.Nil(t, legacyExtsFromMimes([]string{"application/x-totally-unknown"}))

	// A well-known MIME type maps to at least its canonical extension, dot
	// stripped, for the pre-macOS-11 setAllowedFileTypes: fallback (pre-rebrand tracker #632).
	got := legacyExtsFromMimes([]string{"image/png"})
	assert.Contains(t, got, "png")
	for _, ext := range got {
		assert.NotContains(t, ext, ".")
	}
}

func TestDialogTitle(t *testing.T) {
	// Default titles compose "<confirm> File" / "<confirm> Folder".
	d := &FileDialog{}
	assert.Equal(t, "Open File", dialogTitle(d, panelOpenFile))
	assert.Equal(t, "Open Folder", dialogTitle(d, panelOpenFolder))
	assert.Equal(t, "Save File", dialogTitle(d, panelSaveFile))

	// A custom confirm label replaces the verb.
	d = &FileDialog{confirmText: "Choose"}
	assert.Equal(t, "Choose File", dialogTitle(d, panelOpenFile))

	// A custom title wins outright.
	d = &FileDialog{titleText: "Pick an image", confirmText: "Choose"}
	assert.Equal(t, "Pick an image", dialogTitle(d, panelOpenFile))
}

func TestDeliverPanelResult_MultiSelect(t *testing.T) {
	test.NewTempApp(t)

	dir := t.TempDir()
	names := []string{"a.txt", "b.txt", "c.txt"}
	var paths []string
	for _, name := range names {
		p := filepath.Join(dir, name)
		assert.NoError(t, os.WriteFile(p, []byte(name), 0o600))
		paths = append(paths, p)
	}

	var got []bellui.URIReadCloser
	var gotErr error
	called := false
	d := &FileDialog{multiSelect: true}
	d.callback = func(readers []bellui.URIReadCloser, err error) {
		called = true
		got, gotErr = readers, err
	}

	deliverPanelResult(&pendingPanel{dialog: d, kind: panelOpenFile}, paths, false)

	assert.True(t, called, "the []URIReadCloser callback must fire")
	assert.NoError(t, gotErr)
	assert.Len(t, got, 3)
	assert.Equal(t, names, []string{got[0].URI().Name(), got[1].URI().Name(), got[2].URI().Name()})
	for _, r := range got {
		_ = r.Close()
	}
}

func TestDeliverPanelResult_MultiSelectCancel(t *testing.T) {
	test.NewTempApp(t)

	called := false
	var got []bellui.URIReadCloser
	d := &FileDialog{multiSelect: true}
	d.callback = func(readers []bellui.URIReadCloser, err error) {
		called, got = true, readers
	}

	deliverPanelResult(&pendingPanel{dialog: d, kind: panelOpenFile}, nil, true)

	assert.True(t, called, "cancel still reports once")
	assert.Nil(t, got)
}

func TestDeliverPanelResult_SingleSelect(t *testing.T) {
	test.NewTempApp(t)

	path := filepath.Join(t.TempDir(), "one.txt")
	assert.NoError(t, os.WriteFile(path, []byte("x"), 0o600))

	var got bellui.URIReadCloser
	d := &FileDialog{}
	d.callback = func(reader bellui.URIReadCloser, err error) {
		got = reader
	}

	// A single-select open uses the first path even if several arrive.
	deliverPanelResult(&pendingPanel{dialog: d, kind: panelOpenFile}, []string{path}, false)

	assert.NotNil(t, got)
	assert.Equal(t, "one.txt", got.URI().Name())
	_ = got.Close()
}

func TestPanelRegistry(t *testing.T) {
	d := &FileDialog{}
	p := &pendingPanel{dialog: d, kind: panelSaveFile}

	handle := registerPanel(p)
	assert.NotZero(t, handle)

	got := takePanel(handle)
	assert.Same(t, p, got)

	// A handle is single-use: a second lookup is empty.
	assert.Nil(t, takePanel(handle))

	// Unknown handles are safe.
	assert.Nil(t, takePanel(handle+1))
}
