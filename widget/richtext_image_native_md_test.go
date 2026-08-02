// See richtext_image_native_test.go: relative image paths resolve against
// the page origin on wasm, so the native file-resolution assertion is
// !wasm. (#79 Phase B.)
//go:build !wasm

package widget

import (
	"testing"

	"github.com/belluikit/bellui/storage"
	"github.com/stretchr/testify/assert"
)

func TestRichTextMarkdown_Image(t *testing.T) {
	// A schemeless (relative or absolute path) source must NOT be turned into a
	// file:// URI — that fallback let untrusted markup read local files (#285).
	r := NewRichTextFromMarkdown("![title](../../theme/testdata/bellui.png)")

	assert.Len(t, r.Segments, 2)
	if img, ok := r.Segments[0].(*ImageSegment); ok {
		assert.Nil(t, img.Source)
	} else {
		t.Error("Segment should be a Image")
	}

	// An explicit file:// URI is still honoured.
	r = NewRichTextFromMarkdown("![](file:///tmp/bellui.png)")

	assert.Len(t, r.Segments, 2)
	if img, ok := r.Segments[0].(*ImageSegment); ok {
		assert.Equal(t, storage.NewFileURI("/tmp/bellui.png"), img.Source)
	} else {
		t.Error("Segment should be a Image")
	}

	// A remote URI is preserved on the segment (fetching is gated separately by
	// RichText.AllowRemoteContent).
	r = NewRichTextFromMarkdown("![](https://example.com/bellui.png)")

	assert.Len(t, r.Segments, 2)
	if img, ok := r.Segments[0].(*ImageSegment); ok {
		assert.Equal(t, "https", img.Source.Scheme())
	} else {
		t.Error("Segment should be a Image")
	}
}
