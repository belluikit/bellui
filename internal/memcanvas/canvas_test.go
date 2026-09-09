package memcanvas_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/memcanvas"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

// TestMain installs the test app the canvas needs: Capture resolves theme
// colours through bellui.CurrentApp, and widget MinSize needs a theme too.
// The tests drive the package through bellui/test's constructors — the same
// path every widget test takes — rather than reaching for the struct.
func TestMain(m *testing.M) {
	test.NewApp()
	m.Run()
}

// hoverable is the minimum event.Hoverable the Hovered accessors need.
type hoverable struct{}

func (h *hoverable) HoverIn(*event.HoverEvent)    {}
func (h *hoverable) HoverMoved(*event.HoverEvent) {}
func (h *hoverable) HoverOut()                    {}

func Test_canvas_Capture(t *testing.T) {
	c := memcanvas.New()
	c.Resize(geo.NewSquareSize(100))

	img := c.Capture()
	assert.Positive(t, img.Bounds().Size().X)
	assert.Positive(t, img.Bounds().Size().Y)

	r1, g1, b1, a1 := theme.Color(theme.ColorNameBackground).RGBA()
	r2, g2, b2, a2 := img.At(1, 1).RGBA()
	assert.Equal(t, r1, r2)
	assert.Equal(t, g1, g2)
	assert.Equal(t, b1, b2)
	assert.Equal(t, a1, a2)
}

func Test_canvas_InteractiveArea(t *testing.T) {
	c := memcanvas.New()
	c.Resize(geo.NewSize(600, 400))
	pos, size := c.InteractiveArea()
	assert.Equal(t, geo.NewPos(2, 3), pos)
	assert.Equal(t, geo.NewSize(596, 395), size)
}

func Test_canvas_PixelCoordinateAtPosition(t *testing.T) {
	c := memcanvas.New()

	pos := geo.NewPos(4, 4)
	c.SetScale(2.5)
	x, y := c.PixelCoordinateForPosition(pos)
	assert.Equal(t, 10, x)
	assert.Equal(t, 10, y)
}

func Test_canvas_TransparentCapture(t *testing.T) {
	c := memcanvas.NewTransparentWithPainter(nil)
	c.Resize(geo.NewSquareSize(10))

	img := c.Capture()
	assert.Positive(t, img.Bounds().Size().X)
	assert.Positive(t, img.Bounds().Size().Y)

	r1, g1, b1, a1 := color.Transparent.RGBA()
	r2, g2, b2, a2 := img.At(1, 1).RGBA()
	assert.Equal(t, r1, r2)
	assert.Equal(t, g1, g2)
	assert.Equal(t, b1, b2)
	assert.Equal(t, a1, a2)
}

func Test_canvas_Resize(t *testing.T) {
	c := memcanvas.New()
	assert.Equal(t, float32(100), c.Size().Width) // backwards compatible

	smallSize := geo.NewSize(10, 5)
	c.Resize(smallSize)
	c.SetContent(widget.NewLabel("Bigger"))
	assert.NotEqual(t, float32(100), c.Size().Width) // backwards compatible
	assert.Greater(t, c.Size().Width, smallSize.Width)
	assert.Greater(t, c.Size().Height, smallSize.Height)

	c = memcanvas.New()
	largeSize := geo.NewSize(100, 75)
	c.Resize(largeSize)
	c.SetContent(widget.NewLabel("Smaller"))
	assert.Equal(t, largeSize, c.Size())
}

// Test_canvas_Hovered covers the accessor pair the drivers use to decide
// between HoverMoved and a HoverOut/HoverIn transition.
func Test_canvas_Hovered(t *testing.T) {
	c := memcanvas.New()
	assert.Nil(t, c.Hovered())

	h := &hoverable{}
	c.SetHovered(h)
	assert.Equal(t, h, c.Hovered())

	c.SetHovered(nil)
	assert.Nil(t, c.Hovered())
}

// Test_canvas_ObjectTrees pins the root ordering a driver walks: content first,
// then overlays bottom-up.
func Test_canvas_ObjectTrees(t *testing.T) {
	c := memcanvas.New()
	assert.Empty(t, c.ObjectTrees())

	content := widget.NewLabel("content")
	c.SetContent(content)
	assert.Equal(t, []core.CanvasObject{content}, c.ObjectTrees())

	overlay := widget.NewLabel("overlay")
	c.Overlays().Add(overlay)
	assert.Equal(t, []core.CanvasObject{content, overlay}, c.ObjectTrees())
}
