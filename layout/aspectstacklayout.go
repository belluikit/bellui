package layout

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// Declare conformity with Layout interface
var _ core.Layout = (*aspectStackLayout)(nil)

type aspectStackLayout struct {
	ratio float32
}

// NewAspectStackLayout returns a layout that stacks its objects, as
// NewStackLayout does, in the largest box of the given width/height ratio
// that fits the space, centred in it (#993). A ratio of 1 is a square; a
// ratio of 0 or less is taken as 1.
//
// Since: 1.3
func NewAspectStackLayout(ratio float32) core.Layout {
	if ratio <= 0 {
		ratio = 1
	}
	return aspectStackLayout{ratio: ratio}
}

// Layout sets every object to the largest box of the ratio that fits size,
// centred in it.
func (a aspectStackLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	box := a.fit(size)
	pos := geo.NewPos((size.Width-box.Width)/2, (size.Height-box.Height)/2)
	for _, child := range objects {
		child.Resize(box)
		child.Move(pos)
	}
}

// fit is the largest size of the layout's ratio within size.
func (a aspectStackLayout) fit(size geo.Size) geo.Size {
	w, h := size.Width, size.Width/a.ratio
	if h > size.Height {
		h = size.Height
		w = h * a.ratio
	}
	return geo.NewSize(w, h)
}

// MinSize is the largest visible child's MinSize, grown on one side to the
// layout's ratio.
func (a aspectStackLayout) MinSize(objects []core.CanvasObject) geo.Size {
	min := geo.NewSize(0, 0)
	for _, child := range objects {
		if child.Visible() {
			min = min.Max(child.MinSize())
		}
	}
	if min.Width < min.Height*a.ratio {
		min.Width = min.Height * a.ratio
	} else {
		min.Height = min.Width / a.ratio
	}
	return min
}
