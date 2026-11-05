//go:build (gl || (!darwin && !linux && !windows && !wasm)) && (gles || arm || arm64) && !android && !ios && !mobile && !darwin && !wasm && !no_glfw

package gl

import "github.com/belluikit/glfw"

// contextHints asks for the context gl_es.go's bindings load: OpenGL ES 2.
func contextHints() {
	glfw.WindowHint(glfw.ClientAPI, glfw.OpenGLESAPI)
	glfw.WindowHint(glfw.ContextVersionMajor, 2)
	glfw.WindowHint(glfw.ContextVersionMinor, 0)
}
