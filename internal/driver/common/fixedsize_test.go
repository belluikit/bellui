package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// resizeRecorder is a 100×100 canvas whose content needs 500×40, recording
// the Resize calls EnsureMinSize makes.
type resizeRecorder struct {
	core.Canvas
	resized []geo.Size
}

func (r *resizeRecorder) Resize(s geo.Size) { r.resized = append(r.resized, s) }
func (*resizeRecorder) MinSize() geo.Size   { return geo.NewSize(500, 40) }
func (*resizeRecorder) Size() geo.Size      { return geo.NewSize(100, 100) }

func ensureMinSizeResizes(t *testing.T, fixed bool) []geo.Size {
	t.Helper()
	impl := &resizeRecorder{Canvas: test.NewCanvas()}
	c := &Canvas{FixedSize: fixed}
	c.Initialize(impl, func() {})
	wide := canvas.NewRectangle(color.Black)
	wide.SetMinSize(geo.NewSize(500, 40))
	impl.SetContent(wide)
	c.SetContentTreeAndFocusMgr(wide)
	c.EnsureMinSize()
	return impl.resized
}

// Content larger than the canvas grows it — unless the canvas is fixed-size
// (the browser, whose window cannot grow): there a canvas laid out wider than
// the viewport was drawn squeezed into it until the next window resize (#387).
func TestCanvas_EnsureMinSizeRespectsFixedSize(t *testing.T) {
	test.NewTempApp(t)
	t.Run("grows", func(t *testing.T) {
		assert.Equal(t, []geo.Size{geo.NewSize(500, 100)}, ensureMinSizeResizes(t, false))
	})
	t.Run("fixed", func(t *testing.T) {
		assert.Empty(t, ensureMinSizeResizes(t, true), "a fixed-size canvas must not grow to its content")
	})
}
