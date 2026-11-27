package widget_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

// Right-to-left, a horizontal Slider, RangeSlider and ProgressBar run from
// the right: the minimum is at the right end, and a bar fills leftwards
// (#779). Left-to-right is unchanged.
func TestSliderProgress_Direction(t *testing.T) {
	for _, c := range []struct {
		name string
		dir  bellui.Direction
	}{{"ltr", bellui.DirectionLTR}, {"rtl", bellui.DirectionRTL}} {
		t.Run(c.name, func(t *testing.T) {
			test.NewTempApp(t)
			bellui.SetLayoutDirection(c.dir)
			defer bellui.SetLayoutDirection(bellui.DirectionAuto)

			slider := widget.NewSlider(0, 100)
			slider.SetValue(25)
			ranged := widget.NewRangeSlider(0, 100)
			ranged.SetValues(20, 60)
			bar := widget.NewProgressBar()
			bar.SetValue(0.25)
			w := test.NewTempWindow(t, container.NewVBox(slider, ranged, bar))
			w.Resize(geo.NewSize(300, 160))
			test.AssertRendersToImage(t, "slider_progress/"+c.name+".png", w.Canvas())

			// The thumb at a quarter sits in the leading half.
			mid := slider.Size().Width / 2
			if c.dir == bellui.DirectionRTL {
				assert.Greater(t, slider.CursorPosition().X, mid, "right-to-left, a low value is on the right")
			} else {
				assert.Less(t, slider.CursorPosition().X, mid, "left-to-right, a low value is on the left")
			}
		})
	}
}

// Under right-to-left, a pointer near the right end means a low value, and
// the arrow keys move the thumb the way they point: Left raises the value,
// as Android's Slider does (#779). This holds inside a DirectionScope as
// well as under the global direction, since event handlers run outside a
// layout walk.
func TestSlider_RTLInput(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		name := "global"
		if scoped {
			name = "scope"
		}
		t.Run(name, func(t *testing.T) {
			test.NewTempApp(t)
			slider := widget.NewSlider(0, 100)
			slider.Step = 10
			slider.SetValue(50)
			ranged := widget.NewRangeSlider(0, 100)
			ranged.Step = 10
			ranged.SetValues(20, 60)
			content := container.NewVBox(slider, ranged)
			if scoped {
				test.NewTempWindow(t, container.NewDirectionScope(content, bellui.DirectionRTL))
			} else {
				bellui.SetLayoutDirection(bellui.DirectionRTL)
				defer bellui.SetLayoutDirection(bellui.DirectionAuto)
				test.NewTempWindow(t, content)
			}

			slider.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
			assert.Equal(t, 60.0, slider.Value, "Left moves the thumb left, which is up")
			slider.TypedKey(&event.KeyEvent{Name: event.KeyRight})
			slider.TypedKey(&event.KeyEvent{Name: event.KeyRight})
			assert.Equal(t, 40.0, slider.Value, "Right moves the thumb right, which is down")

			size := slider.Size()
			slider.Tapped(&event.PointEvent{Position: geo.NewPos(size.Width-1, size.Height/2)})
			assert.Equal(t, 0.0, slider.Value, "the right end is the minimum")
			slider.Tapped(&event.PointEvent{Position: geo.NewPos(1, size.Height/2)})
			assert.Equal(t, 100.0, slider.Value, "the left end is the maximum")

			// The high thumb has the keys after the tap near the left end.
			rs := ranged.Size()
			ranged.Tapped(&event.PointEvent{Position: geo.NewPos(rs.Width*0.2, rs.Height/2)})
			assert.InDelta(t, 80.0, ranged.HighValue, 10, "a fifth from the left is four fifths up")
			high := ranged.HighValue
			ranged.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
			assert.Equal(t, high+10, ranged.HighValue, "Left raises the thumb with the keys")
		})
	}
}
