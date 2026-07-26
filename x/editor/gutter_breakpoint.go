package editor

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// BreakpointGutter renders breakpoint markers (colored dots) in the gutter
// for lines in the [BreakpointGutter.Lines] set.
// It implements [InteractiveGutter]: tapping a row toggles its breakpoint.
type BreakpointGutter struct {
	// Lines is the set of 1-based line numbers that have breakpoints.
	Lines map[int]bool

	// OnChanged is called after a breakpoint is toggled.
	// The argument is the updated Lines map.
	OnChanged func(map[int]bool)

	// DotColor is the breakpoint marker color. Defaults to red.
	DotColor color.Color

	// DotScale controls the dot diameter relative to text size.
	// Defaults to 0.6 when zero.
	DotScale float32
}

// NewBreakpointGutter creates a breakpoint gutter.
// onChanged is called whenever a breakpoint is toggled; it may be nil.
func NewBreakpointGutter(onChanged func(map[int]bool)) *BreakpointGutter {
	return &BreakpointGutter{
		Lines:     make(map[int]bool),
		OnChanged: onChanged,
		DotColor:  color.NRGBA{R: 220, G: 50, B: 50, A: 255},
	}
}

func (g *BreakpointGutter) dotScale() float32 {
	if g.DotScale > 0 {
		return g.DotScale
	}
	return 0.6
}

// Width implements [GutterProvider].
// The breakpoint column is sized to fit a dot with padding.
func (g *BreakpointGutter) Width(_ int, textSize float32) float32 {
	return textSize * g.dotScale() * 1.6
}

// Render implements [GutterProvider]. line is 1-based.
func (g *BreakpointGutter) Render(line int, size geo.Size, textSize float32) []core.CanvasObject {
	if !g.Lines[line] {
		return nil
	}
	dotSize := textSize * g.dotScale()
	c := g.DotColor
	if c == nil {
		c = color.NRGBA{R: 220, G: 50, B: 50, A: 255}
	}
	dot := canvas.NewCircle(c)
	dot.Resize(geo.NewSize(dotSize, dotSize))
	dot.Move(geo.NewPos(
		(size.Width-dotSize)/2,
		(size.Height-dotSize)/2,
	))
	return []core.CanvasObject{dot}
}

// Tapped implements [InteractiveGutter]. Toggles the breakpoint at the
// given line (1-based) and fires OnChanged.
func (g *BreakpointGutter) Tapped(line int) {
	if g.Lines == nil {
		g.Lines = make(map[int]bool)
	}
	if g.Lines[line] {
		delete(g.Lines, line)
	} else {
		g.Lines[line] = true
	}
	if g.OnChanged != nil {
		g.OnChanged(g.Lines)
	}
}
