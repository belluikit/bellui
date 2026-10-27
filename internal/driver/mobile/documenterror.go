//go:build !wasm

package mobile

import (
	"github.com/belluikit/bellui/storage"
)

// documentError is why Android would not open a document, in the words of the
// Java exception that refused it (#849), such as
// "java.io.FileNotFoundException: tfe: Cello error 2." for a Drive file while
// the device is offline. Every failure used to read "document does not exist".
// A FileNotFoundException still matches storage.ErrNotExists, so existing
// errors.Is checks keep working.
type documentError struct {
	desc     string
	notFound bool
}

func (e *documentError) Error() string { return e.desc }

func (e *documentError) Is(target error) bool {
	return e.notFound && target == storage.ErrNotExists
}

// openFailure is the error for a failed open or save: the Java exception's,
// or storage.ErrNotExists when the provider returned nothing without throwing,
// as before.
func openFailure(desc string, notFound bool) error {
	if desc == "" {
		return storage.ErrNotExists
	}
	return &documentError{desc: desc, notFound: notFound}
}
