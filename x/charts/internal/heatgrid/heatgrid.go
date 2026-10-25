// Package heatgrid lays out a heatmap's cells and its row and column labels.
// The series renderer draws the cells where Layout puts them and the chart
// hit-tests the same rectangle, so both read it from here (#343).
package heatgrid

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/x/charts/coord"
)

// LabelSize is the text size of the row and column labels.
const LabelSize float32 = 10

// gap separates the labels from the cells.
const gap float32 = 4

// Layout returns the rectangle the cells take within viewport: all of it
// without labels, and the viewport less a band on the left for the row labels
// (yLabels) and one at the bottom for the column labels (xLabels).
func Layout(viewport coord.Rect, xLabels, yLabels []string) coord.Rect {
	cells := viewport
	if w := maxWidth(yLabels); w > 0 {
		cells.X += w + gap
		cells.W -= w + gap
	}
	if len(xLabels) > 0 {
		cells.H -= LabelSize + 2 + gap
	}
	if cells.W < 0 {
		cells.W = 0
	}
	if cells.H < 0 {
		cells.H = 0
	}
	return cells
}

// LabelWidth is the width a label takes at LabelSize.
func LabelWidth(s string) float32 {
	return bellui.MeasureText(s, LabelSize, bellui.TextStyle{}).Width
}

// Gap is the space between the labels and the cells.
func Gap() float32 { return gap }

func maxWidth(labels []string) float32 {
	var w float32
	for _, l := range labels {
		if lw := LabelWidth(l); lw > w {
			w = lw
		}
	}
	return w
}
