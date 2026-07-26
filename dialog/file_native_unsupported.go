//go:build !windows && !android && !ios && !wasm && !js && !linux && !openbsd && !freebsd && !netbsd && !darwin

package dialog

// This is the fallback for platforms without a native file dialog backend
// wired up (Linux/BSD and macOS have one; this covers the rest, e.g.
// Solaris/Plan 9). The canvas-drawn dialog is always used, so no override
// fires.

func fileOpenOSOverride(*FileDialog) bool {
	return false
}

func fileSaveOSOverride(*FileDialog) bool {
	return false
}
