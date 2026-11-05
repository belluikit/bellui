//go:build (gl || (!darwin && !linux && !windows && !wasm)) && ((!gles && !arm && !arm64 && !android && !ios && !mobile && !wasm) || (darwin && !mobile && !ios && !wasm)) && !no_glfw

package gl

import "github.com/belluikit/glfw"

// contextHints asks for the context gl_core.go's bindings load: desktop GL 2.1.
func contextHints() {
	glfw.WindowHint(glfw.ClientAPI, glfw.OpenGLAPI)
	glfw.WindowHint(glfw.ContextVersionMajor, 2)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
}
