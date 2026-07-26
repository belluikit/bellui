package widget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

var _ core.Widget = (*Shadow)(nil)

// Shadow is a widget that renders a shadow.
type Shadow struct {
	Base
	level ElevationLevel
	typ   ShadowType
}

// ElevationLevel is the level of elevation of the shadow casting object.
type ElevationLevel int

// ElevationLevel constants inspired by:
// https://storage.googleapis.com/spec-host/mio-staging%2Fmio-design%2F1584058305895%2Fassets%2F0B6xUSjjSulxceF9udnA4Sk5tdU0%2Fbaselineelevation-chart.png
const (
	BaseLevel             ElevationLevel = 0
	CardLevel             ElevationLevel = 1
	ButtonLevel           ElevationLevel = 2
	MenuLevel             ElevationLevel = 4
	PopUpLevel            ElevationLevel = 8
	SubmergedContentLevel ElevationLevel = 8
	DialogLevel           ElevationLevel = 24
)

// ShadowType specifies the type of the shadow.
type ShadowType int

// ShadowType constants
const (
	ShadowAround ShadowType = iota
	ShadowLeft
	ShadowRight
	ShadowBottom
	ShadowTop
)

// NewShadow create a new Shadow.
func NewShadow(typ ShadowType, level ElevationLevel) *Shadow {
	s := &Shadow{typ: typ, level: level}
	s.ExtendBaseWidget(s)
	return s
}

// CreateRenderer returns a new renderer for the shadow.
func (s *Shadow) CreateRenderer() core.WidgetRenderer {
	r := &shadowRenderer{s: s}
	r.createShadows()
	return r
}

type shadowRenderer struct {
	BaseRenderer
	// BoxShadow for ShadowAround (GPU blur path).
	boxShadow *canvas.BoxShadow
	// Gradient fallbacks for directional shadows.
	b, l, r, t     *canvas.LinearGradient
	bl, br, tl, tr *canvas.RadialGradient
	minSize        geo.Size
	s              *Shadow
}

func (r *shadowRenderer) Layout(size geo.Size) {
	if r.boxShadow != nil {
		r.boxShadow.Resize(size)
		r.boxShadow.Move(geo.NewPos(0, 0))
		return
	}

	// Gradient fallback layout (directional shadows)
	depth := float32(r.s.level)
	sideOff, topOff := float32(0.0), float32(0.0)
	if r.s.typ == ShadowAround {
		sideOff = depth * 0.2
		topOff = sideOff * 2
	}

	if r.tl != nil {
		r.tl.Resize(geo.NewSize(depth, depth))
		r.tl.Move(geo.NewPos(-depth+sideOff, -depth+topOff))
	}
	if r.t != nil {
		r.t.Resize(geo.NewSize(size.Width-sideOff*2, depth))
		r.t.Move(geo.NewPos(sideOff, -depth+topOff))
	}
	if r.tr != nil {
		r.tr.Resize(geo.NewSize(depth, depth))
		r.tr.Move(geo.NewPos(size.Width-sideOff, -depth+topOff))
	}
	if r.r != nil {
		r.r.Resize(geo.NewSize(depth, size.Height-topOff))
		r.r.Move(geo.NewPos(size.Width-sideOff, topOff))
	}
	if r.br != nil {
		r.br.Resize(geo.NewSize(depth, depth))
		r.br.Move(geo.NewPos(size.Width-sideOff, size.Height))
	}
	if r.b != nil {
		r.b.Resize(geo.NewSize(size.Width-sideOff*2, depth))
		r.b.Move(geo.NewPos(sideOff, size.Height))
	}
	if r.bl != nil {
		r.bl.Resize(geo.NewSize(depth, depth))
		r.bl.Move(geo.NewPos(-depth+sideOff, size.Height))
	}
	if r.l != nil {
		r.l.Resize(geo.NewSize(depth, size.Height-topOff))
		r.l.Move(geo.NewPos(-depth+sideOff, topOff))
	}
}

func (r *shadowRenderer) MinSize() geo.Size {
	return r.minSize
}

func (r *shadowRenderer) Refresh() {
	r.refreshShadows()
	r.Layout(r.s.Size())
	canvas.Refresh(r.s)
}

func (r *shadowRenderer) createShadows() {
	th := theme.CurrentForWidget(r.s)
	v := bellui.CurrentApp().Settings().ThemeVariant()
	fg := th.Color(theme.ColorNameShadow, v)

	switch r.s.typ {
	case ShadowAround:
		// Use BoxShadow for ShadowAround — produces GPU-blurred soft shadows.
		blurRadius := float32(r.s.level) * 1.5
		bs := canvas.NewBoxShadow(fg, blurRadius)
		bs.CornerRadius = th.Size(theme.SizeNameInputRadius)
		bs.OffsetY = float32(r.s.level) * 0.5
		r.boxShadow = bs
		r.SetObjects([]core.CanvasObject{bs})
	case ShadowLeft:
		r.l = canvas.NewHorizontalGradient(color.Transparent, fg)
		r.SetObjects([]core.CanvasObject{r.l})
	case ShadowRight:
		r.r = canvas.NewHorizontalGradient(fg, color.Transparent)
		r.SetObjects([]core.CanvasObject{r.r})
	case ShadowBottom:
		r.b = canvas.NewVerticalGradient(fg, color.Transparent)
		r.SetObjects([]core.CanvasObject{r.b})
	case ShadowTop:
		r.t = canvas.NewVerticalGradient(color.Transparent, fg)
		r.SetObjects([]core.CanvasObject{r.t})
	}
}

func (r *shadowRenderer) refreshShadows() {
	th := theme.CurrentForWidget(r.s)
	v := bellui.CurrentApp().Settings().ThemeVariant()
	fg := th.Color(theme.ColorNameShadow, v)

	if r.boxShadow != nil {
		r.boxShadow.Color = fg
		r.boxShadow.BlurRadius = float32(r.s.level) * 1.5
		r.boxShadow.CornerRadius = th.Size(theme.SizeNameInputRadius)
		r.boxShadow.OffsetY = float32(r.s.level) * 0.5
		r.boxShadow.Refresh()
		return
	}

	updateShadowEnd(r.l, fg)
	updateShadowStart(r.r, fg)
	updateShadowStart(r.b, fg)
	updateShadowEnd(r.t, fg)

	updateShadowRadial(r.tl, fg)
	updateShadowRadial(r.tr, fg)
	updateShadowRadial(r.bl, fg)
	updateShadowRadial(r.br, fg)
}

func updateShadowEnd(g *canvas.LinearGradient, fg color.Color) {
	if g == nil {
		return
	}
	g.EndColor = fg
	g.Refresh()
}

func updateShadowRadial(g *canvas.RadialGradient, fg color.Color) {
	if g == nil {
		return
	}
	g.StartColor = fg
	g.Refresh()
}

func updateShadowStart(g *canvas.LinearGradient, fg color.Color) {
	if g == nil {
		return
	}
	g.StartColor = fg
	g.Refresh()
}
