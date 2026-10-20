//go:build !ios && !android && !wasm

package mobile

import (
	"io"

	"github.com/belluikit/bellui"
	intRepo "github.com/belluikit/bellui/internal/repository"
	"github.com/belluikit/bellui/storage/repository"
)

func deleteURI(_ bellui.URI) error {
	// no-op as we use the internal FileRepository
	return nil
}

func existsURI(bellui.URI) (bool, error) {
	// no-op as we use the internal FileRepository
	return false, nil
}

func nativeFileOpen(*fileOpen) (io.ReadCloser, error) {
	// no-op as we use the internal FileRepository
	return nil, nil
}

func nativeFileSave(*fileSave, bool) (io.WriteCloser, error) {
	// no-op as we use the internal FileRepository
	return nil, nil
}

func registerRepository(_ *driver) {
	repository.Register("file", intRepo.NewFileRepository())
}
