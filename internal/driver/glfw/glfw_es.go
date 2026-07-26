//go:build (gles || arm || arm64) && !darwin && !wasm

package glfw

import "github.com/belluikit/glfw"

func initWindowHints() {
	glfw.WindowHint(glfw.ClientAPI, glfw.OpenGLESAPI)
	glfw.WindowHint(glfw.ContextVersionMajor, 2)
	glfw.WindowHint(glfw.ContextVersionMinor, 0)
}
