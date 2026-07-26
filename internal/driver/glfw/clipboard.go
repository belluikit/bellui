//go:build !wasm

package glfw

import (
	"runtime"
	"time"

	"github.com/belluikit/bellui"

	"github.com/belluikit/glfw"
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
	// This retry logic is to work around the "Access Denied" error often thrown in windows PR#1679
	if runtime.GOOS != "windows" {
		return c.content()
	}
	for i := 3; i > 0; i-- {
		cb := c.content()
		if cb != "" {
			return cb
		}
		time.Sleep(50 * time.Millisecond)
	}
	// can't log retry as it would also log errors for an empty clipboard
	return ""
}

func (c clipboard) content() string {
	return glfw.GetClipboardString()
}

// SetContent sets the clipboard content
func (c clipboard) SetContent(content string) {
	// This retry logic is to work around the "Access Denied" error often thrown in windows PR#1679
	if runtime.GOOS != "windows" {
		c.setContent(content)
		return
	}
	for i := 3; i > 0; i-- {
		c.setContent(content)
		if c.content() == content {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	bellui.LogError("GLFW clipboard set failed", nil)
}

func (c clipboard) setContent(content string) {
	glfw.SetClipboardString(content)
}

// clipboardFilePaths reduces a URI list to the file paths that can go on the
// system clipboard. Only file-scheme URIs qualify; others are skipped with a
// logged error.
func clipboardFilePaths(files []bellui.URI) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		if file == nil {
			continue
		}
		if file.Scheme() != "file" {
			bellui.LogError("Only file URIs can be placed on the clipboard, skipping "+file.String(), nil)
			continue
		}
		paths = append(paths, file.Path())
	}
	return paths
}
