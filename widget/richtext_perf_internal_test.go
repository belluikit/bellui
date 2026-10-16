package widget

import (
	"fmt"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
)

// Regression gate for the virtualised-RichText cold-build cost (pre-rebrand tracker #317). A
// multi-segment varHeight RichText windows its *rendering* (Scroll10k below is
// O(window)) but currently shapes every row up front in updateRowBounds to
// compute wrap boundaries — O(n) HarfBuzz, ~2s on a Pixel 7 Pro for 10k rows and
// the dominant startup-ANR margin. Build10k is the number Phase C must bring down
// (target <100ms desktop); Lazy_Build10k is the single-segment lazy path (pre-rebrand tracker #157)
// for contrast (no up-front shaping). Run:
//
//	go test -run xxx -bench RichText_VarHeight -benchmem ./widget/

// tenKMixedSegments mirrors the on-device perf harness: 10k interleaved
// heading/paragraph/emphasis/strong/plain segments (inline + block composition)
// that take the variable-height virtualisation path.
func tenKMixedSegments(n int) []RichTextSegment {
	segs := make([]RichTextSegment, 0, n)
	for i := 0; i < n; i++ {
		switch i % 5 {
		case 0:
			segs = append(segs, &TextSegment{Style: RichTextStyleHeading, Text: fmt.Sprintf("Heading %d", i)})
		case 1:
			segs = append(segs, &TextSegment{Style: RichTextStyleParagraph, Text: fmt.Sprintf("Paragraph %d — mixed multi-segment virtualised content for the 10k varHeight measurement.", i)})
		case 2:
			segs = append(segs, &TextSegment{Style: RichTextStyleEmphasis, Text: fmt.Sprintf("emphasis line %d", i)})
		case 3:
			segs = append(segs, &TextSegment{Style: RichTextStyleStrong, Text: fmt.Sprintf("strong line %d", i)})
		default:
			segs = append(segs, &TextSegment{Text: fmt.Sprintf("row %d — plain segment", i)})
		}
	}
	return segs
}

// BenchmarkRichText_VarHeight_Build10k is the regression gate for the first-paint
// build cost of a 10k mixed virtualised RichText (pre-rebrand tracker #317) — the full cold build a
// real app pays before the first frame.
func BenchmarkRichText_VarHeight_Build10k(b *testing.B) {
	test.NewTempApp(b)
	segs := tenKMixedSegments(10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rich := NewRichText(segs...)
		rich.Wrapping = bellui.TextWrapWord
		rich.Scroll = bellui.ScrollVerticalOnly
		rich.virtualize = true
		rich.Resize(geo.NewSize(400, 760))
		rich.CreateRenderer()
		rich.updateRowBounds()
		_ = rich.MinSize()
		cache.DestroyRenderer(rich)
	}
}

// BenchmarkRichText_Lazy_Build10k is the single-segment lazy path (pre-rebrand tracker #157) for the
// same row count — the contrast to the multi-segment dense build. A 10k-line plain
// document does no up-front shaping here.
func BenchmarkRichText_Lazy_Build10k(b *testing.B) {
	test.NewTempApp(b)
	var sb []byte
	for i := 0; i < 10000; i++ {
		sb = append(sb, []byte(fmt.Sprintf("row %d — single-segment lazy virtualised line of content\n", i))...)
	}
	text := string(sb)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rich := NewRichText(&TextSegment{Text: text})
		rich.Wrapping = bellui.TextWrapWord
		rich.Scroll = bellui.ScrollVerticalOnly
		rich.virtualize = true
		rich.Resize(geo.NewSize(400, 760))
		rich.CreateRenderer()
		rich.updateRowBounds()
		_ = rich.MinSize()
		cache.DestroyRenderer(rich)
	}
}

// BenchmarkRichText_VarHeight_Scroll10k measures the per-scroll-step cost once
// built — exercises computeWindowRowsVar. Confirms the scroll window is O(window)
// and not part of the pre-rebrand tracker #317 cold-build problem.
func BenchmarkRichText_VarHeight_Scroll10k(b *testing.B) {
	test.NewTempApp(b)
	rich := NewRichText(tenKMixedSegments(10000)...)
	rich.Wrapping = bellui.TextWrapWord
	rich.Scroll = bellui.ScrollVerticalOnly
	rich.virtualize = true
	rich.Resize(geo.NewSize(400, 760))
	rich.CreateRenderer()
	rich.updateRowBounds()
	total := rich.MinSize().Height
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		top := float32(i%2000) / 2000 * total
		rich.SetVisibleRange(top, 760)
		rich.updateVirtualViewport()
	}
}
