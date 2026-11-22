package widget

import (
	"slices"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

var (
	_ core.Widget      = (*ChipGroup)(nil)
	_ core.Disableable = (*ChipGroup)(nil)
)

// ChipGroup is a set of filter chips to pick from, one per option, as
// CheckGroup and RadioGroup are with checkboxes and radio buttons (#1348).
// Each chip is as wide as its label, and the chips wrap onto the next row
// when the row is full, 8 dp apart at the default theme's padding, as MD3
// spaces chips.
//
// With Multiple set any number of chips can be selected; without it one
// can, and tapping another chip moves the selection to it. Tapping a
// selected chip deselects it, unless Required is set and it is the last
// one selected. A disabled group's chips are disabled.
//
// Alignment places each row of chips at the start, the center or the end
// of the group's width. Start and end follow the layout direction, so under
// right-to-left the rows start at the right edge.
//
// Since: 1.3
type ChipGroup struct {
	DisableableWidget
	Options  []string
	Selected []string
	Multiple bool
	Required bool
	// Alignment places each row of chips: TextAlignLeading (the default)
	// at the start of the group's width, TextAlignCenter in its middle and
	// TextAlignTrailing at its end (#1371).
	//
	// Since: 1.3
	Alignment bellui.TextAlign
	// OnChanged fires with the selected options after a tap or SetSelected
	// changes them, in the order they were selected.
	OnChanged func([]string) `json:"-"`

	chips []*Chip
}

// NewChipGroup creates a chip group with the options, picking one at a
// time unless multiple is set.
//
// Since: 1.3
func NewChipGroup(options []string, multiple bool, changed func([]string)) *ChipGroup {
	g := &ChipGroup{Options: options, Multiple: multiple, OnChanged: changed}
	g.ExtendBaseWidget(g)
	return g
}

// SetSelected selects the options, fires OnChanged and refreshes.
//
// Since: 1.3
func (g *ChipGroup) SetSelected(options []string) {
	g.Selected = options
	if g.OnChanged != nil {
		g.OnChanged(options)
	}
	g.Refresh()
}

// MinSize is the widest chip's width and the height the rows take.
func (g *ChipGroup) MinSize() geo.Size {
	g.ExtendBaseWidget(g)
	return g.BaseWidget.MinSize()
}

// CreateRenderer links the group to its renderer.
func (g *ChipGroup) CreateRenderer() core.WidgetRenderer {
	g.ExtendBaseWidget(g)
	gap := 2 * theme.Padding()
	r := &chipGroupRenderer{group: g, layout: layout.NewRowWrapLayoutWithCustomPadding(gap, gap)}
	r.BaseRenderer = widget.NewBaseRenderer(nil)
	r.sync()
	return r
}

// chipTapped applies a tap on c to the selection.
func (g *ChipGroup) chipTapped(c *Chip) {
	if g.Disabled() {
		return
	}
	opt := c.Text
	sel := slices.Clone(g.Selected)
	if i := slices.Index(sel, opt); i >= 0 {
		if g.Required && len(sel) == 1 {
			return
		}
		sel = slices.Delete(sel, i, i+1)
	} else if g.Multiple {
		sel = append(sel, opt)
	} else {
		sel = []string{opt}
	}
	g.SetSelected(sel)
}

type chipGroupRenderer struct {
	widget.BaseRenderer
	group  *ChipGroup
	layout core.Layout
}

func (r *chipGroupRenderer) Layout(size geo.Size) {
	objects := r.Objects()
	r.layout.Layout(objects, size)
	mirrorIfRTL(objects, size.Width) // the row-wrap layout itself does not (#1372)
	alignRows(objects, size.Width, r.group.Alignment)
}

// alignRows moves each row the row-wrap layout made, the chips that share a
// y, so that it sits at the start, center or end of width. It measures each
// row's span instead of assuming the layout started it at x = 0, so it
// holds whichever edge the layout packed the row against.
func alignRows(objects []core.CanvasObject, width float32, align bellui.TextAlign) {
	start := 0
	for start < len(objects) {
		if !objects[start].Visible() {
			start++
			continue
		}
		y := objects[start].Position().Y
		minX, maxX := width, float32(0)
		end := start
		for ; end < len(objects); end++ {
			o := objects[end]
			if !o.Visible() {
				continue
			}
			if o.Position().Y != y {
				break
			}
			minX = geo.Min(minX, o.Position().X)
			maxX = geo.Max(maxX, o.Position().X+o.Size().Width)
		}
		var dx float32
		switch lang.ResolveAlign(align) {
		case bellui.TextAlignCenter:
			dx = (width-(maxX-minX))/2 - minX
		case bellui.TextAlignTrailing:
			dx = width - maxX
		default:
			dx = -minX
		}
		if dx != 0 {
			for _, o := range objects[start:end] {
				if o.Visible() {
					o.Move(o.Position().AddXY(dx, 0))
				}
			}
		}
		start = end
	}
}

func (r *chipGroupRenderer) MinSize() geo.Size {
	return r.layout.MinSize(r.Objects())
}

func (r *chipGroupRenderer) Refresh() {
	r.sync()
	if size := r.group.Size(); !size.IsZero() {
		r.Layout(size) // the chips' widths may have changed
	}
	canvas.Refresh(r.group.super())
}

// sync makes the chips match the options, the selection and the disabled
// state.
func (r *chipGroupRenderer) sync() {
	g := r.group
	g.Options = removeDuplicates(g.Options)
	chips := g.chips
	for len(chips) < len(g.Options) {
		c := &Chip{Style: ChipStyleFilter}
		c.OnTapped = func() { g.chipTapped(c) }
		c.ExtendBaseWidget(c)
		chips = append(chips, c)
	}
	chips = chips[:len(g.Options)]
	g.chips = chips

	objects := make([]core.CanvasObject, len(chips))
	for i, c := range chips {
		c.Text = g.Options[i]
		c.Selected = slices.Contains(g.Selected, c.Text)
		switch {
		case g.Disabled() && !c.Disabled():
			c.Disable() // also drops a press or hover under way
		case !g.Disabled() && c.Disabled():
			c.Enable()
		}
		c.Refresh()
		objects[i] = c
	}
	r.SetObjects(objects)
}
