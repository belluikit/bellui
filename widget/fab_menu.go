package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

// FABMenuItem is an action in a FAB menu.
type FABMenuItem struct {
	Label    string
	Icon     bellui.Resource
	OnTapped func()
}

// FABMenu is an MD3 speed-dial FAB that expands to show action items.
type FABMenu struct {
	Icon  bellui.Resource // main FAB icon
	Items []*FABMenuItem

	canvas core.Canvas
	popUp  *PopUp
	open   bool
}

// NewFABMenu creates a FAB menu.
func NewFABMenu(icon bellui.Resource, items []*FABMenuItem) *FABMenu {
	return &FABMenu{Icon: icon, Items: items}
}

// Show displays the FAB menu at the given position on the canvas.
// Typically positioned at bottom-right.
func (f *FABMenu) Show(c core.Canvas, pos geo.Position) {
	f.canvas = c
	f.Hide()

	th := theme.Current()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	var objects []core.CanvasObject

	// Mini FABs for each item, stacked vertically.
	for _, item := range f.Items {
		menuItem := item // capture
		miniFab := NewExtendedFAB(menuItem.Label, menuItem.Icon, func() {
			f.Hide()
			if menuItem.OnTapped != nil {
				menuItem.OnTapped()
			}
		})
		miniFab.Style = FABStyleSurface
		objects = append(objects, miniFab)
	}

	bgColor := th.Color(theme.ColorNameSurface, v)
	content := &bellui.Container{Layout: layout.NewVBoxLayout(), Objects: objects}

	f.popUp = newPopUpWithBgColor(content, c, bgColor)

	// Position above the FAB.
	contentSize := content.MinSize()
	showPos := geo.NewPos(pos.X-contentSize.Width, pos.Y-contentSize.Height-8)
	f.popUp.ShowAtPosition(showPos)
	f.open = true
}

// Hide dismisses the FAB menu.
func (f *FABMenu) Hide() {
	if f.popUp != nil {
		f.popUp.Hide()
		f.popUp = nil
	}
	f.open = false
}

// IsOpen returns whether the menu is currently shown.
func (f *FABMenu) IsOpen() bool {
	return f.open
}

// Toggle opens the menu if closed, closes if open.
func (f *FABMenu) Toggle(c core.Canvas, pos geo.Position) {
	if f.open {
		f.Hide()
	} else {
		f.Show(c, pos)
	}
}
