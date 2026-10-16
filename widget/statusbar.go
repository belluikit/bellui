package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

// StatusBarItem represents any interface element that can be added to a status bar.
type StatusBarItem interface {
	StatusBarObject() core.CanvasObject
}

// StatusBarLabel is a text section in the status bar.
//
// By default the text truncates with an ellipsis instead of growing the
// status bar's minimum size: a status bar is a fixed strip, and a long
// message (an error with a file path) must not force a window-wide relayout
// (#1041). Within the bar the section is width-flexible — it shows the full
// text whenever the bar has room and shrinks (truncating) when it does not.
// Set NoTruncate for a section whose text must always be fully visible and
// may dictate the bar's minimum width.
type StatusBarLabel struct {
	Text string
	Icon bellui.Resource

	// NoTruncate opts this section out of the default ellipsis truncation,
	// restoring the sizing where the status bar's minimum width tracks the
	// full text.
	NoTruncate bool

	label *Label
	icon  *canvas.Image
	bar   *StatusBar // owning bar, for width re-distribution on SetText
}

// NewStatusBarLabel returns a new text item for a StatusBar.
func NewStatusBarLabel(text string) *StatusBarLabel {
	return &StatusBarLabel{Text: text}
}

// NewStatusBarLabelWithIcon returns a new text item with an icon for a StatusBar.
func NewStatusBarLabelWithIcon(text string, icon bellui.Resource) *StatusBarLabel {
	return &StatusBarLabel{Text: text, Icon: icon}
}

// SetText updates the text shown in this status bar label.
func (s *StatusBarLabel) SetText(text string) {
	s.Text = text
	if s.label != nil {
		s.label.SetText(text)
	}
	// A truncating label's MinSize is text-independent (#1041), so no relayout
	// reaches the bar from the text change — re-run the row layout ourselves
	// so this section grows into the bar's slack or shrinks back.
	if s.bar != nil {
		s.bar.relayoutSections()
	}
}

// StatusBarObject returns the canvas object for this label.
func (s *StatusBarLabel) StatusBarObject() core.CanvasObject {
	s.label = NewLabel(s.Text)
	s.label.TextStyle.Bold = false
	if !s.NoTruncate {
		s.label.Truncation = bellui.TextTruncateEllipsis
	}
	if s.Icon != nil {
		s.icon = canvas.NewImageFromResource(s.Icon)
		s.icon.FillMode = canvas.ImageFillContain
		s.icon.SetMinSize(geo.NewSquareSize(theme.Size(theme.SizeNameCaptionText) + 2))
		// Border (not HBox) so the label receives the section's remaining
		// width — a truncating label's MinSize width is ~0, and an HBox
		// would collapse it regardless of the space the bar allotted.
		return &bellui.Container{
			Layout:  layout.NewBorderLayout(nil, nil, s.icon, nil),
			Objects: []core.CanvasObject{s.icon, s.label},
		}
	}
	return s.label
}

// StatusBarSpacer is a blank, stretchable space for a status bar.
type StatusBarSpacer struct{}

// NewStatusBarSpacer returns a new spacer item for a StatusBar.
func NewStatusBarSpacer() *StatusBarSpacer {
	return &StatusBarSpacer{}
}

// StatusBarObject returns a spacer canvas object.
func (s *StatusBarSpacer) StatusBarObject() core.CanvasObject {
	return layout.NewSpacer()
}

// StatusBarSeparator is a thin vertical divider between status bar sections.
type StatusBarSeparator struct{}

// NewStatusBarSeparator returns a new separator item for a StatusBar.
func NewStatusBarSeparator() *StatusBarSeparator {
	return &StatusBarSeparator{}
}

// StatusBarObject returns a separator canvas object.
func (s *StatusBarSeparator) StatusBarObject() core.CanvasObject {
	return &Separator{invert: true}
}

// StatusBar is a bottom bar widget with multiple sections.
//
// Text sections containing a [Label] with truncation enabled (what
// [StatusBarLabel] produces by default) are width-flexible: they render at
// their natural width when the bar has room and shrink — truncating with an
// ellipsis — when it does not, so a long status message never grows the
// bar's minimum size (#1041). Other items keep their minimum width and
// [StatusBarSpacer] absorbs any leftover space.
//
// Since: 1.0
type StatusBar struct {
	BaseWidget
	Items []StatusBarItem
}

// NewStatusBar creates a new status bar widget.
//
// Since: 1.0
func NewStatusBar(items ...StatusBarItem) *StatusBar {
	sb := &StatusBar{Items: items}
	sb.ExtendBaseWidget(sb)
	return sb
}

// AccessibleRole returns [core.RoleToolbar]; the bar's items provide their
// own semantics.
func (sb *StatusBar) AccessibleRole() core.AccessibleRole { return core.RoleToolbar }

// AccessibleLabel returns an empty string.
func (sb *StatusBar) AccessibleLabel() string { return "" }

// Append adds a new item to the end of the status bar.
func (sb *StatusBar) Append(item StatusBarItem) {
	sb.Items = append(sb.Items, item)
	sb.Refresh()
}

// SetItems replaces all items in the status bar.
func (sb *StatusBar) SetItems(items ...StatusBarItem) {
	sb.Items = items
	sb.Refresh()
}

// relayoutSections re-runs the row layout so width-flexible sections track a
// text change — growing into the bar's slack or shrinking back — without any
// relayout propagating to ancestors (the bar's MinSize is unchanged by
// design, #1041).
func (sb *StatusBar) relayoutSections() {
	r, ok := cache.Renderer(sb.super()).(*statusBarRenderer)
	if !ok {
		return
	}
	size := sb.Size()
	if size.IsZero() {
		return
	}
	r.Layout(size)
	canvas.Refresh(sb.super())
}

// CreateRenderer returns a new renderer for this status bar.
func (sb *StatusBar) CreateRenderer() core.WidgetRenderer {
	sb.ExtendBaseWidget(sb)
	r := &statusBarRenderer{
		statusBar: sb,
		hLayout:   &statusBarItemsLayout{hbox: layout.NewHBoxLayout()},
		topLine:   NewSeparator(),
	}
	r.resetObjects()
	return r
}

// MinSize returns the minimum size of the status bar.
func (sb *StatusBar) MinSize() geo.Size {
	sb.ExtendBaseWidget(sb)
	return sb.BaseWidget.MinSize()
}

type statusBarRenderer struct {
	widget.BaseRenderer
	hLayout   core.Layout
	items     []core.CanvasObject
	topLine   *Separator
	statusBar *StatusBar
}

func (r *statusBarRenderer) MinSize() geo.Size {
	itemsMin := r.hLayout.MinSize(r.items)
	sepH := r.topLine.MinSize().Height
	return geo.NewSize(itemsMin.Width, itemsMin.Height+sepH)
}

func (r *statusBarRenderer) Layout(size geo.Size) {
	sepH := r.topLine.MinSize().Height
	r.topLine.Resize(geo.NewSize(size.Width, sepH))
	r.topLine.Move(geo.NewPos(0, 0))

	itemsSize := geo.NewSize(size.Width, size.Height-sepH)
	r.hLayout.Layout(r.items, itemsSize)
	for _, item := range r.items {
		item.Move(item.Position().AddXY(0, sepH))
	}
}

func (r *statusBarRenderer) Refresh() {
	r.resetObjects()
	canvas.Refresh(r.statusBar)
}

// statusBarItemsLayout is an HBox variant for the status bar row (#1041).
// Sections containing a truncating label are width-flexible: MinSize charges
// them only their (near-zero) truncated minimum, and Layout grows them toward
// their natural text width when the bar has room — sharing the shortage via
// water-filling (smallest needs satisfied first) when it does not — so long
// status text truncates instead of dictating window-wide layout. Fixed items
// keep their minimum width; spacers share whatever is left, as in a plain
// HBox.
type statusBarItemsLayout struct {
	hbox core.Layout
}

func (l *statusBarItemsLayout) MinSize(objects []core.CanvasObject) geo.Size {
	// A truncating label already reports a near-zero minimum width, so the
	// plain HBox minimum is exactly the fixed-strip contract we want.
	return l.hbox.MinSize(objects)
}

func (l *statusBarItemsLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	padding := theme.Padding()
	spacers, visible := 0, 0
	minTotal := float32(0)
	widths := make([]float32, len(objects)) // final width per object; -1 = spacer
	needs := make([]float32, 0, len(objects))
	needIdxOf := make([]int, len(objects)) // index into needs; -1 = fixed width

	for i, child := range objects {
		needIdxOf[i] = -1
		if !child.Visible() {
			continue
		}
		if isStatusBarSpacer(child) {
			widths[i] = -1
			spacers++
			continue
		}
		visible++
		min, natural, flexible := statusBarSectionWidths(child)
		minTotal += min
		widths[i] = min
		if flexible && natural > min {
			needIdxOf[i] = len(needs)
			needs = append(needs, natural-min)
		}
	}

	extra := size.Width - minTotal - padding*float32(visible-1)
	if extra < 0 {
		extra = 0
	}

	// Flexible sections grow toward their natural width first. When the room
	// does not cover all of them, sections with the smallest need are
	// satisfied fully and the rest split what remains evenly — so one very
	// long message absorbs the shortage instead of starving short sections.
	grants := distributeGrowth(needs, extra)
	for i, n := range needIdxOf {
		if n >= 0 {
			widths[i] += grants[n]
			extra -= grants[n]
		}
	}

	// Spacers split what remains, as in a plain HBox.
	spacerSize := float32(0)
	if spacers > 0 {
		spacerSize = extra / float32(spacers)
	}

	x := float32(0)
	for i, child := range objects {
		if !child.Visible() {
			continue
		}
		if widths[i] < 0 { // spacer
			child.Move(geo.NewPos(x, 0))
			child.Resize(geo.NewSize(spacerSize, size.Height))
			x += spacerSize
			continue
		}

		child.Move(geo.NewPos(x, 0))
		child.Resize(geo.NewSize(widths[i], size.Height))
		x += padding + widths[i]
	}

	mirrorIfRTL(objects, size.Width)
}

// distributeGrowth shares extra width among sections wanting the given extra
// amounts ("needs"), returning the grant for each need in order. Needs that
// fit within an even split are satisfied fully; the remainder is split evenly
// among the rest (water-filling). The total granted never exceeds extra.
func distributeGrowth(needs []float32, extra float32) []float32 {
	grants := make([]float32, len(needs))
	remaining := len(needs)
	for remaining > 0 && extra > 0 {
		share := extra / float32(remaining)
		satisfiedOne := false
		for i, need := range needs {
			if grants[i] == need || need == 0 {
				continue
			}
			if need <= share {
				grants[i] = need
				extra -= need
				remaining--
				satisfiedOne = true
			}
		}
		if !satisfiedOne {
			// no section fits fully within an even share: split evenly
			for i, need := range needs {
				if grants[i] != need {
					grants[i] = share
				}
			}
			break
		}
	}
	return grants
}

// statusBarSectionWidths reports an item's minimum width, the natural width
// it would like when space allows, and whether it is width-flexible. A
// section is flexible when it is — or directly contains — a [Label] with
// truncation enabled; its natural width swaps the label's truncated minimum
// for the label's intrinsic (untruncated) text width.
func statusBarSectionWidths(obj core.CanvasObject) (min, natural float32, flexible bool) {
	min = obj.MinSize().Width
	switch o := obj.(type) {
	case *Label:
		if o.Truncation != bellui.TextTruncateOff {
			return min, o.IntrinsicSize().Width, true
		}
	case *bellui.Container:
		for _, child := range o.Objects {
			if lbl, ok := child.(*Label); ok && lbl.Truncation != bellui.TextTruncateOff {
				return min, min - lbl.MinSize().Width + lbl.IntrinsicSize().Width, true
			}
		}
	}
	return min, min, false
}

func isStatusBarSpacer(obj core.CanvasObject) bool {
	spacer, ok := obj.(layout.SpacerObject)
	return ok && spacer.ExpandHorizontal()
}

func (r *statusBarRenderer) resetObjects() {
	r.items = make([]core.CanvasObject, 0, len(r.statusBar.Items))
	for _, item := range r.statusBar.Items {
		if lbl, ok := item.(*StatusBarLabel); ok {
			lbl.bar = r.statusBar
		}
		r.items = append(r.items, item.StatusBarObject())
	}
	objs := make([]core.CanvasObject, 0, len(r.items)+1)
	objs = append(objs, r.topLine)
	objs = append(objs, r.items...)
	r.SetObjects(objs)
}
