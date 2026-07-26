package canvas

import "image/color"

// AxisConfig3D configures one axis of a 3D chart.
type AxisConfig3D struct {
	Label    string
	Min, Max float32
	Ticks    int // number of tick marks (default 5)
	Color    color.Color
}

// AddAxes adds X, Y, Z axis lines with tick marks and ground grid to the scene.
func (s *Scene3D) AddAxes(xCfg, yCfg, zCfg AxisConfig3D) {
	axisLineWidth := float32(0.015)

	if xCfg.Color == nil {
		xCfg.Color = color.NRGBA{R: 200, G: 80, B: 80, A: 255}
	}
	if yCfg.Color == nil {
		yCfg.Color = color.NRGBA{R: 80, G: 200, B: 80, A: 255}
	}
	if zCfg.Color == nil {
		zCfg.Color = color.NRGBA{R: 80, G: 80, B: 200, A: 255}
	}

	// X axis line
	s.AddMesh(makeLine3D(
		Vec3{X: xCfg.Min, Y: 0, Z: 0},
		Vec3{X: xCfg.Max, Y: 0, Z: 0},
		axisLineWidth, xCfg.Color,
	))

	// Y axis line
	s.AddMesh(makeLine3D(
		Vec3{X: 0, Y: yCfg.Min, Z: 0},
		Vec3{X: 0, Y: yCfg.Max, Z: 0},
		axisLineWidth, yCfg.Color,
	))

	// Z axis line
	s.AddMesh(makeLine3D(
		Vec3{X: 0, Y: 0, Z: zCfg.Min},
		Vec3{X: 0, Y: 0, Z: zCfg.Max},
		axisLineWidth, zCfg.Color,
	))

	// Tick marks
	tickLen := float32(0.08)
	tickWidth := float32(0.008)
	tickColor := color.NRGBA{R: 150, G: 150, B: 150, A: 200}

	addTicks := func(cfg AxisConfig3D, makeStart, makeEnd func(v float32) Vec3) {
		ticks := cfg.Ticks
		if ticks <= 0 {
			ticks = 5
		}
		step := (cfg.Max - cfg.Min) / float32(ticks)
		for i := 0; i <= ticks; i++ {
			v := cfg.Min + float32(i)*step
			s.AddMesh(makeLine3D(makeStart(v), makeEnd(v), tickWidth, tickColor))
		}
	}

	// X ticks (perpendicular in Y)
	addTicks(
		xCfg,
		func(v float32) Vec3 { return Vec3{X: v, Y: -tickLen, Z: 0} },
		func(v float32) Vec3 { return Vec3{X: v, Y: tickLen, Z: 0} },
	)

	// Y ticks (perpendicular in X)
	addTicks(
		yCfg,
		func(v float32) Vec3 { return Vec3{X: -tickLen, Y: v, Z: 0} },
		func(v float32) Vec3 { return Vec3{X: tickLen, Y: v, Z: 0} },
	)

	// Z ticks (perpendicular in Y)
	addTicks(
		zCfg,
		func(v float32) Vec3 { return Vec3{X: 0, Y: -tickLen, Z: v} },
		func(v float32) Vec3 { return Vec3{X: 0, Y: tickLen, Z: v} },
	)

	// Ground grid lines
	gridColor := color.NRGBA{R: 100, G: 100, B: 100, A: 60}
	gridWidth := float32(0.005)
	gridTicks := 5
	xStep := (xCfg.Max - xCfg.Min) / float32(gridTicks)
	zStep := (zCfg.Max - zCfg.Min) / float32(gridTicks)

	for i := 0; i <= gridTicks; i++ {
		// Lines parallel to Z
		x := xCfg.Min + float32(i)*xStep
		s.AddMesh(makeLine3D(
			Vec3{X: x, Y: 0, Z: zCfg.Min},
			Vec3{X: x, Y: 0, Z: zCfg.Max},
			gridWidth, gridColor,
		))
		// Lines parallel to X
		z := zCfg.Min + float32(i)*zStep
		s.AddMesh(makeLine3D(
			Vec3{X: xCfg.Min, Y: 0, Z: z},
			Vec3{X: xCfg.Max, Y: 0, Z: z},
			gridWidth, gridColor,
		))
	}
}

// makeLine3D creates a thin rectangular beam between two 3D points.
func makeLine3D(start, end Vec3, width float32, c color.Color) Mesh3D {
	// Build a thin box oriented along the line direction
	dir := end.Sub(start)
	length := dir.Length()
	if length == 0 {
		return Mesh3D{}
	}

	// Use a simple axis-aligned box approximation for thin lines
	hw := width / 2
	v := []Vec3{
		{X: start.X - hw, Y: start.Y - hw, Z: start.Z - hw},
		{X: end.X + hw, Y: start.Y - hw, Z: start.Z - hw},
		{X: end.X + hw, Y: start.Y + hw, Z: start.Z + hw},
		{X: start.X - hw, Y: start.Y + hw, Z: start.Z + hw},
		{X: start.X - hw, Y: end.Y - hw, Z: end.Z - hw},
		{X: end.X + hw, Y: end.Y - hw, Z: end.Z - hw},
		{X: end.X + hw, Y: end.Y + hw, Z: end.Z + hw},
		{X: start.X - hw, Y: end.Y + hw, Z: end.Z + hw},
	}
	f := []Face{
		{A: 0, B: 1, C: 2},
		{A: 0, B: 2, C: 3},
		{A: 4, B: 5, C: 6},
		{A: 4, B: 6, C: 7},
		{A: 0, B: 1, C: 5},
		{A: 0, B: 5, C: 4},
		{A: 2, B: 3, C: 7},
		{A: 2, B: 7, C: 6},
		{A: 0, B: 3, C: 7},
		{A: 0, B: 7, C: 4},
		{A: 1, B: 2, C: 6},
		{A: 1, B: 6, C: 5},
	}
	return Mesh3D{Vertices: v, Faces: f, FillColor: c}
}
