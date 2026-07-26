package editor

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
)

// editorTextSize returns the current theme text size.
func editorTextSize() float32 {
	return theme.TextSize()
}

// editorCharHeight returns the height of a single character in the monospace
// font at the current theme text size. Used as a fallback before the Entry's
// RowMetrics are available.
func editorCharHeight() float32 {
	return bellui.MeasureText("M", editorTextSize(), bellui.TextStyle{Monospace: true}).Height
}
