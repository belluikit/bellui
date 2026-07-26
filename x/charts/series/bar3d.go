package series

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// Bar3DData holds one bar's position and value.
type Bar3DData struct {
	X, Z  float64 // grid position
	Value float64 // bar height (Y axis)
	Color color.Color
}

// Bar3DSeries configures a 3D bar chart series.
type Bar3DSeries struct {
	SeriesName string
	Data       []Bar3DData

	// BarSize controls the XZ footprint of each bar (0 = auto).
	BarSize float32

	// Camera overrides the default 3D camera.
	Camera *canvas.Camera3D
}

// SeriesType returns "bar3d".
func (s *Bar3DSeries) SeriesType() Type { return TypeBar3D }

// Name returns the series display name.
func (s *Bar3DSeries) Name() string { return s.SeriesName }

func (s *Bar3DSeries) sealed() {}

func init() {
	RegisterRenderer(TypeBar3D, func() Renderer { return &bar3DRenderer{} })
}

type bar3DRenderer struct{}

func (r *bar3DRenderer) Render(ctx RenderContext) []core.CanvasObject {
	bs, ok := ctx.Series.(*Bar3DSeries)
	if !ok || len(bs.Data) == 0 {
		return nil
	}

	scene := canvas.NewScene3D()
	if bs.Camera != nil {
		scene.Camera = *bs.Camera
	} else {
		scene.Camera = canvas.Camera3D{
			Position: canvas.Vec3{X: 4, Y: 4, Z: 6},
			Target:   canvas.Vec3{X: 0, Y: 0, Z: 0},
			Up:       canvas.Vec3{X: 0, Y: 1, Z: 0},
			FOV:      40,
			Near:     0.1,
			Far:      100,
		}
	}

	barSize := ResolveFloat32(bs.BarSize, DefaultBar3DSize)

	// Find max value for normalization
	maxVal := 0.0
	for _, d := range bs.Data {
		if d.Value > maxVal {
			maxVal = d.Value
		}
	}
	if maxVal == 0 {
		maxVal = 1
	}

	palette := DefaultPalette

	for i, d := range bs.Data {
		h := float32(d.Value / maxVal * 2) // normalize to height 0-2
		x := float32(d.X)
		z := float32(d.Z)

		c := palette[i%len(palette)]
		if d.Color != nil {
			c = d.Color
		}

		mesh := makeBox(x-barSize/2, 0, z-barSize/2, x+barSize/2, h, z+barSize/2, c)
		scene.AddMesh(mesh)
	}

	// Add 3D axes with ground grid
	maxX := float32(0)
	maxZ := float32(0)
	maxH := float32(0)
	for _, d := range bs.Data {
		if float32(d.X) > maxX {
			maxX = float32(d.X)
		}
		if float32(d.Z) > maxZ {
			maxZ = float32(d.Z)
		}
		h := float32(d.Value / maxVal * 2)
		if h > maxH {
			maxH = h
		}
	}
	scene.AddAxes(
		canvas.AxisConfig3D{Label: "X", Min: -0.5, Max: maxX + 0.5, Ticks: int(maxX) + 1},
		canvas.AxisConfig3D{Label: "Y", Min: 0, Max: maxH + 0.2, Ticks: 5},
		canvas.AxisConfig3D{Label: "Z", Min: -0.5, Max: maxZ + 0.5, Ticks: int(maxZ) + 1},
	)

	vp := ctx.Viewport
	scene.Move(geo.NewPos(vp.X, vp.Y))
	scene.Resize(geo.NewSize(vp.W, vp.H))

	return []core.CanvasObject{scene}
}

// makeBox creates a box mesh from min/max corners.
func makeBox(x0, y0, z0, x1, y1, z1 float32, c color.Color) canvas.Mesh3D {
	v := []canvas.Vec3{
		{X: x0, Y: y0, Z: z0}, {X: x1, Y: y0, Z: z0}, {X: x1, Y: y1, Z: z0}, {X: x0, Y: y1, Z: z0}, // front
		{X: x0, Y: y0, Z: z1}, {X: x1, Y: y0, Z: z1}, {X: x1, Y: y1, Z: z1}, {X: x0, Y: y1, Z: z1}, // back
	}
	f := []canvas.Face{
		// front
		{A: 0, B: 1, C: 2},
		{A: 0, B: 2, C: 3},
		// back
		{A: 5, B: 4, C: 7},
		{A: 5, B: 7, C: 6},
		// top
		{A: 3, B: 2, C: 6},
		{A: 3, B: 6, C: 7},
		// bottom
		{A: 4, B: 5, C: 1},
		{A: 4, B: 1, C: 0},
		// right
		{A: 1, B: 5, C: 6},
		{A: 1, B: 6, C: 2},
		// left
		{A: 4, B: 0, C: 3},
		{A: 4, B: 3, C: 7},
	}
	return canvas.Mesh3D{Vertices: v, Faces: f, FillColor: c}
}
