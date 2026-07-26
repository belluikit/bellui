// Package storage provides storage access and management functionality.
package storage

import (
	"errors"

	"github.com/belluikit/bellui"
)

// ListerForURI will attempt to use the application's driver to convert a
// standard URI into a listable URI.
//
// Since: 1.4
func ListerForURI(uri bellui.URI) (bellui.ListableURI, error) {
	listable, err := CanList(uri)
	if err != nil {
		return nil, err
	}
	if !listable {
		return nil, errors.New("uri is not listable")
	}

	return &legacyListable{uri}, nil
}

type legacyListable struct {
	bellui.URI
}

func (l *legacyListable) List() ([]bellui.URI, error) {
	return List(l.URI)
}
