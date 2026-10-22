//go:build !no_glfw && !ci && !mobile

// (!ci, like window_test.go: it captures from a real window.)

package glfw

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// A RepaintBoundary draws what its content draws in place, to within one
// level on partly transparent edges (#570), with either painter, and leaves
// the window opaque. GL blended its alpha channel like the color, which left
// a² for an edge's coverage: the window's alpha fell along anti-aliased edges,
// which captures read as a dark fringe, and a boundary's texture held the
// wrong coverage, so its edges came out up to 58 levels off (#580).
func TestCapture_BoundaryMatchesDrawingInPlace(t *testing.T) {
	card := func() core.CanvasObject {
		bg := canvas.NewRectangle(color.NRGBA{R: 40, G: 40, B: 60, A: 255})
		bg.CornerRadius = 12
		bg.StrokeColor = color.NRGBA{R: 200, G: 200, B: 220, A: 255}
		bg.StrokeWidth = 1
		return container.NewStack(bg, widget.NewLabel("Same pixels"))
	}
	const side = 120
	direct, cached := card(), container.NewRepaintBoundary(card())
	direct.Resize(geo.NewSize(side, 60))
	direct.Move(geo.NewPos(10, 10))
	cached.Resize(geo.NewSize(side, 60))
	cached.Move(geo.NewPos(10+side+20, 10)) // whole units apart, so on the same sub-pixel grid

	w := createWindow("boundary pixels")
	defer w.Close()
	w.SetPadded(false)
	w.SetContent(container.NewWithoutLayout(direct, cached))
	runOnMain(func() { w.window.Show() })
	w.Resize(geo.NewSize(2*side+40, 80))
	ensureCanvasSizeOrSkip(t, w, geo.NewSize(2*side+40, 80))
	repaintWindow(w)
	repaintWindow(w)

	img := captureOrSkip(t, w)
	// A capture that drew nothing would match itself: require the card's fill
	// just inside its top edge, clear of the label.
	scale := float32(img.Bounds().Dx()) / w.Canvas().Size().Width
	px := func(v float32) int { return int(v * scale) }
	fill := color.NRGBAModel.Convert(img.At(px(70), px(14))).(color.NRGBA)
	require.Equal(t, color.NRGBA{R: 40, G: 40, B: 60, A: 255}, fill, "the capture must show the card's fill")
	shift := px(side + 20)
	region := image.Rect(px(10)-2, px(10)-2, px(10+side)+2, px(10+60)+2)
	require.True(t, region.Add(image.Pt(shift, 0)).In(img.Bounds()))

	worst, opaque := 0, true
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			a := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			b := color.NRGBAModel.Convert(img.At(x+shift, y)).(color.NRGBA)
			opaque = opaque && a.A == 255 && b.A == 255
			for _, d := range []int{int(a.R) - int(b.R), int(a.G) - int(b.G), int(a.B) - int(b.B)} {
				worst = max(worst, d, -d)
			}
		}
	}
	assert.True(t, opaque, "the window must stay opaque along anti-aliased edges")
	assert.LessOrEqual(t, worst, 1, "the boundary must draw what drawing in place does, to within one level")
}
