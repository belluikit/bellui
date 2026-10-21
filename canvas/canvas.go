package canvas

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/svg"
)

const (
	// RadiusMaximum can be applied to a canvas corner radius to achieve fully rounded corners.
	// This constant represents the maximum possible corner radius, resulting in a circular appearance.
	// Since: 1.0
	RadiusMaximum float32 = math.MaxFloat32
)

// Refresh instructs the containing canvas to refresh the specified obj.
func Refresh(obj core.CanvasObject) {
	app := bellui.CurrentApp()
	if app == nil || app.Driver() == nil {
		return
	}

	c := app.Driver().CanvasForObject(obj)
	if c != nil {
		c.Refresh(obj)
	}
}

// RecolorSVG takes a []byte containing SVG content, and returns
// new SVG content, re-colorized to be monochrome with the given color.
// The content can be assigned to a new bellui.StaticResource with an appropriate name
// to be used in a widget.Button, canvas.Image, etc.
//
// If an error occurs, the returned content will be the original un-modified content,
// and a non-nil error is returned.
//
// Since: 1.0
func RecolorSVG(svgContent []byte, color color.Color) ([]byte, error) {
	return svg.Colorize(svgContent, color)
}

// repaint instructs the containing canvas to redraw, even if nothing changed.
// Callers have moved, resized or hidden obj, and the canvas's partial repaint
// learns where from this alone: it does not walk what was not reported (#599).
func repaint(obj core.CanvasObject) {
	app := bellui.CurrentApp()
	if app == nil || app.Driver() == nil {
		return
	}

	c := app.Driver().CanvasForObject(obj)
	if c != nil {
		if m, ok := c.(interface{ MarkPlacementDirty(core.CanvasObject) }); ok {
			m.MarkPlacementDirty(obj)
		}
		if paint, ok := c.(interface{ SetDirty() }); ok {
			paint.SetDirty()
		}
	}
}
