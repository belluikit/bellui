//go:build !wasm

package mobile

import (
	"errors"

	"github.com/belluikit/bellui"
)

type lister struct {
	bellui.URI
}

func (l *lister) List() ([]bellui.URI, error) {
	return listURI(l)
}

func listerForURI(uri bellui.URI) (bellui.ListableURI, error) {
	if !canListURI(uri) {
		return nil, errors.New("specified URI is not listable")
	}

	return &lister{uri}, nil
}
