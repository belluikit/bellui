//go:build linux && !wasm

package glfw

import (
	"testing"

	"github.com/belluikit/bellui"

	"github.com/stretchr/testify/assert"
)

// Clipboard flavors landed on Linux with pre-rebrand tracker #513 phase 2 (X11 selection /
// Wayland data-source support in the forked GLFW), so the capability
// assertions must succeed like on macOS and Windows.
func TestClipboard_FlavorsOnLinux(t *testing.T) {
	cb := NewClipboard()

	_, ok := cb.(bellui.ImageClipboard)
	assert.True(t, ok)
	_, ok = cb.(bellui.FileClipboard)
	assert.True(t, ok)
	_, ok = cb.(bellui.HTMLClipboard)
	assert.True(t, ok)
}
