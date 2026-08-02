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

func TestVideoSurfaceStreamingAlwaysOn(t *testing.T) {
	s := newVideoSurface(320, 240)
	// Both GPU painters implement the in-place streaming update (GL:
	// TexSubImage2D; WebGPU since #34: WriteTexturePixels), and the software
	// painter treats Streaming as a no-op — so it is set unconditionally
	// (#100). The old per-painter constant guarded against a WebGPU freeze
	// that #34 fixed.
	if !s.raster.Streaming {
		t.Error("Streaming = false, want true under every painter (#100)")
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
// tests could not previously see. Both GPU painters implement the
// Streaming/SetPixels fast path, but the WebGPU painter still calls Generator
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

// TestVideoSurfaceResizeTakesTheNewGeometry covers a capture device switch to a
// camera of a different resolution (#96). Buffers left at the old size would
// truncate a larger frame in the copy, and the generator would keep describing
// the old rectangle over the new pixels.
func TestVideoSurfaceResizeTakesTheNewGeometry(t *testing.T) {
	s := newVideoSurface(2, 2)
	small := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for i := range small.Pix {
		small.Pix[i] = 0xAA
	}
	s.WriteFrame(small)

	s.resize(4, 3)

	if len(s.buf[0]) != 4*3*4 || len(s.buf[1]) != 4*3*4 {
		t.Fatalf("buffer sizes = %d/%d, want %d for 4x3 RGBA",
			len(s.buf[0]), len(s.buf[1]), 4*3*4)
	}
	// Both buffers are replaced, so nothing of the previous camera survives into
	// a frame from the new one.
	for i, b := range s.buf {
		for _, px := range b {
			if px != 0 {
				t.Fatalf("buffer %d still holds pixels from the previous geometry", i)
			}
		}
	}

	large := image.NewRGBA(image.Rect(0, 0, 4, 3))
	for i := range large.Pix {
		large.Pix[i] = 0x5C
	}
	s.WriteFrame(large)

	img := s.raster.Generator(4, 3)
	rgba, ok := img.(*image.RGBA)
	if !ok {
		t.Fatalf("got %T, want *image.RGBA", img)
	}
	if got := rgba.Bounds(); got.Dx() != 4 || got.Dy() != 3 {
		t.Errorf("generator bounds = %v, want 4x3", got)
	}
	if rgba.Stride != 4*4 {
		t.Errorf("stride = %d, want %d — a stale stride shears the image", rgba.Stride, 4*4)
	}
	// The last row proves the whole frame arrived rather than being truncated
	// against a buffer sized for 2x2.
	if last := rgba.Pix[len(rgba.Pix)-1]; last != 0x5C {
		t.Errorf("last pixel = %#x, want the new frame throughout", last)
	}
}

// TestVideoSurfaceResizeIsANoOpAtTheSameSize: two cameras of the same
// resolution are the common switch, and it should cost nothing.
func TestVideoSurfaceResizeIsANoOpAtTheSameSize(t *testing.T) {
	s := newVideoSurface(2, 2)
	frame := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for i := range frame.Pix {
		frame.Pix[i] = 0x3B
	}
	s.WriteFrame(frame)

	s.resize(2, 2)

	if got := s.raster.Generator(2, 2).(*image.RGBA).Pix[0]; got != 0x3B {
		t.Errorf("pixel = %#x after a same-size resize, want the frame kept", got)
	}
}

// TestVideoSurfaceResizeRejectsDegenerateSizes: a decoder that has not settled
// its geometry reports zeroes, and honouring that would leave a surface with no
// buffers at all.
func TestVideoSurfaceResizeRejectsDegenerateSizes(t *testing.T) {
	s := newVideoSurface(2, 2)
	for _, wh := range [][2]int{{0, 4}, {4, 0}, {-1, 4}} {
		s.resize(wh[0], wh[1])
		if s.width != 2 || s.height != 2 {
			t.Fatalf("resize(%d, %d) changed the geometry to %dx%d",
				wh[0], wh[1], s.width, s.height)
		}
	}
}
