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

// GutterColumn classifies where a provider belongs in the gutter's conventional
// left-to-right column order: markers on the leading edge, then the line
// numbers, then the fold control immediately beside the text.
//
// [CompositeGutter] arranges its providers by this classification (see
// [NewCompositeGutter]), so line numbers always sit next to the code with only
// the fold chevron between them — markers never push them away from the text.
type GutterColumn int

const (
	// GutterColumnMarker is the leading marker column: breakpoints,
	// diagnostics, VCS change bars and any other per-line symbol. It is the
	// default for providers that do not declare a column.
	GutterColumnMarker GutterColumn = iota
	// GutterColumnLineNumber is the line-number column.
	GutterColumnLineNumber
	// GutterColumnFold is the trailing fold-control column, drawn between the
	// line numbers and the text.
	GutterColumnFold
)

// GutterColumnProvider is implemented by gutter providers that declare which
// conventional column they belong to. Providers that do not implement it are
// treated as [GutterColumnMarker] and stay on the leading edge.
type GutterColumnProvider interface {
	GutterProvider
	// GutterColumn reports the provider's conventional column.
	GutterColumn() GutterColumn
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
// rendering them side by side.
//
// Columns are arranged in the conventional editor order — markers, then line
// numbers, then the fold control — rather than in the order they were passed,
// so the numbers always sit beside the code (see [GutterColumn]). Providers
// within one column keep their relative order. Use [NewCompositeGutterInOrder]
// to lay the providers out exactly as given instead.
type CompositeGutter struct {
	providers []GutterProvider
	// columns is providers arranged for rendering: the canonical column order
	// unless keepOrder is set, in which case it mirrors providers.
	columns   []GutterProvider
	keepOrder bool
	totalRows int     // cached from most recent Width call
	textSize  float32 // cached from most recent Width call, for column hit-testing
}

// NewCompositeGutter creates a gutter that renders multiple providers side by
// side, arranged in the conventional editor column order: markers (breakpoints,
// diagnostics, …) on the leading edge, then the line numbers, then the fold
// control next to the text. The order the providers are passed in does not
// matter; providers sharing a column keep their relative order.
//
// Each provider's column comes from [GutterColumnProvider]; the built-in
// providers declare their own, and anything else counts as a marker.
func NewCompositeGutter(providers ...GutterProvider) *CompositeGutter {
	c := &CompositeGutter{providers: providers}
	c.columns = orderGutterColumns(providers)
	return c
}

// NewCompositeGutterInOrder creates a composite gutter that renders the
// providers strictly left to right in the order given, bypassing the canonical
// column arrangement [NewCompositeGutter] applies. Use it for a bespoke gutter
// layout the conventional order does not express.
func NewCompositeGutterInOrder(providers ...GutterProvider) *CompositeGutter {
	return &CompositeGutter{providers: providers, columns: providers, keepOrder: true}
}

// orderGutterColumns arranges providers into the canonical column order,
// preserving the given order within each column (a stable partition, so the
// relative order of e.g. two marker providers is the caller's).
func orderGutterColumns(providers []GutterProvider) []GutterProvider {
	ordered := make([]GutterProvider, 0, len(providers))
	for _, col := range []GutterColumn{GutterColumnMarker, GutterColumnLineNumber, GutterColumnFold} {
		for _, p := range providers {
			if gutterColumnOf(p) == col {
				ordered = append(ordered, p)
			}
		}
	}
	return ordered
}

// gutterColumnOf reports p's column, defaulting undeclared providers to the
// leading marker column.
func gutterColumnOf(p GutterProvider) GutterColumn {
	if cp, ok := p.(GutterColumnProvider); ok {
		return cp.GutterColumn()
	}
	return GutterColumnMarker
}

// Providers returns the composite's providers in the order they are rendered
// (the canonical column order unless built with [NewCompositeGutterInOrder]).
func (c *CompositeGutter) Providers() []GutterProvider {
	return append([]GutterProvider(nil), c.columns...)
}

// Width returns the sum of all provider widths.
func (c *CompositeGutter) Width(totalRows int, textSize float32) float32 {
	c.totalRows = totalRows
	c.textSize = textSize
	var w float32
	for _, p := range c.columns {
		w += p.Width(totalRows, textSize)
	}
	return w
}

// Render returns canvas objects from all providers, offset horizontally
// so each provider occupies its own column.
func (c *CompositeGutter) Render(row int, size geo.Size, textSize float32) []core.CanvasObject {
	var objects []core.CanvasObject
	var xOffset float32
	for _, p := range c.columns {
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
	for _, p := range c.columns {
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
	for _, p := range c.columns {
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
	for _, p := range c.columns {
		if bp, ok := p.(*BreakpointGutter); ok {
			return bp
		}
	}
	return nil
}
