//go:build wasm

package glfw

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/glfw-js"
)

// Declare conformity with Clipboard interface
var _ bellui.Clipboard = clipboard{}

func NewClipboard() bellui.Clipboard {
	return clipboard{}
}

// clipboard represents the system clipboard
type clipboard struct{}

// Content returns the clipboard content
func (c clipboard) Content() string {
	return glfw.GetClipboardString()
}

// SetContent sets the clipboard content
func (c clipboard) SetContent(content string) {
	glfw.SetClipboardString(content)
}
