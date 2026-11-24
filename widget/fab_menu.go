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

// Show opens the menu above pos, the FAB's top corner on its trailing side.
//
// In a left-to-right window pos is the FAB's top-right corner: the menu opens
// up and to the left of it, its right edge on pos.X and its bottom edge 8 dp
// above pos.Y, which lines it up with a FAB at the trailing (right) edge. In a
// right-to-left window it is the mirror: pass the FAB's top-left corner and
// the menu opens up and to the right, its left edge on pos.X. The direction
// is the window's; Show is given no FAB, so a container.DirectionScope around
// the FAB does not change it.
//
// The menu is kept fully on the canvas: one that would open past an edge is
// moved inside it, so a FAB at the leading edge gets a menu flush with that
// edge rather than one that opens off-screen (#1424).
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

	// Above the FAB, towards the leading side, then kept on the canvas.
	contentSize := content.MinSize()
	x := pos.X - contentSize.Width
	if root := c.Content(); root != nil && isRTLAt(root) {
		x = pos.X
	}
	showPos := clampToCanvas(geo.NewPos(x, pos.Y-contentSize.Height-8), contentSize, c.Size())
	f.popUp.ShowAtPosition(showPos)
	f.open = true
}

// clampToCanvas moves a box of the given size at pos so that it lies inside
// a canvas of size canvas. A box larger than the canvas is pinned to its
// top-left corner.
func clampToCanvas(pos geo.Position, size, canvas geo.Size) geo.Position {
	pos.X = max(0, min(pos.X, canvas.Width-size.Width))
	pos.Y = max(0, min(pos.Y, canvas.Height-size.Height))
	return pos
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

// Toggle opens the menu if closed, closes if open. pos is as for Show.
func (f *FABMenu) Toggle(c core.Canvas, pos geo.Position) {
	if f.open {
		f.Hide()
	} else {
		f.Show(c, pos)
	}
}
