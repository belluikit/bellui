//go:build !no_glfw && !ci && !no_native_menus && !mobile

package glfw

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
)

// TestDarwinMenu_belluiOwnsMainMenu verifies that bellui — not GLFW — builds the
// native main menu bar (pre-rebrand tracker #831): after setup the app submenu (index 0) carries the
// standard About and Quit items, and the standard Window menu still exists. This
// is what keeps the app menu (and its Quit ⌘Q) present now that GLFW's
// createMenuBar() is suppressed via GLFW_COCOA_MENUBAR=false.
func TestDarwinMenu_belluiOwnsMainMenu(t *testing.T) {
	setExceptionCallback(func(msg string) { t.Error("Obj-C exception:", msg) })
	defer setExceptionCallback(nil)

	runOnMain(func() {
		resetMainMenu()
	})
	w := createWindow("Test")

	menu := bellui.NewMenu("File", bellui.NewMenuItem("New", func() {}))
	runOnMain(func() {
		setupNativeMenu(w.window, bellui.NewMainMenu(menu))
	})

	mm := testDarwinMainMenu()

	// The app submenu (index 0) is bellui-built and must carry About + Quit.
	appMenu := testNSMenuItemSubmenu(testNSMenuItemAtIndex(mm, 0))
	assert.Equal(t, "", testNSMenuTitle(appMenu), "app menu has no title")
	var hasAbout, hasQuit bool
	for i := 0; i < testNSMenuNumberOfItems(appMenu); i++ {
		title := testNSMenuItemTitle(testNSMenuItemAtIndex(appMenu, i))
		if strings.HasPrefix(title, "About ") {
			hasAbout = true
		}
		if strings.HasPrefix(title, "Quit ") {
			hasQuit = true
		}
	}
	assert.True(t, hasAbout, "bellui-built app menu must contain an About item")
	assert.True(t, hasQuit, "bellui-built app menu must contain a Quit item")

	// The standard Window menu must still be present.
	var hasWindow bool
	for i := 0; i < testNSMenuNumberOfItems(mm); i++ {
		sub := testNSMenuItemSubmenu(testNSMenuItemAtIndex(mm, i))
		if sub != nil && testNSMenuTitle(sub) == "Window" {
			hasWindow = true
		}
	}
	assert.True(t, hasWindow, "standard Window menu must still exist")
}

func TestDarwinMenu(t *testing.T) {
	setExceptionCallback(func(msg string) { t.Error("Obj-C exception:", msg) })
	defer setExceptionCallback(nil)

	runOnMain(func() {
		resetMainMenu()
	})

	w := createWindow("Test")

	var lastAction string
	assertLastAction := func(wantAction string) {
		assert.Equal(t, wantAction, lastAction)
	}

	assertNSMenuItemSeparator := func(m unsafe.Pointer, i int) {
		item := testNSMenuItemAtIndex(m, i)
		assert.True(t, testNSMenuItemIsSeparatorItem(item), "item is expected to be a separator")
	}

	itemNew := bellui.NewMenuItem("New", func() { lastAction = "new" })
	itemNew.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyN, Modifier: event.KeyModifierShortcutDefault}
	itemOpen := bellui.NewMenuItem("Open", func() { lastAction = "open" })
	itemOpen.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyO, Modifier: event.KeyModifierAlt}
	itemRecent := bellui.NewMenuItem("Recent", nil)
	itemFoo := bellui.NewMenuItem("Foo", func() { lastAction = "foo" })
	itemRecent.ChildMenu = bellui.NewMenu("", itemFoo)
	itemAbout := bellui.NewMenuItem("About", func() { lastAction = "about" })
	menuFile := bellui.NewMenu("File", itemNew, itemOpen, bellui.NewMenuItemSeparator(), itemRecent, itemAbout)

	itemHelp := bellui.NewMenuItem("Help", func() { lastAction = "Help!!!" })
	itemHelp.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyH, Modifier: event.KeyModifierControl}
	itemHelpMe := bellui.NewMenuItem("Help Me", func() { lastAction = "Help me!!!" })
	itemHelpMe.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyH, Modifier: event.KeyModifierShift}
	menuHelp := bellui.NewMenu("Help", itemHelp, itemHelpMe)

	itemHelloWorld := bellui.NewMenuItem("Hello World", func() { lastAction = "Hello World!" })
	itemHelloWorld.Shortcut = &desktop.CustomShortcut{KeyName: event.KeyH, Modifier: event.KeyModifierControl | event.KeyModifierAlt | event.KeyModifierShift | event.KeyModifierSuper}
	itemPrefs := bellui.NewMenuItem("Preferences", func() { lastAction = "prefs" })
	itemMore := bellui.NewMenuItem("More", func() { lastAction = "more" })
	itemMorePrefs := bellui.NewMenuItem("Preferences…", func() { lastAction = "more prefs" })
	menuMore := bellui.NewMenu("More Stuff", itemHelloWorld, itemPrefs, itemMore, itemMorePrefs)

	itemSettings := bellui.NewMenuItem("Settings", func() { lastAction = "settings" })
	itemMoreSetings := bellui.NewMenuItem("Settings…", func() { lastAction = "more settings" })
	menuSettings := bellui.NewMenu("Settings", itemSettings, bellui.NewMenuItemSeparator(), itemMoreSetings)

	mainMenu := bellui.NewMainMenu(menuFile, menuHelp, menuMore, menuSettings)
	runOnMain(func() {
		setupNativeMenu(w.window, mainMenu)
	})

	mm := testDarwinMainMenu()
	// The custom “Preferences” menu should be moved to the system app menu completely.
	// -> only three custom menus
	assert.Equal(t, 5, testNSMenuNumberOfItems(mm), "two built-in + three custom")

	m := testNSMenuItemSubmenu(testNSMenuItemAtIndex(mm, 0))
	assert.Equal(t, "", testNSMenuTitle(m), "app menu doesn't have a title")
	assertNSMenuItem(t, "About "+filepath.Base(os.Args[0]), "", 0, m, 0)
	assertLastAction("about")
	assertNSMenuItemSeparator(m, 1)
	assertNSMenuItem(t, "Preferences", "", 0, m, 2)
	assertLastAction("prefs")
	assertNSMenuItem(t, "Preferences…", "", 0, m, 3)
	assertLastAction("more prefs")
	assertNSMenuItemSeparator(m, 4)
	assertNSMenuItem(t, "Settings", "", 0, m, 5)
	assertLastAction("settings")
	assertNSMenuItem(t, "Settings…", "", 0, m, 6)
	assertLastAction("more settings")

	m = testNSMenuItemSubmenu(testNSMenuItemAtIndex(mm, 1))
	assert.Equal(t, "File", testNSMenuTitle(m))
	assert.Equal(t, 4, testNSMenuNumberOfItems(m))
	// NSEventModifierFlagCommand = 1 << 20
	assertNSMenuItem(t, "New", "n", 0b100000000000000000000, m, 0)
	assertLastAction("new")
	// NSEventModifierFlagOption = 1 << 19
	assertNSMenuItem(t, "Open", "o", 0b10000000000000000000, m, 1)
	assertLastAction("open")
	assertNSMenuItemSeparator(m, 2)
	i := testNSMenuItemAtIndex(m, 3)
	assert.Equal(t, "Recent", testNSMenuItemTitle(i))
	sm := testNSMenuItemSubmenu(i)
	assert.NotNil(t, sm, "item has submenu")
	assert.Equal(t, 1, testNSMenuNumberOfItems(sm))
	assertNSMenuItem(t, "Foo", "", 0, sm, 0)
	assertLastAction("foo")

	m = testNSMenuItemSubmenu(testNSMenuItemAtIndex(mm, 2))
	assert.Equal(t, "More Stuff", testNSMenuTitle(m))
	assert.Equal(t, 2, testNSMenuNumberOfItems(m))
	assertNSMenuItem(t, "Hello World", "h", 0b111100000000000000000, m, 0)
	assertLastAction("Hello World!")
	assertNSMenuItem(t, "More", "", 0, m, 1)
	assertLastAction("more")

	m = testNSMenuItemSubmenu(testNSMenuItemAtIndex(mm, 3))
	assert.Equal(t, "Window", testNSMenuTitle(m))

	m = testNSMenuItemSubmenu(testNSMenuItemAtIndex(mm, 4))
	assert.Equal(t, "Help", testNSMenuTitle(m))
	assert.Equal(t, 2, testNSMenuNumberOfItems(m))
	// NSEventModifierFlagControl = 1 << 18
	assertNSMenuItem(t, "Help", "h", 0b1000000000000000000, m, 0)
	assertLastAction("Help!!!")
	// NSEventModifierFlagShift = 1 << 17
	assertNSMenuItem(t, "Help Me", "h", 0b100000000000000000, m, 1)
	assertLastAction("Help me!!!")

	// change action works
	itemOpen.Action = func() { lastAction = "new open" }
	m = testNSMenuItemSubmenu(testNSMenuItemAtIndex(mm, 1))
	assertNSMenuItem(t, "Open", "", 0, m, 1)
	assertLastAction("new open")
}

// TestDarwinMenu_emptySubmenuNotAttached verifies fix (3) of pre-rebrand tracker #811: an item whose
// ChildMenu is empty (or separators-only) must not get a native NSMenu attached —
// a stray empty submenu is one of the concrete triggers of the use-after-free.
func TestDarwinMenu_emptySubmenuNotAttached(t *testing.T) {
	setExceptionCallback(func(msg string) { t.Error("Obj-C exception:", msg) })
	defer setExceptionCallback(nil)

	runOnMain(func() {
		resetMainMenu()
	})
	w := createWindow("Test")

	itemReal := bellui.NewMenuItem("Real", func() {})
	itemEmpty := bellui.NewMenuItem("Empty", nil)
	itemEmpty.ChildMenu = bellui.NewMenu("") // no items at all
	itemSepOnly := bellui.NewMenuItem("SepOnly", nil)
	itemSepOnly.ChildMenu = bellui.NewMenu("", bellui.NewMenuItemSeparator()) // separators only
	itemFilled := bellui.NewMenuItem("Filled", nil)
	itemFilled.ChildMenu = bellui.NewMenu("", bellui.NewMenuItem("Child", func() {}))

	menu := bellui.NewMenu("File", itemReal, itemEmpty, itemSepOnly, itemFilled)
	runOnMain(func() {
		setupNativeMenu(w.window, bellui.NewMainMenu(menu))
	})

	mm := testDarwinMainMenu()
	m := testNSMenuItemSubmenu(testNSMenuItemAtIndex(mm, 1))
	assert.Equal(t, "File", testNSMenuTitle(m))
	assert.Equal(t, 4, testNSMenuNumberOfItems(m))

	assert.Nil(t, testNSMenuItemSubmenu(testNSMenuItemAtIndex(m, 1)), "empty ChildMenu must not attach a submenu")
	assert.Nil(t, testNSMenuItemSubmenu(testNSMenuItemAtIndex(m, 2)), "separators-only ChildMenu must not attach a submenu")
	assert.NotNil(t, testNSMenuItemSubmenu(testNSMenuItemAtIndex(m, 3)), "non-empty ChildMenu still attaches")
}

// TestDarwinMenu_rebuildRetainsObjects exercises the reset/rebuild path repeatedly
// (fix (1) of pre-rebrand tracker #811). It doesn't reproduce the timing-dependent UAF, but drives the
// retain-then-deferred-release lifecycle to catch any imbalance (double free / leak
// surfacing as an Obj-C exception) in menu teardown.
func TestDarwinMenu_rebuildRetainsObjects(t *testing.T) {
	setExceptionCallback(func(msg string) { t.Error("Obj-C exception:", msg) })
	defer setExceptionCallback(nil)

	runOnMain(func() {
		resetMainMenu()
	})
	w := createWindow("Test")

	build := func(suffix string) *bellui.MainMenu {
		child := bellui.NewMenu("", bellui.NewMenuItem("Child"+suffix, func() {}))
		parent := bellui.NewMenuItem("Parent"+suffix, nil)
		parent.ChildMenu = child
		return bellui.NewMainMenu(bellui.NewMenu("File"+suffix, bellui.NewMenuItem("New"+suffix, func() {}), parent))
	}

	for i := 0; i < 5; i++ {
		mainMenu := build(fmt.Sprintf("%d", i))
		runOnMain(func() {
			setupNativeMenu(w.window, mainMenu)
		})
		mm := testDarwinMainMenu()
		m := testNSMenuItemSubmenu(testNSMenuItemAtIndex(mm, 1))
		assert.Equal(t, fmt.Sprintf("File%d", i), testNSMenuTitle(m))
	}
}

func TestDarwinMenu_specialKeyShortcuts(t *testing.T) {
	setExceptionCallback(func(msg string) { t.Error("Obj-C exception:", msg) })
	defer setExceptionCallback(nil)

	for name, tt := range map[string]struct {
		key     event.KeyName
		wantKey string
	}{
		"Backspace": {
			key:     event.KeyBackspace,
			wantKey: "\x08", // NSBackspaceCharacter
		},
		"Delete": {
			key:     event.KeyDelete,
			wantKey: "\x7f", // NSDeleteCharacter
		},
		"Down": {
			key:     event.KeyDown,
			wantKey: "\uf701", // NSDownArrowFunctionKey
		},
		"End": {
			key:     event.KeyEnd,
			wantKey: "\uf72b", // NSEndFunctionKey
		},
		"Enter": {
			key:     event.KeyEnter,
			wantKey: "\x03", // NSEnterCharacter
		},
		"Escape": {
			key:     event.KeyEscape,
			wantKey: "\x1b", // escape
		},
		"F10": {
			key:     event.KeyF10,
			wantKey: "\uf70d", // NSF10FunctionKey
		},
		"F11": {
			key:     event.KeyF11,
			wantKey: "\uf70e", // NSF11FunctionKey
		},
		"F12": {
			key:     event.KeyF12,
			wantKey: "\uf70f", // NSF12FunctionKey
		},
		"F1": {
			key:     event.KeyF1,
			wantKey: "\uf704", // NSF1FunctionKey
		},
		"F2": {
			key:     event.KeyF2,
			wantKey: "\uf705", // NSF2FunctionKey
		},
		"F3": {
			key:     event.KeyF3,
			wantKey: "\uf706", // NSF3FunctionKey
		},
		"F4": {
			key:     event.KeyF4,
			wantKey: "\uf707", // NSF4FunctionKey
		},
		"F5": {
			key:     event.KeyF5,
			wantKey: "\uf708", // NSF5FunctionKey
		},
		"F6": {
			key:     event.KeyF6,
			wantKey: "\uf709", // NSF6FunctionKey
		},
		"F7": {
			key:     event.KeyF7,
			wantKey: "\uf70a", // NSF7FunctionKey
		},
		"F8": {
			key:     event.KeyF8,
			wantKey: "\uf70b", // NSF8FunctionKey
		},
		"F9": {
			key:     event.KeyF9,
			wantKey: "\uf70c", // NSF9FunctionKey
		},
		"Home": {
			key:     event.KeyHome,
			wantKey: "\uf729", // NSHomeFunctionKey
		},
		"Insert": {
			key:     event.KeyInsert,
			wantKey: "\uf727", // NSInsertFunctionKey
		},
		"Left": {
			key:     event.KeyLeft,
			wantKey: "\uf702", // NSLeftArrowFunctionKey
		},
		"PageDown": {
			key:     event.KeyPageDown,
			wantKey: "\uf72d", // NSPageDownFunctionKey
		},
		"PageUp": {
			key:     event.KeyPageUp,
			wantKey: "\uf72c", // NSPageUpFunctionKey
		},
		"Return": {
			key:     event.KeyReturn,
			wantKey: "\n",
		},
		"Right": {
			key:     event.KeyRight,
			wantKey: "\uf703", // NSRightArrowFunctionKey
		},
		"Space": {
			key:     event.KeySpace,
			wantKey: " ",
		},
		"Tab": {
			key:     event.KeyTab,
			wantKey: "\t",
		},
		"Up": {
			key:     event.KeyUp,
			wantKey: "\uf700", // NSUpArrowFunctionKey
		},
	} {
		t.Run(name, func(t *testing.T) {
			runOnMain(func() {
				resetMainMenu()
			})
			w := createWindow("Test")
			item := bellui.NewMenuItem("Special", func() {})
			item.Shortcut = &desktop.CustomShortcut{KeyName: tt.key, Modifier: event.KeyModifierShortcutDefault}
			menu := bellui.NewMenu("Special", item)
			mainMenu := bellui.NewMainMenu(menu)
			runOnMain(func() {
				setupNativeMenu(w.window, mainMenu)
			})

			mm := testDarwinMainMenu()
			m := testNSMenuItemSubmenu(testNSMenuItemAtIndex(mm, 1))
			assertNSMenuItem(t, "Special", tt.wantKey, 0b100000000000000000000, m, 0)
		})
	}
}

var (
	initialAppMenuItems []string
	initialMenus        []string
)

func assertNSMenuItem(t *testing.T, wantTitle, wantKey string, wantModifier uint64, m unsafe.Pointer, i int) {
	item := testNSMenuItemAtIndex(m, i)
	assert.Equal(t, wantTitle, testNSMenuItemTitle(item))
	if wantKey != "" {
		assert.Equal(t, wantKey, testNSMenuItemKeyEquivalent(item))
		assert.Equal(t, wantModifier, testNSMenuItemKeyEquivalentModifierMask(item))
	}
	testNSMenuPerformActionForItemAtIndex(m, i)
}

func initMainMenu() {
	createWindow("Test").Close() // ensure GLFW has performed [NSApp run]
	mainMenu := testDarwinMainMenu()
	for i := 0; i < testNSMenuNumberOfItems(mainMenu); i++ {
		menu := testNSMenuItemSubmenu(testNSMenuItemAtIndex(mainMenu, i))
		initialMenus = append(initialMenus, testNSMenuTitle(menu))
	}
	appMenu := testNSMenuItemSubmenu(testNSMenuItemAtIndex(mainMenu, 0))
	for i := 0; i < testNSMenuNumberOfItems(appMenu); i++ {
		item := testNSMenuItemAtIndex(appMenu, i)
		initialAppMenuItems = append(initialAppMenuItems, testNSMenuItemTitle(item))
	}
}

func resetMainMenu() {
	mainMenu := testDarwinMainMenu()
	j := 0
	for i := 0; i < testNSMenuNumberOfItems(mainMenu); i++ {
		menu := testNSMenuItemSubmenu(testNSMenuItemAtIndex(mainMenu, i))
		if j < len(initialMenus) && testNSMenuTitle(menu) == initialMenus[j] {
			j++
			continue
		}
		testNSMenuRemoveItemAtIndex(mainMenu, i)
		i--
	}
	j = 0
	appMenu := testNSMenuItemSubmenu(testNSMenuItemAtIndex(mainMenu, 0))
	for i := 0; i < testNSMenuNumberOfItems(appMenu); i++ {
		item := testNSMenuItemAtIndex(appMenu, i)
		if testNSMenuItemTitle(item) == initialAppMenuItems[j] {
			j++
			continue
		}
		testNSMenuRemoveItemAtIndex(appMenu, i)
		i--
	}
}

// TestDarwinMenu_separatorOwnedLikeOtherItems pins that a separator carries the
// same strong references as any other item bellui installs: one from the NSMenu
// it was added to, one from belluiLiveMenuObjects.
//
// insertDarwinMenuItem releases every item once after handing it to the menu,
// which balances the alloc in the ordinary branch — but +separatorItem returns
// an object the caller does not own, so that release used to be one too many.
// The separator then outlived its own refcount: the next resetDarwinMenu drops
// the live-objects array, the separator deallocates while its NSMenu still
// lists it, and the menu's own dealloc messages the freed item —
// objc_msgSend on garbage from inside -[NSMenu dealloc], reached through the
// autorelease-pool drain in _glfwPollEventsCocoa. That crash needs a second
// menu generation to appear, which is why an app only hits it when it replaces
// its main menu (belluikit/studio#5).
func TestDarwinMenu_separatorOwnedLikeOtherItems(t *testing.T) {
	setExceptionCallback(func(msg string) { t.Error("Obj-C exception:", msg) })
	defer setExceptionCallback(nil)

	runOnMain(func() {
		resetMainMenu()
	})
	w := createWindow("Test")

	mainMenu := bellui.NewMainMenu(bellui.NewMenu(
		"File",
		bellui.NewMenuItem("New", func() {}),
		bellui.NewMenuItemSeparator(),
		bellui.NewMenuItem("Open", func() {}),
	))
	// Bracketed by a pool so the separator's own pending autorelease is
	// settled before the counts are read.
	runOnMain(func() {
		testPushAutoreleasePool()
		setupNativeMenu(w.window, mainMenu)
		testPopAutoreleasePool()
	})

	file := testNSMenuItemSubmenu(testNSMenuItemAtIndex(testDarwinMainMenu(), 1))
	sep := testNSMenuItemAtIndex(file, 1)
	if !testNSMenuItemIsSeparatorItem(sep) {
		t.Fatalf("item 1 of the File menu is not the separator")
	}

	// Compared against a sibling rather than a literal, so the assertion
	// survives any future change to how many references bellui holds.
	want := testNSObjectRetainCount(testNSMenuItemAtIndex(file, 0))
	if got := testNSObjectRetainCount(sep); got != want {
		t.Errorf("separator retain count = %d, want %d (as for a normal item) — "+
			"an over-released separator deallocates under its own NSMenu", got, want)
	}
}
