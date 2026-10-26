// Command colorprobe (PROBE #803, not for merge) fills its window with grey
// swatches at known sRGB values, so a screen capture shows how the platform
// interprets the surface format BELLUI_PROBE_SURFACE_FORMAT forces: as sRGB
// (128 reads back 128) or as linear light (128 reads back about 188).
package main

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

func main() {
	a := app.NewWithID("com.belluikit.colorprobe")
	a.Settings().SetTheme(clearBackground{theme.DefaultTheme()})
	w := a.NewWindow("colorprobe")
	var cells []core.CanvasObject
	for _, v := range []uint8{64, 128, 192} {
		r := canvas.NewRectangle(color.NRGBA{v, v, v, 255})
		r.SetMinSize(geo.NewSize(100, 100))
		cells = append(cells, r)
	}
	red := canvas.NewRectangle(color.NRGBA{255, 0, 0, 255})
	red.SetMinSize(geo.NewSize(100, 100))
	cells = append(cells, red, canvas.NewRectangle(color.Transparent)) // the clear colour shows through
	w.SetContent(container.New(layout.NewGridLayoutWithRows(5), cells...))
	w.SetPadded(false)
	w.Resize(geo.NewSize(240, 480))
	bellui.ShowAndRun(w)
}

// clearBackground leaves the window background transparent, so the painter's
// clear colour (the probe's float override) is what shows behind the swatches.
type clearBackground struct{ bellui.Theme }

func (t clearBackground) Color(n bellui.ThemeColorName, v bellui.ThemeVariant) color.Color {
	if n == theme.ColorNameBackground {
		return color.Transparent
	}
	return t.Theme.Color(n, v)
}
