//go:build windows

package main

import "golang.org/x/sys/windows"

// synthHotKey injects a Ctrl+Alt+K key chord onto the current desktop via
// keybd_event so the probe can drive itself. The process must be running on the
// interactive desktop for the input to be delivered.
func synthHotKey() {
	user32 := windows.NewLazySystemDLL("user32.dll")
	keybdEvent := user32.NewProc("keybd_event")

	const (
		vkControl = 0x11
		vkMenu    = 0x12 // Alt
		vkK       = 0x4B
		keyUp     = 0x0002 // KEYEVENTF_KEYUP
	)
	press := func(vk uintptr) { keybdEvent.Call(vk, 0, 0, 0) }
	release := func(vk uintptr) { keybdEvent.Call(vk, 0, keyUp, 0) }

	press(vkControl)
	press(vkMenu)
	press(vkK)
	release(vkK)
	release(vkMenu)
	release(vkControl)
}
