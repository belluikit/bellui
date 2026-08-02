// On wasm a relative image src resolves against the page origin as an http
// URI (the browser has no file paths) — the documented #73 divergence — so
// this native-resolution assertion is !wasm. A wasm counterpart asserting
// the origin-resolved URI would be real added coverage (see #79).
//go:build !wasm

package widget

import (
	"testing"

	"github.com/belluikit/bellui/storage"
	"github.com/stretchr/testify/assert"
)

func TestRichTextHTML_Image(t *testing.T) {
	// A schemeless src must NOT become a file:// URI — that fallback let
	// untrusted markup read local files (#285). The alt text is still kept.
	r := NewRichTextFromHTML(`<img src="../../theme/testdata/bellui.png" alt="icon">`)

	found := false
	for _, seg := range r.Segments {
		if img, ok := seg.(*ImageSegment); ok {
			assert.Nil(t, img.Source)
			assert.Equal(t, "icon", img.Title)
			found = true
			break
		}
	}
	assert.True(t, found, "Should find image segment")

	// An explicit file:// URI is still honoured.
	r = NewRichTextFromHTML(`<img src="file:///tmp/bellui.png" alt="icon">`)
	found = false
	for _, seg := range r.Segments {
		if img, ok := seg.(*ImageSegment); ok {
			assert.Equal(t, storage.NewFileURI("/tmp/bellui.png"), img.Source)
			found = true
			break
		}
	}
	assert.True(t, found, "Should find image segment")
}
