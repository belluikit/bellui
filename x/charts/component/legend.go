package component

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/style"
)

// LegendPosition specifies where the legend is drawn.
type LegendPosition int

const (
	LegendBottom LegendPosition = iota // below the chart
	LegendTop                          // above the chart
	LegendLeft                         // left of the chart
	LegendRight                        // right of the chart
)

// LegendConfig configures the legend.
type LegendConfig struct {
	Position LegendPosition
}

// LegendItem represents one entry in the legend.
type LegendItem struct {
	Name    string
	Color   color.Color
	Visible bool
}

// LegendLayout holds the computed legend layout for hit testing legend clicks.
type LegendLayout struct {
	Items []legendItemLayout
}

type legendItemLayout struct {
	Rect  geo.Position // top-left of clickable area
	Size  geo.Size
	Index int // index into the items slice
}

// HitTest returns the legend item index at the given position, or -1 if none.
func (ll *LegendLayout) HitTest(pos geo.Position) int {
	for _, item := range ll.Items {
		if pos.X >= item.Rect.X && pos.X <= item.Rect.X+item.Size.Width &&
			pos.Y >= item.Rect.Y && pos.Y <= item.Rect.Y+item.Size.Height {
			return item.Index
		}
	}
	return -1
}

// RenderLegend produces canvas objects for the legend and returns the layout for click handling.
// The legend is rendered at the given position with the given max width.
func RenderLegend(items []LegendItem, cfg *LegendConfig, x, y, maxWidth float32, resolver *style.Resolver) ([]core.CanvasObject, *LegendLayout, float32) {
	if len(items) == 0 {
		return nil, nil, 0
	}

	var objects []core.CanvasObject
	layout := &LegendLayout{}

	swatchSize := resolver.LegendSwatchSize()
	gap := float32(6)
	itemGap := resolver.LegendItemGap()
	textHeight := float32(14)

	// Horizontal layout (default for bottom/top)
	currentX := x
	currentY := y

	for i, item := range items {
		labelW := float32(len(item.Name))*7 + 4
		itemW := swatchSize + gap + labelW

		// Wrap to next line if needed
		if currentX+itemW > x+maxWidth && currentX > x {
			currentX = x
			currentY += textHeight + 4
		}

		// Color swatch
		swatchColor := item.Color
		if !item.Visible {
			// Dimmed color for hidden series
			r, g, b, _ := swatchColor.RGBA()
			swatchColor = color.NRGBA{
				R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 80,
			}
		}

		swatch := canvas.NewRectangle(swatchColor)
		swatch.CornerRadius = 2
		swatch.Move(geo.NewPos(currentX, currentY+2))
		swatch.Resize(geo.NewSize(swatchSize, swatchSize))
		objects = append(objects, swatch)

		// Label
		textColor := resolver.LegendTextColor()
		if !item.Visible {
			textColor = resolver.LegendDimmedTextColor()
		}
		label := canvas.NewText(item.Name, textColor)
		label.TextSize = resolver.LegendTextSize()
		label.Move(geo.NewPos(currentX+swatchSize+gap, currentY))
		label.Resize(geo.NewSize(labelW, textHeight))
		objects = append(objects, label)

		// Track layout for click detection
		layout.Items = append(layout.Items, legendItemLayout{
			Rect:  geo.NewPos(currentX, currentY),
			Size:  geo.NewSize(itemW, textHeight),
			Index: i,
		})

		currentX += itemW + itemGap
	}

	totalHeight := currentY - y + textHeight + 8
	return objects, layout, totalHeight
}
