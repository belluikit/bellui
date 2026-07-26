//go:build !gl

package webgpu

import (
	"image"

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
