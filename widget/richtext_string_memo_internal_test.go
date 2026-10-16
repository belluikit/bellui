package widget

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/test"
)

// Multi-segment String() memoization (pre-rebrand tracker #560): the returned object keeps its
// identity while content is unchanged, so identity-keyed consumers (the AX
// text index, pre-rebrand tracker #555) hit their caches; content changes through the supported
// mutation paths invalidate it.

func sameStringData(a, b string) bool {
	return len(a) == len(b) && unsafe.StringData(a) == unsafe.StringData(b)
}

func TestRichTextString_MultiSegmentMemo(t *testing.T) {
	test.NewTempApp(t)

	rt := NewRichText(
		&TextSegment{Text: "hello ", Style: RichTextStyleInline},
		&TextSegment{Text: "world", Style: RichTextStyleStrong},
	)
	w := test.NewTempWindow(t, rt)
	_ = w

	first := rt.String()
	assert.Equal(t, "hello world", first)
	assert.True(t, sameStringData(first, rt.String()), "unchanged content returns the same object")
	assert.True(t, sameStringData(first, rt.AccessibleLabel()), "AccessibleLabel shares the memo")
	assert.True(t, sameStringData(first, rt.AccessibleText()), "AccessibleText shares the memo")

	// A supported mutation (edit + Refresh) invalidates the memo.
	rt.Segments[1].(*TextSegment).Text = "there!"
	rt.Refresh()
	second := rt.String()
	assert.Equal(t, "hello there!", second)
	assert.True(t, sameStringData(second, rt.String()))

	// Structural changes (append a segment) are caught by the fingerprint
	// even before a Refresh runs.
	rt.Segments = append(rt.Segments, &TextSegment{Text: " again", Style: RichTextStyleInline})
	assert.Equal(t, "hello there! again", rt.String())
}

func TestRichTextString_SingleSegmentKeepsIdentity(t *testing.T) {
	// The single-segment path must stay verbatim: the lazy edit path's
	// seg.Text == lazyStr guard depends on the returned object being the
	// segment's own string (pre-rebrand tracker #380), never a memo copy.
	rt := NewRichTextWithText("stable")
	seg := rt.Segments[0].(*TextSegment)
	assert.True(t, sameStringData(seg.Text, rt.String()))
}
