package painter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/lang"
)

// boundaryOffsets of "abc مرحبا def": offset 4 is the LTR→RTL boundary (after the
// space, before the first Arabic letter) and offset 9 the RTL→LTR boundary (after
// the last Arabic letter, before the trailing space).
const affinityLine = "abc مرحبا def"

// TestCaretXAt_DownstreamMatchesCaretX confirms the downstream affinity is exactly
// the legacy forward-affinity caret, so existing callers are unchanged.
func TestCaretXAt_DownstreamMatchesCaretX(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	l := layoutFor(t, affinityLine, bellui.TextStyle{})
	for i := range l.Carets {
		assert.Equal(t, l.CaretX(i), l.CaretXAt(i, AffinityDownstream),
			"downstream affinity must equal CaretX at offset %d", i)
	}
}

// TestCaretXAt_BoundaryDiffers confirms that at a direction boundary the two
// affinities resolve to different visual x positions, while away from a boundary
// they coincide.
func TestCaretXAt_BoundaryDiffers(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	l := layoutFor(t, affinityLine, bellui.TextStyle{})

	for _, b := range []int{4, 9} {
		down := l.CaretXAt(b, AffinityDownstream)
		up := l.CaretXAt(b, AffinityUpstream)
		assert.NotEqual(t, down, up,
			"offset %d sits at a direction boundary; affinities must differ", b)
		// Upstream hugs the preceding rune's trailing edge (the seam with the LTR
		// run), which is visually left of the downstream leading edge of the RTL run.
		assert.Less(t, up, down, "upstream caret at %d should be left of downstream", b)
	}

	// Interior LTR offsets (within "abc") have a single position.
	for _, i := range []int{1, 2, 3} {
		assert.Equal(t, l.CaretXAt(i, AffinityDownstream), l.CaretXAt(i, AffinityUpstream),
			"non-boundary offset %d must have one position", i)
	}
}

// TestCaretXAt_Endpoints confirms the start/end offsets fold onto their single
// defined side regardless of the affinity asked for.
func TestCaretXAt_Endpoints(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	l := layoutFor(t, affinityLine, bellui.TextStyle{})
	last := len(l.Carets) - 1

	assert.Equal(t, l.CaretXAt(0, AffinityDownstream), l.CaretXAt(0, AffinityUpstream),
		"line start has no upstream rune")
	assert.Equal(t, l.CaretXAt(last, AffinityDownstream), l.CaretXAt(last, AffinityUpstream),
		"line end has no downstream rune")
}

// TestHitTestAffinity_ResolvesClickedSide confirms a click landing on either
// visual side of a boundary offset reports that offset with the matching affinity.
func TestHitTestAffinity_ResolvesClickedSide(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	l := layoutFor(t, affinityLine, bellui.TextStyle{})

	const b = 4
	upX := l.CaretXAt(b, AffinityUpstream)
	downX := l.CaretXAt(b, AffinityDownstream)

	off, aff := l.HitTestAffinity(upX)
	require.Equal(t, b, off, "click at the upstream x should land on the boundary offset")
	assert.Equal(t, AffinityUpstream, aff, "click on the seam side should report upstream affinity")

	off, aff = l.HitTestAffinity(downX)
	require.Equal(t, b, off, "click at the downstream x should land on the boundary offset")
	assert.Equal(t, AffinityDownstream, aff, "click on the RTL side should report downstream affinity")
}

// TestHitTestAffinity_LTRDownstream confirms pure-LTR text always resolves to
// downstream affinity (no boundaries), matching HitTest.
func TestHitTestAffinity_LTRDownstream(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	l := layoutFor(t, "abc def", bellui.TextStyle{})
	for i := range l.Carets {
		off, aff := l.HitTestAffinity(l.Carets[i])
		assert.Equal(t, i, off)
		assert.Equal(t, AffinityDownstream, aff)
	}
}
