package widget_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// plainDisableable is an app's widget with no disabled look of its own: a
// filled square that looks the same enabled or disabled.
type plainDisableable struct {
	widget.DisableableWidget
	fill color.Color
}

func newPlainDisableable(fill color.Color) *plainDisableable {
	w := &plainDisableable{fill: fill}
	w.ExtendBaseWidget(w)
	return w
}

func (w *plainDisableable) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(w.fill))
}

func (w *plainDisableable) MinSize() geo.Size { return geo.NewSize(40, 40) }

// commonest returns the most frequent colour in r of img.
func commonest(img image.Image, r image.Rectangle) color.NRGBA {
	counts := map[color.NRGBA]int{}
	var best color.NRGBA
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if counts[c]++; counts[c] > counts[best] {
				best = c
			}
		}
	}
	return best
}

// over composites c at alpha a over the opaque bg, per channel.
func over(bg, c color.Color, a float64) color.NRGBA {
	b := color.NRGBAModel.Convert(bg).(color.NRGBA)
	f := color.NRGBAModel.Convert(c).(color.NRGBA)
	a *= float64(f.A) / 255
	mix := func(x, y uint8) uint8 { return uint8(float64(x)*(1-a) + float64(y)*a + 0.5) }
	return color.NRGBA{mix(b.R, f.R), mix(b.G, f.G), mix(b.B, f.B), 255}
}

func near(t *testing.T, want, got color.NRGBA, msg string) {
	t.Helper()
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	if d(want.R, got.R) > 2 || d(want.G, got.G) > 2 || d(want.B, got.B) > 2 {
		t.Errorf("%s: drew %v, want %v", msg, got, want)
	}
}

// A disabled Button draws MD3's disabled fill itself, and the paint walk does
// not dim it again. A widget with no disabled look of its own is dimmed to
// 38%. Test captures use the software painter, which now dims as the desktop
// and mobile walks do, so this is how the Button looks on every platform
// (#1223). On the desktop the walk used to dim the Button too: measured on the
// GL driver, its fill drew at 22 instead of 27 and its label at 51 instead of
// 103 (red channel, dark theme).
func TestDisabledLook_ButtonIsNotDimmedTwice(t *testing.T) {
	test.NewTempApp(t)
	btn := widget.NewButton("MMMM", nil)
	btn.Disable()
	square := newPlainDisableable(color.NRGBA{R: 200, G: 40, B: 40, A: 255})
	square.Disable()

	w := test.NewWindow(container.NewVBox(btn, container.NewHBox(square)))
	defer w.Close()
	w.Resize(geo.NewSize(200, 120))
	img := w.Canvas().Capture()
	scale := float32(img.Bounds().Dx()) / w.Canvas().Size().Width
	rect := func(o core.CanvasObject) image.Rectangle {
		p, s := bellui.CurrentApp().Driver().AbsolutePositionForObject(o), o.Size()
		return image.Rect(int(p.X*scale)+2, int(p.Y*scale)+2, int((p.X+s.Width)*scale)-2, int((p.Y+s.Height)*scale)-2)
	}

	th := theme.CurrentForWidget(btn)
	v := bellui.CurrentApp().Settings().ThemeVariant()
	bg := th.Color(theme.ColorNameBackground, v)
	near(t, over(bg, th.Color(theme.ColorNameDisabledButton, v), 1), commonest(img, rect(btn)),
		"the disabled Button's fill")
	near(t, over(bg, square.fill, 0.38), commonest(img, rect(square)),
		"a disabled widget with no disabled look of its own, at 38%")
}
