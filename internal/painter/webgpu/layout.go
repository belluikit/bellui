package webgpu

// Backend-neutral descriptions of each shader's vertex buffer layout, blend
// mode and bind-group shape. These mirror the GL painter's per-program
// VertexAttribPointer / BlendFunc calls so the WebGPU painter can build
// equivalent render pipelines. They are plain data (no GPU binding types) so
// they compile everywhere and are unit-testable; the //go:build webgpu painter
// translates them into wgpu descriptors.

// VertexFormat is a vertex attribute's scalar/vector float format.
type VertexFormat int

const (
	Float32 VertexFormat = iota + 1
	Float32x2
	Float32x3
	Float32x4
)

// Floats returns the number of 32-bit floats in the format.
func (f VertexFormat) Floats() int {
	switch f {
	case Float32:
		return 1
	case Float32x2:
		return 2
	case Float32x3:
		return 3
	case Float32x4:
		return 4
	}
	return 0
}

// Size returns the byte size of the format.
func (f VertexFormat) Size() int { return f.Floats() * 4 }

// VertexAttr is one interleaved vertex attribute.
type VertexAttr struct {
	Location int          // @location in the WGSL vertex input
	Format   VertexFormat // scalar/vector float format
	Offset   int          // byte offset within the vertex
}

// VertexLayout describes one interleaved vertex buffer.
type VertexLayout struct {
	Stride int // bytes per vertex
	Attrs  []VertexAttr
}

// Topology is the primitive topology for a shader's geometry, mirroring the GL
// painter's DrawArrays mode (triangle strip for 4-vertex quads, triangle list
// for explicit triangles).
type Topology int

const (
	// TriangleList draws every 3 vertices as a separate triangle.
	TriangleList Topology = iota
	// TriangleStrip draws a connected strip (4-vertex quads).
	TriangleStrip
)

// BlendMode selects a color-target blend state, mirroring the GL BlendFunc
// pairs the painter uses.
type BlendMode int

const (
	// BlendOver is straight (non-premultiplied) alpha over: src*srcA + dst*(1-srcA).
	BlendOver BlendMode = iota
	// BlendPremul is premultiplied alpha over: src*1 + dst*(1-srcA). Used where
	// the shader already multiplies rgb by alpha (ripple, blur).
	BlendPremul
	// BlendReplace writes the source unchanged (no blending).
	BlendReplace
)

// Descriptor bundles the per-shader data the pipeline cache needs.
type Descriptor struct {
	Name         string       // matches ShaderSource / Names
	Vertex       VertexLayout // interleaved vertex layout
	Textured     bool         // true if the shader binds a texture+sampler at bindings 1 & 2
	DefaultBlend BlendMode    // blend mode the painter uses by default for this shader
	Topology     Topology     // primitive topology for this shader's geometry
}

// quadVertex is the vert(vec2)+normal(vec2) layout shared by every SDF shape
// shader (rectangle, round_rectangle, polygon, arc, ripple) and the line shader.
var quadVertex = VertexLayout{
	Stride: 16,
	Attrs: []VertexAttr{
		{Location: 0, Format: Float32x2, Offset: 0},
		{Location: 1, Format: Float32x2, Offset: 8},
	},
}

// texturedVertex is the vert(vec3)+texCoord(vec2) layout shared by the simple
// (image/text) and blur shaders.
var texturedVertex = VertexLayout{
	Stride: 20,
	Attrs: []VertexAttr{
		{Location: 0, Format: Float32x3, Offset: 0},
		{Location: 1, Format: Float32x2, Offset: 12},
	},
}

var descriptors = map[string]Descriptor{
	// simple multiplies rgb by alpha in-shader, so it composites premultiplied
	// (matching the GL painter's BlendFunc(one, oneMinusSrcAlpha) for textures).
	"simple":          {Name: "simple", Vertex: texturedVertex, Textured: true, DefaultBlend: BlendPremul, Topology: TriangleStrip},
	"blur":            {Name: "blur", Vertex: texturedVertex, Textured: true, DefaultBlend: BlendPremul, Topology: TriangleStrip},
	"rectangle":       {Name: "rectangle", Vertex: quadVertex, DefaultBlend: BlendOver, Topology: TriangleStrip},
	"round_rectangle": {Name: "round_rectangle", Vertex: quadVertex, DefaultBlend: BlendOver, Topology: TriangleStrip},
	"polygon":         {Name: "polygon", Vertex: quadVertex, DefaultBlend: BlendOver, Topology: TriangleStrip},
	"arc":             {Name: "arc", Vertex: quadVertex, DefaultBlend: BlendOver, Topology: TriangleStrip},
	"ripple":          {Name: "ripple", Vertex: quadVertex, DefaultBlend: BlendPremul, Topology: TriangleStrip},
	"line":            {Name: "line", Vertex: quadVertex, DefaultBlend: BlendOver, Topology: TriangleList},
	"path": {Name: "path", DefaultBlend: BlendOver, Topology: TriangleList, Vertex: VertexLayout{
		Stride: 12,
		Attrs: []VertexAttr{
			{Location: 0, Format: Float32x2, Offset: 0},
			{Location: 1, Format: Float32, Offset: 8},
		},
	}},
	"scene3d": {Name: "scene3d", DefaultBlend: BlendOver, Topology: TriangleList, Vertex: VertexLayout{
		Stride: 40,
		Attrs: []VertexAttr{
			{Location: 0, Format: Float32x3, Offset: 0},
			{Location: 1, Format: Float32x3, Offset: 12},
			{Location: 2, Format: Float32x4, Offset: 24},
		},
	}},
}

// DescriptorFor returns the pipeline descriptor for the named shader, and false
// if the name is unknown.
func DescriptorFor(name string) (Descriptor, bool) {
	d, ok := descriptors[name]
	return d, ok
}
