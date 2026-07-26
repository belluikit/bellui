//go:build ((!gles && !arm && !arm64) || darwin) && !wasm

package glfw

import "github.com/belluikit/glfw"

func initWindowHints() {
	glfw.WindowHint(glfw.ContextVersionMajor, 2)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)

	glfw.WindowHint(glfw.CocoaGraphicsSwitching, glfw.True)
}
