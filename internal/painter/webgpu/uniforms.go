package webgpu

// Uniform block layouts for the WGSL shaders (shaders/*.wgsl).
//
// Each struct mirrors a shader's `struct Uniforms` byte-for-byte under WGSL's
// std140-style rules (f32:4, vec2:8, vec3/vec4:16, mat3x3:48 with a 16-byte
// column stride, mat4x4:64; the whole struct is rounded up to a multiple of its
// largest member alignment, i.e. 16). The painter populates one of these and
// uploads it with queue.WriteBuffer, so the Go field offsets MUST match the
// WGSL ones — uniforms_test.go asserts this against the offsets documented in
// each .wgsl module. vec2/vec3/vec4 map to [N]float32; mat3x3 maps to a
// [12]float32 (3 padded columns) and mat4x4 to [16]float32.

// SimpleUniforms matches shaders/simple.wgsl (size 32).
type SimpleUniforms struct {
	Alpha        float32    // offset 0
	CornerRadius float32    // offset 4
	Size         [2]float32 // offset 8
	Inset        [4]float32 // offset 16
}

// LineUniforms matches shaders/line.wgsl (size 32, incl. trailing pad to 16).
type LineUniforms struct {
	Color     [4]float32 // offset 0
	LineWidth float32    // offset 16
	Feather   float32    // offset 20
	_         [2]float32 // pad to 32
}

// PathUniforms matches shaders/path.wgsl (size 16).
type PathUniforms struct {
	FillColor [4]float32 // offset 0
}

// RectangleUniforms matches shaders/rectangle.wgsl (size 112).
type RectangleUniforms struct {
	FrameSize    [2]float32  // offset 0
	StrokeWidth  float32     // offset 8
	_            float32     // pad before rect_coords (vec4 align 16)
	RectCoords   [4]float32  // offset 16
	FillColor    [4]float32  // offset 32
	StrokeColor  [4]float32  // offset 48
	InvTransform [12]float32 // offset 64 (mat3x3, 16-byte column stride)
}

// RoundRectangleUniforms matches shaders/round_rectangle.wgsl (size 144).
type RoundRectangleUniforms struct {
	FrameSize       [2]float32  // offset 0
	StrokeWidthHalf float32     // offset 8
	EdgeSoftness    float32     // offset 12
	RectCoords      [4]float32  // offset 16
	RectSizeHalf    [2]float32  // offset 32
	_               [2]float32  // pad before radius (vec4 align 16)
	Radius          [4]float32  // offset 48
	FillColor       [4]float32  // offset 64
	StrokeColor     [4]float32  // offset 80
	InvTransform    [12]float32 // offset 96
}

// PolygonUniforms matches shaders/polygon.wgsl (size 128).
type PolygonUniforms struct {
	FrameSize    [2]float32  // offset 0
	EdgeSoftness float32     // offset 8
	OuterRadius  float32     // offset 12
	RectCoords   [4]float32  // offset 16
	Angle        float32     // offset 32
	Sides        float32     // offset 36
	CornerRadius float32     // offset 40
	StrokeWidth  float32     // offset 44
	FillColor    [4]float32  // offset 48
	StrokeColor  [4]float32  // offset 64
	InvTransform [12]float32 // offset 80
}

// ArcUniforms matches shaders/arc.wgsl (size 144).
type ArcUniforms struct {
	FrameSize    [2]float32  // offset 0
	EdgeSoftness float32     // offset 8
	InnerRadius  float32     // offset 12
	RectCoords   [4]float32  // offset 16
	OuterRadius  float32     // offset 32
	StartAngle   float32     // offset 36
	EndAngle     float32     // offset 40
	CornerRadius float32     // offset 44
	FillColor    [4]float32  // offset 48
	StrokeWidth  float32     // offset 64
	_            [3]float32  // pad before stroke_color (vec4 align 16)
	StrokeColor  [4]float32  // offset 80
	InvTransform [12]float32 // offset 96
}

// RippleUniforms matches shaders/ripple.wgsl (size 128).
type RippleUniforms struct {
	FrameSize    [2]float32  // offset 0
	Radius       float32     // offset 8
	EdgeSoftness float32     // offset 12
	RectCoords   [4]float32  // offset 16
	RectSizeHalf [2]float32  // offset 32
	RippleCenter [2]float32  // offset 40
	RippleRadius float32     // offset 48
	_            [3]float32  // pad before ripple_color (vec4 align 16)
	RippleColor  [4]float32  // offset 64
	InvTransform [12]float32 // offset 80
}

// BlurUniforms matches shaders/blur.wgsl (size 16, incl. trailing pad to 16).
type BlurUniforms struct {
	Resolution [2]float32 // offset 0
	BlurRadius float32    // offset 8
	_          float32    // pad to 16
}

// Scene3DUniforms matches shaders/scene3d.wgsl (size 96).
type Scene3DUniforms struct {
	MVP        [16]float32 // offset 0  (mat4x4)
	LightDir   [3]float32  // offset 64 (vec3)
	_          float32     // pad after light_dir
	LightColor [3]float32  // offset 80 (vec3)
	Ambient    float32     // offset 92
}

// Mat3ToStd140 expands a column-major 3x3 matrix (9 floats, as produced by
// geo.Matrix.ToGL) into the std140 layout used for a WGSL mat3x3<f32>: three
// columns each padded from vec3 to vec4 (the 4th float of every column unused).
func Mat3ToStd140(m [9]float32) [12]float32 {
	return [12]float32{
		m[0], m[1], m[2], 0,
		m[3], m[4], m[5], 0,
		m[6], m[7], m[8], 0,
	}
}
