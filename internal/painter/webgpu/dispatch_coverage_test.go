//go:build !gl

package webgpu

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// newPathText builds the #210 repro object: text laid along a quadratic bend.
func newPathText(s string) *canvas.TextOnPath {
	top := canvas.NewTextOnPath(s, color.White, func(pt *canvas.TextOnPath) {
		pt.MoveTo(10, 60)
		pt.QuadTo(90, 0, 170, 60)
	})
	top.TextSize = 14
	top.Resize(top.MinSize())
	return top
}

// reachesGPU reports whether drawObject carried obj as far as the texture
// upload. The painter here has no device, so an op that gets that far dies
// there and an op that is never dispatched returns quietly — which is exactly
// the difference #210 was made of, and it is observable without a GPU.
func reachesGPU(obj core.CanvasObject) (reached bool) {
	defer func() {
		if recover() != nil {
			reached = true
		}
	}()
	p := &painter{pixScale: 1, groupOpacity: 1, transform: geo.Identity()}
	p.drawObject(obj, geo.NewPos(0, 0), geo.NewSize(320, 240))
	return reached
}

// TestDrawObject_DispatchesTextOnPathLikeItsPeers guards #210's failure mode.
// drawObject is a type switch, so a primitive with no case is not an error —
// it is silently skipped and never appears, with nothing logged. TextOnPath
// sat like that behind a "not yet ported" comment: it rendered under the GL
// and software painters and nowhere under WebGPU.
//
// The assertion is a comparison rather than a fixed expectation, so it stays
// honest if the device path later becomes nil-safe: TextOnPath must be
// carried exactly as far as canvas.Text, its dispatched texture-op peer.
// Before the fix, Text reached the GPU and TextOnPath did not.
func TestDrawObject_DispatchesTextOnPathLikeItsPeers(t *testing.T) {
	txt := canvas.NewText("peer", color.White)
	txt.Resize(txt.MinSize())

	assert.Equal(t, reachesGPU(txt), reachesGPU(newPathText("around the bend")),
		"TextOnPath must be dispatched like every other texture op, not skipped by the type switch (#210)")
}

// TestRasterizeTextOnPath_ProducesAnImage covers the other half: dispatch can
// reach the op and still draw nothing if the rasterizer declines. It goes
// through the shared painter.DrawTextOnPath that the GL and software painters
// use, which is what makes all three agree (#210).
func TestRasterizeTextOnPath_ProducesAnImage(t *testing.T) {
	p := &painter{pixScale: 1}

	img := p.rasterizeTextOnPath(newPathText("around the bend"))
	if assert.NotNil(t, img, "TextOnPath must rasterise, or the op uploads an empty texture") {
		b := img.Bounds()
		assert.Positive(t, b.Dx())
		assert.Positive(t, b.Dy())
	}
}

// Degenerate input is declined by the op before it reaches the GPU, matching
// the GL painter's guard — so it must NOT reach the texture path.
func TestDrawTextOnPath_DegenerateInput(t *testing.T) {
	noText := canvas.NewTextOnPath("", color.White, func(pt *canvas.TextOnPath) {
		pt.MoveTo(0, 0)
		pt.LineTo(10, 10)
	})
	noPath := canvas.NewTextOnPath("text", color.White, func(*canvas.TextOnPath) {})

	assert.False(t, reachesGPU(noText), "empty text must be declined before the texture path")
	assert.False(t, reachesGPU(noPath), "an empty path must be declined before the texture path")
}

// TestDrawObject_GLViewReportsInsteadOfSkipping is #213. GLView is the one
// primitive this painter genuinely cannot draw — it hands the app a live
// OpenGL context, which WebGPU has none of — so it must say so rather than
// fall through the type switch, which is the silent failure that made #210
// expensive to diagnose.
//
// The report never touches the GPU, so a device-less painter is enough: the
// op must NOT reach the texture path (unlike its drawable peers).
func TestDrawObject_GLViewReportsInsteadOfSkipping(t *testing.T) {
	v := canvas.NewGLView(func(canvas.GLContext, int, int, float32) {})
	v.Resize(geo.NewSize(80, 60))

	assert.False(t, reachesGPU(v), "GLView must be declined before the texture path, not drawn")

	// Once per view, not once per frame: a repainting view must not flood the
	// log. Re-dispatching the same object stays quiet.
	assert.NotPanics(t, func() {
		p := &painter{pixScale: 1, groupOpacity: 1, transform: geo.Identity()}
		for range 3 {
			p.drawObject(v, geo.NewPos(0, 0), geo.NewSize(320, 240))
		}
	})
	_, warned := glViewWarned.Load(v)
	assert.True(t, warned, "the view is recorded as warned, so later frames stay silent")
}
