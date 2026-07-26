package test

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

func Test_knownColor(t *testing.T) {
	for _, name := range knownColorNames {
		assert.NotEmpty(t, knownColor(theme.Color(name)), "color name %s is not known", name)
	}
}

func Test_snapshot(t *testing.T) {
	// content elements
	for name, tt := range map[string]struct {
		content  core.CanvasObject
		size     geo.Size
		pos      geo.Position
		overlays []core.CanvasObject
		want     string
	}{
		"circle": {
			content: belluicanvas.NewCircle(color.NRGBA{R: 200, G: 100, B: 0, A: 50}),
			size:    geo.NewSize(42, 42),
			pos:     geo.NewPos(17, 17),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<circle fillColor=\"rgba(200,100,0,50)\" pos=\"17,17\" size=\"42x42\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"circle with theme color": { // we won’t test _all_ valid values … it’s not that important
			content: belluicanvas.NewCircle(theme.Color(theme.ColorNameScrollBar)),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<circle fillColor=\"scrollbar\" size=\"100x100\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"circle with stroke": {
			content: func() core.CanvasObject {
				c := belluicanvas.NewCircle(color.NRGBA{R: 200, G: 100, B: 0, A: 50})
				c.StrokeWidth = 3
				c.StrokeColor = theme.Color(theme.ColorNameBackground)
				return c
			}(),
			size: geo.NewSize(42, 42),
			pos:  geo.NewPos(17, 17),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<circle fillColor=\"rgba(200,100,0,50)\" pos=\"17,17\" size=\"42x42\" strokeColor=\"background\" strokeWidth=\"3\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"image from file": {
			content: belluicanvas.NewImageFromFile("testdata/markup_renderer_image.svg"),
			size:    geo.NewSize(66, 33),
			pos:     geo.NewPos(10, 20),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<image file=\"testdata/markup_renderer_image.svg\" pos=\"10,20\" size=\"66x33\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"image from image": {
			content: belluicanvas.NewImageFromImage(image.NewUniform(color.Transparent)),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<image img size=\"100x100\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"image from resource": {
			content: belluicanvas.NewImageFromResource(bellui.NewStaticResource("resource name", []byte{})),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<image rsc=\"resource name\" size=\"100x100\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"image theme resource": { // we won’t test _all_ valid values … it’s not that important
			content: belluicanvas.NewImageFromResource(theme.VolumeDownIcon()),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<image rsc=\"volumeDownIcon\" size=\"100x100\" themed=\"foreground\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"image themed resource": {
			content: belluicanvas.NewImageFromResource(theme.NewThemedResource(bellui.NewStaticResource("resource name", []byte{}))),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<image rsc=\"resource name\" size=\"100x100\" themed=\"foreground\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"image disabled resource": {
			content: belluicanvas.NewImageFromResource(theme.NewDisabledResource(theme.CancelIcon())),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<image rsc=\"cancelIcon\" size=\"100x100\" themed=\"disabled\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"image error themed resource": {
			content: belluicanvas.NewImageFromResource(theme.NewErrorThemedResource(theme.ErrorIcon())),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<image rsc=\"errorIcon\" size=\"100x100\" themed=\"error\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"image inverted themed resource": {
			content: belluicanvas.NewImageFromResource(theme.NewInvertedThemedResource(theme.WarningIcon())),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<image rsc=\"warningIcon\" size=\"100x100\" themed=\"inverted\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"image primary themed resource": {
			content: belluicanvas.NewImageFromResource(theme.NewPrimaryThemedResource(theme.VolumeDownIcon())),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<image rsc=\"volumeDownIcon\" size=\"100x100\" themed=\"primary\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"image with translucency": {
			content: func() core.CanvasObject {
				img := belluicanvas.NewImageFromResource(theme.ZoomOutIcon())
				img.Translucency = 0.3
				return img
			}(),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<image rsc=\"zoomOutIcon\" size=\"100x100\" themed=\"foreground\" translucency=\"0.3\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"image with fill mode contain": {
			content: func() core.CanvasObject {
				img := belluicanvas.NewImageFromResource(theme.ZoomOutIcon())
				img.FillMode = belluicanvas.ImageFillContain
				return img
			}(),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<image fillMode=\"contain\" rsc=\"zoomOutIcon\" size=\"100x100\" themed=\"foreground\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"image with fill mode original": {
			content: func() core.CanvasObject {
				img := belluicanvas.NewImageFromResource(theme.ZoomInIcon())
				img.FillMode = belluicanvas.ImageFillOriginal
				return img
			}(),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<image fillMode=\"original\" rsc=\"zoomInIcon\" size=\"100x100\" themed=\"foreground\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"image with scale mode pixels": {
			content: func() core.CanvasObject {
				img := belluicanvas.NewImageFromResource(theme.ViewRestoreIcon())
				img.ScaleMode = belluicanvas.ImageScalePixels
				return img
			}(),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<image rsc=\"viewRestoreIcon\" scaleMode=\"pixels\" size=\"100x100\" themed=\"foreground\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"image with inline icon size": {
			content: belluicanvas.NewImageFromResource(theme.VisibilityIcon()),
			size:    geo.NewSize(theme.IconInlineSize(), theme.IconInlineSize()),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<image rsc=\"visibilityIcon\" size=\"iconInlineSize\" themed=\"foreground\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"line": {
			content: belluicanvas.NewLine(color.NRGBA{R: 17, G: 42, B: 128, A: 255}),
			pos:     geo.NewPos(13, 13),
			size:    geo.NewSize(10, 50),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<line pos=\"13,13\" size=\"10x50\" strokeColor=\"rgba(17,42,128,255)\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"line with theme color": { // we won’t test _all_ valid values … it’s not that important
			content: belluicanvas.NewLine(theme.Color(theme.ColorNameShadow)),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<line size=\"100x100\" strokeColor=\"shadow\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"line with stroke width": {
			content: func() core.CanvasObject {
				l := belluicanvas.NewLine(color.NRGBA{R: 17, G: 42, B: 128, A: 255})
				l.StrokeWidth = 1.125
				return l
			}(),
			pos:  geo.NewPos(13, 13),
			size: geo.NewSize(10, 50),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<line pos=\"13,13\" size=\"10x50\" strokeColor=\"rgba(17,42,128,255)\" strokeWidth=\"1.125\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"linear gradient": {
			content: belluicanvas.NewLinearGradient(color.NRGBA{R: 1, G: 2, B: 3, A: 4}, theme.Color(theme.ColorNameDisabled), 13.25),
			pos:     geo.NewPos(6, 13),
			size:    geo.NewSize(50, 10),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<linearGradient angle=\"13.25\" endColor=\"disabled\" pos=\"6,13\" size=\"50x10\" startColor=\"rgba(1,2,3,4)\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"radial gradient": {
			content: belluicanvas.NewRadialGradient(color.NRGBA{R: 1, G: 2, B: 3, A: 4}, theme.Color(theme.ColorNameDisabled)),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<radialGradient endColor=\"disabled\" size=\"100x100\" startColor=\"rgba(1,2,3,4)\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"radial gradient with offset": {
			content: func() core.CanvasObject {
				g := belluicanvas.NewRadialGradient(color.NRGBA{R: 1, G: 2, B: 3, A: 4}, theme.Color(theme.ColorNameDisabled))
				g.CenterOffsetX = 1.5
				g.CenterOffsetY = -13.7
				return g
			}(),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<radialGradient centerOffset=\"1.5,-13.7\" endColor=\"disabled\" size=\"100x100\" startColor=\"rgba(1,2,3,4)\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"raster": {
			content: &belluicanvas.Raster{Translucency: 1},
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<raster size=\"100x100\" translucency=\"1\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"rectangle": {
			content: belluicanvas.NewRectangle(color.NRGBA{R: 100, G: 200, B: 50, A: 0}),
			size:    geo.NewSize(17, 42),
			pos:     geo.NewPos(42, 17),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<rectangle fillColor=\"rgba(100,200,50,0)\" pos=\"42,17\" size=\"17x42\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"rectangle with theme color": { // we won’t test _all_ valid values … it’s not that important
			content: belluicanvas.NewRectangle(theme.Color(theme.ColorNameHover)),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<rectangle fillColor=\"hover\" size=\"100x100\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"rectangle with stroke": {
			content: func() core.CanvasObject {
				r := belluicanvas.NewRectangle(color.NRGBA{R: 200, G: 100, B: 0, A: 50})
				r.StrokeWidth = 6.375
				r.StrokeColor = theme.Color(theme.ColorNamePlaceHolder)
				return r
			}(),
			size: geo.NewSize(42, 42),
			pos:  geo.NewPos(17, 17),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<rectangle fillColor=\"rgba(200,100,0,50)\" pos=\"17,17\" size=\"42x42\" strokeColor=\"placeholder\" strokeWidth=\"6.375\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"text": {
			content: belluicanvas.NewText("foo", color.NRGBA{R: 123, G: 234, B: 56, A: 0}),
			size:    geo.NewSize(50, 50),
			pos:     geo.NewPos(20, 20),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<text color=\"rgba(123,234,56,0)\" pos=\"20,20\" size=\"50x50\">foo</text>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"text with theme color": {
			content: belluicanvas.NewText("bar", theme.Color(theme.ColorNameForeground)),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<text size=\"100x100\">bar</text>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"text with alignment center": {
			content: func() core.CanvasObject {
				t := belluicanvas.NewText("bar", theme.Color(theme.ColorNameForeground))
				t.Alignment = bellui.TextAlignCenter
				return t
			}(),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<text alignment=\"center\" size=\"100x100\">bar</text>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"text with alignment trailing": {
			content: func() core.CanvasObject {
				txt := belluicanvas.NewText("bar", theme.Color(theme.ColorNameForeground))
				txt.Alignment = bellui.TextAlignTrailing
				return txt
			}(),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<text alignment=\"trailing\" size=\"100x100\">bar</text>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"text with size": {
			content: func() core.CanvasObject {
				txt := belluicanvas.NewText("big", theme.Color(theme.ColorNameForeground))
				txt.TextSize = 42
				return txt
			}(),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<text size=\"100x100\" textSize=\"42\">big</text>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"text bold": {
			content: func() core.CanvasObject {
				txt := belluicanvas.NewText("bold", theme.Color(theme.ColorNameForeground))
				txt.TextStyle.Bold = true
				return txt
			}(),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<text bold size=\"100x100\">bold</text>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"text italic": {
			content: func() core.CanvasObject {
				txt := belluicanvas.NewText("italic", theme.Color(theme.ColorNameForeground))
				txt.TextStyle.Italic = true
				return txt
			}(),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<text italic size=\"100x100\">italic</text>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"text monospace": {
			content: func() core.CanvasObject {
				txt := belluicanvas.NewText("mono", theme.Color(theme.ColorNameForeground))
				txt.TextStyle.Monospace = true
				return txt
			}(),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<text monospace size=\"100x100\">mono</text>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"container": {
			content: container.NewVBox(belluicanvas.NewCircle(color.Black), belluicanvas.NewLine(color.NRGBA{R: 250, G: 250, B: 250, A: 250})),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<container size=\"100x100\">\n" +
				"\t\t\t<circle fillColor=\"rgba(0,0,0,255)\" size=\"100x1\"/>\n" +
				"\t\t\t<line pos=\"0,5\" size=\"100x1\" strokeColor=\"rgba(250,250,250,250)\"/>\n" +
				"\t\t</container>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"widget": {
			content: &markupRendererTestWidget{},
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<widget size=\"100x100\" type=\"*test.markupRendererTestWidget\">\n" +
				"\t\t</widget>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"widget with subobjects": {
			content: &markupRendererTestWidget{
				objs: []core.CanvasObject{
					belluicanvas.NewCircle(color.Black),
					belluicanvas.NewLine(color.NRGBA{R: 250, G: 250, B: 250, A: 250}),
				},
			},
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<widget size=\"100x100\" type=\"*test.markupRendererTestWidget\">\n" +
				"\t\t\t<circle fillColor=\"rgba(0,0,0,255)\" size=\"0x0\"/>\n" +
				"\t\t\t<line size=\"0x0\" strokeColor=\"rgba(250,250,250,250)\"/>\n" +
				"\t\t</widget>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"spacer": {
			content: layout.NewSpacer(),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<spacer size=\"100x100\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"invisible": {
			content: func() core.CanvasObject {
				c := belluicanvas.NewCircle(color.Black)
				c.Hide()
				l := belluicanvas.NewLine(color.NRGBA{R: 250, G: 250, B: 250, A: 250})
				l.Hide()
				w := widget.NewButton("tap me if you can", nil)
				w.Hide()
				return container.NewVBox(c, l, w)
			}(),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<container size=\"100x100\">\n" +
				"\t\t</container>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"polygon": {
			content: belluicanvas.NewPolygon(3, color.NRGBA{R: 100, G: 200, B: 50, A: 0}),
			size:    geo.NewSize(17, 42),
			pos:     geo.NewPos(42, 17),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<polygon fillColor=\"rgba(100,200,50,0)\" pos=\"42,17\" sides=\"3\" size=\"17x42\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"polygon with theme color": { // we won’t test _all_ valid values … it’s not that important
			content: belluicanvas.NewPolygon(5, theme.Color(theme.ColorNameHover)),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<polygon fillColor=\"hover\" sides=\"5\" size=\"100x100\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"polygon with stroke": {
			content: func() core.CanvasObject {
				r := belluicanvas.NewPolygon(4, color.NRGBA{R: 200, G: 100, B: 0, A: 50})
				r.StrokeWidth = 6.375
				r.StrokeColor = theme.Color(theme.ColorNamePlaceHolder)
				r.Angle = 18.25
				r.CornerRadius = 12.5
				return r
			}(),
			size: geo.NewSize(42, 42),
			pos:  geo.NewPos(17, 17),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<polygon angle=\"18.25\" fillColor=\"rgba(200,100,0,50)\" pos=\"17,17\" radius=\"12.5\" sides=\"4\" size=\"42x42\" strokeColor=\"placeholder\" strokeWidth=\"6.375\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"arc": {
			content: belluicanvas.NewArc(25, 98, 0, color.NRGBA{R: 100, G: 200, B: 50, A: 0}),
			size:    geo.NewSize(17, 42),
			pos:     geo.NewPos(42, 17),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<arc endAngle=\"98\" fillColor=\"rgba(100,200,50,0)\" pos=\"42,17\" size=\"17x42\" startAngle=\"25\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"arc with theme color": { // we won’t test _all_ valid values … it’s not that important
			content: belluicanvas.NewArc(0, 150, 0, theme.Color(theme.ColorNameHover)),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<arc endAngle=\"150\" fillColor=\"hover\" size=\"100x100\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
		"arc with stroke": {
			content: func() core.CanvasObject {
				r := belluicanvas.NewArc(-80, -110, 0, color.NRGBA{R: 200, G: 100, B: 0, A: 50})
				r.StrokeWidth = 6.375
				r.StrokeColor = theme.Color(theme.ColorNamePlaceHolder)
				r.CornerRadius = 15
				r.CutoutRatio = 0.375
				return r
			}(),
			size: geo.NewSize(42, 42),
			pos:  geo.NewPos(17, 17),
			want: "<canvas size=\"100x100\">\n" +
				"\t<content>\n" +
				"\t\t<arc cutoutRatio=\"0.375\" endAngle=\"-110\" fillColor=\"rgba(200,100,0,50)\" pos=\"17,17\" radius=\"15\" size=\"42x42\" startAngle=\"-80\" strokeColor=\"placeholder\" strokeWidth=\"6.375\"/>\n" +
				"\t</content>\n" +
				"</canvas>\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := NewCanvas()
			c.SetPadded(false)
			c.SetContent(tt.content)
			c.Resize(geo.NewSize(100, 100))
			if !tt.pos.IsZero() {
				tt.content.Move(tt.pos)
			}
			if !tt.size.IsZero() {
				tt.content.Resize(tt.size)
			}
			assert.Equal(t, tt.want, snapshot(c))
		})
	}

	t.Run("canvas with padding", func(t *testing.T) {
		c := NewCanvas()
		c.SetPadded(true)
		c.SetContent(belluicanvas.NewCircle(color.Black))
		c.Resize(geo.NewSize(100, 100))
		assert.Equal(
			t,
			"<canvas padded size=\"100x100\">\n"+
				"\t<content>\n"+
				"\t\t<circle fillColor=\"rgba(0,0,0,255)\" pos=\"4,4\" size=\"92x92\"/>\n"+
				"\t</content>\n"+
				"</canvas>\n",
			snapshot(c),
		)
	})

	t.Run("canvas with overlays", func(t *testing.T) {
		c := NewCanvas()
		c.SetPadded(true)
		c.SetContent(belluicanvas.NewCircle(color.Black))
		c.Overlays().Add(belluicanvas.NewRectangle(color.NRGBA{R: 250, G: 250, B: 250, A: 250}))
		c.Overlays().Add(belluicanvas.NewRectangle(color.Transparent))
		c.Resize(geo.NewSize(100, 100))
		assert.Equal(
			t,
			"<canvas padded size=\"100x100\">\n"+
				"\t<content>\n"+
				"\t\t<circle fillColor=\"rgba(0,0,0,255)\" pos=\"4,4\" size=\"92x92\"/>\n"+
				"\t</content>\n"+
				"\t<overlay>\n"+
				"\t\t<rectangle fillColor=\"rgba(250,250,250,250)\" size=\"100x100\"/>\n"+
				"\t</overlay>\n"+
				"\t<overlay>\n"+
				"\t\t<rectangle size=\"100x100\"/>\n"+
				"\t</overlay>\n"+
				"</canvas>\n",
			snapshot(c),
		)
	})
}

var benchmarkStringSink string

func Benchmark_knownColor(b *testing.B) {
	lookup := nrgbaColor(theme.Color(theme.ColorNameWarning))
	out := ""

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out = knownColor(lookup)
	}

	benchmarkStringSink = out
}

func Benchmark_knownResource(b *testing.B) {
	lookup := theme.ZoomOutIcon()
	out := ""

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out = knownResource(lookup)
	}

	benchmarkStringSink = out
}

type markupRendererTestWidget struct {
	hidden bool
	objs   []core.CanvasObject
	pos    geo.Position
	size   geo.Size
}

var _ core.Widget = (*markupRendererTestWidget)(nil)

func (w *markupRendererTestWidget) CreateRenderer() core.WidgetRenderer {
	return &markupRendererTestWidgetRenderer{w: w}
}

func (w *markupRendererTestWidget) Hide() {
	w.hidden = true
}

func (w *markupRendererTestWidget) MinSize() geo.Size {
	return geo.Size{}
}

func (w *markupRendererTestWidget) Move(position geo.Position) {
	w.pos = position
}

func (w *markupRendererTestWidget) Position() geo.Position {
	return w.pos
}

func (w *markupRendererTestWidget) Refresh() {
}

func (w *markupRendererTestWidget) Resize(size geo.Size) {
	w.size = size
}

func (w *markupRendererTestWidget) SetObjects(objects ...core.CanvasObject) {
	w.objs = objects
}

func (w *markupRendererTestWidget) Show() {
	w.hidden = false
}

func (w *markupRendererTestWidget) Size() geo.Size {
	return w.size
}

func (w *markupRendererTestWidget) Visible() bool {
	return !w.hidden
}

type markupRendererTestWidgetRenderer struct {
	w *markupRendererTestWidget
}

func (r *markupRendererTestWidgetRenderer) Destroy() {
}

func (r *markupRendererTestWidgetRenderer) Layout(_ geo.Size) {
}

func (r *markupRendererTestWidgetRenderer) MinSize() geo.Size {
	return geo.Size{}
}

func (r *markupRendererTestWidgetRenderer) Objects() []core.CanvasObject {
	return r.w.objs
}

func (r *markupRendererTestWidgetRenderer) Refresh() {
}

var _ core.WidgetRenderer = (*markupRendererTestWidgetRenderer)(nil)
