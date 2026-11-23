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
// With Multiple and Excludable both set, a chip has a third state: each tap
// moves it from off to selected, to excluded, and back to off. Excluded
// options are kept in Excluded, apart from Selected. An option is never in
// both lists. The excluded look is BellUI's own, as MD3 has no excluded
// filter chip: a leading cross, an error-coloured outline and the label
// struck through. A screen reader hears "Rock, excluded" (#1383).
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
	// Excludable gives the chips of a Multiple group the excluded state,
	// reached by tapping a selected chip. It does nothing without Multiple.
	//
	// Since: 1.3
	Excludable bool
	// Excluded holds the excluded options, in the order they were
	// excluded. It is drawn only while Excludable and Multiple are set. An
	// option in Selected too is drawn selected, and the next refresh drops
	// it from Excluded.
	//
	// Since: 1.3
	Excluded []string
	// OnChanged fires with the selected options after a tap or SetSelected
	// changes them, in the order they were selected.
	OnChanged func([]string) `json:"-"`
	// OnExcludedChanged fires with the excluded options after a tap or
	// SetExcluded changes them. A tap that moves a chip from selected to
	// excluded changes both lists: both are updated before OnChanged, then
	// OnExcludedChanged, fires. Either callback can refuse the change by
	// putting the lists back with SetSelectedSilently and
	// SetExcludedSilently.
	//
	// Since: 1.3
	OnExcludedChanged func([]string) `json:"-"`

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

// SetSelected selects the options, fires OnChanged and refreshes. An option
// it selects leaves Excluded, which fires OnExcludedChanged.
//
// Since: 1.3
func (g *ChipGroup) SetSelected(options []string) {
	g.Selected = options
	excludedChanged := g.dropFrom(&g.Excluded, options)
	if g.OnChanged != nil {
		g.OnChanged(options)
	}
	if excludedChanged {
		g.fireExcluded()
	}
	g.Refresh()
}

// SetSelectedSilently selects the options and refreshes, as SetSelected
// does, without firing a callback. The refresh takes them out of Excluded.
//
// Since: 1.3
func (g *ChipGroup) SetSelectedSilently(options []string) {
	g.Selected = options
	g.Refresh()
}

// SetExcluded excludes the options, fires OnExcludedChanged and refreshes.
// An option it excludes leaves Selected, which fires OnChanged.
//
// Since: 1.3
func (g *ChipGroup) SetExcluded(options []string) {
	g.Excluded = options
	selectedChanged := g.dropFrom(&g.Selected, options)
	if selectedChanged && g.OnChanged != nil {
		g.OnChanged(g.Selected)
	}
	g.fireExcluded()
	g.Refresh()
}

// SetExcludedSilently excludes the options and refreshes, as SetExcluded
// does, without firing a callback.
//
// Since: 1.3
func (g *ChipGroup) SetExcludedSilently(options []string) {
	g.Excluded = options
	g.dropFrom(&g.Selected, options)
	g.Refresh()
}

// dropFrom removes the options from *list, reporting whether any was there.
// It replaces the slice rather than editing it, as the app may hold it.
func (g *ChipGroup) dropFrom(list *[]string, options []string) bool {
	kept := slices.DeleteFunc(slices.Clone(*list), func(o string) bool { return slices.Contains(options, o) })
	if len(kept) == len(*list) {
		return false
	}
	*list = kept
	return true
}

func (g *ChipGroup) fireExcluded() {
	if g.OnExcludedChanged != nil {
		g.OnExcludedChanged(g.Excluded)
	}
}

// excludes reports whether the group's chips have the excluded state.
func (g *ChipGroup) excludes() bool { return g.Multiple && g.Excludable }

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
	if g.excludes() {
		g.cycle(c.Text)
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

// cycle moves opt one state on, off → selected → excluded → off, and fires
// the callback of each list that changed once both are updated. A Required
// group's last selected chip stays selected.
func (g *ChipGroup) cycle(opt string) {
	sel, exc := g.Selected, g.Excluded
	switch {
	case slices.Contains(exc, opt):
		exc = slices.DeleteFunc(slices.Clone(exc), func(o string) bool { return o == opt })
	case slices.Contains(sel, opt):
		if g.Required && len(sel) == 1 {
			return
		}
		sel = slices.DeleteFunc(slices.Clone(sel), func(o string) bool { return o == opt })
		exc = append(slices.Clone(exc), opt)
	default:
		sel = append(slices.Clone(sel), opt)
	}
	selChanged, excChanged := len(sel) != len(g.Selected), len(exc) != len(g.Excluded)
	g.Selected, g.Excluded = sel, exc
	if selChanged && g.OnChanged != nil {
		g.OnChanged(sel)
	}
	if excChanged {
		g.fireExcluded()
	}
	g.Refresh()
}

type chipGroupRenderer struct {
	widget.BaseRenderer
	group  *ChipGroup
	layout core.Layout
}

func (r *chipGroupRenderer) Layout(size geo.Size) {
	objects := r.Objects()
	r.layout.Layout(objects, size)
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

// sync makes the chips match the options, the selection, the exclusion and
// the disabled state.
func (r *chipGroupRenderer) sync() {
	g := r.group
	g.Options = removeDuplicates(g.Options)
	g.dropFrom(&g.Excluded, g.Selected) // never in both: selected wins
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
		c.excluded = g.excludes() && slices.Contains(g.Excluded, c.Text)
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
