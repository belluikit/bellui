package series

import "github.com/belluikit/bellui/core"

// CustomRenderFunc is the user-provided rendering function for custom series.
// It receives the render context and returns canvas objects.
type CustomRenderFunc func(ctx RenderContext) []core.CanvasObject

// CustomSeries provides a user-defined rendering escape hatch.
// The RenderFunc callback receives the full RenderContext (coordinate system,
// viewport, color, animation state) and returns arbitrary canvas objects.
type CustomSeries struct {
	SeriesName string
	RenderFunc CustomRenderFunc
}

func (s *CustomSeries) SeriesType() Type { return TypeCustom }
func (s *CustomSeries) Name() string     { return s.SeriesName }
func (s *CustomSeries) sealed()          {}

func init() {
	RegisterRenderer(TypeCustom, func() Renderer { return &customRenderer{} })
}

type customRenderer struct{}

func (r *customRenderer) Render(ctx RenderContext) []core.CanvasObject {
	cs, ok := ctx.Series.(*CustomSeries)
	if !ok || cs.RenderFunc == nil {
		return nil
	}
	return cs.RenderFunc(ctx)
}
