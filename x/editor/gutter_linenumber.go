package editor

import (
	"fmt"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// LineNumberGutter renders line numbers in the gutter area.
// It implements [GutterProvider].
type LineNumberGutter struct {
	// MinDigits is the minimum number of digit columns to reserve.
	// Defaults to 3 when zero.
	MinDigits int
}

// NewLineNumberGutter creates a line number gutter with default settings.
func NewLineNumberGutter() *LineNumberGutter {
	return &LineNumberGutter{}
}

func (g *LineNumberGutter) minDigits() int {
	if g.MinDigits > 0 {
		return g.MinDigits
	}
	return 3
}

// Width implements [GutterProvider].
func (g *LineNumberGutter) Width(totalRows int, textSize float32) float32 {
	digits := len(fmt.Sprint(totalRows))
	if digits < g.minDigits() {
		digits = g.minDigits()
	}
	// Measure digit string plus trailing space for padding.
	measured := bellui.MeasureText(
		strings.Repeat("0", digits)+" ",
		textSize,
		bellui.TextStyle{Monospace: true},
	)
	return measured.Width
}

// Render implements [GutterProvider]. line is 1-based.
func (g *LineNumberGutter) Render(line int, size geo.Size, textSize float32) []core.CanvasObject {
	num := canvas.NewText(fmt.Sprintf("%d", line), themeColor(ColorNameGutterText))
	num.TextSize = textSize
	num.TextStyle = bellui.TextStyle{Monospace: true}
	num.Alignment = bellui.TextAlignTrailing
	num.Resize(size)
	return []core.CanvasObject{num}
}
