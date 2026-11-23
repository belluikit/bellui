package container_test

import (
	"image"
	"image/color"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// These compare renders made on the same machine rather than golden images:
// AppTabs and InnerWindow draw differently per platform (the window buttons'
// side, mobile tab bars), so a golden would hold on one platform only.

// region is the captured pixels of o, in a capture of w.
func region(t *testing.T, w bellui.Window, o core.CanvasObject) *image.NRGBA {
	t.Helper()
	img := w.Canvas().Capture()
	scale := float32(img.Bounds().Dx()) / w.Canvas().Size().Width
	pos := bellui.CurrentApp().Driver().AbsolutePositionForObject(o)
	size := o.Size()
	r := image.Rect(int(pos.X*scale), int(pos.Y*scale), int((pos.X+size.Width)*scale), int((pos.Y+size.Height)*scale))
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			out.Set(x, y, img.At(r.Min.X+x, r.Min.Y+y))
		}
	}
	return out
}

// contrast is the largest luminance difference in img from its corner pixel,
// the background behind the text.
func contrast(img *image.NRGBA) float64 {
	lum := func(c color.Color) float64 { return float64(color.GrayModel.Convert(c).(color.Gray).Y) / 255 }
	bg := lum(img.At(0, 0))
	best := 0.0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if d := lum(img.At(x, y)) - bg; d > best {
				best = d
			} else if -d > best {
				best = -d
			}
		}
	}
	return best
}

func newDimApp(t *testing.T, v bellui.ThemeVariant) {
	a := test.NewTempApp(t)
	test.ApplyTheme(t, theme.DefaultTheme())
	a.Settings().(interface{ SetThemeVariant(bellui.ThemeVariant) }).SetThemeVariant(v)
}

// Inside a disabled AppTabs and InnerWindow, a link and a label are drawn
// dimmed to about MD3's 38%: their contrast with the background is about 0.38
// of the enabled one. They stayed at full colour while taking no input
// (#1393).
func TestDisabledContainer_DimsContent(t *testing.T) {
	for _, v := range []bellui.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		for _, host := range []string{"AppTabs", "InnerWindow"} {
			t.Run(host, func(t *testing.T) {
				newDimApp(t, v)
				u, _ := url.Parse("https://example.com")
				link := widget.NewHyperlink("A link with some text", u)
				label := widget.NewLabel("A label with some text")
				content := container.NewVBox(link, label)
				var c core.Disableable
				var obj core.CanvasObject
				if host == "AppTabs" {
					tabs := container.NewAppTabs(container.NewTabItem("Tab", content))
					c, obj = tabs, tabs
				} else {
					inner := container.NewInnerWindow("Window", content)
					c, obj = inner, inner
				}
				w := test.NewTempWindow(t, obj)
				w.Resize(geo.NewSize(360, 260))
				enabled := []float64{contrast(region(t, w, link)), contrast(region(t, w, label))}
				c.Disable()
				disabled := []float64{contrast(region(t, w, link)), contrast(region(t, w, label))}
				for i, name := range []string{"link", "label"} {
					require.Greater(t, enabled[i], 0.2, "%s: drawn", name)
					assert.InDelta(t, 0.38, disabled[i]/enabled[i], 0.08, "%s: dimmed once (contrast %.3f of %.3f)", name, disabled[i], enabled[i])
				}
			})
		}
	}
}

// A widget that draws its own disabled look, the parts it builds included (a
// Button's label, an Entry's text, a segment, a date cell, a tree-view row and
// its node), looks the same inside a disabled container as disabled on its
// own, compared by contrast: the rule that dims inert content does not dim it
// a second time (#1393).
func TestDisabledContainer_OwnLooksNotDimmedTwice(t *testing.T) {
	newDimApp(t, theme.VariantLight)
	makers := map[string]func() core.CanvasObject{
		"Button": func() core.CanvasObject { return widget.NewButton("A button", nil) },
		"Entry": func() core.CanvasObject {
			e := widget.NewEntry()
			e.SetText("entry text")
			return e
		},
		"SegmentedButton": func() core.CanvasObject { return widget.NewSegmentedButton([]string{"Day", "Week"}, nil) },
		"TreeView": func() core.CanvasObject {
			tv := widget.NewTreeView()
			_ = tv.AddNode("", widget.TreeViewNode{ID: "n", Text: "Tree view node"})
			return container.NewGridWrap(geo.NewSize(260, 60), tv)
		},
		"DatePicker": func() core.CanvasObject {
			return widget.NewDatePicker(time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC), nil)
		},
	}
	disableAll := func(o core.CanvasObject) {
		if d, ok := o.(core.Disableable); ok {
			d.Disable()
		}
		if g, ok := o.(*bellui.Container); ok {
			for _, c := range g.Objects {
				if d, ok := c.(core.Disableable); ok {
					d.Disable()
				}
			}
		}
	}
	// measured is what to measure in o: the whole widget, or for a date
	// picker one day number, as its title, drawn by the picker itself,
	// would otherwise decide its contrast.
	measured := func(name string, o core.CanvasObject) core.CanvasObject {
		if name != "DatePicker" {
			return o
		}
		var day core.CanvasObject
		driver.WalkVisibleObjectTree(o, func(c core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
			if tx, ok := c.(*canvas.Text); ok && tx.Text == "15" {
				day = c
				return true
			}
			return false
		}, nil)
		require.NotNil(t, day, "the date picker's 15th")
		return day
	}
	for name, make := range makers {
		t.Run(name, func(t *testing.T) {
			// Each window is measured before the next is made: the test
			// driver places an object by the most recent window.
			alone := make()
			wa := test.NewTempWindow(t, container.NewVBox(alone))
			wa.Resize(geo.NewSize(360, 520))
			ce := contrast(region(t, wa, measured(name, alone)))
			disableAll(alone)
			ca := contrast(region(t, wa, measured(name, alone)))

			inside := make()
			box := container.NewVBox(inside)
			tabs := container.NewAppTabs(container.NewTabItem("Tab", box))
			wi := test.NewTempWindow(t, tabs)
			wi.Resize(geo.NewSize(360, 580))
			tabs.Disable()
			ci := contrast(region(t, wi, measured(name, inside)))
			t.Logf("contrast enabled %.3f, disabled alone %.3f, inside a disabled tab %.3f", ce, ca, ci)
			require.Greater(t, ce, 0.2, "drawn")
			// A widget's own disabled look keeps about 0.35-0.45 of its
			// enabled contrast; dimmed a second time it would keep about 0.15.
			// Disabled alone it is under a disabled widget too (itself), so
			// the enabled render is the reference that tells.
			assert.Greater(t, ca/ce, 0.25, "disabled alone, dimmed once")
			assert.Greater(t, ci/ce, 0.25, "inside a disabled tab, dimmed once")
			// The two renders differ only in sub-pixel placement.
			assert.InDelta(t, 1, ci/ca, 0.1, "as bright inside the disabled container (%.3f) as disabled alone (%.3f)", ci, ca)
		})
	}
}
