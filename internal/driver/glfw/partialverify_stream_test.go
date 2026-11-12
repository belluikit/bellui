//go:build !no_glfw && !mobile

package glfw

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

// A new video frame written into the same Y'CbCr buffer changes the stream
// hash, so the partial-repaint check sees the stream moved under it (#613)
// whichever way the frame was set (#1107).
func TestStreamHashes_SeeANewYCbCrFrameInTheSameBuffer(t *testing.T) {
	r := canvas.NewRaster(nil)
	r.Streaming = true
	r.Resize(geo.NewSize(4, 4))
	frame := image.NewYCbCr(image.Rect(0, 0, 4, 4), image.YCbCrSubsampleRatio420)
	r.SetYCbCr(frame, canvas.YCbCrBT709, false)

	c := newCanvas()
	c.SetContent(r)
	c.Resize(geo.NewSize(20, 20))

	before := streamHashes(c, nil)
	assert.Len(t, before, 1)
	frame.Cr[0] = 200
	assert.NotEqual(t, before, streamHashes(c, nil), "a chroma change is a new frame")
}
