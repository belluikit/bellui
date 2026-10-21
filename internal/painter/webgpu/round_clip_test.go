//go:build !gl && !wasm

// (!wasm: these acquire a real WebGPU device; see canvas_fbo_test.go.)

package webgpu

import (
	"image"
	"image/color"
	"testing"

	"github.com/belluikit/webgpu/wgpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

const clipDim = 64

var (
	clipBlue = color.NRGBA{B: 255, A: 255}
	clipRed  = color.NRGBA{R: 255, A: 255}
)

// sizeCanvas is the least of a canvas the painter reads while drawing shapes.
type sizeCanvas struct {
	core.Canvas
	size geo.Size
}

func (c sizeCanvas) Size() geo.Size { return c.size }
func (sizeCanvas) Scale() float32   { return 1 }

// offscreenPainter is a painter drawing into a clipDim² off-screen texture, one
// frame at a time, so its clips can be checked from the pixels they leave.
type offscreenPainter struct {
	*painter
	target *Texture
	w, h   int
}

func newOffscreenPainter(t *testing.T) *offscreenPainter {
	t.Helper()
	return newOffscreenPainterSized(t, clipDim, clipDim)
}

// newOffscreenPainterSized is newOffscreenPainter drawing into a w×h texture.
func newOffscreenPainterSized(t *testing.T, w, h int) *offscreenPainter {
	t.Helper()
	instance := wgpu.CreateInstance(chooseInstanceBackends())
	t.Cleanup(instance.Release)
	adapter, err := instance.RequestAdapter(nil)
	if err != nil {
		t.Skipf("no WebGPU adapter available: %v", err)
	}
	t.Cleanup(adapter.Release)
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Skipf("no WebGPU device available: %v", err)
	}

	// d owns device and queue from here: Destroy releases them.
	d := &Device{
		device:        device,
		queue:         device.GetQueue(),
		modules:       map[string]*wgpu.ShaderModule{},
		depthStencils: map[[2]int]*depthStencil{},
		arenaMaxVtx:   64 * 1024,
		arenaMaxUni:   256 * uniformBlockStride,
		config:        &wgpu.SurfaceConfiguration{Format: wgpu.TextureFormatRGBA8Unorm, Width: uint32(w), Height: uint32(h)},
	}
	t.Cleanup(d.Destroy)
	// Linear, like the device's own sampler: the blur depends on it.
	d.sampler, err = device.TryCreateSampler(&wgpu.SamplerDescriptor{
		AddressModeU: wgpu.AddressModeClampToEdge, AddressModeV: wgpu.AddressModeClampToEdge,
		AddressModeW: wgpu.AddressModeClampToEdge, MagFilter: wgpu.FilterModeLinear,
		MinFilter: wgpu.FilterModeLinear, LodMaxClamp: 1, MaxAnisotropy: 1,
	})
	require.NoError(t, err)
	require.NoError(t, d.buildLayouts())
	d.pipelines = NewPipelineCache(d.buildPipeline)

	target, err := d.CreateRenderTexture(w, h)
	require.NoError(t, err)
	t.Cleanup(target.Release)

	p := &painter{
		dev: d, canvas: sizeCanvas{size: geo.NewSize(float32(w), float32(h))},
		pixScale: 1, texScale: 1, groupOpacity: 1, transform: geo.Identity(),
	}
	return &offscreenPainter{painter: p, target: target, w: w, h: h}
}

// frame clears the target to blue, runs draw, submits, and returns the pixels.
func (o *offscreenPainter) frame(t *testing.T, draw func()) *image.RGBA {
	t.Helper()
	d := o.dev
	d.arena.reset()
	d.texBindInUse, d.frameDemandVtx, d.frameDemandUni = 0, 0, 0
	require.NoError(t, d.ensureArenas())
	var err error
	d.encoder, err = d.device.TryCreateCommandEncoder(nil)
	require.NoError(t, err)
	d.targets = append(d.targets[:0], renderTarget{
		view: o.target.View(), tex: o.target.tex, w: o.w, h: o.h,
		clear: wgpu.Color{B: 1, A: 1},
	})
	d.stencilRef = 0
	d.beginTopPass()
	o.roundClips = o.roundClips[:0]

	draw()

	d.endPass()
	if n := d.arena.vtxLen(); n > 0 {
		d.queue.WriteBuffer(d.vtxArena, 0, d.arena.vtx[:n])
	}
	if n := d.arena.uniLen(); n > 0 {
		d.queue.WriteBuffer(d.uniArena, 0, d.arena.uni[:n])
	}
	cmd, err := d.encoder.TryFinish(nil)
	require.NoError(t, err)
	d.queue.Submit(cmd)
	cmd.Release()
	d.encoder.Release()
	d.encoder = nil

	img := d.CaptureTexture(o.target.tex, o.w, o.h)
	require.NotNil(t, img, "capture failed")
	return img
}

// fill paints the whole target red, through whatever clip is active.
func (o *offscreenPainter) fill() {
	r := canvas.NewRectangle(clipRed)
	r.Resize(geo.NewSquareSize(clipDim))
	o.drawRectangle(r, geo.Position{}, geo.NewSquareSize(clipDim))
}

func pixel(img *image.RGBA, x, y int) color.NRGBA {
	// The tests compare opaque pixels, whose RGBA and NRGBA bytes are equal.
	return color.NRGBA(img.RGBAAt(x, y))
}

// TestRoundedClipConfinesDraws checks the clip itself: a fill inside it lands
// inside the rounded rect only, and the stencil is zero again after Stop.
func TestRoundedClipConfinesDraws(t *testing.T) {
	o := newOffscreenPainter(t)
	var zeroAfter bool
	img := o.frame(t, func() {
		o.StartRoundedClipping(geo.NewPos(8, 8), geo.NewSquareSize(48), 12)
		o.fill()
		o.StopRoundedClipping()
		zeroAfter = o.dev.StencilZero()
	})
	assert.Equal(t, clipRed, pixel(img, 32, 32), "inside the clip")
	assert.Equal(t, clipBlue, pixel(img, 4, 32), "left of the clip")
	assert.Equal(t, clipBlue, pixel(img, 9, 9), "the clip's rounded corner")
	assert.True(t, zeroAfter, "Stop left the stencil not zero")
}

// TestRoundedClipLeavesNoStampBehind pins #466's hazard: the stencil is no
// longer cleared over the whole target per clip, so an earlier clip's stamp
// must be erased, or a later clip's content shows through it.
func TestRoundedClipLeavesNoStampBehind(t *testing.T) {
	o := newOffscreenPainter(t)
	img := o.frame(t, func() {
		o.StartRoundedClipping(geo.Position{}, geo.NewSize(32, clipDim), 0)
		o.StopRoundedClipping()
		o.StartRoundedClipping(geo.NewPos(32, 0), geo.NewSize(32, clipDim), 0)
		o.fill() // spills past the second clip, over where the first one stamped
		o.StopRoundedClipping()
	})
	assert.Equal(t, clipBlue, pixel(img, 16, 32), "content leaked through the first clip's stamp")
	assert.Equal(t, clipRed, pixel(img, 48, 32), "inside the second clip")
}

// TestRoundedClipAfterPathFill checks the fallback: a path fill leaves winding
// counts in the stencil, so the next clip must clear the whole target.
func TestRoundedClipAfterPathFill(t *testing.T) {
	o := newOffscreenPainter(t)
	green := color.NRGBA{G: 255, A: 255}
	img := o.frame(t, func() {
		path := canvas.NewPath(func(p *canvas.Path) {
			p.MoveTo(0, 0)
			p.LineTo(32, 0)
			p.LineTo(32, clipDim)
			p.LineTo(0, clipDim)
			p.Close()
		})
		path.FillColor = green
		o.drawPath(path, geo.Position{}, geo.NewSquareSize(clipDim))
		assert.False(t, o.dev.StencilZero(), "a path fill must mark the stencil not zero")

		o.StartRoundedClipping(geo.NewPos(32, 0), geo.NewSize(32, clipDim), 0)
		o.fill()
		o.StopRoundedClipping()
		assert.True(t, o.dev.StencilZero(), "a clip that cleared the whole target leaves it zero")
	})
	assert.Equal(t, green, pixel(img, 16, 32), "content leaked through the path fill's winding counts")
	assert.Equal(t, clipRed, pixel(img, 48, 32), "inside the clip")
}

// TestRoundedClipOutsideScissorIsSkipped checks that a clip lying wholly
// outside the scissor stamps nothing and still lets nothing through.
func TestRoundedClipOutsideScissorIsSkipped(t *testing.T) {
	o := newOffscreenPainter(t)
	var skipped bool
	img := o.frame(t, func() {
		o.StartClipping(geo.Position{}, geo.NewSize(32, clipDim))
		// The paint walk hands over world transforms carrying the object's
		// translation, which drawing ignores; only scale, rotation or skew
		// stop the skip.
		o.SetTransform(geo.NewTranslation(40, 0))
		o.StartRoundedClipping(geo.NewPos(40, 0), geo.NewSize(24, clipDim), 4)
		skipped = o.roundClips[len(o.roundClips)-1].skipped
		o.fill()
		o.StopRoundedClipping()
		o.StopClipping()
	})
	assert.True(t, skipped, "a clip outside the scissor was stamped")
	assert.Equal(t, clipBlue, pixel(img, 16, 32), "content of a skipped clip was drawn")
}

// TestNestedRoundedClips checks that a clip inside another still confines its
// content and both leave the stencil zero.
func TestNestedRoundedClips(t *testing.T) {
	o := newOffscreenPainter(t)
	var zeroAfter bool
	img := o.frame(t, func() {
		o.StartRoundedClipping(geo.Position{}, geo.NewSquareSize(clipDim), 0)
		o.StartRoundedClipping(geo.NewPos(16, 16), geo.NewSquareSize(32), 0)
		o.fill()
		o.StopRoundedClipping()
		o.StopRoundedClipping()
		zeroAfter = o.dev.StencilZero()
	})
	assert.Equal(t, clipRed, pixel(img, 32, 32), "inside the inner clip")
	assert.Equal(t, clipBlue, pixel(img, 4, 4), "outside the inner clip")
	assert.True(t, zeroAfter, "nested clips left the stencil not zero")
}
