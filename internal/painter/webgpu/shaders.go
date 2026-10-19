package webgpu

import "embed"

// shaderFS holds the WGSL shader sources. Unlike the GL backend (which keeps a
// desktop/_es GLSL pair per shader and selects at build time), the WebGPU
// backend has a single WGSL module per shader, so the same files are embedded
// on every platform.
//
//go:embed shaders/*.wgsl
var shaderFS embed.FS

// Entry-point naming convention shared by every WGSL module (see doc.go).
const (
	// VertexEntry is the vertex entry point present in every shader module.
	VertexEntry = "vs_main"
	// FragmentEntry is the fragment entry point present in every shader module
	// except blur, which exposes BlurFragmentEntryH / BlurFragmentEntryV.
	FragmentEntry = "fs_main"
	// BlurFragmentEntryH is the horizontal-pass fragment entry of the blur shader.
	BlurFragmentEntryH = "fs_main_h"
	// BlurFragmentEntryV is the vertical-pass fragment entry of the blur shader.
	BlurFragmentEntryV = "fs_main_v"
)

// shaderNames is the set of shader modules available, in a stable order.
var shaderNames = []string{
	"simple",
	"rectangle",
	"round_rectangle",
	"line",
	"polygon",
	"path",
	"arc",
	"ripple",
	"blur",
	"scene3d",
	"quad",
}

// Names returns the names of the available WGSL shader modules.
func Names() []string {
	out := make([]string, len(shaderNames))
	copy(out, shaderNames)
	return out
}

// ShaderSource returns the WGSL source for the named shader module (e.g.
// "round_rectangle"). It returns nil if the name is unknown. The returned
// module contains both the vertex (VertexEntry) and fragment (FragmentEntry,
// or the blur pair) entry points.
func ShaderSource(name string) []byte {
	data, err := shaderFS.ReadFile("shaders/" + name + ".wgsl")
	if err != nil {
		return nil
	}
	return data
}
