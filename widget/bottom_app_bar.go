package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

// BottomAppBarAction is an action icon in the bottom app bar.
type BottomAppBarAction struct {
	Icon     bellui.Resource
	OnTapped func()
}

var _ core.Accessible = (*BottomAppBar)(nil)

// AccessibleRole returns [core.RoleToolbar].
func (b *BottomAppBar) AccessibleRole() core.AccessibleRole { return core.RoleToolbar }

// AccessibleLabel returns an empty string.
func (b *BottomAppBar) AccessibleLabel() string { return "" }

// BottomAppBar is an MD3 bottom toolbar with action icons and an optional FAB.
type BottomAppBar struct {
	BaseWidget
	Actions []*BottomAppBarAction
	FAB     *FAB // optional, positioned at right
}

// NewBottomAppBar creates a bottom app bar.
func NewBottomAppBar(actions []*BottomAppBarAction, fab *FAB) *BottomAppBar {
	b := &BottomAppBar{Actions: actions, FAB: fab}
	b.ExtendBaseWidget(b)
	return b
}

func (b *BottomAppBar) MinSize() geo.Size {
	b.ExtendBaseWidget(b)
	return b.BaseWidget.MinSize()
}

func (b *BottomAppBar) CreateRenderer() core.WidgetRenderer {
	b.ExtendBaseWidget(b)
	bg := canvas.NewRectangle(nil)

	r := &bottomAppBarRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer([]core.CanvasObject{bg}),
		bg:           bg,
		bar:          b,
	}
	r.Refresh() // populates the buttons and FAB from the public fields
	return r
}

type bottomAppBarRenderer struct {
	internalWidget.BaseRenderer
	bg         *canvas.Rectangle
	actionBtns []*IconButton
	bar        *BottomAppBar
}

func (r *bottomAppBarRenderer) MinSize() geo.Size {
	th := r.bar.Theme()
	return geo.NewSize(200, th.Size(theme.SizeNameBottomBarHeight))
}

func (r *bottomAppBarRenderer) Layout(size geo.Size) {
	r.bg.Resize(size)
	th := r.bar.Theme()
	btnSize := th.Size(theme.SizeNameIconButtonSize)
	padding := btnSize * 0.4 // 16 at default 40
	stride := btnSize + padding*0.5
	x := padding
	for _, btn := range r.actionBtns {
		btn.Move(geo.NewPos(x, (size.Height-btnSize)/2))
		btn.Resize(geo.NewSize(btnSize, btnSize))
		x += stride
	}
	if r.bar.FAB != nil {
		fabSize := r.bar.FAB.MinSize()
		r.bar.FAB.Move(geo.NewPos(size.Width-fabSize.Width-padding, (size.Height-fabSize.Height)/2))
		r.bar.FAB.Resize(fabSize)
	}

	// Under RTL the action icons mirror to the trailing (right) edge and the FAB
	// to the leading (left) edge; the full-width bg maps onto itself.
	mirrorIfRTL(r.Objects(), size.Width)
}

func (r *bottomAppBarRenderer) Refresh() {
	r.updateActions()
	th := r.bar.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	r.bg.FillColor = th.Color(theme.ColorNameSurfaceContainerMedium, v)
	r.bg.Refresh()
	r.Layout(r.bar.Size())
	canvas.Refresh(r.bar.super())
}

// updateActions re-syncs the drawn objects with the bar's current Actions and
// FAB, the radioGroupRenderer.updateItems pattern (#203, #234): both are
// public fields, so they can be appended to, replaced or truncated at any
// time. Surviving buttons are reused with their icon and handler re-synced in
// place. A FAB that arrives after construction is extended here, since only
// its own constructor does that at build time.
func (r *bottomAppBarRenderer) updateActions() {
	if len(r.actionBtns) > len(r.bar.Actions) {
		r.actionBtns = r.actionBtns[:len(r.bar.Actions)]
	}
	for i, a := range r.bar.Actions {
		if i == len(r.actionBtns) {
			r.actionBtns = append(r.actionBtns, NewIconButton(a.Icon, a.OnTapped))
			continue
		}
		btn := r.actionBtns[i]
		btn.Icon = a.Icon
		btn.OnTapped = a.OnTapped
		btn.Refresh()
	}

	objects := make([]core.CanvasObject, 0, len(r.actionBtns)+2)
	objects = append(objects, r.bg)
	for _, btn := range r.actionBtns {
		objects = append(objects, btn)
	}
	if fab := r.bar.FAB; fab != nil {
		fab.ExtendBaseWidget(fab)
		objects = append(objects, fab)
	}
	r.SetObjects(objects)
}
