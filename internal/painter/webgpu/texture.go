//go:build !gl

package webgpu

import (
	"image"
	"image/draw"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/cache"
)

// Texture caching, mirroring internal/painter/gl/texture.go. cache.TextureType
// is a uint32 on desktop, so we store a small integer handle in the shared
// cache and keep the real *Texture in a painter-local registry keyed by that
// handle. This plugs the WebGPU painter into the existing dirty-texture
// lifecycle: canvas.FreeDirtyTextures drives cache eviction, which calls Free
// (objects) or the per-text free callback, releasing the GPU texture.

// registerTexture stores tex under a fresh non-zero handle and returns it.
func (p *painter) registerTexture(tex *Texture) uint32 {
	p.nextTexID++ // start at 1; 0 == cache.NoTexture
	id := p.nextTexID
	if p.textures == nil {
		p.textures = map[uint32]*Texture{}
	}
	p.textures[id] = tex
	return id
}

func (p *painter) releaseTexture(id uint32) {
	if tex, ok := p.textures[id]; ok {
		tex.Release()
		delete(p.textures, id)
	}
}

// createTexture rasterizes an object and uploads it to a persistent GPU texture.
func (p *painter) createTexture(o core.CanvasObject, raster func(core.CanvasObject) image.Image) *Texture {
	img := raster(o)
	if img == nil {
		return nil
	}
	tex, err := p.dev.CreateImageTexture(img)
	if err != nil {
		return nil
	}
	return tex
}

// updateStreamingTexture refreshes a streaming raster's cached texture in place,
// mirroring gl.updateStreamingTexture: same-size frames are re-uploaded with
// writeTexture (the WebGPU counterpart of TexSubImage2D), a size change drops
// the texture so getTexture reallocates it. Without this the texture created on
// the first paint would be cached forever and the raster would freeze (#34),
// because canvas.freeObject deliberately skips streaming rasters.
func (p *painter) updateStreamingTexture(obj *canvas.Raster) {
	id, ok := cache.GetTexture(obj)
	if !ok {
		return // no texture yet; getTexture creates one from the current pixels
	}
	tex := p.textures[uint32(id)]
	if tex == nil {
		return
	}

	// Pixels supplied by SetPixels: upload straight from the caller's buffer,
	// under StreamMu so SetPixels cannot swap it out mid-write. This is the
	// per-frame video path, so it must not copy.
	obj.StreamMu.Lock()
	pixW, pixH := obj.StreamPixW, obj.StreamPixH
	if pixW > 0 && pixH > 0 && len(obj.StreamPix) >= pixW*pixH*4 {
		if texW, texH := tex.Size(); texW == pixW && texH == pixH {
			_ = p.dev.WriteTexturePixels(tex, obj.StreamPix, pixW, pixH)
			obj.StreamMu.Unlock()
			return
		}
		obj.StreamMu.Unlock()
		p.dropTexture(obj, id, tex)
		return
	}
	obj.StreamMu.Unlock()

	// Generator path: rasterize outside the lock — it produces a fresh image,
	// so there is no race with SetPixels.
	rgba := p.streamingGeneratorPixels(obj)
	if rgba == nil {
		return
	}
	genW, genH := rgba.Rect.Dx(), rgba.Rect.Dy()
	if texW, texH := tex.Size(); texW != genW || texH != genH {
		p.dropTexture(obj, id, tex)
		return
	}
	_ = p.dev.WriteTexturePixels(tex, rgba.Pix, genW, genH)
}

// streamingGeneratorPixels runs a streaming raster's Generator and returns its
// output as premultiplied RGBA, or nil when there is nothing to upload.
func (p *painter) streamingGeneratorPixels(obj *canvas.Raster) *image.RGBA {
	if obj.Generator == nil {
		return nil
	}
	w := int(p.textureScale(obj.Size().Width))
	h := int(p.textureScale(obj.Size().Height))
	img := obj.Generator(w, h)
	if img == nil {
		return nil
	}
	rgba, ok := img.(*image.RGBA)
	if !ok {
		b := img.Bounds()
		rgba = image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(rgba, rgba.Rect, img, b.Min, draw.Over)
	}
	if len(rgba.Pix) == 0 {
		return nil
	}
	return rgba
}

// dropTexture releases a cached texture once the current frame is submitted —
// draws already recorded against it stay valid — and forgets it, so the next
// getTexture reallocates at the new size.
func (p *painter) dropTexture(obj core.CanvasObject, id cache.TextureType, tex *Texture) {
	p.dev.ReleaseTextureAfterFrame(tex)
	delete(p.textures, uint32(id))
	cache.DeleteTexture(obj)
}

// getTexture returns the cached texture for an object, creating + caching it on
// miss. Text is keyed by its font cache entry (so it re-rasterizes when content,
// size, style or colour change); other objects are keyed by identity.
func (p *painter) getTexture(o core.CanvasObject, raster func(core.CanvasObject) image.Image) (*Texture, bool) {
	if t, ok := o.(*canvas.Text); ok {
		custom := ""
		if t.FontSource != nil {
			custom = t.FontSource.Name()
		}
		ent := cache.NewFontCacheEntry(t.Text, t.TextSize, t.TextStyle, custom, p.canvas, t.Color)
		if id, ok := cache.GetTextTexture(ent); ok {
			if tex := p.textures[uint32(id)]; tex != nil {
				return tex, true
			}
		}
		tex := p.createTexture(o, raster)
		if tex == nil {
			return nil, false
		}
		id := p.registerTexture(tex)
		cache.SetTextTexture(ent, cache.TextureType(id), p.canvas, func() { p.releaseTexture(id) })
		return tex, true
	}

	if id, ok := cache.GetTexture(o); ok {
		if tex := p.textures[uint32(id)]; tex != nil {
			return tex, true
		}
	}
	tex := p.createTexture(o, raster)
	if tex == nil {
		return nil, false
	}
	id := p.registerTexture(tex)
	cache.SetTexture(o, cache.TextureType(id), p.canvas)
	return tex, true
}
