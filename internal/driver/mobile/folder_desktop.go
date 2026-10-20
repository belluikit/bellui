//go:build !ios && !android && !wasm

package mobile

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage/repository"
)

func canListURI(bellui.URI) bool {
	// no-op as we use the internal FileRepository
	return false
}

func createListableURI(bellui.URI) error {
	// no-op as we use the internal FileRepository
	return nil
}

func listURI(bellui.URI) ([]bellui.URI, error) {
	// no-op as we use the internal FileRepository
	return nil, nil
}

func contentChild(bellui.URI, string) (bellui.URI, error) {
	// content:// is an Android scheme; nothing registers it here
	return nil, repository.ErrOperationNotSupported
}

func contentCreateChild(bellui.URI, string, bool) (bellui.URI, error) {
	return nil, repository.ErrOperationNotSupported
}
