// Package component provides chart UI components (title, legend, tooltip, etc.).
package component

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/style"
)

// TitleConfig configures the chart title.
type TitleConfig struct {
	Text    string
	SubText string
}

// RenderTitle produces canvas objects for the chart title and optional subtitle.
// Returns the objects and the total height consumed.
func RenderTitle(cfg *TitleConfig, resolver *style.Resolver, x, y, maxWidth float32) ([]core.CanvasObject, float32) {
	if cfg == nil || cfg.Text == "" {
		return nil, 0
	}

	var objects []core.CanvasObject
	padding := resolver.Padding()
	currentY := y + padding

	// Main title
	title := canvas.NewText(cfg.Text, resolver.TitleColor())
	title.TextSize = resolver.TitleTextSize()
	title.TextStyle = bellui.TextStyle{Bold: true}
	title.Move(geo.NewPos(x, currentY))
	title.Resize(geo.NewSize(maxWidth, title.TextSize+4))
	objects = append(objects, title)
	currentY += title.TextSize + 2

	// Subtitle
	if cfg.SubText != "" {
		sub := canvas.NewText(cfg.SubText, resolver.AxisLabelColor())
		sub.TextSize = resolver.LabelTextSize()
		sub.Move(geo.NewPos(x, currentY))
		sub.Resize(geo.NewSize(maxWidth, sub.TextSize+4))
		objects = append(objects, sub)
		currentY += sub.TextSize + 2
	}

	totalHeight := currentY - y + padding
	return objects, totalHeight
}
