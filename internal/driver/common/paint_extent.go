package common

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter"
)

// PaintExtent returns the box obj's painters draw in, relative to its
// position: its bounds, grown to what the painters draw past them. A text is
// drawn at its MinSize whatever its own size, so one positioned but never
// resized still draws its glyphs (#602), on the side its alignment puts them;
// a stroke, an italic lean or a path reaching outside its size overflows by
// the painters' VectorPad; a box shadow's blur spreads around its offset box.
//
// Partial repaint decides from this box whether an object needs repainting,
// and records it as where the object was painted, so pixels an object draws
// outside it would be cleared and never redrawn.
func PaintExtent(obj core.CanvasObject) (geo.Position, geo.Size) {
	s := obj.Size()
	var off geo.Position
	switch o := obj.(type) {
	case *canvas.BoxShadow:
		if o.BlurRadius > 0 {
			spread := o.BlurRadius * 2 // as drawBoxShadow pads it
			return geo.NewPos(o.OffsetX-spread, o.OffsetY-spread), geo.NewSize(s.Width+2*spread, s.Height+2*spread)
		}
		return off, s
	case *canvas.Text:
		off, s = textExtent(s, o.MinSize(), o.Alignment)
	case *canvas.StyledText:
		off, s = textExtent(s, o.MinSize(), o.Alignment)
	case *canvas.TextOnPath:
		if s.IsZero() {
			s = o.MinSize()
		}
	case *canvas.Path:
		off, s = painter.PathRasterBounds(o)
	}
	if pad := painter.VectorPad(obj); pad > 0 {
		off = geo.NewPos(off.X-pad, off.Y-pad)
		s = geo.NewSize(s.Width+2*pad, s.Height+2*pad)
	}
	return off, s
}

// textExtent returns where the painters draw a text of the given MinSize in
// a box of size s. A text wider than its box is placed by its alignment, so a
// centred one overflows both sides and a trailing one the leading side. A
// leading or trailing alignment flips under a right-to-left DirectionScope,
// which the dirty-region walk does not track, so for those the extent covers
// the overflow on both sides.
func textExtent(s, min geo.Size, align bellui.TextAlign) (geo.Position, geo.Size) {
	h := max(s.Height, min.Height)
	over := min.Width - s.Width
	if over <= 0 {
		return geo.Position{}, geo.NewSize(s.Width, h)
	}
	if align == bellui.TextAlignCenter {
		return geo.NewPos(-over/2, 0), geo.NewSize(min.Width, h)
	}
	return geo.NewPos(-over, 0), geo.NewSize(s.Width+2*over, h)
}
