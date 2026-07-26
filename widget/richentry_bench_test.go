package widget

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// benchmarkRichEntryType measures the per-keystroke cost of editing a large
// multi-segment RichEntry document (#671, #896, #921) — the perf gate for the
// layout-cost work. Removed so far: the O(cache × segments) visual-cache
// eviction and redundant row-model rebuilds (#671); the provider's accidental
// internal Scroll, per-row re-measurement, the dead wrap-off trailing-width
// measure, and the redundant double syncProvider per keystroke (#896); the
// O(document) row-bounds rebuild, object-list rebuild and full re-layout —
// now edit-incremental (#921), leaving allocations flat regardless of
// document size. The remaining size-dependent cost is allocation-free
// position arithmetic in the document model (segment prefix walks, the
// segment diff, the row-model scan).
func benchmarkRichEntryType(b *testing.B, lines int) {
	test.NewTempApp(b)
	segs := make([]RichTextSegment, 0, lines*2)
	for i := 0; i < lines; i++ {
		segs = append(segs, &TextSegment{Style: normalStyle(), Text: "the quick brown "})
		segs = append(segs, &TextSegment{Style: boldStyle(), Text: "fox jumps over\n"})
	}
	e := NewRichEntry()
	e.SetSegments(segs)
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	e.moveCaret(e.doc.len()/2, false)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.insertRunes([]rune{'x'})
	}
}

func BenchmarkRichEntryTypeBigDoc_100(b *testing.B)  { benchmarkRichEntryType(b, 100) }
func BenchmarkRichEntryTypeBigDoc_1000(b *testing.B) { benchmarkRichEntryType(b, 1000) }
func BenchmarkRichEntryTypeBigDoc_5000(b *testing.B) { benchmarkRichEntryType(b, 5000) }
