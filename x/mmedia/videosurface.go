package mmedia

import (
	"image"
	"sync"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// videoSurface renders decoded frames through a streaming canvas.Raster.
//
// A Raster with Streaming set updates its GPU texture in place on each
// refresh (TexSubImage2D on the GL painter, Device.WriteTexturePixels on
// WebGPU), rather than recreating the texture the way the generic
// canvas.Image path does — the painters call this "the fastest path for
// video frame updates".
//
// The pixel buffers are double-buffered. SetPixels stores the slice by
// reference and the painter reads it later on the GL thread, so writing the
// next frame into the buffer it is still uploading would tear. Alternating
// between two buffers means the one being written is never the one the
// painter holds.
type videoSurface struct {
	raster *canvas.Raster

	mu   sync.Mutex
	buf  [2][]byte
	next int
	cur  int // buffer holding the most recently written frame

	width, height int
}

func newVideoSurface(width, height int) *videoSurface {
	if width <= 0 {
		width = 640
	}
	if height <= 0 {
		height = 360
	}

	s := &videoSurface{width: width, height: height}
	for i := range s.buf {
		s.buf[i] = make([]byte, width*height*4)
	}

	// A Generator is mandatory, not merely a fallback: the WebGPU painter
	// calls it unconditionally in rasterizeRaster, so a nil one segfaults on
	// the first paint.
	//
	// Streaming is unconditional: both GPU painters implement the in-place
	// update (GL: TexSubImage2D; WebGPU since #34: updateStreamingTexture /
	// WriteTexturePixels — the freeze the old per-painter constant guarded
	// against is exactly what #34 fixed, see #100), and the software painter
	// re-draws the current pixels each paint, so Streaming is a no-op there.
	// Verified on-GPU under both painters via hack/rasterstream and
	// x/mmedia/hack/playprobe -auto.
	s.raster = canvas.NewRaster(s.generate)
	s.raster.Streaming = true
	s.raster.ScaleMode = canvas.ImageScaleSmooth
	// Prime the texture so the first paint has something to upload into.
	// Set directly rather than via publish: there is no canvas yet, and this
	// runs on whichever goroutine constructed the widget.
	s.raster.StreamPix = s.buf[0]
	s.raster.StreamPixW = width
	s.raster.StreamPixH = height
	s.next = 1

	return s
}

// resize rebuilds the pixel buffers for a new frame geometry, which a capture
// device switch can bring: the replacement camera is under no obligation to
// match the previous one's resolution.
//
// Both buffers are replaced rather than one, and the raster is repointed at the
// fresh one immediately. Leaving the painter holding a buffer of the old size
// while WriteFrame filled a new one would draw a torn frame at best; leaving the
// buffers at the old size would truncate a larger frame in the copy, or leave a
// tail of the previous camera's pixels below a smaller one.
//
// A no-op when the geometry has not moved, which is the common case for a switch
// between two cameras of the same resolution.
func (s *videoSurface) resize(width, height int) {
	if width <= 0 || height <= 0 {
		return
	}

	s.mu.Lock()
	if width == s.width && height == s.height {
		s.mu.Unlock()
		return
	}
	s.width, s.height = width, height
	for i := range s.buf {
		s.buf[i] = make([]byte, width*height*4)
	}
	s.cur, s.next = 0, 1
	first := s.buf[0]
	s.mu.Unlock()

	// Hand the painter a correctly sized buffer before the next frame arrives.
	// It is blank, so the switch shows one empty frame rather than a garbled
	// one — the streaming raster drops its texture on a size change anyway.
	s.publish(first)
}

// generate returns an image over the most recently written frame. It is called
// on the paint thread; double buffering keeps it off whichever buffer
// WriteFrame is filling. The requested size is ignored — the frame is returned
// at native resolution and the painter scales it into the object's rect.
func (s *videoSurface) generate(_, _ int) image.Image {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &image.RGBA{
		Pix:    s.buf[s.cur],
		Stride: s.width * 4,
		Rect:   image.Rect(0, 0, s.width, s.height),
	}
}

// WriteFrame copies frame into the next buffer and hands it to the raster.
// Safe to call from the decode goroutine: SetPixels takes the raster's own
// lock, and the copy targets a buffer the painter is not reading.
func (s *videoSurface) WriteFrame(frame *image.RGBA) {
	if frame == nil {
		return
	}
	s.mu.Lock()
	idx := s.next
	buf := s.buf[idx]
	s.next ^= 1
	n := copy(buf, frame.Pix)
	if n > 0 {
		s.cur = idx
	}
	s.mu.Unlock()
	if n == 0 {
		return
	}
	s.publish(buf)
}

// publish hands a filled buffer to the raster and asks for a repaint.
//
// It deliberately does not call Raster.SetPixels. That method sets the stream
// fields and then calls Refresh — a UI operation — without marshalling to the
// main thread, unlike Button.SetIcon or Label.SetText. Since WriteFrame runs on
// the decode goroutine, calling it there trips bellui's thread checker.
//
// Splitting the two keeps the pixel handoff synchronous under the mutex the
// painter itself uses, while queueing only the repaint.
func (s *videoSurface) publish(buf []byte) {
	s.raster.StreamMu.Lock()
	s.raster.StreamPix = buf
	s.raster.StreamPixW = s.width
	s.raster.StreamPixH = s.height
	s.raster.StreamMu.Unlock()

	bellui.Do(s.raster.Refresh)
}

// Object returns the canvas object to place in a widget tree.
func (s *videoSurface) Object() core.CanvasObject { return s.raster }

// Refresh triggers a repaint without supplying a new frame.
func (s *videoSurface) Refresh() { bellui.Do(s.raster.Refresh) }

// aspectLayout centres a single object at the largest size that fits the
// available space while preserving the video's aspect ratio.
//
// canvas.Raster is always drawn with ImageFillStretch — drawRaster passes that
// mode literally, so there is no ImageFillContain equivalent. Letterboxing
// therefore has to happen here, by sizing the raster to the contained
// rectangle rather than to the whole area.
type aspectLayout struct {
	width, height int
}

func (a *aspectLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	if len(objects) == 0 {
		return
	}
	obj := objects[0]
	if a.width <= 0 || a.height <= 0 || size.Width <= 0 || size.Height <= 0 {
		obj.Resize(size)
		obj.Move(geo.NewPos(0, 0))
		return
	}

	aspect := float32(a.width) / float32(a.height)
	w, h := size.Width, size.Height
	if w/h > aspect {
		// Area is wider than the video: pillarbox.
		w = h * aspect
	} else {
		// Area is taller than the video: letterbox.
		h = w / aspect
	}
	obj.Resize(geo.NewSize(w, h))
	obj.Move(geo.NewPos((size.Width-w)/2, (size.Height-h)/2))
}

func (a *aspectLayout) MinSize(objects []core.CanvasObject) geo.Size {
	// A video scales to whatever space it is given; a small floor keeps the
	// surface from collapsing inside a flexible container.
	return geo.NewSize(16, 16)
}
