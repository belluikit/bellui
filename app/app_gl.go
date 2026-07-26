//go:build !ci && !android && !ios && !mobile && !tamago && !noos && !tinygo

package app

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/driver/glfw"
)

// NewWithID returns a new app instance using the appropriate runtime driver.
// The ID string should be globally unique to this app.
func NewWithID(id string) bellui.App {
	return newAppWithDriver(glfw.NewGLDriver(), glfw.NewClipboard(), id)
}
