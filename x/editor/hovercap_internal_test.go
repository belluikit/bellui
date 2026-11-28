package editor

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// TestHoverCapture_LeavesScrollBarStrips pins the pre-rebrand tracker #151 fix: the transparent
// hover-capture layer is inset by the scroll bar strips (the vertical bar's
// and the bottom one), so the Entry's scroll bar underneath still receives
// the hover that expands it to its grabbable width — hover goes to the
// topmost hoverable, and a full-size capture layer starved the bar
// permanently. Right-to-left the vertical bar is at the left edge
// (bellui#1496), so the layer leaves the strip there instead.
func TestHoverCapture_LeavesScrollBarStrips(t *testing.T) {
	for _, c := range []struct {
		name string
		dir  bellui.Direction
	}{{"ltr", bellui.DirectionLTR}, {"rtl", bellui.DirectionRTL}} {
		t.Run(c.name, func(t *testing.T) {
			test.NewTempApp(t)
			bellui.SetLayoutDirection(c.dir)
			defer bellui.SetLayoutDirection(bellui.DirectionAuto)
			e := NewEditor()
			e.SetText(strings.Repeat("a long line of editor text\n", 200))
			w := test.NewWindow(e)
			defer w.Close()
			w.Resize(geo.NewSize(400, 300))

			require.NotNil(t, e.hoverCap)
			bar := theme.ScrollBarSize()

			// The entry and the capture layer's wrapper share the same stack, so the
			// entry's size is the full center area; the capture layer must be inset
			// by exactly one bar width on the vertical bar's side and the bottom.
			x := float32(0)
			if c.dir == bellui.DirectionRTL {
				x = bar
			}
			assert.Equal(t, geo.NewPos(x, 0), e.hoverCap.Position())
			assert.Equal(t, e.entry.Size().Width-bar, e.hoverCap.Size().Width)
			assert.Equal(t, e.entry.Size().Height-bar, e.hoverCap.Size().Height)

			// The pointer over the middle of the vertical bar's strip hovers
			// something under the layer (the bar), not the layer.
			if bellui.CurrentDevice().IsMobile() {
				return // no pointer hover on a touch device
			}
			mid := e.entry.Size().Width - bar/2
			if c.dir == bellui.DirectionRTL {
				mid = bar / 2
			}
			at := bellui.CurrentApp().Driver().AbsolutePositionForObject(e.entry).AddXY(mid, e.entry.Size().Height/2)
			test.MoveMouse(w.Canvas(), at)
			hovered := w.Canvas().(interface{ Hovered() event.Hoverable }).Hovered()
			assert.NotNil(t, hovered, "something is hovered over the bar's strip")
			assert.NotSame(t, e.hoverCap, hovered, "the capture layer leaves the bar's strip to the bar")
		})
	}
}
