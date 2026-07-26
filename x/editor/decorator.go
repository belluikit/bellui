package editor

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// RowDecorator provides visual decorations for individual text rows.
// Decorators render behind or over the text content and span the full
// editor width (including the gutter area).
//
// Multiple decorators can be stacked; the editor renders them in order.
type RowDecorator interface {
	// Decorate returns canvas objects to render for the given line.
	// line is 1-based. pos is the top-left corner of the row area and
	// size is (full editor width, row height).
	// Return nil to skip decoration for this line.
	Decorate(line int, pos geo.Position, size geo.Size) []core.CanvasObject
}

// CurrentLineDecorator highlights a single row with a background color,
// typically used to indicate the current execution line in a debugger.
type CurrentLineDecorator struct {
	// Row is the 1-based line to highlight. Set to 0 to disable.
	Row int

	// Color is the highlight background color.
	// Defaults to a translucent yellow when nil.
	Color color.Color
}

// NewCurrentLineDecorator creates a decorator that highlights the given
// line (1-based). Pass 0 to start with no highlight.
func NewCurrentLineDecorator(row int) *CurrentLineDecorator {
	return &CurrentLineDecorator{
		Row:   row,
		Color: color.NRGBA{R: 255, G: 255, B: 100, A: 40},
	}
}

// Decorate implements [RowDecorator]. line is 1-based.
func (d *CurrentLineDecorator) Decorate(line int, pos geo.Position, size geo.Size) []core.CanvasObject {
	if line != d.Row {
		return nil
	}
	c := d.Color
	if c == nil {
		c = color.NRGBA{R: 255, G: 255, B: 100, A: 40}
	}
	bg := canvas.NewRectangle(c)
	bg.Move(pos)
	bg.Resize(size)
	return []core.CanvasObject{bg}
}

// SearchHighlightDecorator highlights rows that contain search matches.
type SearchHighlightDecorator struct {
	// Rows is the set of 1-based line numbers to highlight.
	Rows map[int]bool

	// Color is the highlight background color.
	// Defaults to a translucent orange when nil.
	Color color.Color
}

// NewSearchHighlightDecorator creates a decorator for search result rows.
func NewSearchHighlightDecorator() *SearchHighlightDecorator {
	return &SearchHighlightDecorator{
		Rows:  make(map[int]bool),
		Color: color.NRGBA{R: 255, G: 180, B: 50, A: 30},
	}
}

// Decorate implements [RowDecorator]. line is 1-based.
func (d *SearchHighlightDecorator) Decorate(line int, pos geo.Position, size geo.Size) []core.CanvasObject {
	if !d.Rows[line] {
		return nil
	}
	c := d.Color
	if c == nil {
		c = color.NRGBA{R: 255, G: 180, B: 50, A: 30}
	}
	bg := canvas.NewRectangle(c)
	bg.Move(pos)
	bg.Resize(size)
	return []core.CanvasObject{bg}
}
