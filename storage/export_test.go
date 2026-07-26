package storage

import "github.com/belluikit/bellui"

// NewURI builds a URI, ignoring any parse error. The public storage.NewURI was
// removed (use ParseURI, which surfaces the error); this test-only helper keeps
// the many URI-construction call sites in the storage tests concise.
func NewURI(s string) bellui.URI {
	u, _ := ParseURI(s)
	return u
}
