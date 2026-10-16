package storage

import "errors"

var (
	// ErrAlreadyExists may be thrown by docs. E.g., save a document twice.
	//
	// Since: 1.0
	ErrAlreadyExists = errors.New("document already exists")

	// ErrNotExists may be thrown by docs. E.g., save an unknown document.
	//
	// Since: 1.0
	ErrNotExists = errors.New("document does not exist")
)
