//go:build !wasm

package mobile

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// Test_mobileDriver_MultiWindowStack documents and verifies the mobile
// multi-window semantics (pre-rebrand tracker #499): windows form a stack with only the topmost
// visible one shown at a time. A window opened over a visible one is a child;
// closing it pops back to reveal the previous window.
func Test_mobileDriver_MultiWindowStack(t *testing.T) {
	d := &driver{}

	w1 := d.CreateWindow("main").(*window)
	w1.SetContent(widget.NewLabel("main"))
	w1.Show()
	assert.False(t, w1.isChild, "the first window is not a child")
	assert.Same(t, w1, d.currentWindow(), "the only window is current")

	w2 := d.CreateWindow("child").(*window)
	assert.True(t, w2.isChild, "a window opened over a visible one is a child")
	w2.SetContent(widget.NewLabel("child"))
	w2.Show()
	assert.Same(t, w2, d.currentWindow(), "the topmost visible window is current")
	assert.Len(t, d.AllWindows(), 2)

	// only one window shows at a time — the topmost visible one. Closing the
	// child removes it from its owning driver's list (pre-rebrand tracker #644 — Close() uses the
	// window's own driver, not the current app driver), popping the stack back
	// to reveal the parent.
	w2.Close()
	assert.Len(t, d.AllWindows(), 1, "closing the child removes it from its own driver")
	assert.Same(t, w1, d.currentWindow(), "closing the child reveals the parent again")
}

func Test_mobileDriver_AbsolutePositionForObject(t *testing.T) {
	for name, tt := range map[string]struct {
		want          geo.Position
		windowIsChild bool
		windowPadded  bool
	}{
		"for an unpadded primary (non-child) window it is (0,0)": {
			want:          geo.NewPos(0, 0),
			windowIsChild: false,
			windowPadded:  false,
		},
		"for a padded primary (non-child) window it is (padding,padding)": {
			want:          geo.NewPos(4, 4),
			windowIsChild: false,
			windowPadded:  true,
		},
		"for an unpadded child window it is (0,0)": {
			want:          geo.NewPos(0, 0),
			windowIsChild: true,
			windowPadded:  false,
		},
		"for a padded child window it is (padding,padding)": {
			want:          geo.NewPos(4, 4),
			windowIsChild: true,
			windowPadded:  true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var o core.CanvasObject
			size := geo.NewSize(100, 100)
			d := &driver{}
			w := d.CreateWindow("main")
			w.SetPadded(tt.windowPadded)
			l := widget.NewLabel("main window")
			if !tt.windowIsChild {
				o = l
			}
			w.SetContent(l)
			w.Show()
			w.Resize(size)
			w = d.CreateWindow("child1")
			w.SetContent(widget.NewLabel("first child"))
			if tt.windowIsChild {
				w.Show()
			}
			w.Resize(size)
			w = d.CreateWindow("child2 - hidden")
			w.SetContent(widget.NewLabel("second child"))
			w.Resize(size)
			w = d.CreateWindow("child3")
			r := belluicanvas.NewRectangle(color.White)
			r.SetMinSize(geo.NewSize(42, 17))
			w.SetPadded(tt.windowPadded)
			w.SetContent(container.NewVBox(r))
			if tt.windowIsChild {
				w.Show()
				o = r
			}
			w.Resize(size)
			w = d.CreateWindow("child4 - hidden")
			w.SetContent(widget.NewLabel("fourth child"))
			w.Resize(size)

			got := d.AbsolutePositionForObject(o)
			assert.Equal(t, tt.want, got)
		})
	}
}
