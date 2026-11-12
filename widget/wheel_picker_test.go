package widget

import (
	"image"
	"image/color"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// newWheelBench builds a wheel of n labelled items, five visible, in a
// 100x250 window (horizontal: 250x100), so neighbouring items are 50 points
// apart. Its clock moves only when the test says so; the test driver runs an
// animation to its end as soon as it starts.
func newWheelBench(t *testing.T, n int, o Orientation, wrap bool) (*WheelPicker, *[]int, *time.Duration) {
	t.Helper()
	test.NewTempApp(t)
	elapsed := new(time.Duration)
	base := time.Unix(1000, 0)
	wheelNow = func() time.Time { return base.Add(*elapsed) }
	t.Cleanup(func() { wheelNow = time.Now })

	items := make([]core.CanvasObject, n)
	for i := range items {
		l := NewLabel(strconv.Itoa(i))
		l.SetAccessibilityLabel("item " + strconv.Itoa(i))
		items[i] = l
	}
	changes := &[]int{}
	wp := NewWheelPicker(items, func(i int) { *changes = append(*changes, i) })
	wp.Orientation, wp.Wrap, wp.VisibleItems = o, wrap, 5
	w := test.NewTempWindow(t, wp)
	w.SetPadded(false)
	if o == Vertical {
		w.Resize(geo.NewSize(100, 250))
	} else {
		w.Resize(geo.NewSize(250, 100))
	}
	return wp, changes, elapsed
}

// drag moves the finger by d points along the wheel in steps, 10 ms apart.
func drag(wp *WheelPicker, elapsed *time.Duration, d float32, steps int) {
	for range steps {
		*elapsed += 10 * time.Millisecond
		delta := geo.Delta{DY: d / float32(steps)}
		if wp.Orientation == Horizontal {
			delta = geo.Delta{DX: d / float32(steps)}
		}
		wp.Dragged(&event.DragEvent{Dragged: delta})
	}
}

// centre is the middle of o.
func centre(o core.CanvasObject) geo.Position {
	return o.Position().Add(geo.NewPos(o.Size().Width/2, o.Size().Height/2))
}

// release lifts the finger after it has rested long enough to count as stopped.
func release(wp *WheelPicker, elapsed *time.Duration) {
	*elapsed += 100 * time.Millisecond
	wp.DragEnd()
}

func TestWheelPicker_SettlesOnACentredItem(t *testing.T) {
	for _, o := range []Orientation{Vertical, Horizontal} {
		wp, changes, elapsed := newWheelBench(t, 7, o, false)
		wp.SetSelectedSilently(3)

		// 1.4 items towards the start reveals later items: rests on 4.
		drag(wp, elapsed, -70, 7)
		assert.InDelta(t, 4.4, wp.pos, 0.01, "the wheel follows the finger")
		assert.Equal(t, 3, wp.Selected, "no selection while spinning")
		release(wp, elapsed)
		assert.Equal(t, 4, wp.Selected)
		assert.Equal(t, float32(4), wp.pos, "settled with an item centred")
		assert.Equal(t, []int{4}, *changes)

		// Less than half an item back: it returns to 4, and nothing fires.
		drag(wp, elapsed, 20, 2)
		release(wp, elapsed)
		assert.Equal(t, 4, wp.Selected)
		assert.Equal(t, []int{4}, *changes)

		// The centred item sits in the middle of the wheel.
		mid := centre(wp.Items[4])
		if o == Vertical {
			assert.InDelta(t, 125, mid.Y, 0.5)
		} else {
			assert.InDelta(t, 125, mid.X, 0.5)
		}
	}
}

func TestWheelPicker_OnChangedOncePerChange(t *testing.T) {
	wp, changes, elapsed := newWheelBench(t, 9, Vertical, false)

	// Three items in one slow drag: one change, at the end.
	drag(wp, elapsed, -150, 15)
	assert.Empty(t, *changes, "nothing fires while spinning")
	release(wp, elapsed)
	assert.Equal(t, []int{3}, *changes)

	wp.SetSelectedSilently(5)
	assert.Equal(t, []int{3}, *changes, "SetSelectedSilently does not fire")
	wp.SetSelected(5)
	assert.Equal(t, []int{3}, *changes, "an unchanged selection does not fire")
	wp.SetSelected(1)
	assert.Equal(t, []int{3, 1}, *changes)
}

func TestWheelPicker_FlingCoasts(t *testing.T) {
	wp, changes, elapsed := newWheelBench(t, 20, Vertical, false)

	// 60 points in 30 ms is 2000 points a second: about 20 items of coast
	// at 0.499 s, from 1.2 items of travel. It stops at the last item.
	drag(wp, elapsed, -60, 3)
	wp.DragEnd() // released while moving
	assert.Equal(t, 19, wp.Selected)
	assert.Equal(t, []int{19}, *changes)

	// A slower flick carries fewer items: 1000 points a second down is
	// 499 points, ~10 items back, plus the 0.6 dragged, from 15.
	wp.SetSelectedSilently(15)
	drag(wp, elapsed, 30, 3)
	wp.DragEnd()
	assert.Equal(t, 4, wp.Selected)
}

func TestWheelPicker_EndsWithoutWrap(t *testing.T) {
	wp, changes, elapsed := newWheelBench(t, 5, Vertical, false)

	// Past the start the drag meets resistance and settles back on 0.
	drag(wp, elapsed, 90, 9)
	assert.InDelta(t, -0.6, wp.pos, 0.01, "1.8 items of travel moves a third as far")
	release(wp, elapsed)
	assert.Equal(t, 0, wp.Selected)
	assert.Empty(t, *changes)

	// Past the end likewise.
	wp.SetSelectedSilently(4)
	drag(wp, elapsed, -90, 9)
	assert.InDelta(t, 4.6, wp.pos, 0.01)
	release(wp, elapsed)
	assert.Equal(t, 4, wp.Selected)
	assert.Empty(t, *changes)

	// Stepping stops at the ends.
	wp.PerformAccessibleAction(core.ActionIncrement)
	assert.Equal(t, 4, wp.Selected)
	wp.SetSelectedSilently(0)
	wp.PerformAccessibleAction(core.ActionDecrement)
	assert.Equal(t, 0, wp.Selected)
	assert.Empty(t, *changes)
}

func TestWheelPicker_WrapsAtBothEnds(t *testing.T) {
	wp, changes, elapsed := newWheelBench(t, 5, Vertical, true)

	// Before the first item comes the last.
	drag(wp, elapsed, 50, 5)
	assert.InDelta(t, -1, wp.pos, 0.01, "no resistance on a ring")
	release(wp, elapsed)
	assert.Equal(t, 4, wp.Selected)
	assert.Equal(t, float32(4), wp.pos)

	// After the last comes the first.
	drag(wp, elapsed, -50, 5)
	release(wp, elapsed)
	assert.Equal(t, 0, wp.Selected)
	assert.Equal(t, []int{4, 0}, *changes)

	// The last item shows above the first: one item up from the centre.
	mid := centre(wp.Items[4])
	assert.InDelta(t, 75, mid.Y, 0.5)
	assert.True(t, wp.Items[4].Visible())

	wp.PerformAccessibleAction(core.ActionDecrement)
	assert.Equal(t, 4, wp.Selected)
	wp.PerformAccessibleAction(core.ActionIncrement)
	assert.Equal(t, 0, wp.Selected)
}

func TestWheelPicker_TapCentresAnItem(t *testing.T) {
	wp, changes, _ := newWheelBench(t, 7, Vertical, false)
	wp.SetSelectedSilently(3)

	wp.Tapped(&event.PointEvent{Position: geo.NewPos(50, 225)}) // two items down
	assert.Equal(t, 5, wp.Selected)
	wp.Tapped(&event.PointEvent{Position: geo.NewPos(50, 80)}) // one item up
	assert.Equal(t, 4, wp.Selected)
	wp.Tapped(&event.PointEvent{Position: geo.NewPos(50, 130)}) // the centre
	assert.Equal(t, 4, wp.Selected)
	assert.Equal(t, []int{5, 4}, *changes)
}

func TestWheelPicker_Accessible(t *testing.T) {
	wp, changes, _ := newWheelBench(t, 4, Vertical, false)
	wp.SetAccessibilityLabel("Mood")

	assert.Equal(t, core.RoleSpinButton, wp.AccessibleRole())
	assert.Equal(t, "item 0", wp.AccessibleValue(), "the value is the selected item's label")
	names := []string{}
	for _, a := range wp.AccessibleActions() {
		names = append(names, a.Name)
	}
	assert.ElementsMatch(t, []string{core.ActionIncrement, core.ActionDecrement}, names)

	wp.PerformAccessibleAction(core.ActionIncrement)
	assert.Equal(t, 1, wp.Selected)
	assert.Equal(t, "item 1", wp.AccessibleValue())
	wp.PerformAccessibleAction(core.ActionDecrement)
	assert.Equal(t, []int{1, 0}, *changes)

	lo, hi, cur := wp.AccessibleRange()
	assert.Equal(t, [3]float64{0, 3, 0}, [3]float64{lo, hi, cur})

	wp.Disable()
	wp.PerformAccessibleAction(core.ActionIncrement)
	assert.Equal(t, 0, wp.Selected, "a disabled wheel does not move")
}

// Items away from the centre are drawn smaller, down to ShrinkTo at the edge.
func TestWheelPicker_Shrinks(t *testing.T) {
	wp, _, _ := newWheelBench(t, 5, Vertical, false)
	wp.ShrinkTo = 0.5
	wp.SetSelectedSilently(2)
	wp.Refresh()

	// A label is shorter than the 50 point pitch: it keeps its own height
	// and is not stretched to the slot. Every item keeps that size and is
	// drawn scaled about its centre (#1183).
	natural := wp.Items[2].MinSize()
	require.Less(t, natural.Height, float32(50))
	r := test.TempWidgetRenderer(t, wp).(*wheelPickerRenderer)
	scale := func(i int) float32 {
		assert.Equal(t, natural, wp.Items[i].Size(), "item %d keeps its size", i)
		m := r.slots[i].Transform()
		mid := centre(wp.Items[i])
		x, y := m.TransformPoint(mid.X, mid.Y)
		assert.InDelta(t, mid.X, x, 0.01, "item %d scaled about its centre", i)
		assert.InDelta(t, mid.Y, y, 0.01, "item %d scaled about its centre", i)
		sx, sy := m.TransformVector(1, 1)
		assert.InDelta(t, sx, sy, 1e-6, "both sides scale alike")
		return sx
	}
	assert.InDelta(t, 1, scale(2), 1e-6, "the centred item at its own size")
	assert.True(t, r.slots[2].Transform().IsIdentity())
	assert.InDelta(t, 0.8, scale(1), 1e-6, "one item out of 2.5: 0.8")
	assert.InDelta(t, 0.6, scale(0), 1e-6, "two out of 2.5: 0.6")
	assert.InDelta(t, scale(1), scale(3), 1e-6)
}

// A spin moves the items and scales their slots, but never resizes them: a
// resized image is drawn again from its pixels, and an SVG rasterized again,
// on every frame of the spin (#1183).
func TestWheelPicker_SpinKeepsItemSizes(t *testing.T) {
	for _, o := range []Orientation{Vertical, Horizontal} {
		wp, _, elapsed := newWheelBench(t, 7, o, false)
		wp.SetSelectedSilently(3)
		sizes := make([]geo.Size, len(wp.Items))
		for i, it := range wp.Items {
			sizes[i] = it.MinSize()
		}
		for range 6 {
			drag(wp, elapsed, -13, 1)
			for i, it := range wp.Items {
				if it.Visible() {
					assert.Equal(t, sizes[i], it.Size(), "item %d", i)
				}
			}
		}
		release(wp, elapsed)
	}
}

// Items away from the centre fade, down to FadeTo at the edge; 1 turns the
// fade off.
func TestWheelPicker_Fades(t *testing.T) {
	wp, _, _ := newWheelBench(t, 5, Vertical, false)
	wp.FadeTo = 0.5
	wp.SetSelectedSilently(2)
	wp.Refresh()

	r := test.TempWidgetRenderer(t, wp).(*wheelPickerRenderer)
	op := func(i int) float32 { return r.slots[i].GroupOpacity() }
	assert.InDelta(t, 1, op(2), 1e-6, "the centred item is opaque")
	assert.InDelta(t, 0.8, op(1), 1e-6, "one item out of 2.5")
	assert.InDelta(t, 0.6, op(0), 1e-6, "two out of 2.5")
	assert.InDelta(t, op(1), op(3), 1e-6)

	wp.FadeTo = 1
	wp.Refresh()
	assert.InDelta(t, 1, op(0), 1e-6, "FadeTo 1: no fade")

	wp.FadeTo = 0
	wp.Refresh()
	assert.InDelta(t, 1-(1-0.3)*0.8, op(0), 1e-6, "zero means the default, 0.3")
}

// Items set after the renderer exists, then a selection with no Refresh in
// between, lay out against slots for the new items (#984): no panic, and the
// selected item is centred.
func TestWheelPicker_ItemsSetAfterTheRenderer(t *testing.T) {
	test.NewTempApp(t)
	wp := NewWheelPicker(nil, nil)
	wp.VisibleItems = 5
	w := test.NewTempWindow(t, wp)
	w.SetPadded(false)
	w.Resize(geo.NewSize(100, 250))
	wp.Refresh() // the renderer is built with no slots

	for i := range 3 {
		wp.Items = append(wp.Items, NewLabel(strconv.Itoa(i)))
	}
	require.NotPanics(t, func() { wp.SetSelectedSilently(1) })
	assert.Equal(t, 1, wp.Selected)
	assert.InDelta(t, 125, centre(wp.Items[1]).Y, 0.5, "item 1 is centred")
	assert.True(t, wp.Items[1].Visible())
}

// A selection made before the items arrive survives a Refresh of the empty
// wheel, and is clamped to the items once they are there (#984).
func TestWheelPicker_SelectionBeforeItemsSurvivesARefresh(t *testing.T) {
	test.NewTempApp(t)
	wp := NewWheelPicker(nil, nil)
	w := test.NewTempWindow(t, wp)
	w.Resize(geo.NewSize(100, 250))

	wp.SetSelectedSilently(2)
	wp.Refresh()
	assert.Equal(t, 2, wp.Selected, "an empty wheel keeps the selection")

	for i := range 4 {
		wp.Items = append(wp.Items, NewLabel(strconv.Itoa(i)))
	}
	wp.Refresh()
	assert.Equal(t, 2, wp.Selected)

	wp.Items = wp.Items[:2]
	wp.Refresh()
	assert.Equal(t, 1, wp.Selected, "clamped to the items it has")
}

// The software painter, behind test images and headless capture, draws a
// side item at its scale about its centre: the slot's transform reaches it as
// it reaches the GPU painters (#1183).
func TestWheelPicker_DrawsSideItemsScaled(t *testing.T) {
	test.NewTempApp(t)
	items := make([]core.CanvasObject, 5)
	for i := range items {
		r := canvas.NewRectangle(color.NRGBA{R: 255, A: 255})
		r.SetMinSize(geo.NewSize(40, 40))
		items[i] = r
	}
	wp := NewWheelPicker(items, nil)
	wp.VisibleItems, wp.ShrinkTo, wp.FadeTo = 5, 0.5, 1
	wp.SetSelectedSilently(2)
	w := test.NewTempWindow(t, wp)
	w.SetPadded(false)
	w.Resize(geo.NewSize(100, 250))
	img := w.Canvas().Capture()

	// The red run along a row, and its middle.
	red := func(y int) (width int, mid float32) {
		first, last := -1, -1
		for x := 0; x < img.Bounds().Dx(); x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.R > 200 && c.G < 80 && c.B < 80 {
				if first < 0 {
					first = x
				}
				last = x
			}
		}
		if first < 0 {
			return 0, 0
		}
		return last - first + 1, float32(first+last+1) / 2
	}
	width, mid := red(125)
	assert.InDelta(t, 40, width, 1, "the centred item at its own size")
	assert.InDelta(t, 50, mid, 1)
	width, mid = red(25)
	assert.InDelta(t, 24, width, 1, "two out of 2.5: 0.6 of 40")
	assert.InDelta(t, 50, mid, 1, "scaled about its centre")
	width, _ = red(25 - 13)
	assert.Zero(t, width, "and 0.6 tall: nothing 13 points above its centre")
	width, _ = red(25 - 11)
	assert.NotZero(t, width, "but something 11 points above it")
}

// A spin of SVG items rasterizes none of them again: each keeps the raster
// it was drawn with (#1183).
func TestWheelPicker_SpinRasterizesNoSVG(t *testing.T) {
	_, _, elapsed := newWheelBench(t, 1, Horizontal, false)
	items := make([]core.CanvasObject, 7)
	imgs := make([]*canvas.Image, 7)
	for i := range items {
		imgs[i] = canvas.NewImageFromResource(theme.HomeIcon())
		imgs[i].SetMinSize(geo.NewSize(40, 40))
		items[i] = imgs[i]
	}
	wp := NewWheelPicker(items, nil)
	wp.Orientation, wp.VisibleItems = Horizontal, 5
	wp.SetSelectedSilently(3)
	w := test.NewTempWindow(t, wp)
	w.SetPadded(false)
	w.Resize(geo.NewSize(250, 100))
	w.Canvas().(test.WindowlessCanvas).Capture()

	rasters := make([]image.Image, len(imgs))
	for i, img := range imgs {
		rasters[i] = img.Image
	}
	redrawn := 0
	for range 6 {
		drag(wp, elapsed, -13, 1)
		w.Canvas().(test.WindowlessCanvas).Capture()
		for i, img := range imgs {
			if rasters[i] != nil && img.Image != rasters[i] {
				redrawn++
				rasters[i] = img.Image
			}
		}
	}
	release(wp, elapsed)
	assert.NotNil(t, rasters[3], "the centred item was rasterized")
	assert.Zero(t, redrawn, "no SVG rasterized again during the spin")
}
