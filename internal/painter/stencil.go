package painter

// The GPU painters share one 8-bit stencil between rounded clips and paths
// (#1015). Path fills and path clips count winding in the low bits; rounded
// clips keep their nesting depth in the high bits. A path inside a rounded clip
// then counts its winding only where the clip's depth is, and is cut by it.
// Before, both used the whole stencil, so a path was confined to itself only.
//
// A depth cannot be a count in the high bits: incrementing a stencil value
// changes its low bits, and a replace writes the same reference the test
// compares against. So depth d is stored as the Gray code of d, in which d-1
// and d differ in one bit. Stamping depth d tests every depth bit but that one
// against d-1 and replaces with d's code; only areas at d-1 (or already at d)
// match, so a clip still narrows the one around it. Erasing replaces with
// d-1's code under the same test.
const (
	// StencilWindingMask holds a path's winding count, modulo 16.
	StencilWindingMask = 0x0F
	// StencilDepthMask holds the rounded-clip depth's Gray code.
	StencilDepthMask = 0xF0
	// MaxClipDepth is the deepest rounded clip that narrows the one around it.
	// The last code is kept for RejectClipCode.
	MaxClipDepth = 14
)

// RejectClipCode is a depth code no stencil value holds: content tested
// against it is never drawn (a clip that stamps nothing).
var RejectClipCode = ClipDepthCode(MaxClipDepth + 1)

// ClipDepthCode is the stencil value, in the depth bits, of rounded-clip depth
// d (0 is outside every clip).
func ClipDepthCode(d int) uint32 {
	return uint32(d^(d>>1)) << 4 & StencilDepthMask
}

// ClipStampMask is the read mask that stamps and erases depth d: every depth
// bit but the one in which d-1 and d differ.
func ClipStampMask(d int) uint32 {
	return StencilDepthMask &^ (ClipDepthCode(d) ^ ClipDepthCode(d-1))
}

// ClipStampBit is the index, 0 to 3, of the depth bit in which d-1 and d
// differ. A pipeline bakes its read mask in, so WebGPU keeps one stamp
// pipeline per bit.
func ClipStampBit(d int) int {
	x := (ClipDepthCode(d) ^ ClipDepthCode(d-1)) >> 4
	b := 0
	for x > 1 {
		x >>= 1
		b++
	}
	return b
}
