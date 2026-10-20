package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// TestRichTextSingleLineRows pins #492's shortcut: a single line of plain text,
// neither wrapped nor truncated, gets exactly the rows the dense build gives it,
// which is splitLines' result returned unmeasured.
func TestRichTextSingleLineRows(t *testing.T) {
	test.NewTempApp(t)
	for _, text := range []string{"", "0", "Hello", "héllo wörld", "emoji 😀 x", "tab\tseparated"} {
		for _, style := range []RichTextStyle{RichTextStyleInline, RichTextStyleParagraph, RichTextStyleBlockquote} {
			seg := &TextSegment{Text: text, Style: style}
			r := NewRichText(seg)
			r.Resize(geo.NewSize(200, 40))
			r.Refresh()
			assert.Equal(t, splitLines(seg), r.rowBounds, "text %q", text)
		}
	}

	// Two lines still take the dense build.
	seg := &TextSegment{Text: "one\ntwo", Style: RichTextStyleInline}
	r := NewRichText(seg)
	r.Resize(geo.NewSize(200, 80))
	r.Refresh()
	assert.Len(t, r.rowBounds, 2)
}
