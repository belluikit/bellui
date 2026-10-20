//go:build gl || (!darwin && !linux && !windows && !wasm)

package gl

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

// deleteRecorder counts texture deletes. Every other context method comes
// from the embedded nil interface and panics if called.
type deleteRecorder struct {
	context
	deleted int
}

// Texture is an integer on desktop and a struct on mobile and wasm, so the
// tests tell textures apart by their cache keys, not their values.
func (d *deleteRecorder) DeleteTexture(Texture) { d.deleted++ }

func (d *deleteRecorder) GetError() uint32 { return 0 }

// TestShadowKeyIgnoresPlacement pins what the #444 shadow cache shares: shadows
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

	p.currentTransform = geo.NewTranslation(40, 70)
	assert.Equal(t, base, p.shadowKeyFor(shadow(nil)), "a pure translation must not split the cache")
	p.currentTransform = geo.Identity()

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
	rotated := &painter{pixScale: 2, groupOpacity: 1, currentTransform: geo.NewRotation(0.3)}
	assert.NotEqual(t, base, rotated.shadowKeyFor(shadow(nil)), "the shape is drawn under the linear transform")
}

// TestShadowTextureCacheHit: a cached shadow is returned without rendering
// (the context here panics on any GL call) and marked used.
func TestShadowTextureCacheHit(t *testing.T) {
	p := &painter{ctx: &deleteRecorder{}, pixScale: 1, groupOpacity: 1, frame: 7}
	s := canvas.NewBoxShadow(color.Black, 4)
	s.Resize(geo.NewSize(50, 50))
	p.shadows = map[shadowKey]*shadowEntry{p.shadowKeyFor(s): {used: 3}}

	assert.NotPanics(t, func() { p.shadowTexture(s, 8, 66, 66, 66, 66, geo.NewSize(200, 200)) })
	assert.Equal(t, uint64(7), p.shadows[p.shadowKeyFor(s)].used)
}

// TestEvictShadows: at the start of frame F, shadows drawn in frame F-1 are
// kept and older ones are deleted.
func TestEvictShadows(t *testing.T) {
	ctx := &deleteRecorder{}
	p := &painter{ctx: ctx, frame: 10, shadows: map[shadowKey]*shadowEntry{
		{blur: 1}: {used: 9},
		{blur: 2}: {used: 8},
	}}
	p.evictShadows()

	assert.Equal(t, 1, p.CachedShadows())
	assert.Contains(t, p.shadows, shadowKey{blur: 1})
	assert.Equal(t, 1, ctx.deleted)
}

// TestReleaseCaches: releasing deletes every cached shadow, used or not, and
// leaves the cache empty for a painter that is initialised again.
func TestReleaseCaches(t *testing.T) {
	ctx := &deleteRecorder{}
	p := &painter{ctx: ctx, frame: 10, shadows: map[shadowKey]*shadowEntry{
		{blur: 1}: {used: 10},
		{blur: 2}: {used: 9},
	}}
	p.ReleaseCaches()

	assert.Zero(t, p.CachedShadows())
	assert.Equal(t, 2, ctx.deleted)

	// A window closed before its first frame has an uninitialised painter.
	assert.NotPanics(t, (&painter{}).ReleaseCaches)
}

// TestScissorCulls pins the #444 skip: a draw is culled only when it lies
// wholly outside the active scissor, and never when no scissor is active.
func TestScissorCulls(t *testing.T) {
	p := &painter{}
	assert.False(t, p.scissorCulls(-500, -500, 10, 10), "no scissor: nothing may be skipped")

	p.scissorActive = true
	p.scissor = [4]int32{0, 100, 800, 400}
	for _, c := range []struct {
		name       string
		x, y, w, h int
		culled     bool
	}{
		{"inside", 10, 150, 50, 50, false},
		{"straddles the bottom edge", 10, 80, 50, 50, false},
		{"straddles the top edge", 10, 480, 50, 50, false},
		{"meets the bottom edge: kept for rounding", 10, 50, 50, 50, false},
		{"wholly below", 10, 20, 50, 79, true},
		{"wholly above", 10, 501, 50, 50, true},
		{"wholly right", 801, 150, 50, 50, true},
		{"wholly left", -61, 150, 60, 50, true},
	} {
		assert.Equal(t, c.culled, p.scissorCulls(c.x, c.y, c.w, c.h), c.name)
	}

	p.scissor = [4]int32{5, 5, 0, 0}
	assert.True(t, p.scissorCulls(0, 0, 100, 100), "an empty scissor culls everything")
}
