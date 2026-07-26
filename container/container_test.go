package container

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/stretchr/testify/assert"
)

func TestContainer_NoResize(t *testing.T) {
	rect := &canvas.Rectangle{}
	rect.SetMinSize(geo.NewSize(100, 100))

	container := NewHBox(rect)
	assert.Equal(t, geo.NewSize(0, 0), container.Size())

	container.Resize(container.MinSize())
	assert.Equal(t, rect.MinSize(), container.Size())
}
