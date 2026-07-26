//go:build !tamago && !noos

package test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
)

// NewTempWindow creates and registers a new window for test purposes.
// This window will get removed automatically once the running test ends.
//
// Since: 2.5
func NewTempWindow(t testing.TB, content core.CanvasObject) bellui.Window {
	window := NewWindow(content)
	t.Cleanup(window.Close)
	return window
}
