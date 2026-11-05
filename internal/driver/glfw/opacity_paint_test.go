//go:build !no_glfw && !mobile

package glfw

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/gpu"
	"github.com/belluikit/bellui/widget"
)

// opacityPaintRecorder records the group opacity each named object is
// painted at, and the opacity of each boundary blit.
type opacityPaintRecorder struct {
	boundaryClipPainter
	names   map[core.CanvasObject]string
	opacity float32
}

func (p *opacityPaintRecorder) SetGroupOpacity(v float32)            { p.opacity = v }
func (p *opacityPaintRecorder) ClearGroupOpacity()                   { p.opacity = 1 }
func (p *opacityPaintRecorder) StartClipping(geo.Position, geo.Size) {}
func (p *opacityPaintRecorder) StopClipping()                        {}
func (p *opacityPaintRecorder) Paint(o core.CanvasObject, _ geo.Position, _ geo.Size) {
	if n, ok := p.names[o]; ok {
		p.calls = append(p.calls, fmt.Sprintf("%s@%.3g", n, p.opacity))
	}
}

func (p *opacityPaintRecorder) BlitBoundaryTexture(gpu.BoundaryFBO, geo.Position, geo.Size) {
	p.calls = append(p.calls, fmt.Sprintf("blit@%.3g", p.opacity))
}

func newRect() *canvas.Rectangle {
	r := canvas.NewRectangle(color.White)
	r.SetMinSize(geo.NewSize(10, 10))
	return r
}

// The desktop walk applies core.OpacityGroup as the mobile one does, nested
// with the dimming of disabled widgets, and restores the parent's opacity on
// the way out instead of full opacity. A boundary in a group caches its
// content opaque and is blitted at the group's opacity.
func TestPaint_GroupOpacity(t *testing.T) {
	a, b, c, d := newRect(), newRect(), newRect(), newRect()
	disabled := widget.NewButton("", nil)
	disabled.Disable()
	tree := container.NewVBox(
		container.NewOpacity(container.NewVBox(
			a,
			container.NewOpacity(b, 0.5),
			container.NewRepaintBoundary(container.NewVBox(c)),
		), 0.5),
		d,
	)

	cv := newCanvas()
	cv.SetPadded(false)
	cv.SetContent(container.NewVBox(tree, disabled))
	cv.Resize(geo.NewSize(300, 300))
	p := &opacityPaintRecorder{names: map[core.CanvasObject]string{a: "a", b: "b", c: "c", d: "d", disabled: "disabled"}, opacity: 1}
	cv.SetPainter(p)

	cv.paint(cv.Size())

	assert.Equal(t, []string{"a@0.5", "b@0.25", "bind", "c@1", "unbind", "blit@0.5", "d@1", "disabled@0.38"}, p.calls)
	assert.Equal(t, float32(1), p.opacity, "the paint ends at full opacity")
}
