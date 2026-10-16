// Scene3D represents a 3D scene that can be rendered as a Bellui canvas object.
// It contains meshes, a camera, and lights, and renders them into a 2D image
// using software rasterization with perspective projection and z-buffering.
//
// Since: 1.0
package canvas

import (
	"image"
	"image/color"
	"math"
	"sort"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// Vec3 represents a 3D vector or point.
type Vec3 struct {
	X, Y, Z float32
}

// Add returns v + other.
func (v Vec3) Add(other Vec3) Vec3 {
	return Vec3{v.X + other.X, v.Y + other.Y, v.Z + other.Z}
}

// Sub returns v - other.
func (v Vec3) Sub(other Vec3) Vec3 {
	return Vec3{v.X - other.X, v.Y - other.Y, v.Z - other.Z}
}

// Scale returns v * s.
func (v Vec3) Scale(s float32) Vec3 {
	return Vec3{v.X * s, v.Y * s, v.Z * s}
}

// Dot returns the dot product of v and other.
func (v Vec3) Dot(other Vec3) float32 {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z
}

// Cross returns the cross product of v and other.
func (v Vec3) Cross(other Vec3) Vec3 {
	return Vec3{
		v.Y*other.Z - v.Z*other.Y,
		v.Z*other.X - v.X*other.Z,
		v.X*other.Y - v.Y*other.X,
	}
}

// Length returns the magnitude of v.
func (v Vec3) Length() float32 {
	return float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y + v.Z*v.Z)))
}

// Normalize returns a unit vector in the direction of v.
func (v Vec3) Normalize() Vec3 {
	l := v.Length()
	if l == 0 {
		return Vec3{}
	}
	return Vec3{v.X / l, v.Y / l, v.Z / l}
}

// Face defines a triangle by three vertex indices.
type Face struct {
	A, B, C int // indices into Mesh3D.Vertices
}

// Mesh3D defines a 3D mesh with vertices, faces, and colors.
type Mesh3D struct {
	Vertices  []Vec3
	Faces     []Face
	FillColor color.Color

	// FaceColors overrides FillColor per-face (if len matches Faces).
	FaceColors []color.Color
}

// Camera3D defines the viewpoint for a 3D scene.
type Camera3D struct {
	Position Vec3    // camera position in world space
	Target   Vec3    // point the camera looks at
	Up       Vec3    // up direction (usually {0, 1, 0})
	FOV      float32 // field of view in degrees (default 45)
	Near     float32 // near clipping plane (default 0.1)
	Far      float32 // far clipping plane (default 1000)
}

// DefaultCamera returns a camera with sensible defaults.
func DefaultCamera() Camera3D {
	return Camera3D{
		Position: Vec3{3, 3, 5},
		Target:   Vec3{0, 0, 0},
		Up:       Vec3{0, 1, 0},
		FOV:      45,
		Near:     0.1,
		Far:      1000,
	}
}

// Light3D defines a directional light source.
type Light3D struct {
	Direction Vec3        // light direction (normalized)
	Color     color.Color // light color
	Ambient   float32     // ambient factor [0, 1]
}

// DefaultLight returns a light with sensible defaults.
func DefaultLight() Light3D {
	return Light3D{
		Direction: Vec3{-0.5, -1, -0.7},
		Color:     color.White,
		Ambient:   0.3,
	}
}

// Declare conformity with CanvasObject interface.
var _ core.CanvasObject = (*Scene3D)(nil)

// Scene3D is a canvas object that renders a 3D scene using software rasterization.
// It contains meshes, a camera, and lights, and produces a 2D image via perspective
// projection and z-buffering. It integrates with the Bellui rendering pipeline as a
// standard CanvasObject.
//
// Since: 1.0
type Scene3D struct {
	baseObject

	Camera Camera3D
	Meshes []Mesh3D
	Lights []Light3D

	// Background color for the scene (default transparent).
	Background color.Color

	// BackfaceCulling, when true, skips triangles whose normals face away from
	// the camera. When false (default), both sides of every triangle are rendered
	// with correct lighting (normals are flipped toward the viewer).
	BackfaceCulling bool
}

// NewScene3D creates a Scene3D with a default camera and light.
//
// Since: 1.0
func NewScene3D() *Scene3D {
	return &Scene3D{
		Camera: DefaultCamera(),
		Lights: []Light3D{DefaultLight()},
	}
}

// Hide will set this scene to not be visible.
func (s *Scene3D) Hide() {
	s.baseObject.Hide()
	repaint(s)
}

// Move the scene to a new position, relative to its parent / canvas.
func (s *Scene3D) Move(pos geo.Position) {
	if s.Position() == pos {
		return
	}
	s.baseObject.Move(pos)
	repaint(s)
}

// Refresh causes this scene to be redrawn.
func (s *Scene3D) Refresh() {
	Refresh(s)
}

// Resize updates the size of the scene.
func (s *Scene3D) Resize(size geo.Size) {
	if size == s.Size() {
		return
	}
	s.baseObject.Resize(size)
	repaint(s)
}

// AddMesh adds a mesh to the scene.
func (s *Scene3D) AddMesh(m Mesh3D) {
	s.Meshes = append(s.Meshes, m)
}

// Render produces an NRGBA image of the scene at the given pixel dimensions.
func (s *Scene3D) Render(width, height int) *image.NRGBA {
	if width <= 0 || height <= 0 {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	zbuf := make([]float32, width*height)
	for i := range zbuf {
		zbuf[i] = float32(math.MaxFloat32)
	}

	// Fill background
	if s.Background != nil {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				img.Set(x, y, s.Background)
			}
		}
	}

	// Build view and projection matrices
	cam := s.Camera
	if cam.FOV <= 0 {
		cam.FOV = 45
	}
	if cam.Near <= 0 {
		cam.Near = 0.1
	}
	if cam.Far <= 0 {
		cam.Far = 1000
	}

	aspect := float32(width) / float32(height)
	proj := PerspectiveMatrix(cam.FOV, aspect, cam.Near, cam.Far)
	view := LookAtMatrix(cam.Position, cam.Target, cam.Up)
	mvp := proj.Mul(view)

	hw := float32(width) / 2
	hh := float32(height) / 2

	// Light direction (normalized)
	light := DefaultLight()
	if len(s.Lights) > 0 {
		light = s.Lights[0]
	}
	lightDir := light.Direction.Normalize()

	// Collect and sort faces by depth (painter's algorithm)
	type renderFace struct {
		screenPts [3][2]float32
		depths    [3]float32
		avgDepth  float32
		color     color.Color
		normal    Vec3
	}

	var faces []renderFace

	for _, mesh := range s.Meshes {
		for fi, face := range mesh.Faces {
			if face.A >= len(mesh.Vertices) || face.B >= len(mesh.Vertices) || face.C >= len(mesh.Vertices) {
				continue
			}

			va := mesh.Vertices[face.A]
			vb := mesh.Vertices[face.B]
			vc := mesh.Vertices[face.C]

			// Project to homogeneous clip space (before the perspective divide).
			clip := [3]clipVert{mvp.mulClip(va), mvp.mulClip(vb), mvp.mulClip(vc)}

			// Clip the triangle against the near and far planes in clip space,
			// matching GL's per-edge geometry clipping. A vertex is inside the
			// depth range when -1 <= NDC z <= 1. A triangle straddling a plane is
			// re-tessellated into its visible polygon rather than dropped wholesale.
			poly := clipDepth(clip[:])
			if len(poly) < 3 {
				continue // entirely outside the near/far depth range
			}

			// Face normal for lighting (flat shading, world space).
			edge1 := vb.Sub(va)
			edge2 := vc.Sub(va)
			normal := edge1.Cross(edge2).Normalize()

			// Check if normal faces away from camera.
			viewDir := cam.Position.Sub(va).Normalize()
			if normal.Dot(viewDir) < 0 {
				if s.BackfaceCulling {
					continue // actually cull back-facing triangles
				}
				normal = normal.Scale(-1) // double-sided: flip normal toward viewer
			}

			faceColor := mesh.FillColor
			if fi < len(mesh.FaceColors) && mesh.FaceColors[fi] != nil {
				faceColor = mesh.FaceColors[fi]
			}

			// Perspective divide each clipped vertex to NDC, then to screen.
			pts := make([][2]float32, len(poly))
			depths := make([]float32, len(poly))
			for i, v := range poly {
				pts[i], depths[i] = v.toScreen(hw, hh)
			}

			// Fan-triangulate the (convex) clipped polygon.
			for i := 1; i+1 < len(poly); i++ {
				faces = append(faces, renderFace{
					screenPts: [3][2]float32{pts[0], pts[i], pts[i+1]},
					depths:    [3]float32{depths[0], depths[i], depths[i+1]},
					avgDepth:  (depths[0] + depths[i] + depths[i+1]) / 3,
					color:     faceColor,
					normal:    normal,
				})
			}
		}
	}

	// Sort back-to-front (painter's algorithm)
	sort.Slice(faces, func(i, j int) bool {
		return faces[i].avgDepth > faces[j].avgDepth
	})

	// Rasterize each face
	for _, f := range faces {
		// Compute lighting
		diffuse := -f.normal.Dot(lightDir)
		if diffuse < 0 {
			diffuse = 0
		}
		intensity := light.Ambient + (1-light.Ambient)*diffuse
		if intensity > 1 {
			intensity = 1
		}

		// Apply lighting to face color
		r, g, b, a := f.color.RGBA()
		litColor := color.NRGBA{
			R: uint8(float32(r>>8) * intensity),
			G: uint8(float32(g>>8) * intensity),
			B: uint8(float32(b>>8) * intensity),
			A: uint8(a >> 8),
		}

		rasterizeTriangle(img, zbuf, width, height,
			f.screenPts[0], f.screenPts[1], f.screenPts[2],
			f.depths[0], f.depths[1], f.depths[2],
			litColor)
	}

	return img
}

// --- Matrix math ---

// Mat4 is a 4x4 matrix in column-major order.
type Mat4 [16]float32

// PerspectiveMatrix builds a perspective projection matrix.
// fovDeg is the vertical field of view in degrees, aspect is width/height.
func PerspectiveMatrix(fovDeg, aspect, near, far float32) Mat4 {
	fovRad := fovDeg * math.Pi / 180
	f := float32(1.0 / math.Tan(float64(fovRad/2)))
	var m Mat4
	m[0] = f / aspect
	m[5] = f
	m[10] = (far + near) / (near - far)
	m[11] = -1
	m[14] = (2 * far * near) / (near - far)
	return m
}

// LookAtMatrix builds a view matrix from eye position, center/target, and up vector.
func LookAtMatrix(eye, center, up Vec3) Mat4 {
	f := center.Sub(eye).Normalize()
	s := f.Cross(up).Normalize()
	u := s.Cross(f)

	var m Mat4
	m[0] = s.X
	m[4] = s.Y
	m[8] = s.Z
	m[1] = u.X
	m[5] = u.Y
	m[9] = u.Z
	m[2] = -f.X
	m[6] = -f.Y
	m[10] = -f.Z
	m[12] = -s.Dot(eye)
	m[13] = -u.Dot(eye)
	m[14] = f.Dot(eye)
	m[15] = 1
	return m
}

// Mul multiplies two 4x4 matrices.
func (m Mat4) Mul(b Mat4) Mat4 {
	var r Mat4
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			for k := 0; k < 4; k++ {
				r[j*4+i] += m[k*4+i] * b[j*4+k]
			}
		}
	}
	return r
}

// MulVec3 transforms a Vec3 by this matrix (assumes w=1, performs perspective divide).
func (m Mat4) MulVec3(v Vec3) Vec3 {
	x := m[0]*v.X + m[4]*v.Y + m[8]*v.Z + m[12]
	y := m[1]*v.X + m[5]*v.Y + m[9]*v.Z + m[13]
	z := m[2]*v.X + m[6]*v.Y + m[10]*v.Z + m[14]
	w := m[3]*v.X + m[7]*v.Y + m[11]*v.Z + m[15]
	if w != 0 {
		x /= w
		y /= w
		z /= w
	}
	return Vec3{x, y, z}
}

// --- Depth clipping (clip space) ---

// clipVert is a homogeneous clip-space vertex, kept before the perspective
// divide so the depth planes can be clipped against linearly.
type clipVert struct {
	x, y, z, w float32
}

// mulClip transforms a Vec3 by this matrix into homogeneous clip space, keeping
// w (no perspective divide).
func (m Mat4) mulClip(v Vec3) clipVert {
	return clipVert{
		x: m[0]*v.X + m[4]*v.Y + m[8]*v.Z + m[12],
		y: m[1]*v.X + m[5]*v.Y + m[9]*v.Z + m[13],
		z: m[2]*v.X + m[6]*v.Y + m[10]*v.Z + m[14],
		w: m[3]*v.X + m[7]*v.Y + m[11]*v.Z + m[15],
	}
}

// toScreen applies the perspective divide and maps the resulting NDC point to
// pixel coordinates, returning the screen position and NDC depth.
func (v clipVert) toScreen(hw, hh float32) ([2]float32, float32) {
	inv := float32(1)
	if v.w != 0 {
		inv = 1 / v.w
	}
	ndcX := v.x * inv
	ndcY := v.y * inv
	ndcZ := v.z * inv
	return [2]float32{(ndcX + 1) * hw, (1 - ndcY) * hh}, ndcZ
}

// lerpClip linearly interpolates two clip-space vertices.
func lerpClip(a, b clipVert, t float32) clipVert {
	return clipVert{
		x: a.x + (b.x-a.x)*t,
		y: a.y + (b.y-a.y)*t,
		z: a.z + (b.z-a.z)*t,
		w: a.w + (b.w-a.w)*t,
	}
}

// distNear and distFar are the clip-space signed distances to the near and far
// planes. A vertex is inside the frustum's depth range when both are >= 0, i.e.
// -1 <= NDC z <= 1 (near plane w+z = 0, far plane w-z = 0).
func distNear(v clipVert) float32 { return v.w + v.z }
func distFar(v clipVert) float32  { return v.w - v.z }

// clipPlane clips a convex polygon (given as clip-space vertices, in winding
// order) against a single plane using the Sutherland–Hodgman algorithm. dist
// returns the signed distance of a vertex to the plane; a vertex is kept when
// dist >= 0. Returned intersection vertices lie exactly on the plane.
func clipPlane(in []clipVert, dist func(clipVert) float32) []clipVert {
	n := len(in)
	if n == 0 {
		return in
	}
	out := make([]clipVert, 0, n+1)
	for i := 0; i < n; i++ {
		cur := in[i]
		next := in[(i+1)%n]
		dCur := dist(cur)
		dNext := dist(next)
		if dCur >= 0 {
			out = append(out, cur)
		}
		// If the edge crosses the plane, emit the intersection point.
		if (dCur >= 0) != (dNext >= 0) {
			t := dCur / (dCur - dNext)
			out = append(out, lerpClip(cur, next, t))
		}
	}
	return out
}

// clipDepth clips a triangle (or convex polygon) against both the near and far
// planes, matching GL's per-edge depth clipping. A triangle straddling either
// plane is re-tessellated into its visible polygon rather than dropped wholesale.
// It returns the visible polygon — nil when fully outside the depth range, the
// original vertices when fully inside, otherwise a polygon of >= 3 vertices.
func clipDepth(in []clipVert) []clipVert {
	poly := clipPlane(in, distNear)
	if len(poly) < 3 {
		return nil
	}
	poly = clipPlane(poly, distFar)
	if len(poly) < 3 {
		return nil
	}
	return poly
}

// --- Triangle rasterizer ---

func rasterizeTriangle(img *image.NRGBA, zbuf []float32, w, h int,
	p0, p1, p2 [2]float32, z0, z1, z2 float32, c color.NRGBA,
) {
	// Bounding box
	minX := int(math.Floor(float64(min3f(p0[0], p1[0], p2[0]))))
	maxX := int(math.Ceil(float64(max3f(p0[0], p1[0], p2[0]))))
	minY := int(math.Floor(float64(min3f(p0[1], p1[1], p2[1]))))
	maxY := int(math.Ceil(float64(max3f(p0[1], p1[1], p2[1]))))

	if minX < 0 {
		minX = 0
	}
	if maxX >= w {
		maxX = w - 1
	}
	if minY < 0 {
		minY = 0
	}
	if maxY >= h {
		maxY = h - 1
	}

	// Edge function denominators
	denom := (p1[1]-p2[1])*(p0[0]-p2[0]) + (p2[0]-p1[0])*(p0[1]-p2[1])
	if denom == 0 {
		return
	}

	for py := minY; py <= maxY; py++ {
		for px := minX; px <= maxX; px++ {
			fx := float32(px) + 0.5
			fy := float32(py) + 0.5

			// Barycentric coordinates
			w0 := ((p1[1]-p2[1])*(fx-p2[0]) + (p2[0]-p1[0])*(fy-p2[1])) / denom
			w1 := ((p2[1]-p0[1])*(fx-p2[0]) + (p0[0]-p2[0])*(fy-p2[1])) / denom
			w2 := 1 - w0 - w1

			if w0 < 0 || w1 < 0 || w2 < 0 {
				continue
			}

			// Interpolate depth
			z := w0*z0 + w1*z1 + w2*z2
			idx := py*w + px
			if z < zbuf[idx] {
				zbuf[idx] = z
				img.SetNRGBA(px, py, c)
			}
		}
	}
}

func min3f(a, b, c float32) float32 {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func max3f(a, b, c float32) float32 {
	if a > b {
		if a > c {
			return a
		}
		return c
	}
	if b > c {
		return b
	}
	return c
}
