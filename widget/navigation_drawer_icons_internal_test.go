package widget

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// inkIn reports how many pixels of r differ from the colour at r's top-left
// corner, which lies in the item's background, not its icon.
func inkIn(img image.Image, r image.Rectangle) int {
	bg := color.NRGBAModel.Convert(img.At(r.Min.X, r.Min.Y))
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if color.NRGBAModel.Convert(img.At(x, y)) != bg {
				n++
			}
		}
	}
	return n
}

// The software painter, which captures and goldens use, draws a drawer's item
// icons. They were built with no source, sized, and only then given their
// Resource, so nothing ever drew them and a capture showed the labels next to
// an empty space (#1298).
func TestNavigationDrawer_CaptureDrawsItemIcons(t *testing.T) {
	test.NewTempApp(t)
	items := []*NavigationDrawerItem{
		{Label: "Home", Icon: theme.HomeIcon()},
		{Label: "Settings", Icon: theme.SettingsIcon()},
	}
	d := NewNavigationDrawer(items, nil)
	w := test.NewTempWindow(t, NewLabel("content"))
	w.Resize(geo.NewSize(400, 600))
	d.Show(w.Canvas())

	rows := drawerRows(t, d)
	require.Len(t, rows, 2)
	img := w.Canvas().Capture()
	scale := float32(img.Bounds().Dx()) / w.Canvas().Size().Width
	for i, row := range rows {
		pos := row.icon.Position().Add(bellui.CurrentApp().Driver().AbsolutePositionForObject(row))
		size := row.icon.Size()
		require.Equal(t, geo.NewSize(24, 24), size)
		// Pad the box by 2px so its corner is background, not icon.
		r := image.Rect(int((pos.X-2)*scale), int((pos.Y-2)*scale),
			int((pos.X+size.Width+2)*scale), int((pos.Y+size.Height+2)*scale))
		assert.Greater(t, inkIn(img, r), 20, "row %d: the icon must be drawn in the capture", i)

		// Control: the label beside it is drawn, so the box is where the row is.
		lpos := row.label.Position().Add(bellui.CurrentApp().Driver().AbsolutePositionForObject(row))
		lsize := row.label.Size()
		lr := image.Rect(int((lpos.X-2)*scale), int((lpos.Y-2)*scale),
			int((lpos.X+lsize.Width+2)*scale), int((lpos.Y+lsize.Height+2)*scale))
		require.Greater(t, inkIn(img, lr), 20, "row %d: control, the label must be drawn", i)
	}
}
