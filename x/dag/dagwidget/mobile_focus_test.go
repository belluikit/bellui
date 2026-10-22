package dagwidget

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// On a phone or tablet a tap selects without focusing the canvas: the mobile
// driver opens the on-screen keyboard for whatever has the focus, and the
// canvas takes no text (#415). On desktop a tap focuses it, so Delete and
// Enter act on the selection.
func TestTapFocusesCanvasOnlyOffMobile(t *testing.T) {
	for _, mobile := range []bool{false, true} {
		name := "desktop"
		if mobile {
			name = "mobile"
		}
		t.Run(name, func(t *testing.T) {
			old := isMobile
			isMobile = func() bool { return mobile }
			t.Cleanup(func() { isMobile = old })

			c := newTestCanvas(t)
			addTestNode(t, c, "a")
			w := test.NewWindow(c)
			t.Cleanup(w.Close)
			w.Resize(geo.NewSize(800, 600))

			taps := map[string]func(){
				"node tap":           func() { c.nodes["a"].Tapped(&event.PointEvent{}) },
				"node secondary tap": func() { c.nodes["a"].TappedSecondary(&event.PointEvent{}) },
				"canvas tap":         func() { c.Tapped(&event.PointEvent{Position: geo.NewPos(790, 590)}) },
			}
			for what, tap := range taps {
				w.Canvas().Unfocus()
				tap()
				focused := w.Canvas().Focused() == c
				if focused == mobile {
					t.Errorf("%s: canvas focused = %v, want %v", what, focused, !mobile)
				}
			}

			c.nodes["a"].Tapped(&event.PointEvent{})
			if !c.nodes["a"].Selected {
				t.Error("a node tap did not select the node")
			}
		})
	}
}
