// Package desktop provides desktop specific driver functionality.
package desktop

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
)

// Driver represents the extended capabilities of a desktop driver
type Driver interface {
	// Create a new borderless window that is centered on screen
	CreateSplashWindow() bellui.Window

	// Gets the set of key modifiers that are currently active
	//
	// Since: 2.4
	CurrentKeyModifiers() event.KeyModifier

	// HasSecondaryDisplay returns true if there are more than one screens available on this computer.
	// This is commonly used alongside the desktop [Window.RequestFullScreenSecondary] call.
	//
	// Since: 1.0
	HasSecondaryDisplay() bool
}

// KeyModifierDriver describes drivers that track the window-wide keyboard
// modifier state — updated from every key event (and on the web target from
// pointer events too), regardless of widget focus — and can notify when it
// changes. The GLFW driver implements it on both desktop and web targets, so
// unfocused-widget affordances (e.g. a modifier-hover link highlight) work in
// the browser as well; assert this instead of [Driver] when the splash-window
// capability is not needed.
//
// Since: 1.0
type KeyModifierDriver interface {
	// CurrentKeyModifiers returns the set of key modifiers currently active.
	CurrentKeyModifiers() event.KeyModifier

	// AddKeyModifierListener registers fn to be called on the main goroutine
	// whenever the set of active key modifiers changes, regardless of widget
	// focus — including a modifier pressed while the pointer rests still, so
	// hosts need no hover-move polling. It returns a remove function;
	// multiple listeners may be registered. Call on the main goroutine.
	AddKeyModifierListener(fn func(event.KeyModifier)) (remove func())
}

// GlobalShortcutApp describes a desktop driver that can register system-wide
// keyboard shortcuts — hotkeys that fire while the app is unfocused or
// minimized (push-to-talk, a screenshot tool, a quick-capture note). Unlike
// [bellui.Canvas.AddShortcut] these are seen even when no bellui window has focus.
// Reach it from the running app via
//
//	bellui.CurrentApp().Driver().(desktop.GlobalShortcutApp)
//
// Global hotkeys are a contended, OS-mediated resource, so registration returns
// an error: the combination may already be owned by another application, or the
// platform may not support it. On Linux Wayland there is no client API by
// design — registration goes through the org.freedesktop.portal.GlobalShortcuts
// portal, which may prompt the user for consent and is therefore asynchronous;
// a nil error there means the request was accepted, not that the binding is
// necessarily live yet.
//
// Since: 1.0
type GlobalShortcutApp interface {
	// RegisterGlobalShortcut binds a system-wide shortcut. handler is invoked on
	// the main goroutine each time the shortcut fires. It returns an error if the
	// shortcut could not be registered (already owned by another app, an
	// unmappable key, or an unsupported platform). Registering the same shortcut
	// again replaces its handler.
	RegisterGlobalShortcut(shortcut *CustomShortcut, handler func()) error

	// UnregisterGlobalShortcut removes a previously registered global shortcut.
	// Unknown shortcuts are ignored.
	UnregisterGlobalShortcut(shortcut *CustomShortcut)
}
