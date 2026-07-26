// Package main hosts the #1086 repro scene: a Transform-scaled canvas.Text laid
// out center-aligned inside a container that is *wider and taller* than the
// text's natural content, next to reference markers. #1086 is the residual gap
// left by #960: drawText shifts pos for alignment *before* anchoring, so the
// scale used to pivot on the alignment-shifted glyph position rather than the
// text-box origin. This scene makes that offset visible on a real GPU.
//
// BuildAlignedScene is shared by the software-golden test and the on-GPU main so
// the software reference and the GPU screenshot depict the same geometry; with
// the #1086 fix they must coincide, and a pre-fix draw.go visibly shifts the
// scaled glyphs to the right of the software reference.
package main

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

const alignScale = 1.6

// boxOrigin is the text-box origin every aligned case anchors about. A vertical
// guide line is drawn here so the eye can check that the scaled glyphs pivot on
// this X (fixed) rather than drifting right (pre-fix).
var boxOrigin = geo.NewPos(60, 0)

// oversizedText returns a Scale(alignScale) canvas.Text sized to a container far
// wider/taller than its natural MinSize, with the given alignment. The slack
// (container - content) is what the alignment offset consumes, and the offset is
// exactly what #1086 mis-anchored.
func oversizedText(s string, y float32, align bellui.TextAlign, clr color.Color) *canvas.Text {
	t := canvas.NewText(s, clr)
	t.TextSize = 22
	t.Alignment = align
	// Container box: 240 wide, 120 tall — both well above the glyph's natural
	// size, so there is real horizontal slack (center/trailing) and vertical
	// slack (centering) for the alignment offset to move the glyphs.
	t.Resize(geo.NewSize(240, 120))
	t.Move(geo.NewPos(boxOrigin.X, y))
	t.SetTransform(geo.NewScale(alignScale, alignScale))
	return t
}

func alignLabel(s string, pos geo.Position) *canvas.Text {
	t := canvas.NewText(s, color.NRGBA{R: 0xC0, G: 0xC0, B: 0xC0, A: 0xFF})
	t.TextSize = 12
	t.Resize(t.MinSize())
	t.Move(pos)
	return t
}

// BuildAlignedScene composes three oversized-container cases (leading / center /
// trailing) stacked vertically, each with its container outline drawn so the
// slack is visible, plus a magenta guide at boxOrigin.X. Leading is the control
// (zero horizontal offset — exact before and after); center and trailing carry a
// non-zero alignment offset and are where #1086 shows.
func BuildAlignedScene() core.CanvasObject {
	bg := canvasRect(color.NRGBA{R: 0x1E, G: 0x1E, B: 0x28, A: 0xFF}, geo.NewPos(0, 0), geo.NewSize(400, 700))

	objs := []core.CanvasObject{bg}

	// Vertical guide at the text-box origin X: every scaled glyph must pivot on
	// this line, so its left extent should not drift right of it as alignment
	// changes.
	guide := canvasRect(color.NRGBA{R: 0xFF, G: 0x30, B: 0xC0, A: 0xFF}, geo.NewPos(boxOrigin.X, 40), geo.NewSize(2, 600))

	cases := []struct {
		name  string
		y     float32
		align bellui.TextAlign
		clr   color.Color
	}{
		{"leading (control)", 80, bellui.TextAlignLeading, color.NRGBA{R: 0x60, G: 0xC0, B: 0x60, A: 0xFF}},
		{"center", 260, bellui.TextAlignCenter, color.NRGBA{R: 0xFF, G: 0xC0, B: 0x40, A: 0xFF}},
		{"trailing", 440, bellui.TextAlignTrailing, color.NRGBA{R: 0x60, G: 0xA0, B: 0xFF, A: 0xFF}},
	}
	for _, c := range cases {
		// Container outline (unscaled) at the declared box so the slack the
		// alignment offset consumes is visible.
		outline := strokeRect(color.NRGBA{R: 0x50, G: 0x50, B: 0x60, A: 0xFF}, geo.NewPos(boxOrigin.X, c.y), geo.NewSize(240, 120))
		objs = append(objs, outline, alignLabel(c.name, geo.NewPos(boxOrigin.X, c.y-16)), oversizedText("Ag86", c.y, c.align, c.clr))
	}
	objs = append(objs, guide)

	root := container.NewWithoutLayout(objs...)
	root.Resize(geo.NewSize(400, 700))
	return root
}

func canvasRect(clr color.Color, pos geo.Position, size geo.Size) *canvas.Rectangle {
	r := canvas.NewRectangle(clr)
	r.Resize(size)
	r.Move(pos)
	return r
}

func strokeRect(clr color.Color, pos geo.Position, size geo.Size) *canvas.Rectangle {
	r := canvas.NewRectangle(color.Transparent)
	r.StrokeColor = clr
	r.StrokeWidth = 1
	r.Resize(size)
	r.Move(pos)
	return r
}
