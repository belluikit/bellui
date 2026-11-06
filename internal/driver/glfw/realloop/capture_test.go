//go:build darwin && !ci

package realloop_test

import (
	"image"
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// captureWait is how long capture tries for a frame before it gives up.
const captureWait = 5 * time.Second

// capture returns w's capture, its pixel scale, and where obj sits in the
// canvas. Capture returns nil while the window cannot paint a frame: macOS
// reports a window just shown, or one under another, occluded, and gives it
// no surface (#631). So it tries again until a frame is painted, and fails
// the test with that reason if none is within captureWait (#1019).
func capture(t *testing.T, w bellui.Window, obj core.CanvasObject) (image.Image, float32, geo.Position) {
	t.Helper()
	deadline := time.Now().Add(captureWait)
	for {
		var img image.Image
		var scale float32
		var origin geo.Position
		bellui.DoAndWait(func() {
			img = w.Canvas().Capture()
			if img == nil {
				return
			}
			scale = float32(img.Bounds().Dx()) / w.Canvas().Size().Width
			origin = testApp.Driver().AbsolutePositionForObject(obj)
		})
		if img != nil {
			return img, scale, origin
		}
		if time.Now().After(deadline) {
			t.Fatalf("the window painted no frame in %v: Capture returned nil (is it occluded?)", captureWait)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
