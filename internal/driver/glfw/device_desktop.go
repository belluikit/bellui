//go:build !wasm

package glfw

import (
	"runtime"

	"github.com/belluikit/bellui"
)

func (*glDevice) IsMobile() bool {
	return false
}

func (*glDevice) SystemScaleForWindow(w bellui.Window) float32 {
	if runtime.GOOS == "darwin" {
		return 1.0 // macOS scaling is done at the texture level
	}
	if runtime.GOOS == "windows" {
		xScale, _ := w.(*window).viewport.GetContentScale()
		return xScale
	}

	return scaleAuto
}

func connectKeyboard(*glCanvas) {
	// no-op, mobile web compatibility
}

func isMacOSRuntime() bool {
	return runtime.GOOS == "darwin"
}
