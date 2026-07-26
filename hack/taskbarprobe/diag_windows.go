//go:build windows

package main

import (
	"log"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver"
)

// diagWindow logs the native HWND of the window plus its root-ancestor, owner
// and title, so we can confirm the handle taskbar progress is applied to is the
// real top-level taskbar window (and not a child/owned HWND, which would accept
// SetProgressValue with S_OK yet never render).
func diagWindow(w bellui.Window) {
	native, ok := w.(driver.NativeWindow)
	if !ok {
		log.Println("diag: window is not driver.NativeWindow")
		return
	}
	var hwnd uintptr
	native.RunNative(func(ctx any) {
		if wc, ok := ctx.(driver.WindowsWindowContext); ok {
			hwnd = wc.HWND
		}
	})
	if hwnd == 0 {
		log.Println("diag: HWND == 0")
		return
	}

	user32 := windows.NewLazySystemDLL("user32.dll")
	getAncestor := user32.NewProc("GetAncestor")
	getParent := user32.NewProc("GetParent")
	getWindow := user32.NewProc("GetWindow")
	getText := user32.NewProc("GetWindowTextW")
	getStyle := user32.NewProc("GetWindowLongW")

	const (
		gaRoot        = 2
		gwOwner       = 4
		gwlExStyle    = -20
		wsExAppWindow = 0x00040000
		wsExToolWin   = 0x00000080
	)
	root, _, _ := getAncestor.Call(hwnd, gaRoot)
	parent, _, _ := getParent.Call(hwnd)
	owner, _, _ := getWindow.Call(hwnd, gwOwner)
	exIdx := gwlExStyle // -20; convert via a variable (negative const → uintptr is illegal)
	exStyle, _, _ := getStyle.Call(hwnd, uintptr(exIdx))

	buf := make([]uint16, 256)
	getText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	title := windows.UTF16ToString(buf)

	log.Printf("diag: hwnd=0x%X root=0x%X parent=0x%X owner=0x%X exStyle=0x%X (APPWINDOW=%v TOOLWINDOW=%v) title=%q",
		hwnd, root, parent, owner, exStyle,
		exStyle&wsExAppWindow != 0, exStyle&wsExToolWin != 0, title)
	if root != hwnd {
		log.Printf("diag: WARNING hwnd is not its own root ancestor — taskbar button belongs to 0x%X", root)
	}
}
