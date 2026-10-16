package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/test"
)

// TestRichText_AllowRemoteContentGate verifies that an image segment is only
// fetched when the embedding RichText opts into remote content; otherwise it
// renders a broken-image placeholder and touches neither the network nor the
// disk (pre-rebrand tracker #285).
func TestRichText_AllowRemoteContentGate(t *testing.T) {
	test.NewTempApp(t)

	fileURI := storage.NewFileURI("/tmp/bellui-security-test.png")

	t.Run("blocked by default", func(t *testing.T) {
		seg := &ImageSegment{Source: fileURI}
		rt := NewRichText(seg)

		vis := rt.cachedSegmentVisual(seg, 0).(*richImage)
		assert.False(t, seg.allowRemote)
		assert.Empty(t, vis.img.File, "must not reference a file when not allowed")
		assert.NotNil(t, vis.img.Resource, "should fall back to placeholder resource")
	})

	t.Run("loaded when opted in", func(t *testing.T) {
		seg := &ImageSegment{Source: fileURI}
		rt := &RichText{Segments: []RichTextSegment{seg}, AllowRemoteContent: true}

		vis := rt.cachedSegmentVisual(seg, 0).(*richImage)
		assert.True(t, seg.allowRemote)
		assert.Equal(t, "/tmp/bellui-security-test.png", vis.img.File)
	})

	t.Run("nil source renders placeholder", func(t *testing.T) {
		seg := &ImageSegment{Source: nil}
		rt := &RichText{Segments: []RichTextSegment{seg}, AllowRemoteContent: true}

		vis := rt.cachedSegmentVisual(seg, 0).(*richImage)
		assert.Empty(t, vis.img.File)
		assert.NotNil(t, vis.img.Resource)
	})
}

// TestParsedLinkURL covers the link-scheme allowlist applied to links taken from
// untrusted markup (pre-rebrand tracker #285).
func TestParsedLinkURL(t *testing.T) {
	for _, tt := range []struct {
		raw     string
		allowed bool
	}{
		{"https://bellui.io/", true},
		{"http://bellui.io/", true},
		{"mailto:a@b.com", true},
		{"/relative/path", true},
		{"#anchor", true},
		{"file:///etc/passwd", false},
		{"smb://host/share", false},
		{"javascript:alert(1)", false},
		{"search-ms:query=secret", false},
		{"ms-msdt:/id", false},
	} {
		u := parsedLinkURL(tt.raw)
		if tt.allowed {
			assert.NotNil(t, u, tt.raw)
		} else {
			assert.Nil(t, u, tt.raw)
		}
	}
}
