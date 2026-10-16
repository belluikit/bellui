//go:build !windows || !ci

package mobile

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal"
	"github.com/belluikit/bellui/widget"
)

// The mobile paint walk must honor the RoundedClipCornerRadius renderer seam
// with stencil clipping, like the glfw walk — not clip rectangularly (pre-rebrand tracker #959).

// clipRecordingPainter is a gpu.Painter that records the clip-related call
// sequence; every other operation is a no-op.
type clipRecordingPainter struct {
	calls []string
}

func (p *clipRecordingPainter) Init()                           {}
func (p *clipRecordingPainter) Capture(core.Canvas) image.Image { return nil }
func (p *clipRecordingPainter) Clear()                          {}
func (p *clipRecordingPainter) Free(core.CanvasObject)          {}
func (p *clipRecordingPainter) Paint(obj core.CanvasObject, _ geo.Position, _ geo.Size) {
	p.calls = append(p.calls, fmt.Sprintf("paint %T", obj))
}
func (p *clipRecordingPainter) SetFrameBufferScale(float32) {}
func (p *clipRecordingPainter) SetOutputSize(int, int)      {}
func (p *clipRecordingPainter) StartClipping(geo.Position, geo.Size) {
	p.calls = append(p.calls, "startClip")
}

func (p *clipRecordingPainter) StopClipping() {
	p.calls = append(p.calls, "stopClip")
}

func (p *clipRecordingPainter) StartRoundedClipping(_ geo.Position, _ geo.Size, radius float32) {
	p.calls = append(p.calls, fmt.Sprintf("startRounded %v", radius))
}

func (p *clipRecordingPainter) StopRoundedClipping() {
	p.calls = append(p.calls, "stopRounded")
}
func (p *clipRecordingPainter) StartPathClipping(*belluicanvas.Path, geo.Position, geo.Size) {}
func (p *clipRecordingPainter) StopPathClipping()                                            {}
func (p *clipRecordingPainter) SetGroupOpacity(float32)                                      {}
func (p *clipRecordingPainter) ClearGroupOpacity()                                           {}
func (p *clipRecordingPainter) SetTransform(geo.Matrix)                                      {}

// roundedClipTestWidget's renderer offers RoundedClipCornerRadius, the seam
// Card, the Entry corners (pre-rebrand tracker #467) and the pre-rebrand tracker #942 loupe bubble rely on.
type roundedClipTestWidget struct {
	widget.BaseWidget

	content *belluicanvas.Rectangle
}

func newRoundedClipTestWidget() *roundedClipTestWidget {
	w := &roundedClipTestWidget{content: belluicanvas.NewRectangle(color.Black)}
	w.ExtendBaseWidget(w)
	return w
}

func (w *roundedClipTestWidget) CreateRenderer() core.WidgetRenderer {
	return &roundedClipTestRenderer{content: w.content}
}

type roundedClipTestRenderer struct {
	content *belluicanvas.Rectangle
}

func (r *roundedClipTestRenderer) Destroy()             {}
func (r *roundedClipTestRenderer) Layout(size geo.Size) { r.content.Resize(size) }
func (r *roundedClipTestRenderer) MinSize() geo.Size    { return geo.NewSize(10, 10) }
func (r *roundedClipTestRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.content}
}
func (r *roundedClipTestRenderer) Refresh() {}

func (r *roundedClipTestRenderer) RoundedClipCornerRadius() float32 { return 8 }

func Test_mobileTreeWalker_RoundedClip(t *testing.T) {
	w := newRoundedClipTestWidget()
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(w)
	c.Resize(geo.NewSize(100, 100))

	painter := &clipRecordingPainter{}
	c.SetPainter(painter)

	walk := &mobileTreeWalker{
		c:     c,
		clips: &internal.ClipStack{},
		size:  geo.NewSize(100, 100),
	}
	c.WalkTrees(walk.draw, walk.afterDraw)

	start, paint, stop := -1, -1, -1
	for i, call := range painter.calls {
		switch call {
		case "startRounded 8":
			start = i
		case "paint *canvas.Rectangle":
			paint = i
		case "stopRounded":
			stop = i
		}
	}
	require.GreaterOrEqual(t, start, 0, "rounded clip established for the widget: %v", painter.calls)
	require.GreaterOrEqual(t, stop, 0, "rounded clip torn down: %v", painter.calls)
	require.GreaterOrEqual(t, paint, 0, "widget content painted: %v", painter.calls)
	assert.Less(t, start, paint, "content paints inside the rounded clip")
	assert.Greater(t, stop, paint, "clip is dropped only after the subtree painted")
}
