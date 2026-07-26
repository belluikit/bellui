package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// newMultiSizeRichText builds a RichText whose single row is two inline
// TextSegments of different sizes (body + large title), so the row's true width
// differs from any single-style measurement — the condition issue #42 is about.
func newMultiSizeRichText(t *testing.T) *RichText {
	t.Helper()
	small := &TextSegment{Text: "abc ", Style: RichTextStyle{Inline: true, SizeName: theme.SizeNameBodyMedium}}
	big := &TextSegment{Text: "XYZ", Style: RichTextStyle{Inline: true, SizeName: theme.SizeNameHeadlineLarge}}
	rich := NewRichText(small, big)
	rich.Resize(geo.NewSize(600, 80))
	rich.CreateRenderer()
	test.TempWidgetRenderer(t, rich) // force a layout pass so rowBounds exist
	return rich
}

// TestSelectable_AlignOffsetMultiSegment verifies that the alignment offset for a
// center/trailing-aligned row is derived from the per-segment measured width, not
// a single-style measurement that drifts when segments differ in size (#42).
func TestSelectable_AlignOffsetMultiSegment(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rich := newMultiSizeRichText(t)
	require.Len(t, rich.rowBoundary(0).segments, 2, "both inline segments must share a row")

	th := rich.Theme()
	textSize := th.Size(theme.SizeNameText)
	innerPad := th.Size(theme.SizeNameInnerPadding)
	containerWidth := float32(600)

	// Exact per-segment width: this is what the renderer actually lays out.
	wantWidth := bellui.MeasureText("abc ", th.Size(theme.SizeNameBodyMedium), bellui.TextStyle{}).Width +
		bellui.MeasureText("XYZ", th.Size(theme.SizeNameHeadlineLarge), bellui.TextStyle{}).Width
	assert.InDelta(t, wantWidth, rich.rowWidth(0, textSize), 0.01,
		"rowWidth must sum each segment's own-style measurement")

	// The naive single-style measurement (the pre-#42 behaviour) is smaller here,
	// because the large segment is measured at the default size.
	naiveWidth := bellui.MeasureText(string(rich.row(0)), textSize, bellui.TextStyle{}).Width
	require.Greater(t, wantWidth, naiveWidth+1,
		"the differing-size row must expose a real width gap to test against")

	for _, tc := range []struct {
		name  string
		align bellui.TextAlign
		want  float32
	}{
		{"trailing", bellui.TextAlignTrailing, containerWidth - (wantWidth + 2*innerPad)},
		{"center", bellui.TextAlignCenter, (containerWidth - (wantWidth + 2*innerPad)) / 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &selectable{theme: th, provider: rich, alignment: tc.align}
			s.Resize(geo.NewSize(containerWidth, 80))

			got := s.alignOffsetX(0)
			assert.InDelta(t, tc.want, got, 0.01,
				"offset must use the per-segment width")

			// And it must be measurably different from the old single-style offset.
			naive := containerWidth - (naiveWidth + 2*innerPad)
			if tc.align == bellui.TextAlignCenter {
				naive /= 2
			}
			assert.Greater(t, naive-got, float32(0.5),
				"the fix must shift the offset away from the single-style result")
		})
	}
}

// TestSelectable_AlignOffsetLeadingZero confirms leading alignment still returns a
// zero offset (the default path is unaffected by #42).
func TestSelectable_AlignOffsetLeadingZero(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	rich := newMultiSizeRichText(t)
	s := &selectable{theme: rich.Theme(), provider: rich, alignment: bellui.TextAlignLeading}
	s.Resize(geo.NewSize(600, 80))
	assert.Equal(t, float32(0), s.alignOffsetX(0))
}
