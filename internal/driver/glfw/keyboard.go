//go:build wasm

package glfw

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
)

func hideVirtualKeyboard() {
	if d, ok := bellui.CurrentDevice().(*glDevice); ok {
		d.hideVirtualKeyboard()
	}
}

func handleKeyboard(obj event.Focusable) {
	isDisabled := false
	if disWid, ok := obj.(core.Disableable); ok {
		isDisabled = disWid.Disabled()
	}
	if obj != nil && !isDisabled {
		showVirtualKeyboard()
	} else {
		hideVirtualKeyboard()
	}
}

func showVirtualKeyboard() {
	if d, ok := bellui.CurrentDevice().(*glDevice); ok {
		d.showVirtualKeyboard()
	}
}
