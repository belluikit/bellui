//go:build !mobile

package widget_test

import (
	"image/color"
	"runtime"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	item4 := bellui.NewMenuItem("D", nil)
	subItem4a := bellui.NewMenuItem("a", nil)
	subItem4a.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyA, Modifier: event.KeyModifierControl}
	subItem4b := bellui.NewMenuItem("b", nil)
	subItem4b.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyBackspace, Modifier: event.KeyModifierAlt}
	subItem4c := bellui.NewMenuItem("c", nil)
	subItem4c.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyDelete, Modifier: event.KeyModifierSuper}
	subItem4d := bellui.NewMenuItem("d", nil)
	subItem4d.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyDown, Modifier: event.KeyModifierShift}
	subItem4e := bellui.NewMenuItem("e", nil)
	subItem4e.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyEnd, Modifier: event.KeyModifierControl | event.KeyModifierAlt}
	subItem4f := bellui.NewMenuItem("f", nil)
	subItem4f.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyEnter, Modifier: event.KeyModifierControl | event.KeyModifierSuper}
	subItem4g := bellui.NewMenuItem("g", nil)
	subItem4g.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyEscape, Modifier: event.KeyModifierControl | event.KeyModifierShift}
	subItem4h := bellui.NewMenuItem("h", nil)
	subItem4h.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyHome, Modifier: event.KeyModifierAlt | event.KeyModifierSuper}
	subItem4i := bellui.NewMenuItem("i", nil)
	subItem4i.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyLeft, Modifier: event.KeyModifierAlt | event.KeyModifierShift}
	subItem4j := bellui.NewMenuItem("j", nil)
	subItem4j.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyPageDown, Modifier: event.KeyModifierSuper | event.KeyModifierShift}
	subItem4k := bellui.NewMenuItem("k", nil)
	subItem4k.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyPageUp, Modifier: event.KeyModifierControl | event.KeyModifierAlt | event.KeyModifierSuper}
	subItem4l := bellui.NewMenuItem("l", nil)
	subItem4l.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyReturn, Modifier: event.KeyModifierControl | event.KeyModifierAlt | event.KeyModifierShift}
	subItem4m := bellui.NewMenuItem("m", nil)
	subItem4m.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyRight, Modifier: event.KeyModifierControl | event.KeyModifierSuper | event.KeyModifierShift}
	subItem4n := bellui.NewMenuItem("n", nil)
	subItem4n.Shortcut = &desktop.CustomShortcut{KeyName: event.KeySpace, Modifier: event.KeyModifierControl | event.KeyModifierAlt | event.KeyModifierSuper | event.KeyModifierShift}
	subItem4o := bellui.NewMenuItem("o", nil)
	subItem4o.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyTab, Modifier: event.KeyModifierControl}
	subItem4p := bellui.NewMenuItem("p", nil)
	subItem4p.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyUp, Modifier: event.KeyModifierControl}
	subItem4q := bellui.NewMenuItem("q", nil)
	subItem4q.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyF6}
	item4.ChildMenu = bellui.NewMenu("", subItem4a, subItem4b, subItem4c, subItem4d, subItem4e, subItem4f, subItem4g, subItem4h, subItem4i, subItem4j, subItem4k, subItem4l, subItem4m, subItem4n, subItem4o, subItem4p, subItem4q)

	menu := bellui.NewMenu("", item1, sep, item2, item3, sep, item4)

	var shortcutsMasterPrefixPath string
	if runtime.GOOS == "darwin" {
		shortcutsMasterPrefixPath = "menu/desktop/layout_shortcuts_darwin"
	} else {
		shortcutsMasterPrefixPath = "menu/desktop/layout_shortcuts_other"
	}

	for name, tt := range map[string]struct {
		windowSize         geo.Size
		menuPos            geo.Position
		mousePositions     []geo.Position
		want               string
		wantImage          string
		wantTestThemeImage string
	}{
		"normal": {
			windowSize: geo.NewSize(500, 300),
			menuPos:    geo.NewPos(10, 10),
			want:       "menu/desktop/layout_normal.xml",
		},
		"normal with submenus": {
			windowSize: geo.NewSize(500, 300),
			menuPos:    geo.NewPos(10, 10),
			mousePositions: []geo.Position{
				geo.NewPos(32, 103),
				geo.NewPos(102, 173),
			},
			want: "menu/desktop/layout_normal_with_submenus.xml",
		},
		"background of active submenu parents resets if sibling is focused": {
			windowSize: geo.NewSize(500, 300),
			menuPos:    geo.NewPos(10, 10),
			mousePositions: []geo.Position{
				geo.NewPos(30, 100),  // open submenu
				geo.NewPos(100, 170), // open subsubmenu
				geo.NewPos(300, 170), // focus subsubmenu item
				geo.NewPos(30, 60),   // focus sibling of submenu parent
			},
			want: "menu/desktop/layout_background_reset.xml",
		},
		"no space on right side for submenu": {
			windowSize: geo.NewSize(500, 300),
			menuPos:    geo.NewPos(410, 10),
			mousePositions: []geo.Position{
				geo.NewPos(432, 103), // open submenu
				geo.NewPos(302, 173), // open subsubmenu
			},
			want: "menu/desktop/layout_no_space_on_right.xml",
		},
		"no space on left & right side for submenu": {
			windowSize: geo.NewSize(200, 300),
			menuPos:    geo.NewPos(10, 10),
			mousePositions: []geo.Position{
				geo.NewPos(32, 103),  // open submenu
				geo.NewPos(102, 173), // open subsubmenu
			},
			want: "menu/desktop/layout_no_space_on_both_sides.xml",
		},
		"window too short for submenu": {
			windowSize: geo.NewSize(500, 150),
			menuPos:    geo.NewPos(10, 10),
			mousePositions: []geo.Position{
				geo.NewPos(30, 100),  // open submenu
				geo.NewPos(100, 130), // open subsubmenu
			},
			want: "menu/desktop/layout_window_too_short_for_submenu.xml",
		},
		"theme change": {
			windowSize:         geo.NewSize(500, 300),
			menuPos:            geo.NewPos(10, 10),
			want:               "menu/desktop/layout_theme_changed.xml",
			wantImage:          "menu/desktop/layout_normal.png",
			wantTestThemeImage: "menu/desktop/layout_theme_changed.png",
		},
		"window too short for menu": {
			windowSize: geo.NewSize(100, 50),
			menuPos:    geo.NewPos(10, 10),
			want:       "menu/desktop/layout_window_too_short.xml",
		},
		"menu with shortcuts": {
			windowSize: geo.NewSize(450, 800),
			menuPos:    geo.NewPos(10, 10),
			mousePositions: []geo.Position{
				geo.NewPos(32, 143), // open submenu
			},
			want:               shortcutsMasterPrefixPath + ".xml",
			wantImage:          shortcutsMasterPrefixPath + ".png",
			wantTestThemeImage: shortcutsMasterPrefixPath + "_theme_changed.png",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if name == "menu with shortcuts" && (runtime.GOOS == "windows" || runtime.GOOS == "darwin") {
				return // macOS and Windows are off-by-a-fraction for symbol font TODO find out why
			}
			w.Resize(tt.windowSize)
			m := widget.NewMenu(menu)
			o := internalWidget.NewOverlayContainer(m, c, nil)
			c.Overlays().Add(o)
			defer c.Overlays().Remove(o)
			m.Move(tt.menuPos)
			m.Resize(m.MinSize())
			for _, pos := range tt.mousePositions {
				test.MoveMouse(c, pos)
			}
			test.AssertRendersToMarkup(t, tt.want, w.Canvas())
			if tt.wantImage != "" {
				test.AssertImageMatches(t, tt.wantImage, c.Capture())
			}
			if tt.wantTestThemeImage != "" {
				test.WithTestTheme(t, func() {
					test.AssertImageMatches(t, tt.wantTestThemeImage, c.Capture())
				})
			}
		})
	}
}

func TestMenu_Scrolling(t *testing.T) {
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
	maxScrollDistance := m.MinSize().Height - 90
	test.AssertRendersToMarkup(t, "menu/desktop/scroll_top.xml", w.Canvas())

	test.Scroll(c, geo.NewPos(20, 20), 0, -50)
	test.AssertRendersToMarkup(t, "menu/desktop/scroll_middle.xml", w.Canvas())

	test.Scroll(c, geo.NewPos(20, 20), 0, -maxScrollDistance)
	test.AssertRendersToMarkup(t, "menu/desktop/scroll_bottom.xml", w.Canvas())

	test.Scroll(c, geo.NewPos(20, 20), 0, maxScrollDistance-50)
	test.AssertRendersToMarkup(t, "menu/desktop/scroll_middle.xml", w.Canvas())

	test.Scroll(c, geo.NewPos(20, 20), 0, 50)
	test.AssertRendersToMarkup(t, "menu/desktop/scroll_top.xml", w.Canvas())
}

func TestMenu_TraverseMenu(t *testing.T) {
	test.NewTempApp(t)

	w := bellui.CurrentApp().NewWindow("")
	defer w.Close()
	w.SetPadded(false)
	c := w.Canvas()

	itemWithChild := bellui.NewMenuItem("Bar", nil)
	itemWithChild.ChildMenu = bellui.NewMenu(
		"",
		bellui.NewMenuItem("SubA", nil),
		bellui.NewMenuItem("SubB", nil),
	)
	m := widget.NewMenu(bellui.NewMenu(
		"",
		bellui.NewMenuItem("Foo", nil),
		bellui.NewMenuItemSeparator(),
		itemWithChild,
		bellui.NewMenuItemSeparator(),
		bellui.NewMenuItem("Baz", nil),
	))
	w.SetContent(internalWidget.NewOverlayContainer(m, c, nil))
	// + 4,5 for canvas’ safe area
	w.Resize(m.MinSize().AddWidthHeight(4, 5))
	m.Resize(m.MinSize())

	// going all the way down …
	test.AssertRendersToMarkup(t, "menu/desktop/traverse_initial.xml", c)

	m.ActivateNext()
	test.AssertRendersToMarkup(t, "menu/desktop/traverse_first_active.xml", c)

	m.ActivateNext()
	test.AssertRendersToMarkup(t, "menu/desktop/traverse_second_active.xml", c)

	m.ActivateNext()
	test.AssertRendersToMarkup(t, "menu/desktop/traverse_third_active.xml", c)

	m.ActivateNext()
	test.AssertRendersToMarkup(t, "menu/desktop/traverse_third_active.xml", c, "does not wrap around if last item is already active")

	// … and up again
	m.ActivatePrevious()
	test.AssertRendersToMarkup(t, "menu/desktop/traverse_second_active.xml", c)

	m.ActivatePrevious()
	test.AssertRendersToMarkup(t, "menu/desktop/traverse_first_active.xml", c)

	m.ActivatePrevious()
	test.AssertRendersToMarkup(t, "menu/desktop/traverse_first_active.xml", c, "does not wrap around if on top")

	// activate a submenu (show and activate first item)
	m.ActivateNext()
	assert.True(t, m.ActivateLastSubmenu())
	test.AssertRendersToMarkup(t, "menu/desktop/traverse_submenu_first_active.xml", c)

	assert.False(t, m.ActivateLastSubmenu())
	test.AssertRendersToMarkup(t, "menu/desktop/traverse_submenu_first_active.xml", c, "does nothing if there is no submenu at the last active item")

	// traversing through items of opened submenu
	m.ActivateNext()
	test.AssertRendersToMarkup(t, "menu/desktop/traverse_submenu_second_active.xml", c)

	m.ActivateNext()
	test.AssertRendersToMarkup(t, "menu/desktop/traverse_submenu_second_active.xml", c, "does not wrap around if last item is already active")

	m.ActivatePrevious()
	test.AssertRendersToMarkup(t, "menu/desktop/traverse_submenu_first_active.xml", c)

	m.ActivatePrevious()
	test.AssertRendersToMarkup(t, "menu/desktop/traverse_submenu_first_active.xml", c, "does not wrap around if on top")

	// closing an open submenu
	assert.True(t, m.DeactivateLastSubmenu())
	test.AssertRendersToMarkup(t, "menu/desktop/traverse_second_active.xml", c)

	assert.False(t, m.DeactivateLastSubmenu())
	test.AssertRendersToMarkup(t, "menu/desktop/traverse_second_active.xml", c, "does nothing if there is no submenu opened")
}

func TestMenu_TriggerTraversedMenu(t *testing.T) {
	var triggered string
	var dismissed bool
	setupMenu := func() *widget.Menu {
		triggered = ""
		dismissed = false
		itemWithChild := bellui.NewMenuItem("Bar", func() { triggered = "2nd" })
		itemWithChild.ChildMenu = bellui.NewMenu(
			"",
			bellui.NewMenuItem("SubA", func() { triggered = "1st sub" }),
			bellui.NewMenuItem("SubB", nil),
			bellui.NewMenuItem("SubC", func() { triggered = "3rd sub" }),
		)
		m := widget.NewMenu(bellui.NewMenu(
			"",
			bellui.NewMenuItem("Foo", func() { triggered = "1st" }),
			bellui.NewMenuItemSeparator(),
			itemWithChild,
			bellui.NewMenuItemSeparator(),
			bellui.NewMenuItem("Baz", func() { triggered = "3rd" }),
		))
		m.OnDismiss = func() { dismissed = true }
		w := bellui.CurrentApp().NewWindow("")
		t.Cleanup(w.Close)
		w.SetContent(internalWidget.NewOverlayContainer(m, w.Canvas(), nil))
		return m
	}

	t.Run("without active item", func(t *testing.T) {
		m := setupMenu()
		m.TriggerLast()
		assert.Equal(t, "", triggered)
		assert.True(t, dismissed)
	})
	t.Run("first item in submenu", func(t *testing.T) {
		m := setupMenu()
		m.ActivateNext()
		m.ActivateNext()
		require.True(t, m.ActivateLastSubmenu())
		m.TriggerLast()
		assert.Equal(t, "1st sub", triggered)
		assert.True(t, dismissed)
	})
	t.Run("last item in submenu", func(t *testing.T) {
		m := setupMenu()
		m.ActivateNext()
		m.ActivateNext()
		require.True(t, m.ActivateLastSubmenu())
		m.ActivateNext()
		m.ActivateNext()
		m.TriggerLast()
		assert.Equal(t, "3rd sub", triggered)
		assert.True(t, dismissed)
	})
	t.Run("item in menu", func(t *testing.T) {
		m := setupMenu()
		m.ActivateNext()
		m.ActivateNext()
		m.ActivateNext()
		m.TriggerLast()
		assert.Equal(t, "3rd", triggered)
		assert.True(t, dismissed)
	})
	t.Run("item with (closed) submenu", func(t *testing.T) {
		m := setupMenu()
		m.ActivateNext()
		m.ActivateNext()
		m.TriggerLast()
		assert.Equal(t, "2nd", triggered)
		assert.True(t, dismissed)
	})
	t.Run("item without action", func(t *testing.T) {
		m := setupMenu()
		m.ActivateNext()
		m.ActivateNext()
		require.True(t, m.ActivateLastSubmenu())
		m.ActivateNext()
		m.TriggerLast()
		assert.Equal(t, "", triggered)
		assert.True(t, dismissed)
	})
}
