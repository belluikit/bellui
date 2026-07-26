package canvas_test

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
)

// TestText_MinSizeMemo verifies the per-object measurement memo (#923): a
// repeated MinSize of an unchanged object is served from the memo, any change
// to a measurement input forces a fresh measurement, and a theme/font cache
// reset drops the memo so it can never serve a stale size.
func TestText_MinSizeMemo(t *testing.T) {
	test.NewTempApp(t)
	txt := canvas.NewText("hello world", color.Black)

	s1, base1 := txt.MinSizeWithBaseline()
	require.False(t, s1.IsZero(), "text should measure to a non-zero size")
	require.Positive(t, base1, "baseline should be below the top")
	entry1, gen1, ok := canvas.TextMeasureMemoForTest(txt)
	require.True(t, ok, "first measurement should populate the memo")

	// Unchanged object: memo hit — same entry, byte-identical result.
	assert.Equal(t, s1, txt.MinSize())
	s1b, base1b := txt.MinSizeWithBaseline()
	assert.Equal(t, s1, s1b)
	assert.Equal(t, base1, base1b)
	entry1b, _, _ := canvas.TextMeasureMemoForTest(txt)
	assert.Same(t, entry1, entry1b, "a memo hit must not rebuild the entry")

	// Content change on the same object (mutate-then-measure, no Refresh
	// needed): the guard forces a re-measure, never serving the old size.
	txt.Text = "hi"
	s2 := txt.MinSize()
	assert.Less(t, s2.Width, s1.Width, "shorter text must measure narrower")

	// Size change re-measures too.
	txt.TextSize *= 2
	s3 := txt.MinSize()
	assert.Greater(t, s3.Height, s2.Height)

	// Style change re-measures.
	txt.TextStyle = bellui.TextStyle{Bold: true}
	s4 := txt.MinSize()
	entry4, _, _ := canvas.TextMeasureMemoForTest(txt)
	assert.NotSame(t, entry1, entry4)
	assert.False(t, s4.IsZero())

	// A theme/font cache reset bumps the generation; the next measurement
	// rebuilds the memo in lock-step with the authoritative global cache.
	cache.ResetThemeCaches()
	_ = txt.MinSize()
	_, gen2, ok := canvas.TextMeasureMemoForTest(txt)
	require.True(t, ok)
	assert.Greater(t, gen2, gen1, "theme reset must invalidate the memo")
}

// BenchmarkTextMinSize measures the memo hit path — the per-relayout cost of
// measuring an unchanged text object (#923). Before the memo this cost
// O(len(Text)) hashing per call even on a global-cache hit.
func BenchmarkTextMinSize(b *testing.B) {
	test.NewTempApp(b)
	txt := canvas.NewText("the quick brown fox jumps over the lazy dog, wearing size fourteen text", color.Black)
	txt.MinSize() // prime

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		txt.MinSize()
	}
}
