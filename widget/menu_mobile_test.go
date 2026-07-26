//go:build mobile

package widget_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

func TestMenu_Layout(t *testing.T) {
	test.NewTempApp(t)

	w := test.NewWindow(canvas.NewRectangle(color.Transparent))
	defer w.Close()
	w.SetPadded(false)
	c := w.Canvas()

	item1 := bellui.NewMenuItem("A", nil)
	item2 := bellui.NewMenuItem("B (long)", nil)
	sep := bellui.NewMenuItemSeparator()
	item3 := bellui.NewMenuItem("C", nil)
	subItem1 := bellui.NewMenuItem("subitem A", nil)
	subItem1.Checked = true
	subItem2 := bellui.NewMenuItem("subitem B", nil)
	subItem2.Checked = true
	subItem2.Icon = theme.InfoIcon()
	subItem3 := bellui.NewMenuItem("subitem C (long)", nil)
	subItem3.Icon = theme.MenuIcon()
	subsubItem1 := bellui.NewMenuItem("subsubitem A (long)", nil)
	subsubItem1.Icon = theme.FileIcon()
	subsubItem2 := bellui.NewMenuItem("subsubitem B", nil)
	subItem3.ChildMenu = bellui.NewMenu("", subsubItem1, subsubItem2)
	item3.ChildMenu = bellui.NewMenu("", subItem1, subItem2, subItem3)
	menu := bellui.NewMenu("", item1, sep, item2, item3)

	for name, tt := range map[string]struct {
		windowSize   geo.Size
		menuPos      geo.Position
		tapPositions []geo.Position
		useTestTheme bool
		want         string
	}{
		"normal": {
			windowSize: geo.NewSize(500, 300),
			menuPos:    geo.NewPos(10, 10),
			want:       "menu/mobile/layout_normal.xml",
		},
		"normal with submenus": {
			windowSize: geo.NewSize(500, 300),
			menuPos:    geo.NewPos(10, 10),
			tapPositions: []geo.Position{
				geo.NewPos(32, 103),
				geo.NewPos(102, 173),
			},
			want: "menu/mobile/layout_normal_with_submenus.xml",
		},
		"background of active submenu parents resets if sibling is hovered": {
			windowSize: geo.NewSize(500, 300),
			menuPos:    geo.NewPos(10, 10),
			tapPositions: []geo.Position{
				geo.NewPos(30, 100),  // open submenu
				geo.NewPos(100, 170), // open subsubmenu
				geo.NewPos(300, 170), // hover subsubmenu item
				geo.NewPos(30, 60),   // hover sibling of submenu parent
			},
			want: "menu/mobile/layout_background_reset.xml",
		},
		"no space on right side for submenu": {
			windowSize: geo.NewSize(500, 300),
			menuPos:    geo.NewPos(410, 10),
			tapPositions: []geo.Position{
				geo.NewPos(432, 103), // open submenu
				geo.NewPos(302, 173), // open subsubmenu
			},
			want: "menu/mobile/layout_no_space_on_right.xml",
		},
		"no space on left & right side for submenu": {
			windowSize: geo.NewSize(200, 300),
			menuPos:    geo.NewPos(10, 10),
			tapPositions: []geo.Position{
				geo.NewPos(32, 103),  // open submenu
				geo.NewPos(102, 173), // open subsubmenu
			},
			want: "menu/mobile/layout_no_space_on_both_sides.xml",
		},
		"window too short for submenu": {
			windowSize: geo.NewSize(500, 150),
			menuPos:    geo.NewPos(10, 10),
			tapPositions: []geo.Position{
				geo.NewPos(30, 100),  // open submenu
				geo.NewPos(100, 130), // open subsubmenu
			},
			want: "menu/mobile/layout_window_too_short_for_submenu.xml",
		},
		"theme change": {
			windowSize:   geo.NewSize(500, 300),
			menuPos:      geo.NewPos(10, 10),
			useTestTheme: true,
			want:         "menu/mobile/layout_theme_changed.xml",
		},
		"window too short for menu": {
			windowSize: geo.NewSize(100, 50),
			menuPos:    geo.NewPos(10, 10),
			want:       "menu/mobile/layout_window_too_short.xml",
		},
	} {
		t.Run(name, func(t *testing.T) {
			w.Resize(tt.windowSize)
			m := widget.NewMenu(menu)
			o := internalWidget.NewOverlayContainer(m, c, nil)
			c.Overlays().Add(o)
			defer c.Overlays().Remove(o)
			m.Move(tt.menuPos)
			m.Resize(m.MinSize())
			for _, pos := range tt.tapPositions {
				test.TapCanvas(c, pos)
			}
			test.AssertRendersToMarkup(t, tt.want, w.Canvas())
			if tt.useTestTheme {
				test.AssertImageMatches(t, "menu/mobile/layout_normal.png", c.Capture())
				test.WithTestTheme(t, func() {
					test.AssertImageMatches(t, "menu/mobile/layout_theme_changed.png", c.Capture())
				})
			}
		})
	}
}

func TestMenu_Dragging(t *testing.T) {
	test.NewTempApp(t)

	w := test.NewWindow(canvas.NewRectangle(color.Transparent))
	defer w.Close()
	w.SetPadded(false)
	c := w.Canvas()

	menu := bellui.NewMenu(
		"",
		bellui.NewMenuItem("A", nil),
		bellui.NewMenuItem("B", nil),
		bellui.NewMenuItem("C", nil),
		bellui.NewMenuItem("D", nil),
		bellui.NewMenuItem("E", nil),
		bellui.NewMenuItem("F", nil),
	)

	// 100x100
	// + 4,5 for canvas’ safe area
	w.Resize(geo.NewSize(104, 105))
	m := widget.NewMenu(menu)
	o := internalWidget.NewOverlayContainer(m, c, nil)
	c.Overlays().Add(o)
	defer c.Overlays().Remove(o)
	m.Move(geo.NewPos(10, 10))
	m.Resize(m.MinSize())
	maxDragDistance := m.MinSize().Height - 90
	test.AssertRendersToMarkup(t, "menu/mobile/drag_top.xml", w.Canvas())

	test.Drag(c, geo.NewPos(20, 20), 0, -50)
	test.AssertRendersToMarkup(t, "menu/mobile/drag_middle.xml", w.Canvas())

	test.Drag(c, geo.NewPos(20, 20), 0, -maxDragDistance)
	test.AssertRendersToMarkup(t, "menu/mobile/drag_bottom.xml", w.Canvas())

	test.Drag(c, geo.NewPos(20, 20), 0, maxDragDistance-50)
	test.AssertRendersToMarkup(t, "menu/mobile/drag_middle.xml", w.Canvas())

	test.Drag(c, geo.NewPos(20, 20), 0, 50)
	test.AssertRendersToMarkup(t, "menu/mobile/drag_top.xml", w.Canvas())
}
