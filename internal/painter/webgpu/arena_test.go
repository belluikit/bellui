package webgpu

import "testing"

func TestArenaVertexOffsets(t *testing.T) {
	var a arena
	// Three draws of 4, 6 and 4 float32s (16, 24, 16 bytes).
	o0 := a.addVertices(make([]byte, 16))
	o1 := a.addVertices(make([]byte, 24))
	o2 := a.addVertices(make([]byte, 16))
	if o0 != 0 || o1 != 16 || o2 != 40 {
		t.Fatalf("vertex offsets = %d,%d,%d, want 0,16,40", o0, o1, o2)
	}
	if a.vtxLen() != 56 {
		t.Fatalf("vtxLen = %d, want 56", a.vtxLen())
	}
	for i, off := range []int{o0, o1, o2} {
		if off%4 != 0 {
			t.Errorf("vertex offset %d (%d) not 4-aligned", i, off)
		}
	}
}

func TestArenaUniformOffsetsAndPadding(t *testing.T) {
	var a arena
	// Uniform blocks of varying real sizes all pad to a 256-byte stride and land
	// on 256-aligned dynamic offsets.
	o0 := a.addUniform(make([]byte, 32))  // SimpleUniforms-sized
	o1 := a.addUniform(make([]byte, 144)) // RoundRectangleUniforms-sized (largest)
	o2 := a.addUniform(make([]byte, 16))  // PathUniforms-sized
	if o0 != 0 || o1 != uniformBlockStride || o2 != 2*uniformBlockStride {
		t.Fatalf("uniform offsets = %d,%d,%d, want 0,256,512", o0, o1, o2)
	}
	if a.uniLen() != 3*uniformBlockStride {
		t.Fatalf("uniLen = %d, want %d", a.uniLen(), 3*uniformBlockStride)
	}
	for i, off := range []int{o0, o1, o2} {
		if off%uniformBlockStride != 0 {
			t.Errorf("uniform offset %d (%d) not %d-aligned", i, off, uniformBlockStride)
		}
	}
}

func TestArenaUniformContentsPreserved(t *testing.T) {
	var a arena
	block := []byte{1, 2, 3, 4, 5}
	off := a.addUniform(block)
	if got := a.uni[off : off+len(block)]; string(got) != string(block) {
		t.Fatalf("uniform contents = %v, want %v", got, block)
	}
	// The remainder of the stride must be zero padding.
	for i := off + len(block); i < off+uniformBlockStride; i++ {
		if a.uni[i] != 0 {
			t.Fatalf("pad byte at %d = %d, want 0", i, a.uni[i])
		}
	}
}

func TestArenaReset(t *testing.T) {
	var a arena
	a.addVertices(make([]byte, 64))
	a.addUniform(make([]byte, 96))
	a.reset()
	if a.vtxLen() != 0 || a.uniLen() != 0 {
		t.Fatalf("after reset vtxLen=%d uniLen=%d, want 0,0", a.vtxLen(), a.uniLen())
	}
	// Reset keeps capacity, so the next frame appends from offset 0 again.
	if off := a.addVertices(make([]byte, 16)); off != 0 {
		t.Fatalf("post-reset vertex offset = %d, want 0", off)
	}
}
