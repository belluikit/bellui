// The two cases that list a real directory (storage.ListerForURI on
// storage.NewFileURI(".")): the browser has no filesystem to list, so they
// stay off the wasm leg. The pure-URI fileItem tests remain untagged in
// fileitem_test.go (#77's split convention, #79).
//go:build !wasm

package dialog

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/storage"
)

func TestNewFileItem_Folder(t *testing.T) {
	f := &fileDialog{file: &FileDialog{}}
	_ = f.makeUI()

	currentLister, err := storage.ListerForURI(storage.NewFileURI("."))
	assert.NoError(t, err)

	parentDir, err := storage.Parent(currentLister)
	assert.NoError(t, err)
	parentLister, err := storage.ListerForURI(parentDir)
	assert.NoError(t, err)

	f.setLocation(parentLister)
	item := f.newFileItem(currentLister, true, false)

	assert.Equal(t, currentLister.Name(), item.name)
	assert.Equal(t, currentLister.String(), item.location.String())
}

func TestNewFileItem_ParentFolder(t *testing.T) {
	f := &fileDialog{file: &FileDialog{}}
	_ = f.makeUI()
	currentLister, err := storage.ListerForURI(storage.NewFileURI("."))
	assert.NoError(t, err)

	parentDir, err := storage.Parent(currentLister)
	assert.NoError(t, err)
	f.setLocation(currentLister)

	item := f.newFileItem(parentDir, true, true)
	item.ExtendBaseWidget(item)

	assert.Equal(t, "(Parent)", item.name)
	assert.Equal(t, parentDir.String(), f.data[0].String())
}
