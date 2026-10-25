package component

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/belluikit/bellui/x/charts/interact"
)

// The slider's track and masks take the theme's colours; fixed dark greys drew
// a dark slab under a chart on a light theme (#343).
func TestDataZoomSlider_FollowsTheTheme(t *testing.T) {
	for _, v := range []bellui.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		a := test.NewTempApp(t)
		a.Settings().SetTheme(theme.DefaultTheme())
		a.Settings().(interface{ SetThemeVariant(bellui.ThemeVariant) }).SetThemeVariant(v)

		s := NewDataZoomSlider(nil)
		s.SetWindow(interact.DataWindow{Start: 0.25, End: 0.75})
		r := test.TempWidgetRenderer(t, s)
		r.Layout(geo.NewSize(400, 30))
		objs := r.Objects()
		if len(objs) < 3 {
			t.Fatalf("variant %d: %d objects, want a track and two masks at least", v, len(objs))
		}
		if got, exp := objs[0].(*canvas.Rectangle).FillColor, theme.DefaultTheme().Color(theme.ColorNameInputBackground, v); got != exp {
			t.Errorf("variant %d: the track is %v, want the theme's input background %v", v, got, exp)
		}
		bg := color.NRGBAModel.Convert(theme.DefaultTheme().Color(theme.ColorNameBackground, v)).(color.NRGBA)
		for _, i := range []int{1, 2} {
			if got, exp := objs[i].(*canvas.Rectangle).FillColor, (color.NRGBA{R: bg.R, G: bg.G, B: bg.B, A: 180}); got != exp {
				t.Errorf("variant %d: mask %d is %v, want the theme's background, translucent: %v", v, i, got, exp)
			}
		}
	}
}
