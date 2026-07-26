//go:build !ios && !android

package mobile

import (
	"github.com/belluikit/bellui"
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
