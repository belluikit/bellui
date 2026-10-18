//go:build !gl

package webgpu

import (
	"image/color"
	"testing"

	"github.com/belluikit/webgpu/wgpu"
	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

// TestShadowKeyIgnoresPlacement pins what the #434 shadow cache shares: shadows
// that differ only in where they are drawn map to one texture, and anything
// that changes the blurred pixels maps to another.
func TestShadowKeyIgnoresPlacement(t *testing.T) {
	p := &painter{pixScale: 2, groupOpacity: 1}
	shadow := func(mod func(*canvas.BoxShadow)) *canvas.BoxShadow {
		s := canvas.NewBoxShadow(color.NRGBA{A: 80}, 6)
		s.CornerRadius = 12
		s.Resize(geo.NewSize(180, 200))
		if mod != nil {
			mod(s)
		}
		return s
	}
	base := p.shadowKeyFor(shadow(nil))

	moved := shadow(func(s *canvas.BoxShadow) {
		s.Move(geo.NewPos(400, 300))
		s.OffsetX, s.OffsetY = 3, 5
	})
	assert.Equal(t, base, p.shadowKeyFor(moved), "position and offset must not split the cache")

	for name, mod := range map[string]func(*canvas.BoxShadow){
		"color":  func(s *canvas.BoxShadow) { s.Color = color.NRGBA{A: 120} },
		"blur":   func(s *canvas.BoxShadow) { s.BlurRadius = 8 },
		"corner": func(s *canvas.BoxShadow) { s.CornerRadius = 4 },
		"size":   func(s *canvas.BoxShadow) { s.Resize(geo.NewSize(180, 201)) },
	} {
		assert.NotEqual(t, base, p.shadowKeyFor(shadow(mod)), name)
	}

	scaled := &painter{pixScale: 1, groupOpacity: 1}
	assert.NotEqual(t, base, scaled.shadowKeyFor(shadow(nil)), "pixel scale")
	faded := &painter{pixScale: 2, groupOpacity: 0.5}
	assert.NotEqual(t, base, faded.shadowKeyFor(shadow(nil)), "group opacity is baked into the shape")
}

// TestShadowTextureCacheHit: a cached shadow is returned without rendering
// (the device here has no GPU, so a render would panic) and marked used.
func TestShadowTextureCacheHit(t *testing.T) {
	p := &painter{dev: &Device{}, pixScale: 1, groupOpacity: 1, frame: 7}
	s := canvas.NewBoxShadow(color.Black, 4)
	s.Resize(geo.NewSize(50, 50))
	tex := &Texture{}
	p.shadows = map[shadowKey]*shadowEntry{p.shadowKeyFor(s): {tex: tex, used: 3}}

	assert.Same(t, tex, p.shadowTexture(s, 8, 66, 66, 66, 66))
	assert.Equal(t, uint64(7), p.shadows[p.shadowKeyFor(s)].used)
}

// TestEvictShadows: at the start of frame F, shadows drawn in frame F-1 are kept
// and older ones are released after the frame, not immediately (the previous
// frame's commands may still sample them).
func TestEvictShadows(t *testing.T) {
	kept, stale := &Texture{}, &Texture{}
	p := &painter{dev: &Device{}, frame: 10, shadows: map[shadowKey]*shadowEntry{
		{blur: 1}: {tex: kept, used: 9},
		{blur: 2}: {tex: stale, used: 8},
	}}
	p.evictShadows()

	assert.Len(t, p.shadows, 1)
	assert.Same(t, kept, p.shadows[shadowKey{blur: 1}].tex)
	assert.Equal(t, []*Texture{stale}, p.dev.frameTextures, "evicted texture is released after the frame")
}

// TestDeviceCulls pins the #434 skip: a draw is culled only when it lies wholly
// outside the clip the open pass applies, and never when no pass is open.
func TestDeviceCulls(t *testing.T) {
	d := &Device{}
	assert.False(t, d.Culls(-500, -500, 10, 10), "no pass open: nothing may be skipped")

	d.pass = &wgpu.RenderPassEncoder{} // only its presence is read
	d.passClip = scissorRect{x: 0, y: 100, w: 800, h: 400, active: true}
	for _, c := range []struct {
		name       string
		x, y, w, h int
		culled     bool
	}{
		{"inside", 10, 150, 50, 50, false},
		{"straddles the top edge", 10, 80, 50, 50, false},
		{"straddles the bottom edge", 10, 480, 50, 50, false},
		{"wholly above", 10, 20, 50, 80, true},
		{"wholly below", 10, 500, 50, 50, true},
		{"wholly right", 800, 150, 50, 50, true},
		{"wholly left", -60, 150, 60, 50, true},
		{"empty rect", 10, 150, 0, 50, true},
	} {
		assert.Equal(t, c.culled, d.Culls(c.x, c.y, c.w, c.h), c.name)
	}

	d.passClip = scissorRect{x: 5, y: 5, w: 0, h: 0, active: true} // a fully clipped-out viewport
	assert.True(t, d.Culls(0, 0, 100, 100), "an empty clip culls everything")
}
