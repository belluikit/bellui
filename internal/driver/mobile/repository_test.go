//go:build !wasm

package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/storage/repository"
)

func TestFileRepositoryChild(t *testing.T) {
	f := &mobileFileRepo{}
	repository.Register("file", f)

	p, _ := storage.Child(storage.NewFileURI("/foo/bar"), "baz")
	assert.Equal(t, "file:///foo/bar/baz", p.String())

	p, _ = storage.Child(storage.NewFileURI("/foo/bar/"), "baz")
	assert.Equal(t, "file:///foo/bar/baz", p.String())

	uri, _ := storage.ParseURI("content://thing")
	p, err := storage.Child(uri, "new")
	assert.NotNil(t, err)
	assert.Nil(t, p)
}

func TestFileRepositoryParent(t *testing.T) {
	f := &mobileFileRepo{}
	repository.Register("file", f)

	p, _ := storage.Parent(storage.NewFileURI("/foo/bar"))
	assert.Equal(t, "file:///foo", p.String())

	p, _ = storage.Parent(storage.NewFileURI("/foo/bar/"))
	assert.Equal(t, "file:///foo", p.String())

	uri, _ := storage.ParseURI("content://thing")
	p, err := storage.Parent(uri)
	assert.NotNil(t, err)
	assert.Nil(t, p)
}
