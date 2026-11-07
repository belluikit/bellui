package container

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

var (
	_ event.Draggable   = (*tabDragSurface)(nil)
	_ event.DragClaimer = (*tabDragSurface)(nil)
)

// tabDragSurface takes drags along the tab buttons of an AppTabs with
// DragToSelect set. It lies under the buttons and is neither tappable nor
// hoverable, so a tap or hover still reaches the button above it, while a drag
// that starts on a button reaches it: the buttons are not draggable.
type tabDragSurface struct {
	widget.BaseWidget
	r *appTabsRenderer
}

func newTabDragSurface(r *appTabsRenderer) *tabDragSurface {
	s := &tabDragSurface{r: r}
	s.ExtendBaseWidget(s)
	return s
}

func (s *tabDragSurface) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(nil))
}

// ClaimsDrag takes a touch drag that starts along the tab bar and leaves one
// that starts across it to a scroll around the tabs.
func (s *tabDragSurface) ClaimsDrag(d geo.Delta) bool {
	if s.r.verticalBar() {
		return abs32(d.DY) > abs32(d.DX)
	}
	return abs32(d.DX) > abs32(d.DY)
}

func (s *tabDragSurface) Dragged(e *event.DragEvent) {
	along := e.Position.X
	if s.r.verticalBar() {
		along = e.Position.Y
	}
	s.r.drag.active = true
	s.r.drag.at = along
	s.r.placeDragIndicator()
}

func (s *tabDragSurface) DragEnd() {
	if !s.r.drag.active {
		return
	}
	s.r.drag.active = false
	if i := s.r.nearestEnabledTab(s.r.dragCenter()); i >= 0 && i != s.r.appTabs.current {
		s.r.appTabs.SelectIndex(i) // its refresh animates the indicator onto the tab
		return
	}
	s.r.updateIndicator(true) // back onto the selected tab
}

// tabDrag is the state of a drag along the tab buttons.
type tabDrag struct {
	active bool
	at     float32 // the pointer along the bar's axis, from the start of the buttons
}

// layoutDragSurface lays the drag surface over the tab buttons, leaving the
// overflow button out.
func (r *appTabsRenderer) layoutDragSurface() {
	buttons := r.bar.Objects[0]
	r.dragSurface.Move(r.bar.Position().Add(buttons.Position()))
	r.dragSurface.Resize(buttons.Size())
}

// tabSpan returns where tab button i lies along the bar's axis, measured from
// the start of the buttons.
func (r *appTabsRenderer) tabSpan(i int) (start, length float32) {
	b := r.bar.Objects[0].(*bellui.Container).Objects[i]
	if r.verticalBar() {
		return b.Position().Y, b.Size().Height
	}
	return b.Position().X, b.Size().Width
}

// buttonsOffset returns where the tab buttons start along the bar's axis, in
// the renderer's coordinates. An overflow button before them (the trailing
// one, mirrored in a right-to-left layout) moves them along.
func (r *appTabsRenderer) buttonsOffset() float32 {
	off := r.bar.Position().Add(r.bar.Objects[0].Position())
	if r.verticalBar() {
		return off.Y
	}
	return off.X
}

// dragCenter returns the pointer along the bar, held between the centres of
// the first and last tab buttons, or -1 when there are no buttons. The
// indicator is centred on it.
func (r *appTabsRenderer) dragCenter() float32 {
	n := len(r.bar.Objects[0].(*bellui.Container).Objects)
	if n == 0 {
		return -1
	}
	lo, hi := r.tabCenter(0), r.tabCenter(n-1)
	if lo > hi { // a right-to-left layout puts the first tab last
		lo, hi = hi, lo
	}
	return geo.Max(lo, geo.Min(hi, r.drag.at))
}

func (r *appTabsRenderer) tabCenter(i int) float32 {
	start, length := r.tabSpan(i)
	return start + length/2
}

// placeDragIndicator centres the indicator on the pointer. Between two tabs
// its length moves from one tab's length to the other's, so it fits each tab
// exactly as it passes over it.
func (r *appTabsRenderer) placeDragIndicator() {
	center := r.dragCenter()
	if center < 0 {
		return
	}
	n := len(r.bar.Objects[0].(*bellui.Container).Objects)
	length := r.tabLengthAt(center, n)
	pos, size := r.indicatorRect(r.buttonsOffset()+center-length/2, length)
	r.indicator.Show()
	r.placeIndicator(pos, size)
}

// tabLengthAt interpolates the tab length at center between the two tabs
// whose centres it lies between. dragCenter keeps center within the tabs'
// centres, so with two or more tabs one pair always holds it.
func (r *appTabsRenderer) tabLengthAt(center float32, n int) float32 {
	for i := 0; i+1 < n; i++ {
		a, b := r.tabCenter(i), r.tabCenter(i+1)
		if lo, hi := geo.Min(a, b), geo.Max(a, b); center < lo || center > hi || lo == hi {
			continue
		}
		_, la := r.tabSpan(i)
		_, lb := r.tabSpan(i + 1)
		return la + (lb-la)*(center-a)/(b-a)
	}
	_, length := r.tabSpan(0)
	return length
}

// nearestEnabledTab returns the enabled tab button whose centre is closest to
// center, or -1 when none is enabled.
func (r *appTabsRenderer) nearestEnabledTab(center float32) int {
	best, bestDist := -1, float32(0)
	for i := range r.bar.Objects[0].(*bellui.Container).Objects {
		if r.appTabs.Items[i].Disabled() {
			continue
		}
		if d := abs32(r.tabCenter(i) - center); best < 0 || d < bestDist {
			best, bestDist = i, d
		}
	}
	return best
}

// placeIndicator moves the indicator at once, stopping any animation, and
// records where it is so the next moveIndicator animates from there.
func (r *baseTabsRenderer) placeIndicator(pos geo.Position, size geo.Size) {
	if r.positionAnimation != nil {
		r.positionAnimation.Stop()
		r.positionAnimation = nil
	}
	if r.sizeAnimation != nil {
		r.sizeAnimation.Stop()
		r.sizeAnimation = nil
	}
	r.indicator.Move(pos)
	r.indicator.Resize(size)
	r.indicator.Refresh()
	r.lastIndicatorPos = pos
	r.lastIndicatorSize = size
	r.lastIndicatorHidden = r.indicator.Hidden
}

func abs32(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}
