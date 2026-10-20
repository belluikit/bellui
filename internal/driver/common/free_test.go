package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/gpu"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// Refreshing a widget frees the texture of a textured object it draws, and
// marks the repaint boundary around it dirty so the boundary's cached image is
// drawn again (#543).
func TestCanvas_FreeDirtyTexturesFreesChildAndMarksBoundary(t *testing.T) {
	test.NewTempApp(t)
	c := &Canvas{}
	c.Initialize(sizedCanvas{test.NewCanvas()}, func() {})

	raster := canvas.NewRasterWithPixels(func(_, _, _, _ int) color.Color { return color.Black })
	holder := &rasterHolder{raster: raster}
	holder.ExtendBaseWidget(holder)
	boundary := container.NewRepaintBoundary(holder)
	c.SetContentTreeAndFocusMgr(boundary)
	c.WalkTrees(nil, nil)

	p := &recordingPainter{}
	c.SetPainter(p)
	c.Refresh(holder)
	c.FreeDirtyTextures()

	assert.Contains(t, p.freed, core.CanvasObject(raster), "the widget's raster was not freed")
	assert.Equal(t, []core.CanvasObject{boundary}, p.dirtyBoundaries)
}

// rasterHolder is a widget that draws one raster.
type rasterHolder struct {
	widget.BaseWidget
	raster *canvas.Raster
}

func (r *rasterHolder) CreateRenderer() core.WidgetRenderer {
	return &shadowedClipRenderer{objects: []core.CanvasObject{r.raster}}
}

// recordingPainter records the objects it is asked to free and the
// boundaries it is told are dirty.
type recordingPainter struct {
	stubPainter
	freed           []core.CanvasObject
	dirtyBoundaries []core.CanvasObject
}

func (p *recordingPainter) Free(obj core.CanvasObject) { p.freed = append(p.freed, obj) }

func (p *recordingPainter) MarkBoundaryDirty(obj core.CanvasObject) {
	p.dirtyBoundaries = append(p.dirtyBoundaries, obj)
}

func (*recordingPainter) EnsureBoundaryFBO(core.CanvasObject, int, int, geo.Size) gpu.BoundaryFBO {
	return nil
}
func (*recordingPainter) BindBoundaryFBO(gpu.BoundaryFBO)                             {}
func (*recordingPainter) UnbindBoundaryFBO(int, int)                                  {}
func (*recordingPainter) BlitBoundaryTexture(gpu.BoundaryFBO, geo.Position, geo.Size) {}
func (*recordingPainter) FreeBoundaryFBO(core.CanvasObject)                           {}
func (*recordingPainter) IsBoundaryClean(core.CanvasObject) bool                      { return false }
func (*recordingPainter) InvalidateAllBoundaries()                                    {}

var _ gpu.BoundaryPainter = (*recordingPainter)(nil)
