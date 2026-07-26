package webgpu

import "testing"

// TestDescriptorsCoverShaders ensures every embedded shader has a pipeline
// descriptor and that the descriptors are internally consistent.
func TestDescriptorsCoverShaders(t *testing.T) {
	for _, name := range Names() {
		d, ok := DescriptorFor(name)
		if !ok {
			t.Errorf("shader %q has no pipeline descriptor", name)
			continue
		}
		if d.Name != name {
			t.Errorf("descriptor for %q has Name %q", name, d.Name)
		}

		// Stride must exactly hold the interleaved attributes (4-byte floats).
		sumFloats := 0
		lastEnd := 0
		for _, a := range d.Vertex.Attrs {
			if a.Format.Floats() == 0 {
				t.Errorf("%s: attr at location %d has invalid format", name, a.Location)
			}
			if a.Offset < lastEnd {
				t.Errorf("%s: attr at location %d offset %d overlaps previous (ends %d)", name, a.Location, a.Offset, lastEnd)
			}
			if a.Offset+a.Format.Size() > d.Vertex.Stride {
				t.Errorf("%s: attr at location %d (offset %d, size %d) exceeds stride %d",
					name, a.Location, a.Offset, a.Format.Size(), d.Vertex.Stride)
			}
			lastEnd = a.Offset + a.Format.Size()
			sumFloats += a.Format.Floats()
		}
		if d.Vertex.Stride != sumFloats*4 {
			t.Errorf("%s: stride %d != sum of attrs %d bytes", name, d.Vertex.Stride, sumFloats*4)
		}
	}

	if _, ok := DescriptorFor("nope"); ok {
		t.Error("DescriptorFor(unknown) should return false")
	}
}

// TestTexturedShaders checks that exactly the shaders sampling a texture are
// flagged as textured (they bind a texture+sampler at bindings 1 & 2).
func TestTexturedShaders(t *testing.T) {
	want := map[string]bool{"simple": true, "blur": true}
	for _, name := range Names() {
		d, _ := DescriptorFor(name)
		if d.Textured != want[name] {
			t.Errorf("%s: Textured = %v, want %v", name, d.Textured, want[name])
		}
	}
}

func TestVertexFormatSize(t *testing.T) {
	for f, wantFloats := range map[VertexFormat]int{Float32: 1, Float32x2: 2, Float32x3: 3, Float32x4: 4} {
		if f.Floats() != wantFloats {
			t.Errorf("format %d Floats() = %d, want %d", f, f.Floats(), wantFloats)
		}
		if f.Size() != wantFloats*4 {
			t.Errorf("format %d Size() = %d, want %d", f, f.Size(), wantFloats*4)
		}
	}
}

// TestPipelineCache verifies build-once memoization and keying.
func TestPipelineCache(t *testing.T) {
	builds := 0
	cache := NewPipelineCache(func(k PipelineKey) (string, error) {
		builds++
		return k.Shader + "/" + k.Fragment, nil
	})

	k1 := PipelineKey{Shader: "blur", Fragment: "fs_main_h", Blend: BlendPremul, Format: 1}
	k2 := PipelineKey{Shader: "blur", Fragment: "fs_main_v", Blend: BlendPremul, Format: 1}

	for i := 0; i < 3; i++ {
		if got, _ := cache.Get(k1); got != "blur/fs_main_h" {
			t.Fatalf("Get(k1) = %q", got)
		}
	}
	if builds != 1 {
		t.Errorf("k1 built %d times, want 1", builds)
	}

	// Differing fragment entry is a distinct pipeline.
	cache.Get(k2)
	if builds != 2 {
		t.Errorf("after k2: builds = %d, want 2", builds)
	}
	// Differing blend / format are distinct too.
	cache.Get(PipelineKey{Shader: "blur", Fragment: "fs_main_h", Blend: BlendOver, Format: 1})
	cache.Get(PipelineKey{Shader: "blur", Fragment: "fs_main_h", Blend: BlendPremul, Format: 2})
	if builds != 4 {
		t.Errorf("after blend/format variants: builds = %d, want 4", builds)
	}
	if cache.Len() != 4 {
		t.Errorf("cache.Len() = %d, want 4", cache.Len())
	}

	released := 0
	cache.Release(func(string) { released++ })
	if released != 4 || cache.Len() != 0 {
		t.Errorf("after Release: released=%d len=%d, want 4 and 0", released, cache.Len())
	}
}
