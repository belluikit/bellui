package playground

import (
	"github.com/belluikit/bellui/driver/software"
	"github.com/belluikit/bellui/test"
)

// NewSoftwareCanvas creates a new canvas in memory that can render without hardware support
func NewSoftwareCanvas() test.WindowlessCanvas {
	return software.NewCanvas()
}
