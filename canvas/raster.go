package canvas

import (
	"image"
	"image/color"
	"image/draw"
	"sync"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/ycbcr"
)

// Declare conformity with CanvasObject interface
var _ core.CanvasObject = (*Raster)(nil)

// Raster describes a raster image area that can render in a Bellui canvas
type Raster struct {
	baseObject

	// Render the raster image from code
	Generator func(w, h int) image.Image

	// Set a translucency value > 0.0 to fade the raster
	Translucency float64
	// Specify the type of scaling interpolation applied to the raster if it is not full-size
	// Since: 1.0
	ScaleMode ImageScale

	// Streaming enables in-place texture updates.
	// When true, Refresh() reuses the existing GPU texture — uploading only the
	// new pixels — instead of deleting and recreating it on every frame. Ideal
	// for video playback. Honoured by the GL and WebGPU painters; the software
	// painter re-draws the current pixels each paint, so Streaming is a no-op
	// there rather than a behaviour change.
	Streaming bool

	// StreamMu protects the streaming pixel fields (StreamPix, StreamPixW,
	// StreamPixH, StreamTexW, StreamTexH, and the StreamYCbCr fields) from
	// concurrent access between SetPixels or SetYCbCr (user goroutine) and the
	// painter thread.
	StreamMu sync.Mutex

	// StreamPix holds raw premultiplied-RGBA pixel data set via SetPixels.
	// Read by the painter; do not set directly — use SetPixels.
	StreamPix []byte
	// StreamPixW and StreamPixH are the dimensions of StreamPix data.
	StreamPixW, StreamPixH int

	// StreamTexW and StreamTexH track the allocated GL texture dimensions.
	// Managed by the painter to detect size changes.
	StreamTexW, StreamTexH int

	// StreamYCbCr holds a video frame set via SetYCbCr, with the colour
	// matrix and range it was encoded with. Read by the painter; do not set
	// directly — use SetYCbCr.
	//
	// Experimental: see SetYCbCr.
	StreamYCbCr *image.YCbCr
	// StreamMatrix is StreamYCbCr's colour matrix.
	StreamMatrix YCbCrMatrix
	// StreamFullRange is true when StreamYCbCr uses every byte value, and
	// false for the limited ("TV") range most video uses.
	StreamFullRange bool
}

// YCbCrMatrix is the colour matrix a Y'CbCr video frame was encoded with.
//
// Experimental: see [Raster.SetYCbCr].
type YCbCrMatrix int

const (
	// YCbCrBT601 is the matrix of standard-definition video and of JPEG.
	YCbCrBT601 YCbCrMatrix = ycbcr.BT601
	// YCbCrBT709 is the matrix of high-definition video.
	YCbCrBT709 YCbCrMatrix = ycbcr.BT709
	// YCbCrBT2020 is the matrix of ultra-high-definition and HDR video.
	YCbCrBT2020 YCbCrMatrix = ycbcr.BT2020
)

// Alpha is a convenience function that returns the alpha value for a raster
// based on its Translucency value. The result is 1.0 - Translucency.
func (r *Raster) Alpha() float64 {
	return 1.0 - r.Translucency
}

// Hide will set this raster to not be visible
func (r *Raster) Hide() {
	r.baseObject.Hide()

	repaint(r)
}

// Move the raster to a new position, relative to its parent / canvas
func (r *Raster) Move(pos geo.Position) {
	if r.Position() == pos {
		return
	}

	r.baseObject.Move(pos)

	repaint(r)
}

// Resize on a raster image causes the new size to be set and then calls Refresh.
// This causes the underlying data to be recalculated and a new output to be drawn.
func (r *Raster) Resize(s geo.Size) {
	if s == r.Size() {
		return
	}

	r.baseObject.Resize(s)
	Refresh(r)
}

// Refresh causes this raster to be redrawn with its configured state.
func (r *Raster) Refresh() {
	Refresh(r)
}

// SetPixels updates the raster's pixel data directly with raw RGBA bytes.
// The provided data must be premultiplied RGBA with 4 bytes per pixel, and
// width*height*4 must equal len(pix). This is the fastest path for
// video frame updates when Streaming is true. It automatically calls
// Refresh to trigger a repaint.
//
// A raster driven purely by SetPixels needs no Generator: every painter reads
// the supplied pixels in preference to it. Until the first SetPixels call such
// a raster simply has no content and draws nothing.
func (r *Raster) SetPixels(pix []byte, width, height int) {
	r.StreamMu.Lock()
	r.StreamPix = pix
	r.StreamPixW = width
	r.StreamPixH = height
	r.StreamYCbCr = nil
	r.StreamMu.Unlock()
	Refresh(r)
}

// SetYCbCr updates the raster's content with a video frame as it was decoded:
// Y'CbCr planes, encoded with the given colour matrix and range. The GPU
// painters upload the planes and convert them to RGB on the GPU: at 4:2:0 that
// uploads 1.5 bytes a pixel instead of SetPixels' 4, and converts nothing on
// the processor. The software painter converts in Go. Any subsample ratio
// image.YCbCr has is drawn. It replaces anything set by SetPixels, and
// automatically calls Refresh to trigger a repaint.
//
// The raster keeps img, not a copy: do not write to it until a later
// SetYCbCr or SetPixels call has replaced it. A video surface alternates
// between two frames for that reason.
//
// Experimental: added for x/mmedia's video playback (#1107), and its shape
// follows what that needs; a dedicated video-frame type may replace it. The
// stable alternative is SetPixels with RGBA converted on the processor.
func (r *Raster) SetYCbCr(img *image.YCbCr, matrix YCbCrMatrix, fullRange bool) {
	r.StreamMu.Lock()
	r.StreamYCbCr = img
	r.StreamMatrix = matrix
	r.StreamFullRange = fullRange
	r.StreamPix = nil
	r.StreamPixW, r.StreamPixH = 0, 0
	r.StreamMu.Unlock()
	Refresh(r)
}

// PixelImage returns the raster's current content at the requested size: the
// data supplied by [Raster.SetPixels] or [Raster.SetYCbCr] when there is any
// (a Y'CbCr frame converted to RGBA by its own colour matrix and range),
// otherwise the output of Generator. It returns nil when the raster has neither, so painters can
// skip an empty raster instead of dereferencing a nil Generator.
//
// The pixel data is copied, so the caller may keep the result after the next
// SetPixels call. Painters with an in-place upload path should read StreamPix
// under StreamMu directly rather than pay for that copy every frame.
//
// Since: 1.0
func (r *Raster) PixelImage(w, h int) image.Image {
	r.StreamMu.Lock()
	pix, pixW, pixH := r.StreamPix, r.StreamPixW, r.StreamPixH
	if pixW > 0 && pixH > 0 && len(pix) >= pixW*pixH*4 {
		img := image.NewRGBA(image.Rect(0, 0, pixW, pixH))
		copy(img.Pix, pix)
		r.StreamMu.Unlock()
		return img
	}
	if yuv := r.StreamYCbCr; yuv != nil && !yuv.Rect.Empty() {
		img := ycbcr.ToRGBA(yuv, int(r.StreamMatrix), r.StreamFullRange)
		r.StreamMu.Unlock()
		return img
	}
	r.StreamMu.Unlock()

	if r.Generator == nil {
		return nil
	}
	return r.Generator(w, h)
}

// NewRaster returns a new Image instance that is rendered dynamically using
// the specified generate function.
// Images returned from this method should draw dynamically to fill the width
// and height parameters passed to pixelColor.
func NewRaster(generate func(w, h int) image.Image) *Raster {
	return &Raster{Generator: generate}
}

type pixelRaster struct {
	r *Raster

	img draw.Image
}

// NewRasterWithPixels returns a new Image instance that is rendered dynamically
// by iterating over the specified pixelColor function for each x, y pixel.
// Images returned from this method should draw dynamically to fill the width
// and height parameters passed to pixelColor.
func NewRasterWithPixels(pixelColor func(x, y, w, h int) color.Color) *Raster {
	pix := &pixelRaster{}
	pix.r = &Raster{
		Generator: func(w, h int) image.Image {
			if pix.img == nil || pix.img.Bounds().Size().X != w || pix.img.Bounds().Size().Y != h {
				// raster first pixel, figure out color type
				var dst draw.Image
				rect := image.Rect(0, 0, w, h)
				switch pixelColor(0, 0, w, h).(type) {
				case color.Alpha:
					dst = image.NewAlpha(rect)
				case color.Alpha16:
					dst = image.NewAlpha16(rect)
				case color.CMYK:
					dst = image.NewCMYK(rect)
				case color.Gray:
					dst = image.NewGray(rect)
				case color.Gray16:
					dst = image.NewGray16(rect)
				case color.NRGBA:
					dst = image.NewNRGBA(rect)
				case color.NRGBA64:
					dst = image.NewNRGBA64(rect)
				case color.RGBA:
					dst = image.NewRGBA(rect)
				case color.RGBA64:
					dst = image.NewRGBA64(rect)
				default:
					dst = image.NewRGBA(rect)
				}
				pix.img = dst
			}

			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					pix.img.Set(x, y, pixelColor(x, y, w, h))
				}
			}

			return pix.img
		},
	}
	return pix.r
}

type subImg interface {
	SubImage(r image.Rectangle) image.Image
}

// NewRasterFromImage returns a new Raster instance that is rendered from the Go
// image.Image passed in.
// Rasters returned from this method will map pixel for pixel to the screen
// starting img.Bounds().Min pixels from the top left of the canvas object.
// Truncates rather than scales the image.
// If smaller than the target space, the image will be padded with zero-pixels to the target size.
func NewRasterFromImage(img image.Image) *Raster {
	return &Raster{
		Generator: func(w int, h int) image.Image {
			bounds := img.Bounds()

			rect := image.Rect(0, 0, w, h)

			switch {
			case w == bounds.Max.X && h == bounds.Max.Y:
				return img
			case w >= bounds.Max.X && h >= bounds.Max.Y:
				// try quickly truncating
				if sub, ok := img.(subImg); ok {
					return sub.SubImage(image.Rectangle{
						Min: bounds.Min,
						Max: image.Point{
							X: bounds.Min.X + w,
							Y: bounds.Min.Y + h,
						},
					})
				}
			default:
				if !rect.Overlaps(bounds) {
					return image.NewUniform(color.RGBA{})
				}
				bounds = bounds.Intersect(rect)
			}

			// respect the user's pixel format (if possible)
			var dst draw.Image
			switch i := img.(type) {
			case *image.Alpha:
				dst = image.NewAlpha(rect)
			case *image.Alpha16:
				dst = image.NewAlpha16(rect)
			case *image.CMYK:
				dst = image.NewCMYK(rect)
			case *image.Gray:
				dst = image.NewGray(rect)
			case *image.Gray16:
				dst = image.NewGray16(rect)
			case *image.NRGBA:
				dst = image.NewNRGBA(rect)
			case *image.NRGBA64:
				dst = image.NewNRGBA64(rect)
			case *image.Paletted:
				dst = image.NewPaletted(rect, i.Palette)
			case *image.RGBA:
				dst = image.NewRGBA(rect)
			case *image.RGBA64:
				dst = image.NewRGBA64(rect)
			default:
				dst = image.NewRGBA(rect)
			}

			draw.Draw(dst, bounds, img, bounds.Min, draw.Over)
			return dst
		},
	}
}
