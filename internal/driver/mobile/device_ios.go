//go:build ios

package mobile

import "github.com/belluikit/bellui"

const tapYOffset = -8.0 // to compensate for how we hold our fingers on the device

func (*device) SystemScaleForWindow(_ bellui.Window) float32 {
	if currentDPI >= 450 {
		return 3
	} else if currentDPI >= 340 {
		return 2.5
	}
	return 2
}
