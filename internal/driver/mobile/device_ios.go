//go:build ios && !wasm

package mobile

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/driver/mobile/app"
)

const tapYOffset = -8.0 // to compensate for how we hold our fingers on the device

// SystemScaleForWindow is the screen's own pixels per point (nativeScale), so
// the layout has exactly the points the screen shows, Display Zoom included
// (#1016). Before the screen is set up it falls back to a guess from the DPI.
func (*device) SystemScaleForWindow(_ bellui.Window) float32 {
	if s := app.ScreenScale(); s > 0 {
		return s
	}
	if currentDPI >= 450 {
		return 3
	} else if currentDPI >= 340 {
		return 2.5
	}
	return 2
}
