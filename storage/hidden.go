package storage

import "github.com/belluikit/bellui"

// IsHidden reports whether the item the given URI references is considered
// hidden by the platform's file-browsing convention — the same rule the file
// dialog's "show hidden files" toggle applies. On Unix-like systems a "file"
// URI is hidden when its name starts with a dot, on Windows when it carries
// the hidden file attribute. Non-"file" URIs, and every URI on mobile and
// web targets, report false.
//
// Since: 2.15
func IsHidden(u bellui.URI) bool {
	return isHidden(u)
}
