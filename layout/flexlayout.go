package layout

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// CrossAlign says where a flex box places a child on its cross axis: the
// vertical one of a FlexHBoxLayout, the horizontal one of a FlexVBoxLayout.
//
// Since: 1.3
type CrossAlign int

const (
	// CrossStretch, the default, stretches each child to the container's
	// cross extent, clamped by the per-child cross Min/Max arrays; a clamped
	// child sits at the start of the freed space.
	CrossStretch CrossAlign = iota
	// CrossStart gives each child its MinSize on the cross axis (within the
	// per-child cross Min/Max) and puts it at the start: the top of a row, the
	// leading edge of a column.
	CrossStart
	// CrossCenter sizes a child as CrossStart does and centres it.
	CrossCenter
	// CrossEnd sizes a child as CrossStart does and puts it at the end.
	CrossEnd
)

// place returns a child's cross-axis offset and size within extent. natural
// is its MinSize on that axis; minC and maxC are its cross Min/Max entries,
// 0 meaning unset.
func (a CrossAlign) place(extent, natural, minC, maxC float32) (pos, size float32) {
	size = extent
	if a != CrossStretch {
		size = natural
	}
	if minC > 0 && size < minC {
		size = minC
	}
	if maxC > 0 && size > maxC {
		size = maxC
	}
	switch a {
	case CrossCenter:
		pos = (extent - size) / 2
	case CrossEnd:
		pos = extent - size
	}
	return pos, size
}

// entry returns s[i], or 0 when s is too short.
func entry(s []float32, i int) float32 {
	if i < len(s) {
		return s[i]
	}
	return 0
}

// FlexHBoxLayout is a horizontal layout where children can have flex grow
// factors and min/max width constraints. Children with flex=0 get their
// natural MinSize. Remaining space is distributed proportionally among
// children with flex>0.
//
// The main axis is horizontal: Flex/MinWidth/MaxWidth constrain width. The
// cross axis is vertical and children are stretched to the container height by
// default; the optional MinHeight/MaxHeight arrays clamp a child's height
// instead (start/top-aligned within the freed space). CrossAlign places
// children at their natural height instead, at the top, centre or bottom.
//
// Since: 1.0
type FlexHBoxLayout struct {
	Flex       []float32  // flex grow factor per child (0 = fixed size)
	MinWidth   []float32  // minimum width per child (0 = use MinSize)
	MaxWidth   []float32  // maximum width per child (0 = no limit)
	MinHeight  []float32  // cross-axis minimum height per child (0 = unset); Since: 1.0
	MaxHeight  []float32  // cross-axis maximum height per child (0 = no limit, full stretch); Since: 1.0
	Padding    float32    // spacing between children (0 = theme default, unless ExactPadding)
	CrossAlign CrossAlign // cross-axis placement (default CrossStretch); Since: 1.3

	// ExactPadding makes Padding the spacing as given, so 0 puts the children
	// flush against each other. Without it, a Padding of 0 or less means the
	// theme's padding.
	//
	// Since: 1.3
	ExactPadding bool
}

// NewFlexHBoxLayout creates a horizontal flex layout with the given per-child
// grow factors. Children with flex=0 use their natural MinSize; remaining
// space is split proportionally among children with flex>0.
func NewFlexHBoxLayout(flex []float32) *FlexHBoxLayout {
	return &FlexHBoxLayout{Flex: flex}
}

func (l *FlexHBoxLayout) padding() float32 {
	if l.ExactPadding || l.Padding > 0 {
		return l.Padding
	}
	return theme.Padding()
}

func (l *FlexHBoxLayout) MinSize(objects []core.CanvasObject) geo.Size {
	pad := l.padding()
	var w, h float32
	first := true
	for i, o := range objects {
		if !o.Visible() {
			continue
		}
		ms := o.MinSize()
		childW := ms.Width
		if i < len(l.MinWidth) && l.MinWidth[i] > childW {
			childW = l.MinWidth[i]
		}
		childH := ms.Height
		if i < len(l.MinHeight) && l.MinHeight[i] > childH {
			childH = l.MinHeight[i]
		}
		if !first {
			w += pad
		}
		w += childW
		if childH > h {
			h = childH
		}
		first = false
	}
	return geo.NewSize(w, h)
}

func (l *FlexHBoxLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	pad := l.padding()

	// Pass 1: compute fixed widths and total flex.
	var totalFixed, totalFlex float32
	visible := 0
	for i, o := range objects {
		if !o.Visible() {
			continue
		}
		if i < len(l.Flex) && l.Flex[i] > 0 {
			totalFlex += l.Flex[i]
		} else {
			w := o.MinSize().Width
			if i < len(l.MinWidth) && l.MinWidth[i] > w {
				w = l.MinWidth[i]
			}
			totalFixed += w
		}
		visible++
	}

	spacing := pad * float32(max(0, visible-1))
	available := size.Width - totalFixed - spacing
	if available < 0 {
		available = 0
	}

	// Pass 2: position and size each child.
	x := float32(0)
	first := true
	for i, o := range objects {
		if !o.Visible() {
			continue
		}
		if !first {
			x += pad
		}

		var w float32
		if i < len(l.Flex) && l.Flex[i] > 0 && totalFlex > 0 {
			w = available * (l.Flex[i] / totalFlex)
		} else {
			w = o.MinSize().Width
		}

		if i < len(l.MinWidth) && l.MinWidth[i] > 0 && w < l.MinWidth[i] {
			w = l.MinWidth[i]
		}
		if i < len(l.MaxWidth) && l.MaxWidth[i] > 0 && w > l.MaxWidth[i] {
			w = l.MaxWidth[i]
		}

		// Cross axis (height): full stretch by default, clamped to [MinHeight, MaxHeight].
		y, h := l.CrossAlign.place(size.Height, o.MinSize().Height, entry(l.MinHeight, i), entry(l.MaxHeight, i))

		o.Move(geo.NewPos(x, y))
		o.Resize(geo.NewSize(w, h))
		x += w
		first = false
	}
}

// FlexVBoxLayout is the vertical equivalent of FlexHBoxLayout.
// Children with flex>0 share remaining vertical space proportionally.
//
// The main axis is vertical: Flex/MinHeight/MaxHeight constrain height. The
// cross axis is horizontal and children are stretched to the container width by
// default; the optional MinWidth/MaxWidth arrays clamp a child's width instead
// (start/left-aligned within the freed space). CrossAlign places children at
// their natural width instead, at the left, centre or right.
//
// Since: 1.0
type FlexVBoxLayout struct {
	Flex       []float32  // flex grow factor per child
	MinHeight  []float32  // minimum height per child
	MaxHeight  []float32  // maximum height per child
	MinWidth   []float32  // cross-axis minimum width per child (0 = unset); Since: 1.0
	MaxWidth   []float32  // cross-axis maximum width per child (0 = no limit, full stretch); Since: 1.0
	Padding    float32    // spacing between children (0 = theme default, unless ExactPadding)
	CrossAlign CrossAlign // cross-axis placement (default CrossStretch); Since: 1.3

	// ExactPadding makes Padding the spacing as given, so 0 puts the children
	// flush against each other. Without it, a Padding of 0 or less means the
	// theme's padding.
	//
	// Since: 1.3
	ExactPadding bool
}

// NewFlexVBoxLayout creates a vertical flex layout with the given per-child
// grow factors.
func NewFlexVBoxLayout(flex []float32) *FlexVBoxLayout {
	return &FlexVBoxLayout{Flex: flex}
}

func (l *FlexVBoxLayout) padding() float32 {
	if l.ExactPadding || l.Padding > 0 {
		return l.Padding
	}
	return theme.Padding()
}

func (l *FlexVBoxLayout) MinSize(objects []core.CanvasObject) geo.Size {
	pad := l.padding()
	var w, h float32
	first := true
	for i, o := range objects {
		if !o.Visible() {
			continue
		}
		ms := o.MinSize()
		childH := ms.Height
		if i < len(l.MinHeight) && l.MinHeight[i] > childH {
			childH = l.MinHeight[i]
		}
		childW := ms.Width
		if i < len(l.MinWidth) && l.MinWidth[i] > childW {
			childW = l.MinWidth[i]
		}
		if !first {
			h += pad
		}
		h += childH
		if childW > w {
			w = childW
		}
		first = false
	}
	return geo.NewSize(w, h)
}

func (l *FlexVBoxLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	pad := l.padding()

	var totalFixed, totalFlex float32
	visible := 0
	for i, o := range objects {
		if !o.Visible() {
			continue
		}
		if i < len(l.Flex) && l.Flex[i] > 0 {
			totalFlex += l.Flex[i]
		} else {
			h := o.MinSize().Height
			if i < len(l.MinHeight) && l.MinHeight[i] > h {
				h = l.MinHeight[i]
			}
			totalFixed += h
		}
		visible++
	}

	spacing := pad * float32(max(0, visible-1))
	available := size.Height - totalFixed - spacing
	if available < 0 {
		available = 0
	}

	y := float32(0)
	first := true
	for i, o := range objects {
		if !o.Visible() {
			continue
		}
		if !first {
			y += pad
		}

		var h float32
		if i < len(l.Flex) && l.Flex[i] > 0 && totalFlex > 0 {
			h = available * (l.Flex[i] / totalFlex)
		} else {
			h = o.MinSize().Height
		}

		if i < len(l.MinHeight) && l.MinHeight[i] > 0 && h < l.MinHeight[i] {
			h = l.MinHeight[i]
		}
		if i < len(l.MaxHeight) && l.MaxHeight[i] > 0 && h > l.MaxHeight[i] {
			h = l.MaxHeight[i]
		}

		// Cross axis (width): full stretch by default, clamped to [MinWidth, MaxWidth].
		x, w := l.CrossAlign.place(size.Width, o.MinSize().Width, entry(l.MinWidth, i), entry(l.MaxWidth, i))

		o.Move(geo.NewPos(x, y))
		o.Resize(geo.NewSize(w, h))
		y += h
		first = false
	}
}
