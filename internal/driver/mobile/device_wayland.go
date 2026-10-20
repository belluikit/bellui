//go:build wayland && !wasm

package mobile

import "github.com/belluikit/bellui"

const tapYOffset = -4.0 // to compensate for how we hold our fingers on the device

func (*device) SystemScaleForWindow(_ bellui.Window) float32 {
	return 1 // PinePhone simplification, our only wayland mobile currently
}

func setDisableScreenBlank(_ bool) {
	// ignore in mobile simulation mode
}
