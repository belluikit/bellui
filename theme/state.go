package theme

import (
	"image/color"

	col "github.com/belluikit/bellui/internal/color"
)

// MD3 state layer opacity values.
const (
	stateLayerHoverOpacity   = 0.08
	stateLayerFocusOpacity   = 0.12
	stateLayerPressedOpacity = 0.12
	stateLayerDraggedOpacity = 0.16
)

// StateLayerHover returns a state layer color for hover state (8% opacity of content color).
func StateLayerHover(contentColor color.Color) color.Color {
	return stateLayer(contentColor, stateLayerHoverOpacity)
}

// StateLayerFocus returns a state layer color for focus state (12% opacity of content color).
func StateLayerFocus(contentColor color.Color) color.Color {
	return stateLayer(contentColor, stateLayerFocusOpacity)
}

// StateLayerPressed returns a state layer color for pressed state (12% opacity of content color).
func StateLayerPressed(contentColor color.Color) color.Color {
	return stateLayer(contentColor, stateLayerPressedOpacity)
}

// StateLayerDragged returns a state layer color for dragged state (16% opacity of content color).
func StateLayerDragged(contentColor color.Color) color.Color {
	return stateLayer(contentColor, stateLayerDraggedOpacity)
}

func stateLayer(contentColor color.Color, opacity float64) color.Color {
	r, g, b, _ := col.ToNRGBA(contentColor)
	return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(opacity * 255)}
}
