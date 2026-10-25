package series

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// Scatter3DSeries configures a 3D scatter chart series.
type Scatter3DSeries struct {
	SeriesName string
	// Data as [x, y, z] triplets stored flat: [x0, y0, z0, x1, y1, z1, ...]
	Data []float64

	// PointSize controls the radius of each sphere (default 0.08).
	PointSize float32

	Color color.Color

	// Camera overrides the default 3D camera.
	Camera *canvas.Camera3D
}

// SeriesType returns "scatter3d".
func (s *Scatter3DSeries) SeriesType() Type { return TypeScatter3D }

// Name returns the series display name.
func (s *Scatter3DSeries) Name() string { return s.SeriesName }

func (s *Scatter3DSeries) sealed() {}

// SeriesColor returns the colour set for the series, or nil for the palette
// colour (see ColoredSeries).
//
// Since: 1.3
func (s *Scatter3DSeries) SeriesColor() color.Color { return s.Color }

// PointCount returns the number of 3D data points.
func (s *Scatter3DSeries) PointCount() int {
	return len(s.Data) / 3
}

// Point returns the (x, y, z) values at index i.
func (s *Scatter3DSeries) Point(i int) (float64, float64, float64) {
	idx := i * 3
	if idx+2 >= len(s.Data) {
		return 0, 0, 0
	}
	return s.Data[idx], s.Data[idx+1], s.Data[idx+2]
}

func init() {
	RegisterRenderer(TypeScatter3D, func() Renderer { return &scatter3DRenderer{} })
}

type scatter3DRenderer struct{}

func (r *scatter3DRenderer) Render(ctx RenderContext) []core.CanvasObject {
	ss, ok := ctx.Series.(*Scatter3DSeries)
	if !ok || ss.PointCount() == 0 {
		return nil
	}

	scene := canvas.NewScene3D()
	if ss.Camera != nil {
		scene.Camera = *ss.Camera
	} else {
		scene.Camera = canvas.Camera3D{
			Position: canvas.Vec3{X: 4, Y: 3, Z: 5},
			Target:   canvas.Vec3{X: 0, Y: 0, Z: 0},
			Up:       canvas.Vec3{X: 0, Y: 1, Z: 0},
			FOV:      45,
			Near:     0.1,
			Far:      100,
		}
	}

	ptSize := ResolveFloat32(ss.PointSize, DefaultScatter3DSize)

	c := ResolveColor(ss.Color, ctx.Color)

	var minP, maxP canvas.Vec3
	minP = canvas.Vec3{X: 1e9, Y: 1e9, Z: 1e9}
	maxP = canvas.Vec3{X: -1e9, Y: -1e9, Z: -1e9}

	for i := 0; i < ss.PointCount(); i++ {
		x, y, z := ss.Point(i)
		center := canvas.Vec3{X: float32(x), Y: float32(y), Z: float32(z)}
		mesh := makeOctahedron(center, ptSize, c)
		scene.AddMesh(mesh)

		if center.X < minP.X {
			minP.X = center.X
		}
		if center.Y < minP.Y {
			minP.Y = center.Y
		}
		if center.Z < minP.Z {
			minP.Z = center.Z
		}
		if center.X > maxP.X {
			maxP.X = center.X
		}
		if center.Y > maxP.Y {
			maxP.Y = center.Y
		}
		if center.Z > maxP.Z {
			maxP.Z = center.Z
		}
	}

	// Add 3D axes
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

// makeOctahedron creates a simple 6-vertex polyhedron approximating a sphere.
func makeOctahedron(center canvas.Vec3, radius float32, c color.Color) canvas.Mesh3D {
	v := []canvas.Vec3{
		{X: center.X, Y: center.Y + radius, Z: center.Z}, // top
		{X: center.X, Y: center.Y - radius, Z: center.Z}, // bottom
		{X: center.X + radius, Y: center.Y, Z: center.Z}, // right
		{X: center.X - radius, Y: center.Y, Z: center.Z}, // left
		{X: center.X, Y: center.Y, Z: center.Z + radius}, // front
		{X: center.X, Y: center.Y, Z: center.Z - radius}, // back
	}
	f := []canvas.Face{
		{A: 0, B: 2, C: 4},
		{A: 0, B: 4, C: 3},
		{A: 0, B: 3, C: 5},
		{A: 0, B: 5, C: 2},
		{A: 1, B: 4, C: 2},
		{A: 1, B: 3, C: 4},
		{A: 1, B: 5, C: 3},
		{A: 1, B: 2, C: 5},
	}
	return canvas.Mesh3D{Vertices: v, Faces: f, FillColor: c}
}
