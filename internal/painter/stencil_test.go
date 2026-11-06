package painter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestClipDepthCodes pins the stencil layout's properties the painters rely
// on (#1015): depth codes are distinct and in the depth bits, and a stamp's
// mask lets through only the depth below it and its own.
func TestClipDepthCodes(t *testing.T) {
	assert.Zero(t, ClipDepthCode(0), "outside every clip the stencil is zero")
	seen := map[uint32]int{}
	for d := 0; d <= MaxClipDepth+1; d++ {
		c := ClipDepthCode(d)
		assert.Zero(t, c&^StencilDepthMask, "depth %d's code leaves the depth bits", d)
		if prev, ok := seen[c]; ok {
			t.Fatalf("depths %d and %d share code %#x", prev, d, c)
		}
		seen[c] = d
	}
	for d := 1; d <= MaxClipDepth; d++ {
		m := ClipStampMask(d)
		assert.Equal(t, uint32(0x10)<<ClipStampBit(d), StencilDepthMask&^m, "depth %d", d)
		for k := 0; k <= MaxClipDepth+1; k++ {
			matches := ClipDepthCode(k)&m == ClipDepthCode(d)&m
			assert.Equal(t, k == d-1 || k == d, matches, "stamping depth %d against depth %d", d, k)
		}
	}
}
