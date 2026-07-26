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

	var objects []core.CanvasObject
	objects = append(objects, bg)

	var actionBtns []*IconButton
	for _, a := range b.Actions {
		btn := NewIconButton(a.Icon, a.OnTapped)
		actionBtns = append(actionBtns, btn)
		objects = append(objects, btn)
	}

	if b.FAB != nil {
		objects = append(objects, b.FAB)
	}

	r := &bottomAppBarRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer(objects),
		bg:           bg,
		actionBtns:   actionBtns,
		bar:          b,
	}
	r.Refresh()
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
	th := r.bar.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	r.bg.FillColor = th.Color(theme.ColorNameSurfaceContainerMedium, v)
	r.bg.Refresh()
	r.Layout(r.bar.Size())
	canvas.Refresh(r.bar.super())
}
