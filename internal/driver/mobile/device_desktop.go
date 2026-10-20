//go:build !ios && !android && !wayland && !wasm

package mobile

import "github.com/belluikit/bellui"

const tapYOffset = 0 // no finger compensation on desktop (simulation)

func (*device) SystemScaleForWindow(_ bellui.Window) float32 {
	return 2 // this is simply due to the high number of pixels on a mobile device - just an approximation
}

func setDisableScreenBlank(_ bool) {
	// ignore in mobile simulation mode
}
