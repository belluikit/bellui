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
//
// A disabled bar draws its action icons in the disabled colours and refuses
// every press, its FAB's included: Disable disables the FAB too, and Enable
// enables it again only if the bar was the one that disabled it, so a FAB the
// app disabled itself stays disabled.
type BottomAppBar struct {
	DisableableWidget
	Actions []*BottomAppBarAction
	FAB     *FAB // optional, positioned at right

	// fabDisabled is the FAB this bar disabled, which it enables again when
	// the bar is enabled or the FAB leaves it (#1214).
	fabDisabled *FAB
}

// NewBottomAppBar creates a bottom app bar.
func NewBottomAppBar(actions []*BottomAppBarAction, fab *FAB) *BottomAppBar {
	b := &BottomAppBar{Actions: actions, FAB: fab}
	b.ExtendBaseWidget(b)
	return b
}

// Disable disables the bar, its action buttons and its FAB.
func (b *BottomAppBar) Disable() {
	b.DisableableWidget.Disable()
	onMain(b.syncFAB)
}

// Enable enables the bar and its action buttons, and the FAB if the bar
// disabled it.
func (b *BottomAppBar) Enable() {
	b.DisableableWidget.Enable()
	onMain(b.syncFAB)
}

// syncFAB gives the FAB the bar's disabled state. The FAB is the app's
// widget, so the bar enables only a FAB it disabled itself: an app's own
// FAB.Disable is not undone by the bar's Enable. A FAB replaced or removed
// while the bar is disabled is enabled again as it leaves.
func (b *BottomAppBar) syncFAB() {
	if held := b.fabDisabled; held != nil && (held != b.FAB || !b.Disabled()) {
		b.fabDisabled = nil
		held.Enable()
	}
	if fab := b.FAB; fab != nil && b.Disabled() && b.fabDisabled != fab && !fab.Disabled() {
		fab.ExtendBaseWidget(fab)
		b.fabDisabled = fab
		fab.Disable()
	}
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
		clear(r.actionBtns[len(r.bar.Actions):]) // drop the tail's references too
		r.actionBtns = r.actionBtns[:len(r.bar.Actions)]
	}
	for i, a := range r.bar.Actions {
		if i == len(r.actionBtns) {
			btn := NewIconButton(a.Icon, a.OnTapped)
			btn.disabled = r.bar.Disabled() // the buttons are the bar's own parts (#1214)
			r.actionBtns = append(r.actionBtns, btn)
			continue
		}
		btn := r.actionBtns[i]
		btn.Icon = a.Icon
		btn.OnTapped = a.OnTapped
		btn.disabled = r.bar.Disabled()
		btn.Refresh()
	}
	r.bar.syncFAB()

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
