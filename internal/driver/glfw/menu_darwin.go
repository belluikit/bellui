//go:build darwin && !no_native_menus

package glfw

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"strings"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/internal/svg"
	"github.com/belluikit/bellui/theme"

	"github.com/belluikit/glfw"
)

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework AppKit

#include <stdlib.h>
#include <AppKit/AppKit.h>

// Using void* as type for pointers is a workaround. See https://github.com/golang/go/issues/12065.
void        assignDarwinSubmenu(const void*, const void*);
void        completeDarwinMenu(void* menu, bool prepend);
void        createBelluiMainMenu(const char* appName);
void        ensureRegularActivationPolicy(void);
const void* createDarwinMenu(const char* label);
const void* darwinAppMenu();
void        getTextColorRGBA(int* r, int* g, int* b, int* a);
const void* insertDarwinMenuItem(const void* menu, const char* label, const char* keyEquivalent, unsigned int keyEquivalentModifierMask, int id, int index, bool isSeparator, const void *imageData, unsigned int imageDataLength);
int         menuFontSize();
void        resetDarwinMenu();

// Used for tests.
const void*   test_darwinMainMenu();
const void*   test_NSMenu_itemAtIndex(const void*, NSInteger);
NSInteger     test_NSMenu_numberOfItems(const void*);
void          test_NSMenu_performActionForItemAtIndex(const void*, NSInteger);
void          test_NSMenu_removeItemAtIndex(const void* m, NSInteger i);
const char*   test_NSMenu_title(const void*);
bool          test_NSMenuItem_isSeparatorItem(const void*);
const char*   test_NSMenuItem_keyEquivalent(const void*);
unsigned long test_NSMenuItem_keyEquivalentModifierMask(const void*);
const void*   test_NSMenuItem_submenu(const void*);
const char*   test_NSMenuItem_title(const void*);
*/
import "C"

type menuCallbacks struct {
	action  func()
	enabled func() bool
	checked func() bool
}

var (
	callbacks   []*menuCallbacks
	ecb         func(string)
	specialKeys = map[event.KeyName]string{
		event.KeyBackspace: "\x08",
		event.KeyDelete:    "\x7f",
		event.KeyDown:      "\uf701",
		event.KeyEnd:       "\uf72b",
		event.KeyEnter:     "\x03",
		event.KeyEscape:    "\x1b",
		event.KeyF10:       "\uf70d",
		event.KeyF11:       "\uf70e",
		event.KeyF12:       "\uf70f",
		event.KeyF1:        "\uf704",
		event.KeyF2:        "\uf705",
		event.KeyF3:        "\uf706",
		event.KeyF4:        "\uf707",
		event.KeyF5:        "\uf708",
		event.KeyF6:        "\uf709",
		event.KeyF7:        "\uf70a",
		event.KeyF8:        "\uf70b",
		event.KeyF9:        "\uf70c",
		event.KeyHome:      "\uf729",
		event.KeyInsert:    "\uf727",
		event.KeyLeft:      "\uf702",
		event.KeyPageDown:  "\uf72d",
		event.KeyPageUp:    "\uf72c",
		event.KeyReturn:    "\n",
		event.KeyRight:     "\uf703",
		event.KeySpace:     " ",
		event.KeyTab:       "\t",
		event.KeyUp:        "\uf700",
	}
)

// menuContainsItems reports whether a menu has at least one non-separator item.
// An empty (or separators-only) bellui.Menu must never be handed to AppKit as a
// native NSMenu — a stray empty submenu in the key-equivalent routing path is one
// of the concrete triggers of the #811 use-after-free.
func menuContainsItems(menu *bellui.Menu) bool {
	for _, item := range menu.Items {
		if !item.IsSeparator {
			return true
		}
	}
	return false
}

func addNativeMenu(w *window, menu *bellui.Menu, nextItemID int, prepend bool) int {
	menu, nextItemID = handleSpecialItems(w, menu, nextItemID, true)

	if !menuContainsItems(menu) {
		return nextItemID
	}

	nsMenu, nextItemID := createNativeMenu(w, menu, nextItemID)
	C.completeDarwinMenu(nsMenu, C.bool(prepend))
	return nextItemID
}

func addNativeSubmenu(w *window, nsParentMenuItem unsafe.Pointer, menu *bellui.Menu, nextItemID int) int {
	nsMenu, nextItemID := createNativeMenu(w, menu, nextItemID)
	C.assignDarwinSubmenu(nsParentMenuItem, nsMenu)
	return nextItemID
}

func clearNativeMenu() {
	C.resetDarwinMenu()
}

func createNativeMenu(w *window, menu *bellui.Menu, nextItemID int) (unsafe.Pointer, int) {
	cLabel := C.CString(menu.Label)
	nsMenu := C.createDarwinMenu(cLabel)
	C.free(unsafe.Pointer(cLabel))
	for _, item := range menu.Items {
		nsMenuItem := insertNativeMenuItem(nsMenu, item, nextItemID, -1)
		nextItemID = registerCallback(w, item, nextItemID)
		if item.ChildMenu != nil && menuContainsItems(item.ChildMenu) {
			nextItemID = addNativeSubmenu(w, nsMenuItem, item.ChildMenu, nextItemID)
		}
	}
	return nsMenu, nextItemID
}

//export exceptionCallback
func exceptionCallback(e *C.char) {
	msg := C.GoString(e)
	if ecb == nil {
		panic("unhandled Obj-C exception: " + msg)
	}
	ecb(msg)
}

func handleSpecialItems(w *window, menu *bellui.Menu, nextItemID int, addSeparator bool) (*bellui.Menu, int) {
	menu = bellui.NewMenu(menu.Label, menu.Items...) // copy so we can manipulate
	for i := 0; i < len(menu.Items); i++ {
		item := menu.Items[i]
		switch item.Label {
		case "About", "Settings", "Settings…", "Preferences", "Preferences…":
			items := make([]*bellui.MenuItem, 0, len(menu.Items)-1)
			items = append(items, menu.Items[:i]...)
			items = append(items, menu.Items[i+1:]...)
			menu, nextItemID = handleSpecialItems(w, bellui.NewMenu(menu.Label, items...), nextItemID, false)
			i--

			insertNativeMenuItem(C.darwinAppMenu(), item, nextItemID, 1)
			if addSeparator && item.Label != "About" {
				cEmptyLabel := C.CString("")
				cEmptyKey := C.CString("")
				C.insertDarwinMenuItem(
					C.darwinAppMenu(),
					cEmptyLabel,
					cEmptyKey,
					C.uint(0),
					C.int(nextItemID),
					C.int(1),
					C.bool(true),
					unsafe.Pointer(nil),
					C.uint(0),
				)
				C.free(unsafe.Pointer(cEmptyLabel))
				C.free(unsafe.Pointer(cEmptyKey))
			}
			nextItemID = registerCallback(w, item, nextItemID)
		}
	}
	return menu, nextItemID
}

// TODO: theme change support, see NSSystemColorsDidChangeNotification
func insertNativeMenuItem(nsMenu unsafe.Pointer, item *bellui.MenuItem, nextItemID, index int) unsafe.Pointer {
	var imgData unsafe.Pointer
	var imgDataLength uint
	if item.Icon != nil {
		if svg.IsResourceSVG(item.Icon) {
			rsc := item.Icon
			if _, isThemed := rsc.(*theme.ThemedResource); isThemed {
				var r, g, b, a C.int
				C.getTextColorRGBA(&r, &g, &b, &a)
				content, err := svg.Colorize(rsc.Content(), color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)})
				if err != nil {
					bellui.LogError("", err)
				}
				rsc = &bellui.StaticResource{
					StaticName:    rsc.Name(),
					StaticContent: content,
				}
			}
			size := int(C.menuFontSize())
			img := painter.PaintImage(&canvas.Image{Resource: rsc}, nil, size, size)
			var buf bytes.Buffer
			if err := png.Encode(&buf, img); err != nil {
				bellui.LogError("failed to render menu icon", err)
			} else {
				imgData = unsafe.Pointer(&buf.Bytes()[0])
				imgDataLength = uint(buf.Len())
			}
		} else {
			imgData = unsafe.Pointer(&item.Icon.Content()[0])
			imgDataLength = uint(len(item.Icon.Content()))
		}
	}
	cLabel := C.CString(item.Label)
	cKey := C.CString(keyEquivalent(item))
	ret := C.insertDarwinMenuItem(
		nsMenu,
		cLabel,
		cKey,
		C.uint(keyEquivalentModifierMask(item)),
		C.int(nextItemID),
		C.int(index),
		C.bool(item.IsSeparator),
		imgData,
		C.uint(imgDataLength),
	)
	C.free(unsafe.Pointer(cLabel))
	C.free(unsafe.Pointer(cKey))
	return ret
}

func keyEquivalent(item *bellui.MenuItem) (key string) {
	if s, ok := item.Shortcut.(bellui.KeyboardShortcut); ok {
		if key = specialKeys[s.Key()]; key == "" {
			if len(s.Key()) > 1 {
				bellui.LogError(fmt.Sprintf("unsupported key “%s” for menu shortcut", s.Key()), nil)
			}
			key = strings.ToLower(string(s.Key()))
		}
	}
	return key
}

func keyEquivalentModifierMask(item *bellui.MenuItem) (mask uint) {
	if s, ok := item.Shortcut.(bellui.KeyboardShortcut); ok {
		if (s.Mod() & event.KeyModifierShift) != 0 {
			mask |= 1 << 17 // NSEventModifierFlagShift
		}
		if (s.Mod() & event.KeyModifierAlt) != 0 {
			mask |= 1 << 19 // NSEventModifierFlagOption
		}
		if (s.Mod() & event.KeyModifierControl) != 0 {
			mask |= 1 << 18 // NSEventModifierFlagControl
		}
		if (s.Mod() & event.KeyModifierSuper) != 0 {
			mask |= 1 << 20 // NSEventModifierFlagCommand
		}
	}
	return mask
}

func registerCallback(w *window, item *bellui.MenuItem, nextItemID int) int {
	if !item.IsSeparator {
		callbacks = append(callbacks, &menuCallbacks{
			action: func() {
				if item.Action != nil {
					item.Action()
				}
			},
			enabled: func() bool {
				return !item.Disabled
			},
			checked: func() bool {
				return item.Checked
			},
		})
		nextItemID++
	}
	return nextItemID
}

func setExceptionCallback(cb func(string)) {
	ecb = cb
}

//export menuCallback
func menuCallback(id int) {
	callbacks[id].action()
}

//export menuEnabled
func menuEnabled(id int) bool {
	return callbacks[id].enabled()
}

//export menuChecked
func menuChecked(id int) bool {
	return callbacks[id].checked()
}

// disableGLFWMenubar tells GLFW not to build its own basic menu bar / Services
// menu when AppKit initialises during glfw.Init(). bellui constructs and owns the
// native main menu itself (createBelluiMainMenu), so GLFW's orphaned scaffolding —
// the dangling native reference behind the macOS-26 use-after-free (#831) — never
// exists. Must be called before glfw.Init().
func disableGLFWMenubar() {
	glfw.InitHint(glfw.CocoaMenubar, glfw.False)
}

// ensureRegularActivationPolicy makes an unbundled process a regular foreground
// UI app so its windows can become key and take keyboard focus. GLFW did this
// during glfw.Init() only when its Cocoa menu bar was enabled, and
// disableGLFWMenubar turns that off (#831) — which also dropped the promotion,
// leaving terminal-launched apps as background processes that route typed keys
// to the launching terminal. Since bellui owns the menu bar now, it owns this too.
// Must run after glfw.Init() (AppKit initialised), on the main goroutine. No-op
// for a bundled .app (already Regular). See the C comment for the full chain.
func ensureRegularActivationPolicy() {
	C.ensureRegularActivationPolicy()
}

// belluiMainMenuAppName returns the app name used for the standard app-menu items
// (About/Hide/Quit …): the app metadata name, falling back (in C) to the process
// name when empty.
func belluiMainMenuAppName() string {
	if app := bellui.CurrentApp(); app != nil {
		return app.Metadata().Name
	}
	return ""
}

// ensureBelluiMainMenu builds bellui's native main menu bar — the app-named submenu
// (About/Services/Hide/Show All/Quit) plus the standard Window menu — replacing
// what GLFW's createMenuBar() used to build. It is idempotent (the bar is created
// exactly once) and retains the scaffolding for the whole process lifetime, so it
// is never freed underfoot (#831). Safe to call on the main goroutine only.
func ensureBelluiMainMenu() {
	cName := C.CString(belluiMainMenuAppName())
	C.createBelluiMainMenu(cName)
	C.free(unsafe.Pointer(cName))
}

func setupNativeMenu(w *window, main *bellui.MainMenu) {
	// GLFW no longer builds the menu bar (disableGLFWMenubar); make sure bellui's
	// own bar + app submenu exist before we mutate them via darwinAppMenu(). No-op
	// after the first call.
	ensureBelluiMainMenu()
	clearNativeMenu()
	nextItemID := 0
	callbacks = []*menuCallbacks{}
	var helpMenu *bellui.Menu
	for i := len(main.Items) - 1; i >= 0; i-- {
		menu := main.Items[i]
		if menu.Label == "Help" {
			helpMenu = menu
			continue
		}
		nextItemID = addNativeMenu(w, menu, nextItemID, true)
	}
	if helpMenu != nil {
		addNativeMenu(w, helpMenu, nextItemID, false)
	}
}

//
// Test support methods
// These are needed because CGo is not supported inside test files.
//

func testDarwinMainMenu() unsafe.Pointer {
	return C.test_darwinMainMenu()
}

func testNSMenuItemAtIndex(m unsafe.Pointer, i int) unsafe.Pointer {
	return C.test_NSMenu_itemAtIndex(m, C.long(i))
}

func testNSMenuNumberOfItems(m unsafe.Pointer) int {
	return int(C.test_NSMenu_numberOfItems(m))
}

func testNSMenuPerformActionForItemAtIndex(m unsafe.Pointer, i int) {
	C.test_NSMenu_performActionForItemAtIndex(m, C.long(i))
}

func testNSMenuRemoveItemAtIndex(m unsafe.Pointer, i int) {
	C.test_NSMenu_removeItemAtIndex(m, C.long(i))
}

func testNSMenuTitle(m unsafe.Pointer) string {
	return C.GoString(C.test_NSMenu_title(m))
}

func testNSMenuItemIsSeparatorItem(i unsafe.Pointer) bool {
	return bool(C.test_NSMenuItem_isSeparatorItem(i))
}

func testNSMenuItemKeyEquivalent(i unsafe.Pointer) string {
	return C.GoString(C.test_NSMenuItem_keyEquivalent(i))
}

func testNSMenuItemKeyEquivalentModifierMask(i unsafe.Pointer) uint64 {
	return uint64(C.ulong(C.test_NSMenuItem_keyEquivalentModifierMask(i)))
}

func testNSMenuItemSubmenu(i unsafe.Pointer) unsafe.Pointer {
	return C.test_NSMenuItem_submenu(i)
}

func testNSMenuItemTitle(i unsafe.Pointer) string {
	return C.GoString(C.test_NSMenuItem_title(i))
}
