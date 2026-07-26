package bellui

import "errors"

// ErrShareNotSupported is returned by [ShareText] and [ShareFile] (and the
// [SharerApp] methods) when the current platform has no share surface — for
// example a plain desktop window manager, or a build without share support.
var ErrShareNotSupported = errors.New("sharing is not supported on this platform")

// SharerApp is an optional interface that an [App] can implement to hand
// content to the operating system's share UI (the OS-owned chooser that offers
// Messages, Mail, AirDrop, other installed apps, …). It is the "send" half of
// platform intent/share integration.
//
// It is supported on macOS (NSSharingServicePicker), and on mobile
// (ACTION_SEND / UIActivityViewController). Platforms without a share surface
// return [ErrShareNotSupported] rather than silently doing nothing.
//
// Since: 2.19
type SharerApp interface {
	// ShareText offers the given text to the platform share UI.
	ShareText(text string) error

	// ShareFile offers the file identified by uri to the platform share UI.
	// The URI must use the "file" scheme.
	ShareFile(uri URI) error
}

// ShareText offers text to the operating system's share UI via the current app.
// It returns [ErrShareNotSupported] if the platform has no share surface.
func ShareText(text string) error {
	if a := CurrentApp(); a != nil {
		if sharer, ok := a.(SharerApp); ok {
			return sharer.ShareText(text)
		}
	}
	return ErrShareNotSupported
}

// ShareFile offers a file to the operating system's share UI via the current
// app. The URI must use the "file" scheme. It returns [ErrShareNotSupported] if
// the platform has no share surface.
func ShareFile(uri URI) error {
	if a := CurrentApp(); a != nil {
		if sharer, ok := a.(SharerApp); ok {
			return sharer.ShareFile(uri)
		}
	}
	return ErrShareNotSupported
}
