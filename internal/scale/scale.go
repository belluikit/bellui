package scale

import (
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// ToScreenCoordinate converts a bellui coordinate in the given canvas to a screen coordinate
func ToScreenCoordinate(c core.Canvas, v float32) int {
	if c.Scale() == 0 {
		bellui.LogError("Incorrect scale (0.0) on canvas, most likely not set", nil)
		return int(math.Ceil(float64(v)))
	}
	return int(math.Ceil(float64(v * c.Scale())))
}

// ToBelluiCoordinate converts a screen coordinate for a given canvas to a bellui coordinate
func ToBelluiCoordinate(c core.Canvas, v int) float32 {
	switch c.Scale() {
	case 0.0:
		bellui.LogError("Incorrect scale (0.0) on canvas, most likely not set", nil)
		return float32(v)
	case 1.0:
		return float32(v)
	default:
		return float32(v) / c.Scale()
	}
}

// ToBelluiSize returns the scaled size of an object based on pixel coordinates, typically for images.
// This method will attempt to find the canvas for an object to get its scale.
// In the event that this fails it will assume a 1:1 mapping (scale=1 or low DPI display).
func ToBelluiSize(obj core.CanvasObject, width, height int) geo.Size {
	app := bellui.CurrentApp()
	if app == nil {
		return geo.NewSize(float32(width), float32(height)) // can occur if called before app.New
	}
	driver := app.Driver()
	if driver == nil {
		return geo.NewSize(float32(width), float32(height))
	}
	c := driver.CanvasForObject(obj)
	if c == nil {
		return geo.NewSize(float32(width), float32(height)) // this will happen a lot during init
	}
	return geo.NewSize(ToBelluiCoordinate(c, width), ToBelluiCoordinate(c, height))
}
