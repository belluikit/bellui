package webgpu

import (
	"image/color"
	"math"
	"testing"

	"github.com/belluikit/bellui/canvas"
)

// clipMulVec4 multiplies a column-major [16]float32 by (x,y,z,1) and returns the
// clip-space vec4 (no perspective divide).
func clipMulVec4(m [16]float32, x, y, z float32) (cx, cy, cz, cw float32) {
	cx = m[0]*x + m[4]*y + m[8]*z + m[12]
	cy = m[1]*x + m[5]*y + m[9]*z + m[13]
	cz = m[2]*x + m[6]*y + m[10]*z + m[14]
	cw = m[3]*x + m[7]*y + m[11]*z + m[15]
	return
}

// TestComputeScene3DMVPDepthRemap verifies the WebGPU MVP maps the view frustum's
// depth into [0, 1] (not GL's [-1, 1]): a point on the near plane lands at NDC
// z≈0 and one on the far plane at NDC z≈1, with both inside the clip volume. This
// is the core WebGPU-specific correctness — without the remap, near geometry has
// NDC z<0 and is wrongly clipped.
func TestComputeScene3DMVPDepthRemap(t *testing.T) {
	cam := canvas.Camera3D{
		Position: canvas.Vec3{X: 0, Y: 0, Z: 0},
		Target:   canvas.Vec3{X: 0, Y: 0, Z: -1},
		Up:       canvas.Vec3{X: 0, Y: 1, Z: 0},
		FOV:      90,
		Near:     1,
		Far:      10,
	}
	mvp := computeScene3DMVP(cam, 100, 100)

	// Near plane: world z = -1 (distance == Near).
	_, _, nz, nw := clipMulVec4(mvp, 0, 0, -1)
	nearNDC := nz / nw
	if math.Abs(float64(nearNDC)) > 1e-3 {
		t.Errorf("near-plane NDC z = %v, want ~0 (WebGPU [0,1] depth)", nearNDC)
	}

	// Far plane: world z = -10 (distance == Far).
	_, _, fz, fw := clipMulVec4(mvp, 0, 0, -10)
	farNDC := fz / fw
	if math.Abs(float64(farNDC-1)) > 1e-3 {
		t.Errorf("far-plane NDC z = %v, want ~1", farNDC)
	}

	// A mid-frustum point must be inside [0, 1] and nearer (smaller) than far.
	_, _, mz, mw := clipMulVec4(mvp, 0, 0, -3)
	midNDC := mz / mw
	if midNDC < 0 || midNDC > 1 {
		t.Errorf("mid NDC z = %v, want within [0,1]", midNDC)
	}
	if !(nearNDC < midNDC && midNDC < farNDC) {
		t.Errorf("depth not monotonic: near=%v mid=%v far=%v", nearNDC, midNDC, farNDC)
	}
}

// TestComputeScene3DMVPNoYFlip verifies clip Y is NOT inverted (unlike the GL
// path): a point above the camera target maps to positive NDC y (screen top),
// matching the top-down render-to-texture + V=0-at-top blit.
func TestComputeScene3DMVPNoYFlip(t *testing.T) {
	cam := canvas.Camera3D{
		Position: canvas.Vec3{X: 0, Y: 0, Z: 0},
		Target:   canvas.Vec3{X: 0, Y: 0, Z: -1},
		Up:       canvas.Vec3{X: 0, Y: 1, Z: 0},
		FOV:      90,
		Near:     0.1,
		Far:      100,
	}
	mvp := computeScene3DMVP(cam, 100, 100)
	_, cy, _, cw := clipMulVec4(mvp, 0, 1, -2) // world +Y, in front
	if cy/cw <= 0 {
		t.Errorf("world +Y mapped to NDC y = %v, want > 0 (no Y flip)", cy/cw)
	}
}

// TestBuildScene3DVerts checks the mesh expansion: 10 floats per vertex, 3
// vertices per face, a normalized normal facing the camera, and the face color.
func TestBuildScene3DVerts(t *testing.T) {
	scene := &canvas.Scene3D{
		Camera: canvas.Camera3D{Position: canvas.Vec3{X: 0, Y: 0, Z: 5}},
		Meshes: []canvas.Mesh3D{{
			Vertices: []canvas.Vec3{
				{X: -1, Y: -1, Z: 0},
				{X: 1, Y: -1, Z: 0},
				{X: 0, Y: 1, Z: 0},
			},
			Faces:     []canvas.Face{{A: 0, B: 1, C: 2}},
			FillColor: color.NRGBA{R: 255, G: 0, B: 0, A: 255},
		}},
	}
	verts := buildScene3DVerts(scene, scene.Camera.Position, false)
	if len(verts) != 30 {
		t.Fatalf("got %d floats, want 30 (1 face × 3 verts × 10)", len(verts))
	}

	// First vertex position == first mesh vertex.
	if verts[0] != -1 || verts[1] != -1 || verts[2] != 0 {
		t.Errorf("vertex 0 pos = (%v,%v,%v), want (-1,-1,0)", verts[0], verts[1], verts[2])
	}
	// Normal is unit length and faces the +Z camera (z component > 0).
	nx, ny, nz := verts[3], verts[4], verts[5]
	if l := math.Sqrt(float64(nx*nx + ny*ny + nz*nz)); math.Abs(l-1) > 1e-4 {
		t.Errorf("normal length = %v, want 1", l)
	}
	if nz <= 0 {
		t.Errorf("normal z = %v, want > 0 (facing camera at +Z)", nz)
	}
	// Color is opaque red.
	if verts[6] < 0.99 || verts[7] > 0.01 || verts[8] > 0.01 || verts[9] < 0.99 {
		t.Errorf("color = (%v,%v,%v,%v), want opaque red", verts[6], verts[7], verts[8], verts[9])
	}
}

// TestBuildScene3DVertsBackfaceCulling drops a face pointing away from the camera.
func TestBuildScene3DVertsBackfaceCulling(t *testing.T) {
	// Triangle with winding such that its geometric normal points away from a
	// camera at +Z (normal is -Z here: A,B,C are clockwise as seen from +Z).
	scene := &canvas.Scene3D{
		Camera: canvas.Camera3D{Position: canvas.Vec3{X: 0, Y: 0, Z: 5}},
		Meshes: []canvas.Mesh3D{{
			Vertices: []canvas.Vec3{
				{X: -1, Y: -1, Z: 0},
				{X: 0, Y: 1, Z: 0},
				{X: 1, Y: -1, Z: 0},
			},
			Faces:     []canvas.Face{{A: 0, B: 1, C: 2}},
			FillColor: color.White,
		}},
	}
	if v := buildScene3DVerts(scene, scene.Camera.Position, true); len(v) != 0 {
		t.Errorf("back-facing triangle not culled: got %d floats", len(v))
	}
	// Without culling the same face is kept (normal flipped toward the viewer).
	if v := buildScene3DVerts(scene, scene.Camera.Position, false); len(v) != 30 {
		t.Errorf("double-sided face dropped: got %d floats, want 30", len(v))
	}
}

// TestBuildScene3DVertsNoGeometry returns nil for an empty scene.
func TestBuildScene3DVertsNoGeometry(t *testing.T) {
	if v := buildScene3DVerts(&canvas.Scene3D{}, canvas.Vec3{}, false); v != nil {
		t.Errorf("empty scene = %v, want nil", v)
	}
}

// TestSceneUniforms checks the directional light flows into the std140 block.
func TestSceneUniforms(t *testing.T) {
	scene := &canvas.Scene3D{
		Camera: canvas.DefaultCamera(),
		Lights: []canvas.Light3D{{
			Direction: canvas.Vec3{X: 0, Y: 0, Z: -2}, // normalizes to (0,0,-1)
			Color:     color.NRGBA{R: 255, G: 128, B: 0, A: 255},
			Ambient:   0.25,
		}},
	}
	u := sceneUniforms(scene, 200, 100)
	if u.LightDir != [3]float32{0, 0, -1} {
		t.Errorf("light dir = %v, want (0,0,-1) normalized", u.LightDir)
	}
	if u.Ambient != 0.25 {
		t.Errorf("ambient = %v, want 0.25", u.Ambient)
	}
	if math.Abs(float64(u.LightColor[0]-1)) > 1e-3 || u.LightColor[2] > 1e-3 {
		t.Errorf("light color = %v, want ~(1, .5, 0)", u.LightColor)
	}
}
