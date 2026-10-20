//go:build !ios && !android && !wasm

package mobile

import "github.com/belluikit/bellui"

// Content returns the clipboard content for mobile simulator runs
func (c mobileClipboard) Content() string {
	bellui.LogError("Clipboard is not supported in mobile simulation", nil)
	return ""
}

// SetContent sets the clipboard content for mobile simulator runs
func (c mobileClipboard) SetContent(content string) {
	bellui.LogError("Clipboard is not supported in mobile simulation", nil)
}
