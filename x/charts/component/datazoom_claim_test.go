package component

import (
	"testing"

	"github.com/belluikit/bellui/geo"
)

// The slider takes a touch drag along it and leaves one across it to the
// scroll around it, as widget.Slider does (#1266).
func TestDataZoomSlider_ClaimsDragAlongItsAxis(t *testing.T) {
	s := NewDataZoomSlider(nil)
	if !s.ClaimsDrag(geo.Delta{DX: 20, DY: 3}) {
		t.Error("the slider left a sideways drag to the page")
	}
	if s.ClaimsDrag(geo.Delta{DX: 3, DY: -20}) {
		t.Error("the slider claimed an upward drag, which it cannot use")
	}
}
