package editor

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/widget"
)

// StyleRun represents a styled segment of text within a single line.
// A [Highlighter] returns a slice of StyleRuns that together cover the
// full line text.
type StyleRun struct {
	// Text is the literal content of this run.
	Text string

	// ColorName is a theme-aware foreground color for this run.
	// When empty, the default foreground color is used.
	// Ignored when ForegroundColor is non-nil.
	ColorName bellui.ThemeColorName

	// ForegroundColor overrides ColorName with a direct color value.
	ForegroundColor color.Color

	// BackgroundColor, when non-nil, draws a background behind this run.
	BackgroundColor color.Color

	// Bold renders this run in bold weight.
	Bold bool

	// Italic renders this run in italic style.
	Italic bool

	// Underline draws a line beneath this run.
	Underline bool

	// UnderlineStyle selects the underline shape (solid by default, or a wavy
	// squiggle / dotted line for diagnostics). Only meaningful when Underline
	// is true.
	UnderlineStyle UnderlineStyle

	// UnderlineColor overrides the underline color. Defaults to the run's
	// foreground color when nil.
	UnderlineColor color.Color
}

// UnderlineStyle selects the shape of an underline, shared by [StyleRun]
// underlines and [DecoratedSpan] underline spans.
type UnderlineStyle int

const (
	// UnderlineSolid is a straight line, e.g. for link decorations.
	UnderlineSolid UnderlineStyle = iota
	// UnderlineWavy is a sine-wave squiggle, e.g. for error/warning diagnostics.
	UnderlineWavy
	// UnderlineDotted is a dotted line, an alternative diagnostic style.
	UnderlineDotted
)

// toWidget maps a fycoder UnderlineStyle to the fork's widget.UnderlineStyle.
func (u UnderlineStyle) toWidget() widget.UnderlineStyle {
	switch u {
	case UnderlineWavy:
		return widget.UnderlineWavy
	case UnderlineDotted:
		return widget.UnderlineDotted
	default:
		return widget.UnderlineSolid
	}
}

// Highlighter provides syntax highlighting for source code.
//
// Implementations are stateful across lines: the state value returned for
// one line is passed as input to the next, enabling multi-line constructs
// such as block comments and triple-quoted strings. State 0 is the initial
// (no active multi-line construct) state.
type Highlighter interface {
	// Highlight processes a single line of text and returns styled runs that
	// together cover the full line. state is 0 for the first line of a
	// document and the previous line's returned state thereafter.
	Highlight(line string, state int) ([]StyleRun, int)
}

// HighlighterFunc adapts a plain function to the [Highlighter] interface.
type HighlighterFunc func(line string, state int) ([]StyleRun, int)

// Highlight implements [Highlighter].
func (f HighlighterFunc) Highlight(line string, state int) ([]StyleRun, int) {
	return f(line, state)
}
