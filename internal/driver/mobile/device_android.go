//go:build android && !wasm

package mobile

import "github.com/belluikit/bellui"

const tapYOffset = -8.0 // to compensate for how we hold our fingers on the device

func (*device) SystemScaleForWindow(_ bellui.Window) float32 {
	return androidScaleForDPI(currentDPI)
}
