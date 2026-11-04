package widget_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

func TestIcon_Layout(t *testing.T) {
	test.NewTempApp(t)

	for name, tt := range map[string]struct {
		resource bellui.Resource
	}{
		"empty": {},
		"resource": {
			resource: theme.CancelIcon(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			icon := &widget.Icon{
				Resource: tt.resource,
			}

			window := test.NewTempWindow(t, &bellui.Container{Layout: layout.NewCenterLayout(), Objects: []core.CanvasObject{icon}})
			window.Resize(icon.MinSize().Max(geo.NewSize(150, 200)))

			test.AssertRendersToMarkup(t, "icon/layout_"+name+".xml", window.Canvas())
		})
	}
}

// TestIcon_IconSize pins #937: IconSize sets the icon's min size and the
// size its image is drawn at, inside a layout that keeps children at their
// min size; changing it and refreshing re-lays the icon out; zero keeps the
// theme's inline icon size.
func TestIcon_IconSize(t *testing.T) {
	test.NewTempApp(t)

	icon := widget.NewIcon(theme.CancelIcon())
	themed := icon.MinSize()
	if want := test.Theme().Size(theme.SizeNameInlineIcon); themed != geo.NewSquareSize(want) {
		t.Fatalf("zero IconSize: MinSize = %v, want the theme's %v", themed, want)
	}

	box := &bellui.Container{Layout: layout.NewCenterLayout(), Objects: []core.CanvasObject{icon}}
	window := test.NewTempWindow(t, box)
	window.Resize(geo.NewSize(200, 200))

	icon.IconSize = 96
	icon.Refresh()
	box.Refresh()
	if got := icon.MinSize(); got != geo.NewSquareSize(96) {
		t.Errorf("IconSize 96: MinSize = %v, want 96x96", got)
	}
	if got := icon.Size(); got != geo.NewSquareSize(96) {
		t.Errorf("IconSize 96: laid out at %v, want 96x96", got)
	}
	img := test.WidgetRenderer(icon).Objects()[0]
	if got := img.Size(); got != geo.NewSquareSize(96) {
		t.Errorf("IconSize 96: image drawn at %v, want 96x96", got)
	}

	icon.IconSize = 0
	icon.Refresh()
	box.Refresh()
	if got := icon.Size(); got != themed {
		t.Errorf("IconSize back to 0: laid out at %v, want the theme's %v", got, themed)
	}
}

// TestIcon_ColorizedResource pins #939: an icon tinted with
// theme.NewColorizedResource paints its own colour under both test themes.
func TestIcon_ColorizedResource(t *testing.T) {
	test.NewTempApp(t)
	red := color.NRGBA{R: 0xff, A: 0xff}

	for i, th := range []bellui.Theme{test.Theme(), test.NewTheme()} {
		test.ApplyTheme(t, th)
		icon := widget.NewIcon(theme.NewColorizedResource(theme.CancelIcon(), red))
		icon.IconSize = 48
		window := test.NewTempWindow(t, &bellui.Container{Layout: layout.NewCenterLayout(), Objects: []core.CanvasObject{icon}})
		window.Resize(geo.NewSize(100, 100))

		img := window.Canvas().Capture()
		reds := 0
		for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
			for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
				if color.NRGBAModel.Convert(img.At(x, y)) == red {
					reds++
				}
			}
		}
		if reds == 0 {
			t.Errorf("theme %d: no pixel of the icon is red", i)
		}
	}
}

// TestIcon_Color pins #952: Color tints the icon under both test themes,
// keeps the tint across a new Resource, and nil draws the theme colour again.
func TestIcon_Color(t *testing.T) {
	test.NewTempApp(t)
	red := color.NRGBA{R: 0xff, A: 0xff}
	reds := func(w bellui.Window) int {
		img := w.Canvas().Capture()
		n := 0
		for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
			for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
				if color.NRGBAModel.Convert(img.At(x, y)) == red {
					n++
				}
			}
		}
		return n
	}

	for i, th := range []bellui.Theme{test.Theme(), test.NewTheme()} {
		test.ApplyTheme(t, th)
		icon := widget.NewIcon(theme.CancelIcon())
		icon.IconSize = 48
		icon.Color = red
		icon.Refresh()
		window := test.NewTempWindow(t, &bellui.Container{Layout: layout.NewCenterLayout(), Objects: []core.CanvasObject{icon}})
		window.Resize(geo.NewSize(100, 100))
		if reds(window) == 0 {
			t.Errorf("theme %d: Color red, but no pixel of the icon is red", i)
		}

		icon.SetResource(theme.ConfirmIcon())
		if reds(window) == 0 {
			t.Errorf("theme %d: a new Resource lost the tint", i)
		}

		icon.Color = nil
		icon.Refresh()
		if n := reds(window); n != 0 {
			t.Errorf("theme %d: Color back to nil, but %d pixels are still red", i, n)
		}
	}
}

// TestIcon_ColorLeavesNonSVGAlone pins #952: a resource that is not an SVG is
// drawn as given, whatever Color says.
func TestIcon_ColorLeavesNonSVGAlone(t *testing.T) {
	test.NewTempApp(t)
	png := bellui.NewStaticResource("dot.png", []byte("\x89PNG\r\n\x1a\n"))
	icon := widget.NewIcon(png)
	icon.Color = color.NRGBA{R: 0xff, A: 0xff}
	icon.Refresh()

	raster := test.WidgetRenderer(icon).Objects()[0].(*canvas.Image)
	if raster.Resource != png {
		t.Errorf("drew %v, want the PNG resource unchanged", raster.Resource.Name())
	}
}
