package series

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// SurfaceSeries configures a 3D surface chart.
// Data is a 2D grid stored row-major: Data[row*Cols + col] gives the Y (height) value.
type SurfaceSeries struct {
	SeriesName string
	Data       []float64
	Rows       int
	Cols       int

	// XRange and ZRange define the world-space extent of the grid.
	// Defaults: [0, Cols-1] and [0, Rows-1].
	XMin, XMax float64
	ZMin, ZMax float64

	// ColorStops maps normalized height [0, 1] to colors.
	// If nil, a default blue-green-yellow gradient is used.
	ColorStops []HeatmapColorStop

	// Camera overrides the default 3D camera.
	Camera *canvas.Camera3D
}

// SeriesType returns "surface".
func (s *SurfaceSeries) SeriesType() Type { return TypeSurface }

// Name returns the series display name.
func (s *SurfaceSeries) Name() string { return s.SeriesName }

func (s *SurfaceSeries) sealed() {}

func init() {
	RegisterRenderer(TypeSurface, func() Renderer { return &surfaceRenderer{} })
}

type surfaceRenderer struct{}

func (r *surfaceRenderer) Render(ctx RenderContext) []core.CanvasObject {
	ss, ok := ctx.Series.(*SurfaceSeries)
	if !ok || len(ss.Data) == 0 || ss.Rows < 2 || ss.Cols < 2 {
		return nil
	}

	scene := canvas.NewScene3D()
	if ss.Camera != nil {
		scene.Camera = *ss.Camera
	} else {
		scene.Camera = canvas.Camera3D{
			Position: canvas.Vec3{X: 3, Y: 3, Z: 4},
			Target:   canvas.Vec3{X: 0, Y: 0, Z: 0},
			Up:       canvas.Vec3{X: 0, Y: 1, Z: 0},
			FOV:      45,
			Near:     0.1,
			Far:      100,
		}
	}

	// Data ranges
	xMin, xMax := ss.XMin, ss.XMax
	if xMin == 0 && xMax == 0 {
		xMax = float64(ss.Cols - 1)
	}
	zMin, zMax := ss.ZMin, ss.ZMax
	if zMin == 0 && zMax == 0 {
		zMax = float64(ss.Rows - 1)
	}

	// Find Y range
	yMin := math.Inf(1)
	yMax := math.Inf(-1)
	for _, v := range ss.Data {
		if v < yMin {
			yMin = v
		}
		if v > yMax {
			yMax = v
		}
	}
	if yMin == yMax {
		yMax = yMin + 1
	}

	stops := ss.ColorStops
	if len(stops) == 0 {
		stops = defaultSurfaceStops()
	}

	// Normalize to [-1, 1] range for nice centering
	xSpan := xMax - xMin
	zSpan := zMax - zMin
	ySpan := yMax - yMin

	// Build vertices
	verts := make([]canvas.Vec3, ss.Rows*ss.Cols)
	for row := 0; row < ss.Rows; row++ {
		for col := 0; col < ss.Cols; col++ {
			idx := row*ss.Cols + col
			val := 0.0
			if idx < len(ss.Data) {
				val = ss.Data[idx]
			}
			// Map to centered world coordinates
			x := float32((float64(col)/float64(ss.Cols-1)*xSpan + xMin - xSpan/2) / xSpan * 3)
			y := float32((val - yMin) / ySpan * 2)
			z := float32((float64(row)/float64(ss.Rows-1)*zSpan + zMin - zSpan/2) / zSpan * 3)
			verts[idx] = canvas.Vec3{X: x, Y: y, Z: z}
		}
	}

	// Build faces (two triangles per grid cell) with per-face colors
	var faces []canvas.Face
	var faceColors []color.Color

	for row := 0; row < ss.Rows-1; row++ {
		for col := 0; col < ss.Cols-1; col++ {
			tl := row*ss.Cols + col
			tr := tl + 1
			bl := (row+1)*ss.Cols + col
			br := bl + 1

			// Average height for color
			avgVal := (ss.Data[tl] + ss.Data[tr] + ss.Data[bl] + ss.Data[br]) / 4
			norm := (avgVal - yMin) / ySpan
			c := interpolateColor(stops, norm)

			faces = append(
				faces,
				canvas.Face{A: tl, B: bl, C: tr},
				canvas.Face{A: tr, B: bl, C: br},
			)
			faceColors = append(faceColors, c, c)
		}
	}

	mesh := canvas.Mesh3D{
		Vertices:   verts,
		Faces:      faces,
		FillColor:  color.White,
		FaceColors: faceColors,
	}
	scene.AddMesh(mesh)

	// Add 3D axes
	scene.AddAxes(
		canvas.AxisConfig3D{Label: "X", Min: -1.5, Max: 1.5, Ticks: 5},
		canvas.AxisConfig3D{Label: "Y", Min: 0, Max: 2.2, Ticks: 5},
		canvas.AxisConfig3D{Label: "Z", Min: -1.5, Max: 1.5, Ticks: 5},
	)

	vp := ctx.Viewport
	scene.Move(geo.NewPos(vp.X, vp.Y))
	scene.Resize(geo.NewSize(vp.W, vp.H))

	return []core.CanvasObject{scene}
}

func defaultSurfaceStops() []HeatmapColorStop {
	return []HeatmapColorStop{
		{Position: 0.0, Color: color.NRGBA{R: 49, G: 130, B: 189, A: 255}},
		{Position: 0.3, Color: color.NRGBA{R: 65, G: 182, B: 196, A: 255}},
		{Position: 0.6, Color: color.NRGBA{R: 199, G: 233, B: 180, A: 255}},
		{Position: 1.0, Color: color.NRGBA{R: 255, G: 255, B: 178, A: 255}},
	}
}
