package editor

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// GutterProvider renders content in the gutter area to the left of the text.
// The gutter is a separate widget that does not scroll horizontally with the
// text, matching the behavior of production code editors.
//
// Multiple providers can be composed side by side via [NewCompositeGutter].
type GutterProvider interface {
	// Width returns the desired gutter width in device-independent pixels.
	// totalRows is the line count of the document (useful for sizing based
	// on digit count). textSize is the current theme text size.
	Width(totalRows int, textSize float32) float32

	// Render returns canvas objects for the gutter cell at the given line.
	// line is 1-based. size is the allocated cell area (provider width x row
	// height). textSize is the current theme text size.
	Render(line int, size geo.Size, textSize float32) []core.CanvasObject
}

// InteractiveGutter extends [GutterProvider] with mouse interaction.
// When a gutter provider implements this interface, the gutter widget routes
// tap events to it.
type InteractiveGutter interface {
	GutterProvider
	// Tapped is called when the user clicks the gutter at the given line
	// (1-based).
	Tapped(line int)
}

// columnTappable is implemented by gutter providers that route taps by column —
// currently only [CompositeGutter]. The gutter widget prefers it over the plain
// [InteractiveGutter.Tapped] so a tap reaches only the provider whose column was
// clicked. x is the click's horizontal offset within the gutter (which does not
// scroll horizontally, so it is measured from the gutter's left edge).
type columnTappable interface {
	TappedAtX(line int, x float32)
}

// CompositeGutter combines multiple [GutterProvider] implementations,
// rendering them left to right in the order they appear.
type CompositeGutter struct {
	providers []GutterProvider
	totalRows int     // cached from most recent Width call
	textSize  float32 // cached from most recent Width call, for column hit-testing
}

// NewCompositeGutter creates a gutter that renders multiple providers
// side by side. For example, line numbers on the left and breakpoint
// markers on the right.
func NewCompositeGutter(providers ...GutterProvider) *CompositeGutter {
	return &CompositeGutter{providers: providers}
}

// Width returns the sum of all provider widths.
func (c *CompositeGutter) Width(totalRows int, textSize float32) float32 {
	c.totalRows = totalRows
	c.textSize = textSize
	var w float32
	for _, p := range c.providers {
		w += p.Width(totalRows, textSize)
	}
	return w
}

// Render returns canvas objects from all providers, offset horizontally
// so each provider occupies its own column.
func (c *CompositeGutter) Render(row int, size geo.Size, textSize float32) []core.CanvasObject {
	var objects []core.CanvasObject
	var xOffset float32
	for _, p := range c.providers {
		pw := p.Width(c.totalRows, textSize)
		cellSize := geo.NewSize(pw, size.Height)
		objs := p.Render(row, cellSize, textSize)
		for _, obj := range objs {
			pos := obj.Position()
			obj.Move(geo.NewPos(pos.X+xOffset, pos.Y))
			objects = append(objects, obj)
		}
		xOffset += pw
	}
	return objects
}

// Tapped dispatches the tap to every provider that implements
// [InteractiveGutter]. It is the column-agnostic fallback used when the click's
// horizontal position is unknown; the gutter widget prefers [CompositeGutter.TappedAtX]
// so a tap reaches only the clicked column.
func (c *CompositeGutter) Tapped(row int) {
	for _, p := range c.providers {
		if ig, ok := p.(InteractiveGutter); ok {
			ig.Tapped(row)
		}
	}
}

// TappedAtX routes a tap by the column x lands in, using the same left-to-right
// widths as [CompositeGutter.Render].
//
// A tap on an interactive provider's column reaches only that provider, so
// click-to-reveal (diagnostics) and a breakpoint column in the same composite
// don't both fire on a single tap (#95).
//
// A tap on a non-interactive column — the wide line-number margin in the common
// [line numbers, diagnostic, breakpoint] composition — falls back to toggling
// the breakpoint gutter, so the whole margin acts as the breakpoint hit area
// (VS Code-style) instead of swallowing the tap (#109). Because interactive
// marker columns are handled first, this cannot double-fire a reveal and a
// breakpoint toggle. A click past every column stays a no-op.
func (c *CompositeGutter) TappedAtX(row int, x float32) {
	var xOffset float32
	for _, p := range c.providers {
		pw := p.Width(c.totalRows, c.textSize)
		if x >= xOffset && x < xOffset+pw {
			if ig, ok := p.(InteractiveGutter); ok {
				ig.Tapped(row)
				return
			}
			// Non-interactive column (e.g. line numbers): treat the margin as
			// the breakpoint hit area rather than dropping the tap (#109).
			if bp := c.breakpointProvider(); bp != nil {
				bp.Tapped(row)
			}
			return
		}
		xOffset += pw
	}
}

// breakpointProvider returns the first [BreakpointGutter] among the composite's
// providers, or nil if none is present. It is the fallback target for taps that
// land on a non-interactive column (#109).
func (c *CompositeGutter) breakpointProvider() *BreakpointGutter {
	for _, p := range c.providers {
		if bp, ok := p.(*BreakpointGutter); ok {
			return bp
		}
	}
	return nil
}
