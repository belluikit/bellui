package embedded

import (
	"image"

	"github.com/belluikit/bellui/geo"
)

// Driver is an embedded driver designed for handling custom hardware.
// Various standard driver implementations are available in the fyne-x project.
//
// Since: 1.0
type Driver interface {
	Render(image.Image)
	Run(func())

	ScreenSize() geo.Size
	Queue() chan Event
}
