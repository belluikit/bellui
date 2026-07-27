package editor

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// FoldGutter draws fold/unfold chevrons in the gutter on foldable lines and
// toggles a region when its chevron is tapped. It implements [InteractiveGutter]
// and composes with the line-number and breakpoint gutters via
// [NewCompositeGutter], mirroring [BreakpointGutter] and [DiagnosticGutter].
//
// An editor keeps an internal FoldGutter wired to its fold state — obtain it via
// [Editor.FoldGutter] and drop it into your CompositeGutter:
//
//	e.Gutter = NewCompositeGutter(NewLineNumberGutter(), e.FoldGutter())
type FoldGutter struct {
	// foldable/collapsed/toggle bridge to the owning editor's fold state, keyed by
	// 0-based row. They are nil on a bare FoldGutter (drawn empty, inert on tap).
	foldable  func(row int) bool
	collapsed func(row int) bool
	toggle    func(row int)

	// MarkerColor is the chevron color. Nil falls back to the theme's
	// [ColorNameFoldMarker].
	MarkerColor color.Color

	// Scale controls the chevron size relative to the text size. Defaults to 0.5
	// when zero.
	Scale float32
}

// newFoldGutter builds a FoldGutter bridged to an editor's fold controller.
func newFoldGutter(foldable, collapsed func(int) bool, toggle func(int)) *FoldGutter {
	return &FoldGutter{foldable: foldable, collapsed: collapsed, toggle: toggle}
}

func (g *FoldGutter) scale() float32 {
	if g.Scale > 0 {
		return g.Scale
	}
	return 0.5
}

// GutterColumn implements [GutterColumnProvider]: the fold control is the
// trailing column, drawn immediately beside the text.
func (g *FoldGutter) GutterColumn() GutterColumn { return GutterColumnFold }

// Width implements [GutterProvider]. The column fits a chevron with padding.
func (g *FoldGutter) Width(_ int, textSize float32) float32 {
	return textSize * g.scale() * 2.4
}

// Render implements [GutterProvider]. line is 1-based. It draws a chevron on
// foldable lines — pointing right when collapsed, down when expanded — and
// nothing on other lines.
func (g *FoldGutter) Render(line int, size geo.Size, textSize float32) []core.CanvasObject {
	row := line - 1
	if g.foldable == nil || !g.foldable(row) {
		return nil
	}
	col := g.MarkerColor
	if col == nil {
		col = themeColor(ColorNameFoldMarker)
	}
	folded := g.collapsed != nil && g.collapsed(row)
	return chevron(size, textSize*g.scale(), folded, col)
}

// Tapped implements [InteractiveGutter]. line is 1-based; a tap on a foldable
// line toggles its region.
func (g *FoldGutter) Tapped(line int) {
	row := line - 1
	if g.foldable != nil && g.foldable(row) && g.toggle != nil {
		g.toggle(row)
	}
}

// chevron builds a two-stroke chevron centered in size: ">" (pointing right)
// when folded, "v" (pointing down) when expanded, matching the affordance code
// editors use. arm is the chevron's half-extent.
func chevron(size geo.Size, arm float32, folded bool, col color.Color) []core.CanvasObject {
	cx, cy := size.Width/2, size.Height/2
	h := arm / 2
	stroke := max(float32(1.25), arm/6)

	var p1, apex, p2 geo.Position
	if folded {
		// Right-pointing: top-left → mid-right → bottom-left.
		p1 = geo.NewPos(cx-h, cy-h)
		apex = geo.NewPos(cx+h, cy)
		p2 = geo.NewPos(cx-h, cy+h)
	} else {
		// Down-pointing: top-left → bottom-mid → top-right.
		p1 = geo.NewPos(cx-h, cy-h)
		apex = geo.NewPos(cx, cy+h)
		p2 = geo.NewPos(cx+h, cy-h)
	}

	a := canvas.NewLine(col)
	a.StrokeWidth = stroke
	a.Position1, a.Position2 = p1, apex
	b := canvas.NewLine(col)
	b.StrokeWidth = stroke
	b.Position1, b.Position2 = apex, p2
	return []core.CanvasObject{a, b}
}
