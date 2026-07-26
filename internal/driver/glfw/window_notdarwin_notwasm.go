//go:build !darwin && !wasm

package glfw

import "github.com/belluikit/glfw"

type monitor = glfw.Monitor

func (w *window) getSecondaryMonitor() *monitor {
	primary := glfw.GetPrimaryMonitor()
	for _, m := range glfw.GetMonitors() {
		if m.GetName() != primary.GetName() {
			return m
		}
	}

	return primary
}
