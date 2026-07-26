package canvas

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/belluikit/bellui/geo"

	"github.com/stretchr/testify/assert"
)

// --- Vec3 tests ---

func TestVec3_Add(t *testing.T) {
	a := Vec3{1, 2, 3}
	b := Vec3{4, 5, 6}
	r := a.Add(b)
	assert.Equal(t, Vec3{5, 7, 9}, r)
}

func TestVec3_Sub(t *testing.T) {
	a := Vec3{5, 7, 9}
	b := Vec3{1, 2, 3}
	r := a.Sub(b)
	assert.Equal(t, Vec3{4, 5, 6}, r)
}

func TestVec3_Scale(t *testing.T) {
	v := Vec3{2, 3, 4}
	r := v.Scale(3)
	assert.Equal(t, Vec3{6, 9, 12}, r)
}

func TestVec3_Scale_Zero(t *testing.T) {
	v := Vec3{2, 3, 4}
	r := v.Scale(0)
	assert.Equal(t, Vec3{0, 0, 0}, r)
}

func TestVec3_Scale_Negative(t *testing.T) {
	v := Vec3{1, 2, 3}
	r := v.Scale(-1)
	assert.Equal(t, Vec3{-1, -2, -3}, r)
}

func TestVec3_Dot(t *testing.T) {
	a := Vec3{1, 0, 0}
	b := Vec3{0, 1, 0}
	assert.Equal(t, float32(0), a.Dot(b)) // perpendicular

	c := Vec3{1, 2, 3}
	d := Vec3{4, 5, 6}
	assert.Equal(t, float32(32), c.Dot(d)) // 1*4 + 2*5 + 3*6 = 32
}

func TestVec3_Cross(t *testing.T) {
	x := Vec3{1, 0, 0}
	y := Vec3{0, 1, 0}
	z := x.Cross(y)
	assert.Equal(t, Vec3{0, 0, 1}, z) // X × Y = Z

	// Anti-commutativity: Y × X = -Z
	nz := y.Cross(x)
	assert.Equal(t, Vec3{0, 0, -1}, nz)
}

func TestVec3_Cross_Parallel(t *testing.T) {
	a := Vec3{2, 4, 6}
	b := Vec3{1, 2, 3}
	r := a.Cross(b)
	assert.Equal(t, Vec3{0, 0, 0}, r) // parallel vectors → zero cross product
}

func TestVec3_Length(t *testing.T) {
	// Unit vectors
	assert.InDelta(t, 1.0, Vec3{1, 0, 0}.Length(), 1e-6)
	assert.InDelta(t, 1.0, Vec3{0, 1, 0}.Length(), 1e-6)
	assert.InDelta(t, 1.0, Vec3{0, 0, 1}.Length(), 1e-6)

	// Known: (3, 4, 0) → 5
	assert.InDelta(t, 5.0, Vec3{3, 4, 0}.Length(), 1e-5)

	// Zero vector
	assert.Equal(t, float32(0), Vec3{0, 0, 0}.Length())
}

func TestVec3_Normalize(t *testing.T) {
	v := Vec3{3, 0, 0}
	n := v.Normalize()
	assert.InDelta(t, 1.0, n.X, 1e-6)
	assert.InDelta(t, 0.0, n.Y, 1e-6)
	assert.InDelta(t, 0.0, n.Z, 1e-6)

	// Arbitrary vector → unit length
	v2 := Vec3{1, 2, 3}
	n2 := v2.Normalize()
	assert.InDelta(t, 1.0, n2.Length(), 1e-5)
}

func TestVec3_Normalize_Zero(t *testing.T) {
	v := Vec3{0, 0, 0}
	n := v.Normalize()
	assert.Equal(t, Vec3{0, 0, 0}, n) // zero vector normalizes to zero
}

// --- Mat4 tests ---

func TestMat4_Identity(t *testing.T) {
	var id Mat4
	id[0] = 1
	id[5] = 1
	id[10] = 1
	id[15] = 1

	// Identity * vector = same vector
	v := Vec3{3, 7, 11}
	r := id.MulVec3(v)
	assert.InDelta(t, 3.0, r.X, 1e-5)
	assert.InDelta(t, 7.0, r.Y, 1e-5)
	assert.InDelta(t, 11.0, r.Z, 1e-5)
}

func TestMat4_Mul_Identity(t *testing.T) {
	var id Mat4
	id[0] = 1
	id[5] = 1
	id[10] = 1
	id[15] = 1

	a := PerspectiveMatrix(45, 1.0, 0.1, 100)
	r := a.Mul(id)

	for i := 0; i < 16; i++ {
		assert.InDelta(t, a[i], r[i], 1e-5, "index %d", i)
	}
}

func TestMat4_MulVec3_Translation(t *testing.T) {
	// Build a translation matrix (translate by 10, 20, 30).
	var m Mat4
	m[0] = 1
	m[5] = 1
	m[10] = 1
	m[15] = 1
	m[12] = 10
	m[13] = 20
	m[14] = 30

	v := Vec3{1, 2, 3}
	r := m.MulVec3(v)
	assert.InDelta(t, 11.0, r.X, 1e-5)
	assert.InDelta(t, 22.0, r.Y, 1e-5)
	assert.InDelta(t, 33.0, r.Z, 1e-5)
}

func TestPerspectiveMatrix(t *testing.T) {
	m := PerspectiveMatrix(45, 1.0, 0.1, 100)

	// Verify key properties of a perspective matrix:
	// m[0] = f/aspect, m[5] = f, m[11] = -1
	assert.Positive(t, m[0])
	assert.Positive(t, m[5])
	assert.Equal(t, float32(-1), m[11])

	// For aspect=1, m[0] should equal m[5]
	assert.InDelta(t, m[0], m[5], 1e-5)
}

func TestPerspectiveMatrix_DepthTerms(t *testing.T) {
	// m[10] and m[14] are the near/far depth terms behind the #29/#280 clip
	// fixes; assert their exact values rather than just sign/positivity.
	near, far := float32(0.1), float32(100)
	m := PerspectiveMatrix(45, 1.0, near, far)

	assert.InDelta(t, (far+near)/(near-far), m[10], 1e-5)
	assert.InDelta(t, (2*far*near)/(near-far), m[14], 1e-5)

	// Both must be negative for a right-handed, looking-down-(-z) projection.
	assert.Negative(t, m[10])
	assert.Negative(t, m[14])
}

func TestMat4_Mul_OrderAndColumnMajor(t *testing.T) {
	// Translation (10,20,30) in the column-major translation column.
	var trans Mat4
	trans[0], trans[5], trans[10], trans[15] = 1, 1, 1, 1
	trans[12], trans[13], trans[14] = 10, 20, 30

	// Non-uniform scale (2,3,4).
	var scale Mat4
	scale[0], scale[5], scale[10], scale[15] = 2, 3, 4, 1

	// trans.Mul(scale) means "scale first, then translate" for a vector. A
	// transpose/order bug would scatter the translation into the wrong row.
	r := trans.Mul(scale)
	assert.InDelta(t, float32(2), r[0], 1e-5)
	assert.InDelta(t, float32(3), r[5], 1e-5)
	assert.InDelta(t, float32(4), r[10], 1e-5)
	// Scale column 3 is identity, so translation survives unchanged.
	assert.InDelta(t, float32(10), r[12], 1e-5)
	assert.InDelta(t, float32(20), r[13], 1e-5)
	assert.InDelta(t, float32(30), r[14], 1e-5)

	// Applied to (1,1,1): scaled to (2,3,4) then translated to (12,23,34).
	v := r.MulVec3(Vec3{1, 1, 1})
	assert.InDelta(t, 12.0, v.X, 1e-5)
	assert.InDelta(t, 23.0, v.Y, 1e-5)
	assert.InDelta(t, 34.0, v.Z, 1e-5)

	// Multiplication is order-dependent: scale-then-translate differs from
	// translate-then-scale (which would scale the translation: 20,60,120).
	r2 := scale.Mul(trans)
	assert.InDelta(t, float32(20), r2[12], 1e-5)
	assert.InDelta(t, float32(60), r2[13], 1e-5)
	assert.InDelta(t, float32(120), r2[14], 1e-5)
}

func TestPerspectiveMatrix_AspectRatio(t *testing.T) {
	m := PerspectiveMatrix(45, 2.0, 0.1, 100)

	// With aspect=2, m[0] should be half of m[5]
	assert.InDelta(t, m[5]/2, m[0], 1e-5)
}

func TestLookAtMatrix(t *testing.T) {
	eye := Vec3{0, 0, 5}
	center := Vec3{0, 0, 0}
	up := Vec3{0, 1, 0}
	m := LookAtMatrix(eye, center, up)

	// The origin should transform to (0, 0, -5) in view space (in front of camera)
	v := m.MulVec3(Vec3{0, 0, 0})
	assert.InDelta(t, 0.0, v.X, 1e-4)
	assert.InDelta(t, 0.0, v.Y, 1e-4)
	assert.InDelta(t, -5.0, v.Z, 1e-4)
}

func TestLookAtMatrix_EyeAtOrigin(t *testing.T) {
	eye := Vec3{0, 0, 0}
	center := Vec3{0, 0, -1}
	up := Vec3{0, 1, 0}
	m := LookAtMatrix(eye, center, up)

	// Eye position maps to origin in view space
	v := m.MulVec3(eye)
	assert.InDelta(t, 0.0, v.X, 1e-4)
	assert.InDelta(t, 0.0, v.Y, 1e-4)
	assert.InDelta(t, 0.0, v.Z, 1e-4)
}

func TestMat4_Mul_Associativity(t *testing.T) {
	a := PerspectiveMatrix(60, 1.5, 0.1, 50)
	b := LookAtMatrix(Vec3{3, 3, 5}, Vec3{0, 0, 0}, Vec3{0, 1, 0})

	var id Mat4
	id[0] = 1
	id[5] = 1
	id[10] = 1
	id[15] = 1

	// (A * B) * I = A * B
	ab := a.Mul(b)
	abi := ab.Mul(id)
	for i := 0; i < 16; i++ {
		assert.InDelta(t, ab[i], abi[i], 1e-4, "index %d", i)
	}
}

// --- Scene3D object tests ---

func TestNewScene3D(t *testing.T) {
	s := NewScene3D()
	assert.NotNil(t, s)

	// Default camera
	assert.Equal(t, float32(45), s.Camera.FOV)
	assert.InDelta(t, 0.1, s.Camera.Near, 1e-5)
	assert.Equal(t, float32(1000), s.Camera.Far)

	// Default light
	assert.Len(t, s.Lights, 1)
	assert.InDelta(t, 0.3, s.Lights[0].Ambient, 1e-5)

	// No meshes
	assert.Empty(t, s.Meshes)
}

func TestDefaultCamera(t *testing.T) {
	c := DefaultCamera()
	assert.Equal(t, Vec3{3, 3, 5}, c.Position)
	assert.Equal(t, Vec3{0, 0, 0}, c.Target)
	assert.Equal(t, Vec3{0, 1, 0}, c.Up)
	assert.Equal(t, float32(45), c.FOV)
}

func TestDefaultLight(t *testing.T) {
	l := DefaultLight()
	assert.Equal(t, color.White, l.Color)
	assert.InDelta(t, 0.3, l.Ambient, 1e-5)
}

func TestScene3D_AddMesh(t *testing.T) {
	s := NewScene3D()
	assert.Empty(t, s.Meshes)

	s.AddMesh(Mesh3D{
		Vertices:  []Vec3{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
		Faces:     []Face{{0, 1, 2}},
		FillColor: color.White,
	})
	assert.Len(t, s.Meshes, 1)

	s.AddMesh(Mesh3D{})
	assert.Len(t, s.Meshes, 2)
}

func TestScene3D_MoveResize(t *testing.T) {
	s := NewScene3D()
	assert.Equal(t, geo.Position{}, s.Position())

	s.Move(geo.NewPos(10, 20))
	assert.Equal(t, geo.NewPos(10, 20), s.Position())

	s.Resize(geo.NewSize(200, 150))
	assert.Equal(t, geo.NewSize(200, 150), s.Size())
}

func TestScene3D_HideShow(t *testing.T) {
	s := NewScene3D()
	assert.True(t, s.Visible())

	s.Hide()
	assert.False(t, s.Visible())

	s.Show()
	assert.True(t, s.Visible())
}

// --- Render tests (software rasterizer) ---

func TestScene3D_Render_Empty(t *testing.T) {
	s := NewScene3D()
	img := s.Render(100, 100)
	assert.NotNil(t, img)
	assert.Equal(t, 100, img.Bounds().Dx())
	assert.Equal(t, 100, img.Bounds().Dy())
}

func TestScene3D_Render_InvalidDimensions(t *testing.T) {
	s := NewScene3D()

	img := s.Render(0, 0)
	assert.NotNil(t, img)
	assert.Equal(t, 1, img.Bounds().Dx())

	img = s.Render(-5, -5)
	assert.NotNil(t, img)
	assert.Equal(t, 1, img.Bounds().Dx())
}

func TestScene3D_Render_Background(t *testing.T) {
	s := NewScene3D()
	bg := color.NRGBA{R: 0, G: 0, B: 255, A: 255}
	s.Background = bg

	img := s.Render(50, 50)

	// All pixels should be the background color (no meshes)
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			assert.Equal(t, uint32(0), r>>8, "red at (%d,%d)", x, y)
			assert.Equal(t, uint32(0), g>>8, "green at (%d,%d)", x, y)
			assert.Equal(t, uint32(255), b>>8, "blue at (%d,%d)", x, y)
			assert.Equal(t, uint32(255), a>>8, "alpha at (%d,%d)", x, y)
		}
	}
}

func TestScene3D_Render_SingleTriangle(t *testing.T) {
	s := NewScene3D()
	s.Camera = Camera3D{
		Position: Vec3{0, 0, 3},
		Target:   Vec3{0, 0, 0},
		Up:       Vec3{0, 1, 0},
		FOV:      60,
		Near:     0.1,
		Far:      100,
	}

	red := color.NRGBA{R: 255, G: 0, B: 0, A: 255}
	s.AddMesh(Mesh3D{
		Vertices: []Vec3{
			{-1, -1, 0},
			{1, -1, 0},
			{0, 1, 0},
		},
		Faces:     []Face{{0, 1, 2}},
		FillColor: red,
	})

	img := s.Render(100, 100)
	assert.NotNil(t, img)

	// The center pixel should have red content (the triangle covers the center)
	cx, cy := 50, 50
	r, _, _, a := img.At(cx, cy).RGBA()
	assert.Positive(t, r>>8, "center pixel should have red component")
	assert.Positive(t, a>>8, "center pixel should be non-transparent")
}

func TestScene3D_Render_PerFaceColor(t *testing.T) {
	s := NewScene3D()
	s.Camera = Camera3D{
		Position: Vec3{0, 0, 5},
		Target:   Vec3{0, 0, 0},
		Up:       Vec3{0, 1, 0},
		FOV:      60,
		Near:     0.1,
		Far:      100,
	}

	green := color.NRGBA{R: 0, G: 255, B: 0, A: 255}
	s.AddMesh(Mesh3D{
		Vertices: []Vec3{
			{-1, -1, 0},
			{1, -1, 0},
			{0, 1, 0},
		},
		Faces:      []Face{{0, 1, 2}},
		FillColor:  color.White,
		FaceColors: []color.Color{green},
	})

	img := s.Render(100, 100)

	// Center pixel should have green tint (per-face color overrides FillColor)
	cx, cy := 50, 50
	_, g, _, _ := img.At(cx, cy).RGBA()
	assert.Positive(t, g>>8, "center pixel should have green component from per-face color")
}

func TestScene3D_Render_ZBuffer(t *testing.T) {
	s := NewScene3D()
	s.Camera = Camera3D{
		Position: Vec3{0, 0, 5},
		Target:   Vec3{0, 0, 0},
		Up:       Vec3{0, 1, 0},
		FOV:      60,
		Near:     0.1,
		Far:      100,
	}

	// Front triangle (z=0) is blue, back triangle (z=-2) is red.
	// Front should occlude back at center.
	blue := color.NRGBA{R: 0, G: 0, B: 255, A: 255}
	red := color.NRGBA{R: 255, G: 0, B: 0, A: 255}

	s.AddMesh(Mesh3D{
		Vertices: []Vec3{
			{-2, -2, -2}, {2, -2, -2}, {0, 2, -2},
		},
		Faces:     []Face{{0, 1, 2}},
		FillColor: red,
	})
	s.AddMesh(Mesh3D{
		Vertices: []Vec3{
			{-1, -1, 0}, {1, -1, 0}, {0, 1, 0},
		},
		Faces:     []Face{{0, 1, 2}},
		FillColor: blue,
	})

	img := s.Render(100, 100)
	cx, cy := 50, 50
	_, _, b, _ := img.At(cx, cy).RGBA()
	r, _, _, _ := img.At(cx, cy).RGBA()

	// Blue (front) should dominate over red (back) at center
	assert.Greater(t, b>>8, r>>8, "front (blue) triangle should occlude back (red)")
}

func TestScene3D_Render_InvalidFaceIndices(t *testing.T) {
	s := NewScene3D()
	s.Camera = DefaultCamera()

	// Face references out-of-bounds vertex indices — should be skipped, not panic
	s.AddMesh(Mesh3D{
		Vertices:  []Vec3{{0, 0, 0}},
		Faces:     []Face{{0, 1, 2}}, // indices 1, 2 out of bounds
		FillColor: color.White,
	})

	assert.NotPanics(t, func() {
		img := s.Render(50, 50)
		assert.NotNil(t, img)
	})
}

func TestScene3D_Render_CameraDefaults(t *testing.T) {
	s := NewScene3D()
	// Set FOV/Near/Far to zero — Render should use sensible defaults
	s.Camera.FOV = 0
	s.Camera.Near = 0
	s.Camera.Far = 0

	assert.NotPanics(t, func() {
		img := s.Render(50, 50)
		assert.NotNil(t, img)
	})
}

// --- Backface culling tests ---

func TestScene3D_BackfaceCulling_SkipsBackFaces(t *testing.T) {
	s := NewScene3D()
	s.BackfaceCulling = true
	s.Camera = Camera3D{
		Position: Vec3{0, 0, 3},
		Target:   Vec3{0, 0, 0},
		Up:       Vec3{0, 1, 0},
		FOV:      60,
		Near:     0.1,
		Far:      100,
	}

	// Triangle with vertices wound so that the normal faces AWAY from the camera
	// (clockwise winding when viewed from camera). Normal = (0, 0, -1).
	s.AddMesh(Mesh3D{
		Vertices: []Vec3{
			{-1, -1, 0},
			{0, 1, 0},
			{1, -1, 0},
		},
		Faces:     []Face{{0, 1, 2}},
		FillColor: color.NRGBA{R: 255, A: 255},
	})

	img := s.Render(100, 100)

	// With culling on, the back-facing triangle should be invisible (transparent).
	_, _, _, a := img.At(50, 50).RGBA()
	assert.Equal(t, uint32(0), a>>8, "back-facing triangle should be culled")
}

func TestScene3D_DoubleSided_RendersBackFaces(t *testing.T) {
	s := NewScene3D()
	// BackfaceCulling defaults to false — both sides should render.
	s.Camera = Camera3D{
		Position: Vec3{0, 0, 3},
		Target:   Vec3{0, 0, 0},
		Up:       Vec3{0, 1, 0},
		FOV:      60,
		Near:     0.1,
		Far:      100,
	}

	// Same back-facing triangle (normal faces away from camera).
	s.AddMesh(Mesh3D{
		Vertices: []Vec3{
			{-1, -1, 0},
			{0, 1, 0},
			{1, -1, 0},
		},
		Faces:     []Face{{0, 1, 2}},
		FillColor: color.NRGBA{R: 255, A: 255},
	})

	img := s.Render(100, 100)

	// With double-sided rendering (default), the triangle should still be visible
	// with correct lighting (normal flipped toward viewer).
	r, _, _, a := img.At(50, 50).RGBA()
	assert.Positive(t, a>>8, "back-facing triangle should be rendered in double-sided mode")
	assert.Positive(t, r>>8, "back-facing triangle should have red color with lighting applied")
}

func TestScene3D_BackfaceCulling_KeepsFrontFaces(t *testing.T) {
	s := NewScene3D()
	s.BackfaceCulling = true
	s.Camera = Camera3D{
		Position: Vec3{0, 0, 3},
		Target:   Vec3{0, 0, 0},
		Up:       Vec3{0, 1, 0},
		FOV:      60,
		Near:     0.1,
		Far:      100,
	}

	// Front-facing triangle (counter-clockwise winding from camera). Normal = (0, 0, 1).
	s.AddMesh(Mesh3D{
		Vertices: []Vec3{
			{-1, -1, 0},
			{1, -1, 0},
			{0, 1, 0},
		},
		Faces:     []Face{{0, 1, 2}},
		FillColor: color.NRGBA{G: 255, A: 255},
	})

	img := s.Render(100, 100)

	// Front-facing triangle should be rendered even with culling enabled.
	_, g, _, a := img.At(50, 50).RGBA()
	assert.Positive(t, a>>8, "front-facing triangle should be rendered with culling on")
	assert.Positive(t, g>>8, "front-facing triangle should have green color")
}

// --- Depth clipping tests ---

func countOpaque(img *image.NRGBA) int {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a>>8 > 0 {
				n++
			}
		}
	}
	return n
}

// A triangle straddling the near plane must render its visible portion rather
// than being dropped wholesale (the GL hardware clips per-edge; the software
// rasterizer now matches by clipping the geometry in clip space). See #29.
func TestScene3D_Render_NearPlaneClip_StraddlingTriangleVisible(t *testing.T) {
	s := NewScene3D()
	// Camera at z=2 looking toward -z; near plane sits at world z = 1.9.
	s.Camera = Camera3D{
		Position: Vec3{0, 0, 2},
		Target:   Vec3{0, 0, 0},
		Up:       Vec3{0, 1, 0},
		FOV:      60,
		Near:     0.1,
		Far:      100,
	}

	// Two vertices in front of the near plane (z=0), one apex behind the camera
	// (z=3). The triangle therefore straddles the near plane.
	s.AddMesh(Mesh3D{
		Vertices: []Vec3{
			{-3, -3, 0},
			{3, -3, 0},
			{0, 3, 3},
		},
		Faces:     []Face{{0, 1, 2}},
		FillColor: color.NRGBA{R: 255, A: 255},
	})

	img := s.Render(100, 100)
	assert.Positive(t, countOpaque(img),
		"straddling triangle should render its visible (clipped) portion")
}

// A triangle entirely behind the near plane must render nothing.
func TestScene3D_Render_NearPlaneClip_FullyBehindSkipped(t *testing.T) {
	s := NewScene3D()
	s.Camera = Camera3D{
		Position: Vec3{0, 0, 2},
		Target:   Vec3{0, 0, 0},
		Up:       Vec3{0, 1, 0},
		FOV:      60,
		Near:     0.1,
		Far:      100,
	}

	// All vertices behind the camera (z > 2 > near plane at z=1.9).
	s.AddMesh(Mesh3D{
		Vertices: []Vec3{
			{-1, -1, 5},
			{1, -1, 5},
			{0, 1, 5},
		},
		Faces:     []Face{{0, 1, 2}},
		FillColor: color.NRGBA{R: 255, A: 255},
	})

	img := s.Render(100, 100)
	assert.Equal(t, 0, countOpaque(img),
		"triangle fully behind the near plane should render nothing")
}

// A triangle straddling the far plane must render its visible portion rather than
// being dropped, and the part beyond the far plane must not appear at all (the GL
// hardware clips per-edge; the software rasterizer now matches). See #280.
func TestScene3D_Render_FarPlaneClip_StraddlingTriangleVisible(t *testing.T) {
	s := NewScene3D()
	// Camera at z=10 looking toward -z; far plane sits at world z = 10-far.
	s.Camera = Camera3D{
		Position: Vec3{0, 0, 10},
		Target:   Vec3{0, 0, 0},
		Up:       Vec3{0, 1, 0},
		FOV:      60,
		Near:     0.1,
		Far:      5, // far plane at world z = 5
	}

	// Two vertices in front of the far plane (z=6, distance 4 < 5), one apex well
	// beyond it (z=-20, distance 30 > 5). The triangle straddles the far plane.
	s.AddMesh(Mesh3D{
		Vertices: []Vec3{
			{-3, -3, 6},
			{3, -3, 6},
			{0, 3, -20},
		},
		Faces:     []Face{{0, 1, 2}},
		FillColor: color.NRGBA{R: 255, A: 255},
	})

	img := s.Render(100, 100)
	assert.Positive(t, countOpaque(img),
		"triangle straddling the far plane should render its visible (clipped) portion")
}

// A triangle entirely beyond the far plane must render nothing.
func TestScene3D_Render_FarPlaneClip_FullyBeyondSkipped(t *testing.T) {
	s := NewScene3D()
	s.Camera = Camera3D{
		Position: Vec3{0, 0, 10},
		Target:   Vec3{0, 0, 0},
		Up:       Vec3{0, 1, 0},
		FOV:      60,
		Near:     0.1,
		Far:      5, // far plane at world z = 5
	}

	// All vertices beyond the far plane (z <= -20, distance >= 30 > 5).
	s.AddMesh(Mesh3D{
		Vertices: []Vec3{
			{-1, -1, -20},
			{1, -1, -20},
			{0, 1, -20},
		},
		Faces:     []Face{{0, 1, 2}},
		FillColor: color.NRGBA{R: 255, A: 255},
	})

	img := s.Render(100, 100)
	assert.Equal(t, 0, countOpaque(img),
		"triangle fully beyond the far plane should render nothing")
}

// --- clipPlane / clipDepth unit tests ---

func TestClipPlane_Near_FullyInFront(t *testing.T) {
	in := []clipVert{
		{x: 0, y: 0, z: 0, w: 1},
		{x: 1, y: 0, z: 0, w: 1},
		{x: 0, y: 1, z: 0, w: 1},
	}
	out := clipPlane(in, distNear)
	assert.Equal(t, in, out, "fully-in-front triangle should pass through unchanged")
}

func TestClipPlane_Near_FullyBehind(t *testing.T) {
	in := []clipVert{
		{x: 0, y: 0, z: -2, w: 1}, // w+z = -1 < 0
		{x: 1, y: 0, z: -2, w: 1},
		{x: 0, y: 1, z: -2, w: 1},
	}
	out := clipPlane(in, distNear)
	assert.Empty(t, out, "fully-behind triangle should be clipped away entirely")
}

func TestClipPlane_Near_Straddling(t *testing.T) {
	in := []clipVert{
		{x: 0, y: 0, z: 0, w: 1},  // w+z = 1 (in)
		{x: 1, y: 0, z: 0, w: 1},  // w+z = 1 (in)
		{x: 0, y: 1, z: -2, w: 1}, // w+z = -1 (out)
	}
	out := clipPlane(in, distNear)
	// One vertex outside → the polygon gains a vertex (3 in - 1 out + 2 cuts).
	assert.Len(t, out, 4, "straddling triangle should clip to a 4-vertex polygon")
	for i, v := range out {
		assert.GreaterOrEqual(t, distNear(v), float32(-1e-6),
			"clipped vertex %d should lie on or in front of the near plane", i)
	}
}

func TestClipPlane_Far_Straddling(t *testing.T) {
	in := []clipVert{
		{x: 0, y: 0, z: 0, w: 1}, // w-z = 1 (in)
		{x: 1, y: 0, z: 0, w: 1}, // w-z = 1 (in)
		{x: 0, y: 1, z: 2, w: 1}, // w-z = -1 (out, beyond far)
	}
	out := clipPlane(in, distFar)
	assert.Len(t, out, 4, "straddling triangle should clip to a 4-vertex polygon")
	for i, v := range out {
		assert.GreaterOrEqual(t, distFar(v), float32(-1e-6),
			"clipped vertex %d should lie on or in front of the far plane", i)
	}
}

func TestClipDepth_FullyInside(t *testing.T) {
	in := []clipVert{
		{x: 0, y: 0, z: 0, w: 1},
		{x: 1, y: 0, z: 0, w: 1},
		{x: 0, y: 1, z: 0, w: 1},
	}
	out := clipDepth(in)
	assert.Equal(t, in, out, "triangle inside the depth range should pass through unchanged")
}

func TestClipDepth_FullyOutside(t *testing.T) {
	// Beyond the far plane on every vertex.
	in := []clipVert{
		{x: 0, y: 0, z: 2, w: 1}, // w-z = -1 < 0
		{x: 1, y: 0, z: 2, w: 1},
		{x: 0, y: 1, z: 2, w: 1},
	}
	assert.Nil(t, clipDepth(in), "triangle fully beyond the far plane should clip to nil")

	// Behind the near plane on every vertex.
	in = []clipVert{
		{x: 0, y: 0, z: -2, w: 1}, // w+z = -1 < 0
		{x: 1, y: 0, z: -2, w: 1},
		{x: 0, y: 1, z: -2, w: 1},
	}
	assert.Nil(t, clipDepth(in), "triangle fully behind the near plane should clip to nil")
}

func TestClipDepth_StraddlingBothPlanes(t *testing.T) {
	// A long triangle whose vertices span from behind the near plane to beyond the
	// far plane; the visible middle band must survive within [-1, 1] NDC z.
	in := []clipVert{
		{x: 0, y: 0, z: -2, w: 1}, // behind near (w+z = -1)
		{x: 1, y: 0, z: 2, w: 1},  // beyond far (w-z = -1)
		{x: 0, y: 1, z: 0, w: 1},  // inside
	}
	out := clipDepth(in)
	assert.GreaterOrEqual(t, len(out), 3, "straddling triangle should retain a visible polygon")
	for i, v := range out {
		assert.GreaterOrEqual(t, distNear(v), float32(-1e-6), "vertex %d behind near plane", i)
		assert.GreaterOrEqual(t, distFar(v), float32(-1e-6), "vertex %d beyond far plane", i)
	}
}

func TestClipPlane_Empty(t *testing.T) {
	assert.Empty(t, clipPlane(nil, distNear), "clipping an empty polygon returns empty")
}

// --- rasterizeTriangle tests ---

func TestRasterizeTriangle_FullCoverage(t *testing.T) {
	w, h := 10, 10
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	zbuf := make([]float32, w*h)
	for i := range zbuf {
		zbuf[i] = float32(math.MaxFloat32)
	}

	// Triangle covering the entire 10x10 image
	p0 := [2]float32{0, 0}
	p1 := [2]float32{10, 0}
	p2 := [2]float32{0, 10}
	c := color.NRGBA{R: 255, A: 255}

	rasterizeTriangle(img, zbuf, w, h, p0, p1, p2, 0.5, 0.5, 0.5, c)

	// Center pixel should be colored
	r, _, _, a := img.At(3, 3).RGBA()
	assert.Positive(t, r>>8)
	assert.Positive(t, a>>8)
}

func TestRasterizeTriangle_Degenerate(t *testing.T) {
	w, h := 10, 10
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	zbuf := make([]float32, w*h)
	for i := range zbuf {
		zbuf[i] = float32(math.MaxFloat32)
	}

	// Degenerate triangle (all points colinear)
	p0 := [2]float32{0, 0}
	p1 := [2]float32{5, 0}
	p2 := [2]float32{10, 0}
	c := color.NRGBA{R: 255, A: 255}

	assert.NotPanics(t, func() {
		rasterizeTriangle(img, zbuf, w, h, p0, p1, p2, 0.5, 0.5, 0.5, c)
	})
}

func TestRasterizeTriangle_ZBufferOcclusion(t *testing.T) {
	w, h := 10, 10
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	zbuf := make([]float32, w*h)
	for i := range zbuf {
		zbuf[i] = float32(math.MaxFloat32)
	}

	// Draw a front triangle (z=0.1)
	p0 := [2]float32{0, 0}
	p1 := [2]float32{10, 0}
	p2 := [2]float32{0, 10}
	front := color.NRGBA{G: 255, A: 255}
	rasterizeTriangle(img, zbuf, w, h, p0, p1, p2, 0.1, 0.1, 0.1, front)

	// Try to draw a behind triangle (z=0.5) over the same area
	back := color.NRGBA{R: 255, A: 255}
	rasterizeTriangle(img, zbuf, w, h, p0, p1, p2, 0.5, 0.5, 0.5, back)

	// Front (green) should still be visible
	_, g, _, _ := img.At(2, 2).RGBA()
	r, _, _, _ := img.At(2, 2).RGBA()
	assert.Positive(t, g>>8, "front triangle (green) should remain")
	assert.Equal(t, uint32(0), r>>8, "back triangle (red) should be occluded")
}

// --- min3f / max3f tests ---

func TestMin3f(t *testing.T) {
	assert.Equal(t, float32(1), min3f(1, 2, 3))
	assert.Equal(t, float32(1), min3f(3, 1, 2))
	assert.Equal(t, float32(1), min3f(2, 3, 1))
	assert.Equal(t, float32(-5), min3f(-5, 0, 5))
	assert.Equal(t, float32(7), min3f(7, 7, 7))
}

func TestMax3f(t *testing.T) {
	assert.Equal(t, float32(3), max3f(1, 2, 3))
	assert.Equal(t, float32(3), max3f(3, 1, 2))
	assert.Equal(t, float32(3), max3f(2, 3, 1))
	assert.Equal(t, float32(5), max3f(-5, 0, 5))
	assert.Equal(t, float32(7), max3f(7, 7, 7))
}
