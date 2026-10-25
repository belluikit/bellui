package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

var (
	_ core.Widget     = (*Menu)(nil)
	_ event.Tappable  = (*Menu)(nil)
	_ core.Accessible = (*Menu)(nil)
)

// AccessibleRole returns [core.RoleMenu].
func (m *Menu) AccessibleRole() core.AccessibleRole { return core.RoleMenu }

// AccessibleLabel returns an empty string; menus have no intrinsic label.
func (m *Menu) AccessibleLabel() string { return "" }

// Menu is a widget for displaying a bellui.Menu.
type Menu struct {
	BaseWidget
	alignment     bellui.TextAlign
	Items         []core.CanvasObject
	OnDismiss     func() `json:"-"`
	activeItem    *menuItem
	customSized   bool
	containsCheck bool
}

// NewMenu creates a new Menu.
func NewMenu(menu *bellui.Menu) *Menu {
	m := &Menu{}
	m.ExtendBaseWidget(m)
	m.setMenu(menu)
	return m
}

// ActivateLastSubmenu finds the last active menu item traversing through the open submenus
// and activates its submenu if any.
// It returns `true` if there was a submenu and it was activated and `false` elsewhere.
// Activating a submenu does show it and activate its first item.
func (m *Menu) ActivateLastSubmenu() bool {
	if m.activeItem == nil {
		return false
	}
	if !m.activeItem.activateLastSubmenu() {
		return false
	}
	m.Refresh()
	return true
}

// ActivateNext activates the menu item following the currently active menu item.
// If there is no menu item active, it activates the first menu item.
// If there is no menu item after the current active one, it does nothing.
// If a submenu is open, it delegates the activation to this submenu.
func (m *Menu) ActivateNext() {
	if m.activeItem != nil && m.activeItem.isSubmenuOpen() {
		m.activeItem.Child().ActivateNext()
		return
	}

	found := m.activeItem == nil
	for _, item := range m.Items {
		if mItem, ok := item.(*menuItem); ok {
			if found {
				m.activateItem(mItem)
				return
			}
			if mItem == m.activeItem {
				found = true
			}
		}
	}
}

// ActivatePrevious activates the menu item preceding the currently active menu item.
// If there is no menu item active, it activates the last menu item.
// If there is no menu item before the current active one, it does nothing.
// If a submenu is open, it delegates the activation to this submenu.
func (m *Menu) ActivatePrevious() {
	if m.activeItem != nil && m.activeItem.isSubmenuOpen() {
		m.activeItem.Child().ActivatePrevious()
		return
	}

	found := m.activeItem == nil
	for i := len(m.Items) - 1; i >= 0; i-- {
		item := m.Items[i]
		if mItem, ok := item.(*menuItem); ok {
			if found {
				m.activateItem(mItem)
				return
			}
			if mItem == m.activeItem {
				found = true
			}
		}
	}
}

// CreateRenderer returns a new renderer for the menu.
func (m *Menu) CreateRenderer() core.WidgetRenderer {
	m.ExtendBaseWidget(m)
	box := newMenuBox(m.Items)
	// Scroll in both directions so a menu larger than the canvas (too tall or
	// too wide) stays reachable instead of being clipped (pre-rebrand tracker #293). Bars overlay
	// rather than reserve space, so a tall menu doesn't gain a spurious
	// horizontal bar from the vertical bar shrinking the content width.
	scroll := widget.NewScroll(box)
	scroll.ReserveBarSpace = false
	scroll.SetMinSize(box.MinSize())
	objects := []core.CanvasObject{scroll}
	for _, i := range m.Items {
		if item, ok := i.(*menuItem); ok && item.Child() != nil {
			objects = append(objects, item.Child())
		}
	}

	return &menuRenderer{
		widget.NewShadowingRenderer(objects, widget.MenuLevel),
		box,
		m,
		scroll,
	}
}

// DeactivateChild deactivates the active menu item and hides its submenu if any.
func (m *Menu) DeactivateChild() {
	if m.activeItem != nil {
		defer m.activeItem.Refresh()
		if c := m.activeItem.Child(); c != nil {
			c.Hide()
		}
		m.activeItem = nil
	}
}

// DeactivateLastSubmenu finds the last open submenu traversing through the open submenus,
// deactivates its active item and hides it.
// This also deactivates any submenus of the deactivated submenu.
// It returns `true` if there was a submenu open and closed and `false` elsewhere.
func (m *Menu) DeactivateLastSubmenu() bool {
	if m.activeItem == nil {
		return false
	}
	return m.activeItem.deactivateLastSubmenu()
}

// MinSize returns the minimal size of the menu.
func (m *Menu) MinSize() geo.Size {
	m.ExtendBaseWidget(m)
	return m.BaseWidget.MinSize()
}

// Refresh updates the menu to reflect changes in the data.
func (m *Menu) Refresh() {
	for _, item := range m.Items {
		item.Refresh()
	}
	m.BaseWidget.Refresh()
}

func (m *Menu) getContainsCheck() bool {
	for _, item := range m.Items {
		if mi, ok := item.(*menuItem); ok && mi.Item.Checked {
			return true
		}
	}
	return false
}

// Tapped catches taps on separators and the menu background. It doesn't perform any action.
func (m *Menu) Tapped(*event.PointEvent) {
	// Hit a separator or padding -> do nothing.
}

// TriggerLast finds the last active menu item traversing through the open submenus and triggers it.
func (m *Menu) TriggerLast() {
	if m.activeItem == nil {
		m.Dismiss()
		return
	}
	m.activeItem.triggerLast()
}

// Dismiss dismisses the menu by dismissing and hiding the active child and performing OnDismiss.
func (m *Menu) Dismiss() {
	if m.activeItem != nil {
		if m.activeItem.Child() != nil {
			defer m.activeItem.Child().Dismiss()
		}
		m.DeactivateChild()
	}
	if m.OnDismiss != nil {
		m.OnDismiss()
	}
}

func (m *Menu) activateItem(item *menuItem) {
	if item.Child() != nil {
		item.Child().DeactivateChild()
	}
	if m.activeItem == item {
		return
	}

	m.DeactivateChild()
	m.activeItem = item
	m.activeItem.Refresh()
	if m.activeItem.child != nil {
		m.Refresh()
	}
}

func (m *Menu) setMenu(menu *bellui.Menu) {
	m.Items = make([]core.CanvasObject, len(menu.Items))
	for i, item := range menu.Items {
		if item.IsSeparator {
			m.Items[i] = NewSeparator()
		} else {
			m.Items[i] = newMenuItem(item, m)
		}
	}
	m.containsCheck = m.getContainsCheck()
}

type menuRenderer struct {
	*widget.ShadowingRenderer
	box    *menuBox
	m      *Menu
	scroll *widget.Scroll
}

func (r *menuRenderer) Layout(s geo.Size) {
	minSize := r.MinSize()
	var boxSize geo.Size
	if r.m.customSized {
		boxSize = minSize.Max(s)
	} else {
		boxSize = minSize
	}
	scrollSize := boxSize

	driver := bellui.CurrentApp().Driver()
	if c := driver.CanvasForObject(r.m.super()); c != nil {
		ap := driver.AbsolutePositionForObject(r.m.super())
		_, areaSize := c.InteractiveArea()
		if ah := areaSize.Height - ap.Y; ah < scrollSize.Height {
			scrollSize.Height = ah
		}
		// Only clamp width for the top-level popup (customSized). Submenus
		// resolve horizontal overflow by flipping to the other side of their
		// parent, so clamping them would shrink a flipped submenu into a tiny
		// horizontally-scrolling strip instead of showing it at full width.
		if r.m.customSized {
			if aw := areaSize.Width - ap.X; aw < scrollSize.Width {
				scrollSize.Width = aw
			}
		}
	}
	if scrollSize != r.m.Size() {
		r.m.Resize(scrollSize)
		return
	}

	r.LayoutShadow(scrollSize, geo.NewPos(0, 0))
	r.scroll.Resize(scrollSize)
	r.box.Resize(boxSize)
	r.layoutActiveChild()
}

func (r *menuRenderer) MinSize() geo.Size {
	return r.box.MinSize()
}

func (r *menuRenderer) Refresh() {
	r.layoutActiveChild()
	r.ShadowingRenderer.RefreshShadow()

	for _, i := range r.m.Items {
		if txt, ok := i.(*menuItem); ok {
			txt.alignment = r.m.alignment
			txt.Refresh()
		}
	}

	canvas.Refresh(r.m)
}

func (r *menuRenderer) layoutActiveChild() {
	item := r.m.activeItem
	if item == nil || item.Child() == nil {
		return
	}

	if item.Child().Size().IsZero() {
		item.Child().Resize(item.Child().MinSize())
	}

	itemSize := item.Size()
	childSize := item.Child().Size()
	// A submenu opens on the trailing side of its item: to the right, or to
	// the left under a right-to-left layout (#769). It flips to the other side
	// when it does not fit, and is pinned to the canvas edge when neither does.
	rtl := isRTLAt(r.m.super())
	cp := geo.NewPos(itemSize.Width, item.Position().Y)
	if rtl {
		cp.X = -childSize.Width
	}
	d := bellui.CurrentApp().Driver()
	c := d.CanvasForObject(item)
	if c != nil {
		absPos := d.AbsolutePositionForObject(item)
		fitsRight := absPos.X+itemSize.Width+childSize.Width <= c.Size().Width
		fitsLeft := absPos.X-childSize.Width >= 0
		switch {
		case rtl && !fitsLeft && fitsRight:
			cp.X = itemSize.Width
		case rtl && !fitsLeft:
			cp.X = -absPos.X
		case !rtl && !fitsRight && fitsLeft:
			cp.X = -childSize.Width
		case !rtl && !fitsRight:
			cp.X = c.Size().Width - absPos.X - childSize.Width
		}
		requiredHeight := childSize.Height - r.m.Theme().Size(theme.SizeNamePadding)
		availableHeight := c.Size().Height - absPos.Y
		missingHeight := requiredHeight - availableHeight
		if missingHeight > 0 {
			cp.Y -= missingHeight
		}
	}
	item.Child().Move(cp)
}

type menuBox struct {
	BaseWidget
	items []core.CanvasObject
}

var _ core.Widget = (*menuBox)(nil)

func newMenuBox(items []core.CanvasObject) *menuBox {
	b := &menuBox{items: items}
	b.ExtendBaseWidget(b)
	return b
}

func (b *menuBox) CreateRenderer() core.WidgetRenderer {
	th := b.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	background := canvas.NewRectangle(th.Color(theme.ColorNameSurfaceContainerHigh, v))
	background.CornerRadius = th.Size(theme.SizeNameShapeExtraSmall)
	cont := &bellui.Container{Layout: layout.NewVBoxLayout(), Objects: b.items}
	return &menuBoxRenderer{
		BaseRenderer: widget.NewBaseRenderer([]core.CanvasObject{background, cont}),
		b:            b,
		background:   background,
		cont:         cont,
	}
}

type menuBoxRenderer struct {
	widget.BaseRenderer
	b          *menuBox
	background *canvas.Rectangle
	cont       *bellui.Container
}

var _ core.WidgetRenderer = (*menuBoxRenderer)(nil)

func (r *menuBoxRenderer) Layout(size geo.Size) {
	s := geo.NewSize(size.Width, size.Height)
	r.background.Resize(s)
	r.cont.Resize(s)
}

func (r *menuBoxRenderer) MinSize() geo.Size {
	return r.cont.MinSize()
}

func (r *menuBoxRenderer) Refresh() {
	th := r.b.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	r.background.FillColor = th.Color(theme.ColorNameSurfaceContainerHigh, v)
	r.background.CornerRadius = th.Size(theme.SizeNameShapeExtraSmall)
	r.background.Refresh()
	canvas.Refresh(r.b)
}
