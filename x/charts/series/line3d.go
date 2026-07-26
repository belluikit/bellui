package series

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// DefaultLine3DWidth is the default ribbon half-width for 3D lines.
const DefaultLine3DWidth float32 = 0.15

// Line3DSeries configures a 3D line chart series.
//
// Two data modes:
//   - 3D path: Data as [x, y, z] triplets when ZPosition is not set.
//   - 2D at depth: Data as [x0, y0, x1, y1, ...] pairs when ZPosition is set.
//     This mode places the line at a fixed Z depth, matching Excel-style 3D charts
//     where multiple series are stacked at different Z positions.
type Line3DSeries struct {
	SeriesName string
	Data       []float64

	// ZPosition places the line at a fixed Z depth (2D-at-depth mode).
	// When non-nil, Data is treated as [x, y] pairs instead of [x, y, z] triplets.
	ZPosition *float64

	// LineWidth controls the ribbon thickness (default 0.04).
	LineWidth float32

	Color color.Color

	// Camera overrides the default 3D camera.
	Camera *canvas.Camera3D
}

// SeriesType returns TypeLine3D.
func (s *Line3DSeries) SeriesType() Type { return TypeLine3D }

// Name returns the series display name.
func (s *Line3DSeries) Name() string { return s.SeriesName }

func (s *Line3DSeries) sealed() {}

// PointCount returns the number of data points.
func (s *Line3DSeries) PointCount() int {
	if s.ZPosition != nil {
		return len(s.Data) / 2 // [x, y] pairs
	}
	return len(s.Data) / 3 // [x, y, z] triplets
}

// Point returns the (x, y, z) values at index i.
func (s *Line3DSeries) Point(i int) (float64, float64, float64) {
	if s.ZPosition != nil {
		idx := i * 2
		if idx+1 >= len(s.Data) {
			return 0, 0, *s.ZPosition
		}
		return s.Data[idx], s.Data[idx+1], *s.ZPosition
	}
	idx := i * 3
	if idx+2 >= len(s.Data) {
		return 0, 0, 0
	}
	return s.Data[idx], s.Data[idx+1], s.Data[idx+2]
}

func init() {
	RegisterRenderer(TypeLine3D, func() Renderer { return &line3DRenderer{} })
}

type line3DRenderer struct{}

func (r *line3DRenderer) Render(ctx RenderContext) []core.CanvasObject {
	ls, ok := ctx.Series.(*Line3DSeries)
	if !ok || ls.PointCount() < 2 {
		return nil
	}

	scene := canvas.NewScene3D()
	if ls.Camera != nil {
		scene.Camera = *ls.Camera
	} else {
		scene.Camera = canvas.Camera3D{
			Position: canvas.Vec3{X: 8, Y: 6, Z: 10},
			Target:   canvas.Vec3{X: 2, Y: 1, Z: 1},
			Up:       canvas.Vec3{X: 0, Y: 1, Z: 0},
			FOV:      35,
			Near:     0.1,
			Far:      200,
		}
	}

	radius := ResolveFloat32(ls.LineWidth, DefaultLine3DWidth)
	c := ResolveColor(ls.Color, ctx.Color)

	var minP, maxP canvas.Vec3
	minP = canvas.Vec3{X: 1e9, Y: 1e9, Z: 1e9}
	maxP = canvas.Vec3{X: -1e9, Y: -1e9, Z: -1e9}

	// Build ribbon segments between consecutive points.
	// Each segment is a flat quad (2 triangles) with a small vertical thickness,
	// matching the Excel-style 3D line chart appearance.
	for i := 0; i < ls.PointCount()-1; i++ {
		x0, y0, z0 := ls.Point(i)
		x1, y1, z1 := ls.Point(i + 1)

		p0 := canvas.Vec3{X: float32(x0), Y: float32(y0), Z: float32(z0)}
		p1 := canvas.Vec3{X: float32(x1), Y: float32(y1), Z: float32(z1)}

		mesh := makeRibbonSegment(p0, p1, radius, c)
		scene.AddMesh(mesh)

		// Track bounds.
		for _, p := range []canvas.Vec3{p0, p1} {
			if p.X < minP.X {
				minP.X = p.X
			}
			if p.Y < minP.Y {
				minP.Y = p.Y
			}
			if p.Z < minP.Z {
				minP.Z = p.Z
			}
			if p.X > maxP.X {
				maxP.X = p.X
			}
			if p.Y > maxP.Y {
				maxP.Y = p.Y
			}
			if p.Z > maxP.Z {
				maxP.Z = p.Z
			}
		}
	}

	margin := float32(0.3)
	scene.AddAxes(
		canvas.AxisConfig3D{Label: "X", Min: minP.X - margin, Max: maxP.X + margin, Ticks: 5},
		canvas.AxisConfig3D{Label: "Y", Min: minP.Y - margin, Max: maxP.Y + margin, Ticks: 5},
		canvas.AxisConfig3D{Label: "Z", Min: minP.Z - margin, Max: maxP.Z + margin, Ticks: 5},
	)

	vp := ctx.Viewport
	scene.Move(geo.NewPos(vp.X, vp.Y))
	scene.Resize(geo.NewSize(vp.W, vp.H))

	return []core.CanvasObject{scene}
}

// makeRibbonSegment creates a 3D ribbon (box cross-section) between two points.
// The ribbon has height (Y) and depth (Z) giving it a solid 3D appearance.
func makeRibbonSegment(p0, p1 canvas.Vec3, halfW float32, c color.Color) canvas.Mesh3D {
	h := halfW       // half-height (Y direction)
	d := halfW * 0.5 // half-depth (Z direction)

	// 8 vertices per endpoint = 16 total (box cross-section at each end).
	verts := []canvas.Vec3{
		// p0 end (indices 0-7)
		{X: p0.X, Y: p0.Y + h, Z: p0.Z - d}, // 0: top-back
		{X: p0.X, Y: p0.Y + h, Z: p0.Z + d}, // 1: top-front
		{X: p0.X, Y: p0.Y - h, Z: p0.Z + d}, // 2: bottom-front
		{X: p0.X, Y: p0.Y - h, Z: p0.Z - d}, // 3: bottom-back
		// p1 end (indices 4-7)
		{X: p1.X, Y: p1.Y + h, Z: p1.Z - d}, // 4: top-back
		{X: p1.X, Y: p1.Y + h, Z: p1.Z + d}, // 5: top-front
		{X: p1.X, Y: p1.Y - h, Z: p1.Z + d}, // 6: bottom-front
		{X: p1.X, Y: p1.Y - h, Z: p1.Z - d}, // 7: bottom-back
	}
	// 6 faces × 2 triangles = 12 triangles (box between the two endpoints).
	faces := []canvas.Face{
		// Top face
		{A: 0, B: 1, C: 5},
		{A: 0, B: 5, C: 4},
		// Bottom face
		{A: 2, B: 3, C: 7},
		{A: 2, B: 7, C: 6},
		// Front face
		{A: 1, B: 2, C: 6},
		{A: 1, B: 6, C: 5},
		// Back face
		{A: 0, B: 4, C: 7},
		{A: 0, B: 7, C: 3},
		// Left cap (p0 end)
		{A: 0, B: 3, C: 2},
		{A: 0, B: 2, C: 1},
		// Right cap (p1 end)
		{A: 4, B: 5, C: 6},
		{A: 4, B: 6, C: 7},
	}
	return canvas.Mesh3D{Vertices: verts, Faces: faces, FillColor: c}
}

// suppress unused import
var _ = math.Pi
