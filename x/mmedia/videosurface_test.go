package mmedia

import (
	"image"
	"os"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// TestMain starts a test app so bellui.Do has a driver to queue onto. Without
// one it panics with "Attempt to access current Bellui app when none is started",
// which is also what a library consumer would hit if a frame arrived before the
// app was running.
func TestMain(m *testing.M) {
	test.NewApp()
	os.Exit(m.Run())
}

func TestAspectLayoutLetterboxes(t *testing.T) {
	// 16:9 video in a 4:3 area — bars top and bottom, full width.
	l := &aspectLayout{width: 1920, height: 1080}
	obj := canvas.NewRaster(nil)
	l.Layout([]core.CanvasObject{obj}, geo.NewSize(800, 600))

	if got := obj.Size(); got.Width != 800 || got.Height != 450 {
		t.Errorf("size = %v, want 800x450", got)
	}
	if got := obj.Position(); got.X != 0 || got.Y != 75 {
		t.Errorf("position = %v, want (0,75) — centred vertically", got)
	}
}

func TestAspectLayoutPillarboxes(t *testing.T) {
	// 4:3 video in a 16:9 area — bars left and right, full height.
	l := &aspectLayout{width: 640, height: 480}
	obj := canvas.NewRaster(nil)
	l.Layout([]core.CanvasObject{obj}, geo.NewSize(1600, 900))

	if got := obj.Size(); got.Width != 1200 || got.Height != 900 {
		t.Errorf("size = %v, want 1200x900", got)
	}
	if got := obj.Position(); got.X != 200 || got.Y != 0 {
		t.Errorf("position = %v, want (200,0) — centred horizontally", got)
	}
}

func TestAspectLayoutExactMatchFills(t *testing.T) {
	l := &aspectLayout{width: 1920, height: 1080}
	obj := canvas.NewRaster(nil)
	l.Layout([]core.CanvasObject{obj}, geo.NewSize(1280, 720))

	if got := obj.Size(); got.Width != 1280 || got.Height != 720 {
		t.Errorf("size = %v, want 1280x720 (no bars)", got)
	}
}

// TestAspectLayoutDegenerate guards the paths that would divide by zero.
func TestAspectLayoutDegenerate(t *testing.T) {
	obj := canvas.NewRaster(nil)
	for _, tc := range []struct {
		name string
		l    *aspectLayout
		size geo.Size
	}{
		{"zero video size", &aspectLayout{0, 0}, geo.NewSize(800, 600)},
		{"zero area", &aspectLayout{1920, 1080}, geo.NewSize(0, 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.l.Layout([]core.CanvasObject{obj}, tc.size) // must not panic
		})
	}
	// No objects at all.
	(&aspectLayout{1920, 1080}).Layout(nil, geo.NewSize(800, 600))
}

// TestVideoSurfaceAlternatesBuffers is the core invariant: SetPixels stores the
// slice by reference and the painter reads it later on the GL thread, so two
// consecutive frames must never land in the same backing array.
func TestVideoSurfaceAlternatesBuffers(t *testing.T) {
	s := newVideoSurface(4, 4)
	frame := image.NewRGBA(image.Rect(0, 0, 4, 4))

	seen := make([]*byte, 0, 4)
	for i := 0; i < 4; i++ {
		for j := range frame.Pix {
			frame.Pix[j] = byte(i + 1)
		}
		s.WriteFrame(frame)
		seen = append(seen, &s.raster.StreamPix[0])
	}

	for i := 1; i < len(seen); i++ {
		if seen[i] == seen[i-1] {
			t.Fatalf("frames %d and %d share a backing buffer — the painter "+
				"could still be uploading the one being overwritten", i-1, i)
		}
	}
	// Only two buffers, so it must alternate rather than keep allocating.
	if seen[0] != seen[2] || seen[1] != seen[3] {
		t.Error("expected the two buffers to alternate")
	}
}

func TestVideoSurfaceCopiesPixels(t *testing.T) {
	s := newVideoSurface(2, 2)
	frame := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for i := range frame.Pix {
		frame.Pix[i] = 0xAB
	}
	s.WriteFrame(frame)

	if got := s.raster.StreamPix; got[0] != 0xAB || got[len(got)-1] != 0xAB {
		t.Errorf("pixels not copied into the stream buffer: %v", got[:4])
	}
	// Mutating the source afterwards must not affect what was handed over —
	// the pipeline recycles that frame immediately.
	for i := range frame.Pix {
		frame.Pix[i] = 0
	}
	if s.raster.StreamPix[0] != 0xAB {
		t.Error("stream buffer aliases the caller's frame instead of copying it")
	}
}

func TestVideoSurfaceStreamingMatchesPainter(t *testing.T) {
	s := newVideoSurface(320, 240)
	// Streaming is only safe on the GL painter. Under WebGPU it must stay off:
	// Canvas.freeObject skips releasing a streaming raster's texture, so the
	// video would freeze on its first frame.
	if s.raster.Streaming != streamingSupported {
		t.Errorf("Streaming = %v, want %v for this painter backend",
			s.raster.Streaming, streamingSupported)
	}
	if s.raster.StreamPixW != 320 || s.raster.StreamPixH != 240 {
		t.Errorf("stream dims = %dx%d, want 320x240", s.raster.StreamPixW, s.raster.StreamPixH)
	}
}

func TestVideoSurfaceHandlesNilAndZeroSize(t *testing.T) {
	s := newVideoSurface(0, 0) // falls back to a default rather than a 0-byte buffer
	if len(s.raster.StreamPix) == 0 {
		t.Error("expected a non-empty default buffer")
	}
	s.WriteFrame(nil) // must not panic
}

// TestVideoSurfaceGeneratorNotNil is a regression test for a crash that unit
// tests could not previously see. Only the GL painter implements the
// Streaming/SetPixels fast path; the WebGPU painter calls Generator
// unconditionally in rasterizeRaster, so a nil Generator segfaults on the
// first paint. The gallery runs WebGPU, and it crashed there.
func TestVideoSurfaceGeneratorNotNil(t *testing.T) {
	s := newVideoSurface(320, 240)
	if s.raster.Generator == nil {
		t.Fatal("Generator is nil — the WebGPU painter dereferences it unconditionally")
	}
}

// TestVideoSurfaceGeneratorReturnsFrame drives the exact call the WebGPU
// painter makes and checks it yields the current frame.
func TestVideoSurfaceGeneratorReturnsFrame(t *testing.T) {
	s := newVideoSurface(4, 4)
	frame := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for i := range frame.Pix {
		frame.Pix[i] = 0x7F
	}
	s.WriteFrame(frame)

	// Mirrors painter.rasterizeRaster: Generator(scaledW, scaledH).
	img := s.raster.Generator(8, 8)
	if img == nil {
		t.Fatal("Generator returned nil")
	}
	if b := img.Bounds(); b.Dx() != 4 || b.Dy() != 4 {
		t.Errorf("bounds = %v, want 4x4 (native frame size)", b)
	}
	rgba, ok := img.(*image.RGBA)
	if !ok {
		t.Fatalf("got %T, want *image.RGBA", img)
	}
	if rgba.Pix[0] != 0x7F {
		t.Errorf("generator returned stale pixels: %#x", rgba.Pix[0])
	}
}

// TestVideoSurfaceGeneratorSeesLatestFrame checks the generator follows the
// buffer that was most recently written rather than a fixed one.
func TestVideoSurfaceGeneratorSeesLatestFrame(t *testing.T) {
	s := newVideoSurface(2, 2)
	frame := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for gen := byte(1); gen <= 4; gen++ {
		for i := range frame.Pix {
			frame.Pix[i] = gen
		}
		s.WriteFrame(frame)
		got := s.raster.Generator(2, 2).(*image.RGBA).Pix[0]
		if got != gen {
			t.Fatalf("after writing frame %d the generator returned %d", gen, got)
		}
	}
}
