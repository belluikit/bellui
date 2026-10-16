//go:build !gl

package webgpu

import (
	"github.com/belluikit/webgpu/wgpu"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
)

// Scene3D for WebGPU (pre-rebrand tracker #224), porting internal/painter/gl/draw_scene3d.go. The GL
// path renders the mesh into an FBO with a depth texture then draws that color
// texture as a canvas quad; here we render into an off-screen color target with
// the shared depth-stencil attachment (hardware z-buffering via the depth-enabled
// scene3d pipeline) using the Device's render-target stack, then blit the result.
//
// The rendered color texture is cached on the scene object: an unchanged scene
// just re-blits, a Refresh frees the texture (standard cache lifecycle) forcing a
// re-render, and a resize (which does not invalidate the cache) is caught by the
// pixel-size stored alongside.

// sceneState caches a Scene3D's rendered color texture handle and the pixel size
// it was rendered at.
type sceneState struct {
	texID uint32
	pixW  int
	pixH  int
}

func (p *painter) drawScene3D(scene *canvas.Scene3D, pos geo.Position, frame geo.Size) {
	size := scene.Size()
	if size.Width <= 0 || size.Height <= 0 {
		return
	}
	tex, ok := p.sceneTexture(scene, size)
	if !ok {
		return
	}
	// Draw the rendered scene as a stretched canvas quad (top-down, no V-flip).
	p.blitTexture(tex.View(), size, pos, frame)
}

// scenePixSize returns the scene's pixel dimensions (at least 1x1).
func (p *painter) scenePixSize(size geo.Size) (int, int) {
	w := int(size.Width * p.pixScale)
	h := int(size.Height * p.pixScale)
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	return w, h
}

// sceneTexture returns the scene's rendered color texture, rendering (and
// caching) it on a miss. It self-heals against both a Refresh (the cache frees
// the texture, dropping it from p.textures) and a resize (the cached pixel size
// no longer matches), re-rendering in either case.
func (p *painter) sceneTexture(scene *canvas.Scene3D, size geo.Size) (*Texture, bool) {
	pixW, pixH := p.scenePixSize(size)

	if st, ok := p.scenes[scene]; ok {
		if st.pixW == pixW && st.pixH == pixH {
			if tex := p.textures[st.texID]; tex != nil {
				return tex, true // unchanged and still resident
			}
		}
		// Stale (resized) or freed by Refresh: drop the old texture + cache hook.
		p.releaseTexture(st.texID)
		cache.DeleteTexture(scene)
		delete(p.scenes, scene)
	}

	tex := p.renderScene(scene, pixW, pixH)
	if tex == nil {
		return nil, false
	}
	id := p.registerTexture(tex)
	cache.SetTexture(scene, cache.TextureType(id), p.canvas)
	if p.scenes == nil {
		p.scenes = map[core.CanvasObject]*sceneState{}
	}
	p.scenes[scene] = &sceneState{texID: id, pixW: pixW, pixH: pixH}
	return tex, true
}

// renderScene renders the scene's meshes into a fresh off-screen color texture
// with hardware z-buffering and returns it. With no geometry it falls back to the
// software renderer (background only), matching the GL path.
func (p *painter) renderScene(scene *canvas.Scene3D, pixW, pixH int) *Texture {
	verts := buildScene3DVerts(scene, scene.Camera.Position, scene.BackfaceCulling)
	if len(verts) == 0 {
		tex, err := p.dev.CreateImageTexture(scene.Render(pixW, pixH))
		if err != nil {
			return nil
		}
		return tex
	}

	target, err := p.dev.CreateRenderTexture(pixW, pixH)
	if err != nil {
		return nil
	}

	clear := wgpu.Color{}
	if scene.Background != nil {
		r, g, b, a := scene.Background.RGBA()
		clear = wgpu.Color{R: float64(r) / 0xffff, G: float64(g) / 0xffff, B: float64(b) / 0xffff, A: float64(a) / 0xffff}
	}

	// Render into the off-screen target (its own dedicated depth buffer, cleared
	// to far — never the canvas pass's shared one, pre-rebrand tracker #231), then pop back to the
	// enclosing pass so the caller can blit the result.
	p.dev.PushOffscreenTarget(target, pixW, pixH, clear)
	u := sceneUniforms(scene, pixW, pixH)
	p.recordScene3D(verts, uniformBytes(&u))
	p.dev.PopTarget()

	return target
}

// recordScene3D records the scene mesh draw: the scene3d shader with the
// depth-enabled pipeline (z-buffering) and Replace blend (opaque 3D, depth
// resolves occlusion — mirrors the GL painter's BlendFunc(one, 0)).
func (p *painter) recordScene3D(verts []float32, uniform []byte) {
	_ = p.dev.Record(DrawCall{
		Key: PipelineKey{
			Shader:   "scene3d",
			Fragment: FragmentEntry,
			Blend:    BlendReplace,
			Stencil:  StencilNone,
			Depth:    true,
			Format:   p.dev.Format(),
		},
		Vertices:    verts,
		VertexCount: len(verts) / 10,
		Uniform:     uniform,
	})
}
