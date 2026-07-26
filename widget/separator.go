package widget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

var _ core.Widget = (*Separator)(nil)

// Separator is a widget for displaying a separator with themeable color.
//
// Since: 1.4
type Separator struct {
	BaseWidget

	invert bool
}

// NewSeparator creates a new separator.
//
// Since: 1.4
func NewSeparator() *Separator {
	s := &Separator{}
	s.ExtendBaseWidget(s)
	return s
}

// CreateRenderer returns a new renderer for the separator.
func (s *Separator) CreateRenderer() core.WidgetRenderer {
	s.ExtendBaseWidget(s)
	th := s.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	var col color.Color
	if s.invert {
		col = th.Color(theme.ColorNameOnSurface, v)
	} else {
		col = th.Color(theme.ColorNameOutlineVariant, v)
	}
	bar := canvas.NewRectangle(col)

	return &separatorRenderer{
		WidgetRenderer: NewSimpleRenderer(bar),
		bar:            bar,
		d:              s,
	}
}

// MinSize returns the minimal size of the separator.
func (s *Separator) MinSize() geo.Size {
	s.ExtendBaseWidget(s)
	return s.BaseWidget.MinSize()
}

var _ core.WidgetRenderer = (*separatorRenderer)(nil)

type separatorRenderer struct {
	core.WidgetRenderer
	bar *canvas.Rectangle
	d   *Separator
}

func (r *separatorRenderer) MinSize() geo.Size {
	return geo.NewSquareSize(r.d.Theme().Size(theme.SizeNameSeparatorThickness))
}

func (r *separatorRenderer) Refresh() {
	th := r.d.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	if r.d.invert {
		r.bar.FillColor = th.Color(theme.ColorNameOnSurface, v)
	} else {
		r.bar.FillColor = th.Color(theme.ColorNameOutlineVariant, v)
	}
	canvas.Refresh(r.d)
}
