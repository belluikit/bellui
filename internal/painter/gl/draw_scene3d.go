//go:build gl || (!darwin && !linux && !windows && !wasm)

package gl

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

func (p *painter) drawScene3D(scene *canvas.Scene3D, pos geo.Position, frame geo.Size) {
	size := scene.Size()
	if size.Width <= 0 || size.Height <= 0 {
		return
	}

	creator := func(_ core.CanvasObject) Texture {
		pixW := int(size.Width * p.pixScale)
		pixH := int(size.Height * p.pixScale)
		if pixW <= 0 {
			pixW = 1
		}
		if pixH <= 0 {
			pixH = 1
		}

		// Build expanded vertex buffer from all meshes.
		verts := buildScene3DVerts(scene, scene.Camera.Position, scene.BackfaceCulling)
		if len(verts) == 0 {
			// No geometry — render background only via software fallback.
			rendered := scene.Render(pixW, pixH)
			return p.imgToTexture(rendered, canvas.ImageScaleSmooth)
		}

		// Create FBO with a color texture. The depth buffer is attached by
		// attachSceneDepth, which is platform-specific: a depth texture on
		// desktop/mobile GL, but a depth renderbuffer on WebGL 1.0 where depth
		// textures need the WEBGL_depth_texture extension and would otherwise
		// leave the FBO incomplete (pre-rebrand tracker #136). The depth buffer is only used for
		// z-buffering this pass and is never sampled.
		fbo := p.ctx.CreateFramebuffer()

		colorTex := p.ctx.CreateTexture()
		p.ctx.ActiveTexture(texture0)
		p.ctx.BindTexture(texture2D, colorTex)
		p.ctx.TexImage2D(texture2D, 0, pixW, pixH, colorFormatRGBA, unsignedByte, nil)
		p.ctx.TexParameteri(texture2D, textureMinFilter, textureFilterToGL[0])
		p.ctx.TexParameteri(texture2D, textureMagFilter, textureFilterToGL[0])
		p.ctx.TexParameteri(texture2D, textureWrapS, clampToEdge)
		p.ctx.TexParameteri(texture2D, textureWrapT, clampToEdge)

		p.ctx.BindFramebuffer(glFramebuffer, fbo)
		p.ctx.FramebufferTexture2D(glFramebuffer, colorAttachment0, colorTex, 0)
		releaseDepth := p.attachSceneDepth(pixW, pixH)
		p.checkFBOComplete()
		p.logError()

		// An active clip (e.g. the chart widget's own scissor, or a Scroll
		// container's) is in window coordinates and would clip this FBO
		// render — suspend it until the offscreen pass is done.
		restoreScissor := p.suspendScissor()
		defer restoreScissor()

		// Render to FBO.
		p.ctx.Viewport(0, 0, pixW, pixH)
		defer func() {
			fullW := int(p.textureScale(frame.Width))
			fullH := int(p.textureScale(frame.Height))
			p.ctx.Viewport(0, 0, fullW, fullH)
		}()
		p.ctx.Enable(depthTest)

		// Clear with background color.
		if scene.Background != nil {
			r, g, b, a := scene.Background.RGBA()
			p.ctx.ClearColor(float32(r)/65535, float32(g)/65535, float32(b)/65535, float32(a)/65535)
		} else {
			p.ctx.ClearColor(0, 0, 0, 0)
		}
		p.ctx.Clear(bitColorBuffer | bitDepthBuffer)

		// Use scene3d program.
		program := p.scene3dProgram
		p.ctx.UseProgram(program.ref)

		// Upload vertex data.
		p.ctx.BindBuffer(arrayBuffer, program.buff)
		p.ctx.BufferData(arrayBuffer, verts, staticDraw)
		// Stride: 10 floats (pos:3 + normal:3 + color:4).
		p.UpdateVertexArray(program, "vert", 3, 10, 0)
		p.UpdateVertexArray(program, "normal", 3, 10, 3)
		p.UpdateVertexArray(program, "color", 4, 10, 6)

		// Compute and upload MVP. The scene renders into an FBO whose texture
		// is later drawn by the standard canvas-texture quad path, which
		// assumes V=0 at the top — but FBO textures have GL orientation
		// (Y=0 at bottom). Negate clip-space Y (row 1, column-major indices
		// 1/5/9/13) so the FBO content comes out pre-flipped and displays
		// right-side up. Winding is irrelevant: face culling happens CPU-side
		// in buildScene3DVerts and GL cull is never enabled here.
		mvp := computeScene3DMVP(scene.Camera, pixW, pixH)
		mvp[1], mvp[5], mvp[9], mvp[13] = -mvp[1], -mvp[5], -mvp[9], -mvp[13]
		p.SetUniformMatrix4fv(program, "mvp", mvp)

		// Upload lighting.
		light := canvas.DefaultLight()
		if len(scene.Lights) > 0 {
			light = scene.Lights[0]
		}
		dir := light.Direction.Normalize()
		p.SetUniform3f(program, "light_dir", dir.X, dir.Y, dir.Z)

		lr, lg, lb := lightColorToRGB(light.Color)
		p.SetUniform3f(program, "light_color", lr, lg, lb)

		p.SetUniform1f(program, "ambient", light.Ambient)

		// Disable blending for opaque 3D rendering (depth test handles occlusion).
		p.blendFunc(one, 0)

		// Draw all triangles.
		vertCount := len(verts) / 10
		p.ctx.DrawArrays(triangles, 0, vertCount)
		p.logError()

		// Restore state and the active render target (canvas/boundary FBO or
		// screen); the deferred viewport restore below handles an empty stack.
		p.ctx.Disable(depthTest)
		p.blendFunc(one, oneMinusSrcAlpha)
		p.rebindRenderTarget(0, 0)

		// Cleanup FBO and depth buffer (keep color texture for cache).
		// Viewport is restored by the deferred call above.
		p.ctx.DeleteFramebuffer(fbo)
		releaseDepth()

		return colorTex
	}

	p.drawTextureWithDetails(scene, creator, pos, size, frame, canvas.ImageFillStretch, 1.0, 0)
}

// buildScene3DVerts expands all scene meshes into a flat vertex buffer.
// Each triangle becomes 3 vertices with format: [posX, posY, posZ, normX, normY, normZ, r, g, b, a].
// Normals are corrected relative to the camera position so that both renderers
// (software and GL) produce identical lighting. When backfaceCulling is true,
// back-facing triangles are omitted instead of having their normals flipped.
// Returns nil if no geometry.
func buildScene3DVerts(scene *canvas.Scene3D, camPos canvas.Vec3, backfaceCulling bool) []float32 {
	// Count total faces for pre-allocation.
	totalFaces := 0
	for i := range scene.Meshes {
		totalFaces += len(scene.Meshes[i].Faces)
	}
	if totalFaces == 0 {
		return nil
	}

	// 3 vertices per face, 10 floats per vertex.
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

			// Face normal — skip degenerate (zero-area) faces whose cross
			// product is near-zero, which would produce a zero normal and
			// corrupt lighting calculations in the shader.
			edge1 := vb.Sub(va)
			edge2 := vc.Sub(va)
			cross := edge1.Cross(edge2)
			if cross.Length() < 1e-7 {
				continue
			}
			normal := cross.Normalize()

			// Check if normal faces away from camera (same logic as software renderer).
			viewDir := camPos.Sub(va).Normalize()
			if normal.Dot(viewDir) < 0 {
				if backfaceCulling {
					continue // cull back-facing triangle
				}
				normal = normal.Scale(-1) // double-sided: flip normal toward viewer
			}

			// Face color.
			faceColor := mesh.FillColor
			if fi < len(mesh.FaceColors) && mesh.FaceColors[fi] != nil {
				faceColor = mesh.FaceColors[fi]
			}
			cr, cg, cb, ca := colorToFloat(faceColor)

			// Emit 3 vertices.
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

// computeScene3DMVP builds the model-view-projection matrix from a Camera3D.
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
	mvp := proj.Mul(view)
	return [16]float32(mvp)
}

// colorToFloat converts a color.Color to RGBA float32 components in [0, 1].
func colorToFloat(c color.Color) (r, g, b, a float32) {
	if c == nil {
		return 0.5, 0.5, 0.5, 1.0
	}
	ri, gi, bi, ai := c.RGBA()
	if ai == 0 {
		return 0, 0, 0, 0
	}
	// RGBA() returns premultiplied values; convert to straight alpha.
	fa := float32(ai) / 65535
	return float32(ri) / 65535 / fa, float32(gi) / 65535 / fa, float32(bi) / 65535 / fa, fa
}

// lightColorToRGB extracts RGB components from a light color as float32 in [0, 1].
func lightColorToRGB(c color.Color) (r, g, b float32) {
	if c == nil {
		return 1, 1, 1
	}
	ri, gi, bi, _ := c.RGBA()
	return float32(ri) / 65535, float32(gi) / 65535, float32(bi) / 65535
}
