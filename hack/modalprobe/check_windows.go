//go:build windows

package main

import (
	"fmt"
	"syscall"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/dialog"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/widget"
)

// runWindowsCheck drives a same-process verification of the #498/#682 Windows
// native block: it opens a window-hosted modal over w, reads the parent HWND
// (via RunNative) and asserts IsWindowEnabled is FALSE while the dialog is up
// and TRUE again after it closes. Runs over plain SSH (no window-station issue
// — the check is in-process), unlike external enumeration.
func runWindowsCheck(a bellui.App, w bellui.Window) {
	time.Sleep(800 * time.Millisecond) // let the parent map

	var d dialog.Dialog
	bellui.Do(func() {
		d = dialog.NewCustom("modalprobe-dialog-check", "Close", widget.NewLabel("modal"), w)
		if h, ok := d.(interface{ SetWindowHosted(bool) }); ok {
			h.SetWindowHosted(true)
		}
		d.Show()
	})
	time.Sleep(600 * time.Millisecond)

	hwnd := parentHWND(w)
	blockedWhileModal := !isWindowEnabled(hwnd)
	fmt.Printf("CHECK parent HWND=%#x enabled-while-modal=%v (want false)\n", hwnd, isWindowEnabled(hwnd))

	bellui.Do(func() { d.Hide() })
	time.Sleep(600 * time.Millisecond)

	restored := isWindowEnabled(hwnd)
	fmt.Printf("CHECK parent enabled-after-close=%v (want true)\n", restored)

	if blockedWhileModal && restored {
		fmt.Println("RESULT: PASS — parent disabled while modal, re-enabled after close")
	} else {
		fmt.Println("RESULT: FAIL")
	}
	bellui.Do(a.Quit)
}

func parentHWND(w bellui.Window) uintptr {
	var hwnd uintptr
	if nw, ok := w.(driver.NativeWindow); ok {
		nw.RunNative(func(ctx any) {
			if c, ok := ctx.(driver.WindowsWindowContext); ok {
				hwnd = c.HWND
			}
		})
	}
	return hwnd
}

func isWindowEnabled(hwnd uintptr) bool {
	r, _, _ := syscall.NewLazyDLL("user32.dll").NewProc("IsWindowEnabled").Call(hwnd)
	return r != 0
}
