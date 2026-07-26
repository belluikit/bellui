// Command renderprobe is an on-device verification harness for the GPU render
// pipeline (roadmap §6 / #34): it puts a rotating 3D cube (canvas.Scene3D), an
// elevated Card (canvas.BoxShadow), and a RepaintBoundary-wrapped animated
// widget on screen at once, so a device screenshot confirms shadows, 3D
// rasterization+compositing, and boundary-cached subtrees all render correctly
// under the WebGPU painter. The continuous cube/label animation drives the
// dirty-region path every frame.
package main

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func cubeMesh() canvas.Mesh3D {
	v := []canvas.Vec3{
		{X: -1, Y: -1, Z: -1},
		{X: 1, Y: -1, Z: -1},
		{X: 1, Y: 1, Z: -1},
		{X: -1, Y: 1, Z: -1},
		{X: -1, Y: -1, Z: 1},
		{X: 1, Y: -1, Z: 1},
		{X: 1, Y: 1, Z: 1},
		{X: -1, Y: 1, Z: 1},
	}
	// two triangles per cube face, 6 faces = 12
	f := []canvas.Face{
		{A: 0, B: 1, C: 2}, {A: 0, B: 2, C: 3}, // back
		{A: 4, B: 6, C: 5}, {A: 4, B: 7, C: 6}, // front
		{A: 0, B: 7, C: 4}, {A: 0, B: 3, C: 7}, // left
		{A: 1, B: 5, C: 6}, {A: 1, B: 6, C: 2}, // right
		{A: 3, B: 2, C: 6}, {A: 3, B: 6, C: 7}, // top
		{A: 0, B: 4, C: 5}, {A: 0, B: 5, C: 1}, // bottom
	}
	cols := []color.Color{}
	palette := []color.Color{
		color.RGBA{220, 60, 60, 255},
		color.RGBA{60, 200, 90, 255},
		color.RGBA{70, 120, 240, 255},
		color.RGBA{240, 200, 60, 255},
		color.RGBA{200, 80, 220, 255},
		color.RGBA{60, 210, 220, 255},
	}
	for i := range f {
		cols = append(cols, palette[(i/2)%len(palette)])
	}
	return canvas.Mesh3D{Vertices: v, Faces: f, FaceColors: cols}
}

func main() {
	a := app.New()
	w := a.NewWindow("Render Probe (#34)")

	// #34 — Scene3D: a rotating cube, animated (also drives dirty-region).
	scene := canvas.NewScene3D()
	scene.Background = color.RGBA{18, 18, 28, 255}
	scene.Meshes = []canvas.Mesh3D{cubeMesh()}
	scene.Resize(geo.NewSize(300, 300))
	scene.Move(geo.NewPos(30, 200))

	// #34 — BoxShadow via an elevated Card (shadow + fill).
	card := widget.NewCard("Elevated Card", "canvas.BoxShadow behind me",
		widget.NewLabel("soft Gaussian shadow"))

	// #34 — RepaintBoundary-cached subtree with a live-updating label inside.
	tick := widget.NewLabelWithStyle("t=0.00", bellui.TextAlignCenter, bellui.TextStyle{Bold: true})
	boundary := container.NewRepaintBoundary(container.NewVBox(
		widget.NewLabel("RepaintBoundary subtree:"), tick,
	))

	status := widget.NewLabel("rendering cube + shadow + boundary…")

	root := container.NewWithoutLayout()
	header := container.NewVBox(
		widget.NewLabelWithStyle("Render pipeline probe — #34", bellui.TextAlignCenter, bellui.TextStyle{Bold: true}),
		status,
		card,
		boundary,
	)
	header.Resize(geo.NewSize(680, 190))
	header.Move(geo.NewPos(10, 10))
	root.Add(header)
	root.Add(scene)

	w.SetContent(root)
	w.Resize(geo.NewSize(700, 560))

	anim := bellui.NewAnimation(6*time.Second, func(p float32) {
		ang := float64(p) * 2 * math.Pi
		r := 6.0
		scene.Camera.Position = canvas.Vec3{X: float32(r * math.Sin(ang)), Y: 3, Z: float32(r * math.Cos(ang))}
		scene.Refresh()
		tick.SetText(fmt.Sprintf("t=%.2f", p))
	})
	anim.RepeatCount = bellui.AnimationRepeatForever
	anim.Start()

	bellui.ShowAndRun(w)
}
