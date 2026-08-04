package layout

// Responsive layout, inspired by the bellui-x project's layout.Responsive
// (fyne.io/x/fyne, layout/responsive.go, Copyright (C) 2020 Fyne.io
// developers and community, BSD-3-Clause license) and by Bootstrap's grid
// system. Reimplemented for this fork: size classes follow the Material
// Design 3 window size classes (in dp, configurable), the class is derived
// from the container width (no window lookup, so it works inside Split/Paned
// panes and is deterministic), and rows mirror under a right-to-left layout
// direction like the other standard layouts.

import (
	"fmt"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// DefaultResponsiveBreakpoints are the Material Design 3 window size class
// boundaries, in dp: compact <600, medium <840, expanded <1200, large <1600,
// extra-large >=1600. Each value is the exclusive upper bound of one size
// class; widths at or beyond the last bound fall in the final class.
// Assign a custom ascending table to [ResponsiveLayout].Breakpoints to use
// different classes — e.g. Bootstrap's 576, 768, 992, 1200.
var DefaultResponsiveBreakpoints = []float32{600, 840, 1200, 1600}

// responsiveClassCount is the number of size classes the default breakpoint
// table produces, and therefore the number of ratios a responsive
// configuration holds.
const responsiveClassCount = 5

// Declare conformity with Layout interface
var _ core.Layout = (*ResponsiveLayout)(nil)

// ResponsiveLayout lays out objects in rows, giving each object a
// width that is its configured ratio of the container width for the active
// size class (see [Responsive]) and wrapping to a new row when a row's
// ratios would exceed the full width. Rows are as tall as their tallest
// member and mirror under a right-to-left layout direction.
//
// Since: 1.0
type ResponsiveLayout struct {
	// Breakpoints are the ascending, exclusive upper bounds (in dp) of each
	// size class except the last. When nil, [DefaultResponsiveBreakpoints]
	// (the MD3 window size classes) apply. A ratio configuration always
	// holds five entries; with a shorter table the trailing entries are
	// unused, with a longer one the last entry repeats.
	Breakpoints []float32

	minSize geo.Size
}

// NewResponsiveLayout returns a container with a [ResponsiveLayout] using the
// MD3 window size classes. Each object may declare per-class width ratios by
// being wrapped with [Responsive]; a plain object occupies a full row in
// every class.
//
// Example:
//
//	container := layout.NewResponsiveLayout(
//	    layout.Responsive(label, 1, .5, .25),  // full width when compact, half when medium, quarter beyond
//	    layout.Responsive(button, 1, .5, .25),
//	    label2,                                // full row in every class
//	)
//
// Since: 1.0
func NewResponsiveLayout(objects ...core.CanvasObject) *bellui.Container {
	wrapped := make([]core.CanvasObject, 0, len(objects))
	for _, o := range objects {
		if _, ok := responsiveConfig(o); !ok {
			o = Responsive(o)
		}
		wrapped = append(wrapped, o)
	}

	return &bellui.Container{Layout: &ResponsiveLayout{}, Objects: wrapped}
}

// Responsive wraps the given object with a responsive width configuration
// for use inside [NewResponsiveLayout]. The optional ratios must satisfy
// 0 < ratio <= 1 (out-of-range values are logged and treated as 1) and apply
// to the size classes in ascending order:
//
//	Responsive(object, compact, medium, expanded, large, extraLarge)
//
// A class without a ratio inherits the previous one, or 1.0 when none was
// given at all.
//
// Since: 1.0
func Responsive(object core.CanvasObject, ratios ...float32) core.CanvasObject {
	return &bellui.Container{
		Layout:  &responsiveStack{config: newResponsiveConf(ratios...)},
		Objects: []core.CanvasObject{object},
	}
}

// responsiveStack is the stack layout of a [Responsive] wrapper and carries the
// child's per-class width ratios. Tagging the wrapper through its Layout keeps
// the wrapper a plain *bellui.Container: an earlier design embedded
// bellui.Container in a custom wrapper type, which the exact-type switches in the
// object-tree walker and the test markup renderer do not match — wrapped
// children were never walked (not painted, not hit-tested) and markup tests
// panicked (#767).
type responsiveStack struct {
	stackLayout // the wrapped object fills the wrapper

	config [responsiveClassCount]float32
}

// responsiveConfig returns the per-class width ratios of a [Responsive]
// wrapper, or ok=false when the object is not one.
func responsiveConfig(o core.CanvasObject) (conf [responsiveClassCount]float32, ok bool) {
	c, isContainer := o.(*bellui.Container)
	if !isContainer {
		return conf, false
	}
	rs, isWrap := c.Layout.(*responsiveStack)
	if !isWrap {
		return conf, false
	}
	return rs.config, true
}

// newResponsiveConf normalizes the given ratios into one per size class,
// inheriting the previous value where unspecified.
func newResponsiveConf(ratios ...float32) [responsiveClassCount]float32 {
	if len(ratios) > responsiveClassCount {
		bellui.LogError(fmt.Sprintf("Responsive: %d ratios declared, only the first %d are used",
			len(ratios), responsiveClassCount), nil)
	}

	var conf [responsiveClassCount]float32
	for i := range conf {
		r := float32(1)
		if i < len(ratios) {
			r = ratios[i]
			if r <= 0 || r > 1 {
				bellui.LogError(fmt.Sprintf("Responsive: ratio must be > 0 and <= 1, got %f", r), nil)
				r = 1
			}
		} else if i > 0 {
			r = conf[i-1]
		}
		conf[i] = r
	}
	return conf
}

// breakpoints returns the active breakpoint table.
func (r *ResponsiveLayout) breakpoints() []float32 {
	if r.Breakpoints == nil {
		return DefaultResponsiveBreakpoints
	}
	return r.Breakpoints
}

// sizeClass returns the index of the size class the given width falls in.
func (r *ResponsiveLayout) sizeClass(width float32) int {
	bps := r.breakpoints()
	for i, bound := range bps {
		if width < bound {
			return min(i, responsiveClassCount-1)
		}
	}
	return min(len(bps), responsiveClassCount-1)
}

// ratioFor returns the width ratio the given object uses in the given class.
// An object that was not wrapped with [Responsive] occupies a full row.
func ratioFor(o core.CanvasObject, class int) float32 {
	if conf, ok := responsiveConfig(o); ok {
		return conf[class]
	}
	return 1
}

// Layout is called to pack all child objects into a specified size.
//
// Implements: core.Layout
func (r *ResponsiveLayout) Layout(objects []core.CanvasObject, containerSize geo.Size) {
	pad := theme.Padding()
	class := r.sizeClass(containerSize.Width)

	// A row breaks when adding the next ratio would exceed the full width;
	// the epsilon absorbs float error from thirds and the like.
	const epsilon = 0.0001

	maxMinWidth := float32(0)
	y := float32(0)
	row := make([]core.CanvasObject, 0, len(objects))
	i := 0
	for i < len(objects) {
		row = row[:0]
		sum := float32(0)
		for ; i < len(objects); i++ {
			o := objects[i]
			if o == nil || !o.Visible() {
				continue
			}
			ratio := ratioFor(o, class)
			if len(row) > 0 && sum+ratio > 1+epsilon {
				break // start a new row
			}
			row = append(row, o)
			sum += ratio
		}
		if len(row) == 0 {
			break // only invisible objects remained
		}

		// Distribute the width left of the inter-object padding by ratio,
		// so ratios summing to 1 fill the row exactly.
		content := containerSize.Width - pad*float32(len(row)-1)
		rowHeight := float32(0)
		for _, o := range row {
			m := o.MinSize()
			rowHeight = max(rowHeight, m.Height)
			maxMinWidth = max(maxMinWidth, m.Width)
		}

		x := float32(0)
		for _, o := range row {
			w := ratioFor(o, class) * content
			o.Resize(geo.NewSize(w, rowHeight))
			o.Move(geo.NewPos(x, y))
			x += w + pad
		}
		y += rowHeight + pad
	}
	if y > 0 {
		y -= pad // no padding below the last row
	}
	r.minSize = geo.NewSize(maxMinWidth, y)

	if layoutRTL() {
		MirrorChildren(objects, containerSize.Width)
	}
}

// MinSize returns the smallest size this layout needs: the widest child's
// minimum width (rows reflow, so no row is forced wider than one child) and
// the height of the current arrangement. Before the first layout pass it
// estimates a single-column stack.
//
// Implements: core.Layout
func (r *ResponsiveLayout) MinSize(objects []core.CanvasObject) geo.Size {
	if !r.minSize.IsZero() {
		return r.minSize
	}

	pad := theme.Padding()
	size := geo.NewSize(0, 0)
	for _, o := range objects {
		if o == nil || !o.Visible() {
			continue
		}
		m := o.MinSize()
		size.Width = max(size.Width, m.Width)
		if size.Height > 0 {
			size.Height += pad
		}
		size.Height += m.Height
	}
	return size
}
