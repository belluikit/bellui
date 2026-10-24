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

const batchDim = 96

// batchCanvas is the least of a canvas the painter reads while drawing.
type batchCanvas struct {
	core.Canvas
	size geo.Size
}

func (c batchCanvas) Size() geo.Size { return c.size }
func (batchCanvas) Scale() float32   { return 1 }

// batchPainter draws into a batchDim² off-screen texture, a frame at a time.
type batchPainter struct {
	*painter
	target *Texture
}

func newBatchPainter(t *testing.T) *batchPainter {
	t.Helper()
	instance := wgpu.CreateInstance(chooseInstanceBackends())
	t.Cleanup(instance.Release)
	adapter, err := instance.RequestAdapter(nil)
	if err != nil {
		noGPU(t, "no WebGPU adapter available: %v", err)
	}
	t.Cleanup(adapter.Release)
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		noGPU(t, "no WebGPU device available: %v", err)
	}
	d := &Device{
		device:        device,
		queue:         device.GetQueue(),
		modules:       map[string]*wgpu.ShaderModule{},
		depthStencils: map[[2]int]*depthStencil{},
		arenaMaxVtx:   64 * 1024,
		arenaMaxUni:   256 * uniformBlockStride,
		config:        &wgpu.SurfaceConfiguration{Format: wgpu.TextureFormatRGBA8Unorm, Width: batchDim, Height: batchDim},
	}
	t.Cleanup(d.Destroy)
	d.sampler, err = device.TryCreateSampler(&wgpu.SamplerDescriptor{
		AddressModeU: wgpu.AddressModeClampToEdge, AddressModeV: wgpu.AddressModeClampToEdge,
		AddressModeW: wgpu.AddressModeClampToEdge, MagFilter: wgpu.FilterModeLinear, MinFilter: wgpu.FilterModeLinear,
		LodMaxClamp: 1, MaxAnisotropy: 1,
	})
	require.NoError(t, err)
	require.NoError(t, d.buildLayouts())
	d.pipelines = NewPipelineCache(d.buildPipeline)
	target, err := d.CreateRenderTexture(batchDim, batchDim)
	require.NoError(t, err)
	t.Cleanup(target.Release)
	return &batchPainter{
		painter: &painter{
			dev: d, canvas: batchCanvas{size: geo.NewSquareSize(batchDim)},
			pixScale: 1, texScale: 1, groupOpacity: 1, transform: geo.Identity(),
			textures: map[uint32]*Texture{},
		},
		target: target,
	}
}

// frame clears the target to black, runs draw with batching on or off, and
// returns the pixels.
func (b *batchPainter) frame(t *testing.T, batched bool, draw func()) *image.RGBA {
	t.Helper()
	prev := batchEnabled
	batchEnabled = batched
	defer func() { batchEnabled = prev }()

	d := b.dev
	d.arena.reset()
	d.texBindInUse, d.frameDemandVtx, d.frameDemandUni = 0, 0, 0
	require.NoError(t, d.ensureArenas())
	var err error
	d.encoder, err = d.device.TryCreateCommandEncoder(nil)
	require.NoError(t, err)
	d.targets = append(d.targets[:0], renderTarget{
		view: b.target.View(), tex: b.target.tex, w: batchDim, h: batchDim,
		clear: wgpu.Color{A: 1},
	})
	d.stencilRef = 0
	d.beginTopPass()
	b.painter.frame++
	b.resetFullAtlas()
	b.framing = true
	defer func() { b.framing = false }()

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
	for _, tex := range d.frameTextures {
		tex.Release()
	}
	d.frameTextures = d.frameTextures[:0]

	img := d.CaptureTexture(b.target.tex, batchDim, batchDim)
	require.NotNil(t, img, "capture failed")
	return img
}

func batchFrame() geo.Size { return geo.NewSquareSize(batchDim) }

// diffPixels counts pixels whose channels differ by more than tol.
func diffPixels(a, b *image.RGBA, tol uint8) (n int) {
	for i := 0; i+3 < len(a.Pix); i += 4 {
		for c := 0; c < 3; c++ {
			d := int(a.Pix[i+c]) - int(b.Pix[i+c])
			if d < 0 {
				d = -d
			}
			if d > int(tol) {
				n++
				break
			}
		}
	}
	return n
}

// ink counts pixels that are not the black background.
func ink(img *image.RGBA) (n int) {
	for i := 0; i+3 < len(img.Pix); i += 4 {
		if img.Pix[i]|img.Pix[i+1]|img.Pix[i+2] > 24 {
			n++
		}
	}
	return n
}

// drawShapes draws one of each batched shape.
func (b *batchPainter) drawShapes() {
	rect := canvas.NewRectangle(color.NRGBA{R: 200, A: 255})
	rect.Resize(geo.NewSize(30, 20))
	b.drawRectangle(rect, geo.NewPos(4, 4), batchFrame())

	stroked := canvas.NewRectangle(color.NRGBA{G: 180, A: 255})
	stroked.StrokeColor = color.NRGBA{R: 255, G: 255, A: 255}
	stroked.StrokeWidth = 3
	stroked.Resize(geo.NewSize(30, 20))
	b.drawRectangle(stroked, geo.NewPos(44, 4), batchFrame())

	rounded := canvas.NewRectangle(color.NRGBA{B: 220, A: 255})
	rounded.CornerRadius = 8
	rounded.StrokeColor = color.NRGBA{R: 220, B: 220, A: 255}
	rounded.StrokeWidth = 2
	rounded.Resize(geo.NewSize(40, 30))
	b.drawRectangle(rounded, geo.NewPos(4, 40), batchFrame())

	circle := canvas.NewCircle(color.NRGBA{R: 240, G: 160, A: 255})
	circle.Resize(geo.NewSize(30, 30))
	b.drawCircle(circle, geo.NewPos(54, 44), batchFrame())
}

// TestBatchedShapesMatchUnbatched checks that a shape drawn as an instance
// covers what the per-object shaders draw: the instance shader computes the
// same distance to the edges, so only anti-aliased edge pixels may differ.
func TestBatchedShapesMatchUnbatched(t *testing.T) {
	b := newBatchPainter(t)
	off := b.frame(t, false, b.drawShapes)
	on := b.frame(t, true, b.drawShapes)
	require.Greater(t, ink(off), 2000, "the shapes drew nothing")
	assert.Zero(t, diffPixels(off, on, 48), "batched shapes differ from per-object ones beyond edge anti-aliasing")
	assert.Less(t, diffPixels(off, on, 2), ink(off)/10, "too many pixels differ at all")
}

// TestBatchedShapesAreOneDraw checks the point of batching: consecutive shapes
// reach the pass as one instanced draw.
func TestBatchedShapesAreOneDraw(t *testing.T) {
	b := newBatchPainter(t)
	b.frame(t, true, func() {
		b.drawShapes()
		assert.Equal(t, 4, b.dev.batchN, "the four shapes should wait in one batch")
	})
	assert.Zero(t, b.dev.batchN, "the frame's end must draw the batch")
}

// TestBatchKeepsDrawOrder checks that a draw outside the batch still lands
// between the instances before and after it: the batch is drawn first.
func TestBatchKeepsDrawOrder(t *testing.T) {
	b := newBatchPainter(t)
	img := b.frame(t, true, func() {
		red := canvas.NewRectangle(color.NRGBA{R: 255, A: 255})
		red.Resize(geo.NewSize(60, 60))
		b.drawRectangle(red, geo.NewPos(0, 0), batchFrame())

		// A gradient is drawn as a texture, outside the batch.
		green := canvas.NewHorizontalGradient(color.NRGBA{G: 255, A: 255}, color.NRGBA{G: 255, A: 255})
		green.Resize(geo.NewSize(90, 20))
		b.Paint(green, geo.NewPos(0, 20), batchFrame())

		blue := canvas.NewRectangle(color.NRGBA{B: 255, A: 255})
		blue.Resize(geo.NewSize(20, 60))
		b.drawRectangle(blue, geo.NewPos(40, 0), batchFrame())
	})
	at := func(x, y int) color.RGBA { return img.RGBAAt(x, y) }
	assert.Equal(t, uint8(255), at(10, 10).R, "red, under nothing")
	assert.Equal(t, uint8(255), at(10, 30).G, "the gradient, outside the batch, drew over the red batch")
	assert.Equal(t, uint8(255), at(50, 30).B, "blue, batched after the line, drew over it")
}

// TestBatchedTextDraws checks that atlas text draws about as much ink as the
// texture path for the same text.
func TestBatchedTextDraws(t *testing.T) {
	b := newBatchPainter(t)
	draw := func() {
		text := canvas.NewText("Batched 0123", color.White)
		text.TextSize = 16
		text.Resize(text.MinSize())
		b.drawText(text, geo.NewPos(4, 30), batchFrame())
	}
	off := b.frame(t, false, draw)
	on := b.frame(t, true, draw)
	require.Greater(t, ink(off), 100, "the text drew nothing")
	assert.InEpsilon(t, ink(off), ink(on), 0.15, "atlas text should cover about as many pixels")
	assert.NotEmpty(t, b.batch.atlas.entries, "the text should have gone through the atlas")
}

// TestFullAtlasFallsBackAndResets checks that text which no longer fits the
// atlas still draws, by the texture path, and that the next frame starts the
// atlas over.
func TestFullAtlasFallsBackAndResets(t *testing.T) {
	b := newBatchPainter(t)
	text := canvas.NewText("Full", color.White)
	text.TextSize = 16
	text.Resize(text.MinSize())
	draw := func() {
		require.True(t, b.ensureAtlas())
		a := &b.batch.atlas
		a.shelfX, a.shelfY = 0, atlasSize // no room left for a new glyph
		b.drawText(text, geo.NewPos(4, 30), batchFrame())
	}
	img := b.frame(t, true, draw)
	assert.Greater(t, ink(img), 50, "text that does not fit the atlas must still draw")
	assert.True(t, b.batch.atlas.full)

	b.frame(t, true, func() {})
	assert.False(t, b.batch.atlas.full, "a new frame starts a full atlas over")
	assert.Empty(t, b.batch.atlas.entries)
}
