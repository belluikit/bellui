//go:build ci

package app

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/painter/software"
	"github.com/belluikit/bellui/test"
)

// NewWithID returns a new app instance using the test (headless) driver.
// The ID string should be globally unique to this app.
func NewWithID(id string) bellui.App {
	return newAppWithDriver(test.NewDriverWithPainter(software.NewPainter()), test.NewClipboard(), id)
}
