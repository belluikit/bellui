// Package main hosts the #960 repro scene: a Transform-scaled canvas.Text next
// to an untransformed Rectangle and a Transform-scaled Rectangle, laid out at
// explicit (non-zero) positions so any position-dependent transform error is
// visible. The three cases together answer: is the fault text-specific or
// shared by all Transform-scaled objects, and is it Adreno-specific or general?
//
// BuildScene is the deterministic (no-animation) scene shared by the on-device
// main() and the host software-render test, so the software golden and the GPU
// screenshot depict the same geometry.
package main

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// scale factor used for every transformed object in the scene.
const reproScale = 1.5

// text rendered by the text cases (ascender + descender for baseline checks).
const reproText = "Ag960"

func label(s string, pos geo.Position) *canvas.Text {
	t := canvas.NewText(s, color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF})
	t.TextSize = 14
	t.Resize(t.MinSize())
	t.Move(pos)
	return t
}

// scaledText returns a canvas.Text at pos with a Scale(reproScale) transform —
// the exact construct the #942 loupe used and the primary suspect.
func scaledText(s string, pos geo.Position, clr color.Color) *canvas.Text {
	t := canvas.NewText(s, clr)
	t.TextSize = 20
	t.Resize(t.MinSize())
	t.Move(pos)
	t.SetTransform(geo.NewScale(reproScale, reproScale))
	return t
}

func rect(clr color.Color, pos geo.Position, size geo.Size) *canvas.Rectangle {
	r := canvas.NewRectangle(clr)
	r.Resize(size)
	r.Move(pos)
	return r
}

func scaledRect(clr color.Color, pos geo.Position, size geo.Size) *canvas.Rectangle {
	r := rect(clr, pos, size)
	r.SetTransform(geo.NewScale(reproScale, reproScale))
	return r
}

// BuildScene composes the static repro. All suspects sit at deliberately
// non-zero positions: a Scale about a transform that also carries the object's
// translation will drift the object by pos*(scale-1), so a double-counted
// translation shows as the scaled objects landing far below/right of their
// untransformed reference markers.
func BuildScene() core.CanvasObject {
	bg := rect(color.NRGBA{R: 0x1E, G: 0x1E, B: 0x28, A: 0xFF}, geo.NewPos(0, 0), geo.NewSize(400, 700))

	// Reference markers (untransformed) at each suspect's declared position, so
	// the screenshot shows where each scaled object *should* be anchored.
	refA := rect(color.NRGBA{R: 0x40, G: 0x40, B: 0x40, A: 0xFF}, geo.NewPos(40, 120), geo.NewSize(4, 40))
	refT := label("↓ scaled text should start here", geo.NewPos(48, 128))

	// (a) Transform-scaled Text — the suspect.
	txt := scaledText(reproText, geo.NewPos(40, 160), color.NRGBA{R: 0xFF, G: 0xC0, B: 0x40, A: 0xFF})

	// (b) untransformed Rectangle (control — known-good).
	rOK := rect(color.NRGBA{R: 0x40, G: 0xC0, B: 0x60, A: 0xFF}, geo.NewPos(40, 320), geo.NewSize(120, 40))
	rOKlbl := label("untransformed rect", geo.NewPos(40, 300))

	// (c) Transform-scaled Rectangle (tells us text-specific vs all-transformed).
	rScaled := scaledRect(color.NRGBA{R: 0x40, G: 0x80, B: 0xFF, A: 0xFF}, geo.NewPos(40, 440), geo.NewSize(120, 40))
	rSlbl := label("scaled rect", geo.NewPos(40, 420))

	// (d) untransformed reference Text at same size (no transform), for a direct
	// glyph comparison against (a).
	refText := label("Ag960 (ref, no transform)", geo.NewPos(40, 600))

	c := container.NewWithoutLayout(bg, refA, refT, txt, rOKlbl, rOK, rSlbl, rScaled, refText)
	c.Resize(geo.NewSize(400, 700))
	return c
}
