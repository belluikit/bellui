package webgpu

// Backend-neutral geometry and uniform helpers for Scene3D (#202/#224), ported
// from internal/painter/gl/draw_scene3d.go. They are pure canvas/geo math (no
// wgpu binding) so they compile everywhere and unit-test without a GPU — the
// //go:build webgpu painter (draw_scene3d.go) drives the GPU render on top.
//
// The one WebGPU-specific divergence from the GL port lives in
// computeScene3DMVP: GL keeps clip-space depth in [-1, 1] (its native NDC) and
// negates clip Y to compensate for bottom-up FBO textures, whereas WebGPU clips
// depth to [0, 1] and renders to texture top-down. So here we remap the GL-style
// projection's depth to [0, 1] and do NOT flip Y.

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
)

// buildScene3DVerts expands all scene meshes into a flat interleaved vertex
// buffer matching the scene3d descriptor: 3 vertices per face, 10 floats each
// ([posX,posY,posZ, normX,normY,normZ, r,g,b,a]). Normals are corrected relative
// to the camera so lighting matches the software renderer; with backfaceCulling,
// back-facing triangles are dropped instead of having their normals flipped.
// Returns nil if there is no geometry. (Identical to the GL port — pure math.)
func buildScene3DVerts(scene *canvas.Scene3D, camPos canvas.Vec3, backfaceCulling bool) []float32 {
	totalFaces := 0
	for i := range scene.Meshes {
		totalFaces += len(scene.Meshes[i].Faces)
	}
	if totalFaces == 0 {
		return nil
	}

	verts := make([]float32, 0, totalFaces*30)

	for mi := range scene.Meshes {
		mesh := &scene.Meshes[mi]
		for fi, face := range mesh.Faces {
			if face.A >= len(mesh.Vertices) || face.B >= len(mesh.Vertices) || face.C >= len(mesh.Vertices) {
				continue
			}

			va := mesh.Vertices[face.A]
			vb := mesh.Vertices[face.B]
			vc := mesh.Vertices[face.C]

			// Face normal — skip degenerate (zero-area) faces whose cross product
			// is near-zero (a zero normal would corrupt the shader lighting).
			edge1 := vb.Sub(va)
			edge2 := vc.Sub(va)
			cross := edge1.Cross(edge2)
			if cross.Length() < 1e-7 {
				continue
			}
			normal := cross.Normalize()

			// Flip / cull faces pointing away from the camera (same logic as the
			// software renderer).
			viewDir := camPos.Sub(va).Normalize()
			if normal.Dot(viewDir) < 0 {
				if backfaceCulling {
					continue
				}
				normal = normal.Scale(-1)
			}

			faceColor := mesh.FillColor
			if fi < len(mesh.FaceColors) && mesh.FaceColors[fi] != nil {
				faceColor = mesh.FaceColors[fi]
			}
			cr, cg, cb, ca := colorToFloat(faceColor)

			verts = append(
				verts,
				va.X, va.Y, va.Z, normal.X, normal.Y, normal.Z, cr, cg, cb, ca,
				vb.X, vb.Y, vb.Z, normal.X, normal.Y, normal.Z, cr, cg, cb, ca,
				vc.X, vc.Y, vc.Z, normal.X, normal.Y, normal.Z, cr, cg, cb, ca,
			)
		}
	}

	return verts
}

// depthRemapZeroToOne converts a GL-style perspective projection (clip-space
// depth in [-1, 1]) into WebGPU/Metal/D3D-style [0, 1] depth by post-multiplying
// with z' = 0.5*z + 0.5*w. Without it, fragments near the camera (GL NDC z in
// [-1, 0)) fall outside WebGPU's clip volume and are wrongly discarded. Column-
// major (index = col*4 + row): row 2 picks up 0.5 from both the z and w columns.
var depthRemapZeroToOne = canvas.Mat4{
	0:  1,
	5:  1,
	10: 0.5,
	14: 0.5,
	15: 1,
}

// computeScene3DMVP builds the model-view-projection matrix from a Camera3D,
// remapped to WebGPU's [0, 1] clip-depth range. Unlike the GL path it does NOT
// negate clip Y: WebGPU renders into the off-screen texture top-down, so the
// later textured-quad blit (V=0 at the top) already shows the scene right-side up.
func computeScene3DMVP(cam canvas.Camera3D, pixW, pixH int) [16]float32 {
	if cam.FOV <= 0 {
		cam.FOV = 45
	}
	if cam.Near <= 0 {
		cam.Near = 0.1
	}
	if cam.Far <= 0 {
		cam.Far = 1000
	}

	aspect := float32(pixW) / float32(pixH)
	proj := canvas.PerspectiveMatrix(cam.FOV, aspect, cam.Near, cam.Far)
	view := canvas.LookAtMatrix(cam.Position, cam.Target, cam.Up)
	mvp := depthRemapZeroToOne.Mul(proj).Mul(view)
	return [16]float32(mvp)
}

// sceneUniforms populates the std140 Scene3DUniforms block for a scene: the
// depth-remapped MVP plus the first (or default) directional light.
func sceneUniforms(scene *canvas.Scene3D, pixW, pixH int) Scene3DUniforms {
	light := canvas.DefaultLight()
	if len(scene.Lights) > 0 {
		light = scene.Lights[0]
	}
	dir := light.Direction.Normalize()
	lr, lg, lb := lightColorToRGB(light.Color)
	return Scene3DUniforms{
		MVP:        computeScene3DMVP(scene.Camera, pixW, pixH),
		LightDir:   [3]float32{dir.X, dir.Y, dir.Z},
		LightColor: [3]float32{lr, lg, lb},
		Ambient:    light.Ambient,
	}
}

// colorToFloat converts a color to straight (non-premultiplied) RGBA in [0, 1].
func colorToFloat(c color.Color) (r, g, b, a float32) {
	if c == nil {
		return 0.5, 0.5, 0.5, 1.0
	}
	ri, gi, bi, ai := c.RGBA()
	if ai == 0 {
		return 0, 0, 0, 0
	}
	fa := float32(ai) / 65535
	return float32(ri) / 65535 / fa, float32(gi) / 65535 / fa, float32(bi) / 65535 / fa, fa
}

// lightColorToRGB extracts a light color's RGB components as float32 in [0, 1].
func lightColorToRGB(c color.Color) (r, g, b float32) {
	if c == nil {
		return 1, 1, 1
	}
	ri, gi, bi, _ := c.RGBA()
	return float32(ri) / 65535, float32(gi) / 65535, float32(bi) / 65535
}
