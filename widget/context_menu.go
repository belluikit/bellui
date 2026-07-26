package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

var (
	_ core.Widget             = (*ContextMenu)(nil)
	_ event.SecondaryTappable = (*ContextMenu)(nil)
	_ event.Tappable          = (*ContextMenu)(nil)
	_ event.Hoverable         = (*ContextMenu)(nil)
)

// ContextMenu is a decorator widget that wraps any CanvasObject and shows
// a right-click (secondary tap) popup menu. The wrapped content receives
// all normal interactions (tap, hover, drag, etc.) except the right-click
// is intercepted to display the menu.
//
// Since: 2.7
type ContextMenu struct {
	BaseWidget

	Content core.CanvasObject  // The wrapped widget
	Menu    *bellui.Menu       // Menu shown on right-click
	Items   []*bellui.MenuItem // Alternative: menu items (used if Menu is nil)

	popUp *PopUpMenu
}

// NewContextMenu creates a context menu wrapper around the given content.
// The provided items will be shown as a popup menu on right-click.
//
// Since: 2.7
func NewContextMenu(content core.CanvasObject, items ...*bellui.MenuItem) *ContextMenu {
	cm := &ContextMenu{
		Content: content,
		Items:   items,
	}
	cm.ExtendBaseWidget(cm)
	return cm
}

// NewContextMenuWithMenu creates a context menu wrapper using a pre-built bellui.Menu.
//
// Since: 2.7
func NewContextMenuWithMenu(content core.CanvasObject, menu *bellui.Menu) *ContextMenu {
	cm := &ContextMenu{
		Content: content,
		Menu:    menu,
	}
	cm.ExtendBaseWidget(cm)
	return cm
}

// CreateRenderer returns the renderer for the context menu.
func (cm *ContextMenu) CreateRenderer() core.WidgetRenderer {
	cm.ExtendBaseWidget(cm)
	return &contextMenuRenderer{cm: cm}
}

// Tapped delegates the tap to the wrapped content.
func (cm *ContextMenu) Tapped(e *event.PointEvent) {
	if tappable, ok := cm.Content.(event.Tappable); ok {
		tappable.Tapped(e)
	}
}

// TappedSecondary shows the context menu at the tap position.
func (cm *ContextMenu) TappedSecondary(pe *event.PointEvent) {
	menu := cm.buildMenu()
	if menu == nil || len(menu.Items) == 0 {
		return
	}

	c := bellui.CurrentApp().Driver().CanvasForObject(cm)
	if c == nil {
		return
	}

	entryPos := bellui.CurrentApp().Driver().AbsolutePositionForObject(cm)
	popUpPos := entryPos.Add(pe.Position)
	cm.popUp = NewPopUpMenu(menu, c)
	cm.popUp.ShowAtPosition(popUpPos)
}

// HoverIn delegates hover to the wrapped content.
func (cm *ContextMenu) HoverIn(e *event.HoverEvent) {
	if h, ok := cm.Content.(event.Hoverable); ok {
		h.HoverIn(e)
	}
}

// HoverMoved delegates hover to the wrapped content.
func (cm *ContextMenu) HoverMoved(e *event.HoverEvent) {
	if h, ok := cm.Content.(event.Hoverable); ok {
		h.HoverMoved(e)
	}
}

// HoverOut delegates hover to the wrapped content.
func (cm *ContextMenu) HoverOut() {
	if h, ok := cm.Content.(event.Hoverable); ok {
		h.HoverOut()
	}
}

// Hide hides the widget and any visible popup.
func (cm *ContextMenu) Hide() {
	if cm.popUp != nil {
		cm.popUp.Hide()
	}
	cm.BaseWidget.Hide()
}

func (cm *ContextMenu) buildMenu() *bellui.Menu {
	if cm.Menu != nil {
		return cm.Menu
	}
	if len(cm.Items) == 0 {
		return nil
	}
	return bellui.NewMenu("", cm.Items...)
}

// --- contextMenuRenderer ---

type contextMenuRenderer struct {
	cm *ContextMenu
}

// Destroy has nothing to release: the renderer owns no resource, and the
// right-click popup is owned by the ContextMenu widget and dismissed in Hide.
func (r *contextMenuRenderer) Destroy() {}

func (r *contextMenuRenderer) Layout(size geo.Size) {
	if r.cm.Content != nil {
		r.cm.Content.Resize(size)
		r.cm.Content.Move(geo.NewPos(0, 0))
	}
}

func (r *contextMenuRenderer) MinSize() geo.Size {
	if r.cm.Content != nil {
		return r.cm.Content.MinSize()
	}
	return geo.NewSize(0, 0)
}

func (r *contextMenuRenderer) Objects() []core.CanvasObject {
	if r.cm.Content != nil {
		return []core.CanvasObject{r.cm.Content}
	}
	return nil
}

func (r *contextMenuRenderer) SetObjects(_ []core.CanvasObject) {}

func (r *contextMenuRenderer) Refresh() {
	if r.cm.Content != nil {
		r.cm.Content.Refresh()
	}
}
