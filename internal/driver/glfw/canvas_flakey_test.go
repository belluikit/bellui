//go:build !no_glfw && !ci && !mobile && flakey

package glfw

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
	"github.com/stretchr/testify/assert"
)

func TestGlCanvas_Resize(t *testing.T) {
	w := createWindow("Test")
	w.SetPadded(false)

	content := widget.NewLabel("Content")
	w.SetContent(content)
	ensureCanvasSize(t, w, geo.NewSize(69, 36))

	size := geo.NewSize(200, 100)
	runOnMain(func() {
		assert.NotEqual(t, size, content.Size())
	})

	w.Resize(size)
	ensureCanvasSize(t, w, size)

	runOnMain(func() {
		assert.Equal(t, size, content.Size())
	})
}
