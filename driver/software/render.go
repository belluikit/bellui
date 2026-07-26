package software

import (
	"image"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/app"
)

// RenderCanvas takes a canvas and renders it to a regular Go image using the provided Theme.
// This is the same as setting the application theme and then calling Canvas.Capture().
func RenderCanvas(c core.Canvas, t bellui.Theme) image.Image {
	bellui.CurrentApp().Settings().SetTheme(t)
	app.ApplyThemeTo(c.Content(), c)

	return c.Capture()
}

// Render takes a canvas object and renders it to a regular Go image using the provided Theme.
// The returned image will be set to the object's minimum size.
// Pass theme.DefaultTheme() (or a custom theme) to control the appearance.
func Render(obj core.CanvasObject, t bellui.Theme) image.Image {
	bellui.CurrentApp().Settings().SetTheme(t)

	c := NewCanvas()
	c.SetPadded(false)
	c.SetContent(obj)

	app.ApplyThemeTo(obj, c)
	return c.Capture()
}
