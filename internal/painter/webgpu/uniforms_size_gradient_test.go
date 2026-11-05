package webgpu

import (
	"testing"
	"unsafe"
)

// TestGradientUniformsFitABlock pins #956: a block over uniformBlockStride is
// refused by record, and the draw is dropped without a word.
func TestGradientUniformsFitABlock(t *testing.T) {
	if n := unsafe.Sizeof(GradientUniforms{}); n > uniformBlockStride {
		t.Fatalf("GradientUniforms is %d bytes; a uniform block may be at most %d", n, uniformBlockStride)
	}
}
