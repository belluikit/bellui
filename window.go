package bellui

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// Window describes a user interface window. Depending on the platform an app
// may have many windows or just the one.
//
// Multi-window support is platform-dependent (see [App.NewWindow]): desktop
// gives each window a real, independent OS window shown concurrently; web
// overlays additional windows as draggable in-canvas windows on the single
// browser canvas; mobile stacks windows and shows only the topmost visible one
// at a time.
type Window interface {
	// Title returns the current window title.
	// This is typically displayed in the window decorations.
	Title() string
	// SetTitle updates the current title of the window.
	SetTitle(string)

	// FullScreen returns whether or not this window is currently full screen.
	FullScreen() bool
	// SetFullScreen changes the requested fullScreen property
	// true for a fullScreen window and false to unset this.
	SetFullScreen(bool)

	// Resize this window to the requested content size.
	// The result may not be exactly as desired due to various desktop or
	// platform constraints.
	Resize(geo.Size)

	// RequestFocus attempts to raise and focus this window.
	// This should only be called when you are sure the user would want this window
	// to steal focus from any current focused window.
	RequestFocus()

	// FixedSize returns whether or not this window should disable resizing.
	FixedSize() bool
	// SetFixedSize sets a hint that states whether the window should be a fixed
	// size or allow resizing.
	SetFixedSize(bool)

	// CenterOnScreen places a window at the center of the monitor
	// the Window object is currently positioned on.
	CenterOnScreen()

	// Padded, normally true, states whether the window should have inner
	// padding so that components do not touch the window edge.
	Padded() bool
	// SetPadded allows applications to specify that a window should have
	// no inner padding. Useful for fullscreen or graphic based applications.
	SetPadded(bool)

	// SelectableMode returns the window-level selectable mode.
	// The default value is [SelectablePerWidget].
	//
	// Since: 2.8
	SelectableMode() SelectableMode
	// SetSelectableMode sets the window-level selectable mode which overrides
	// per-widget Selectable properties for display widgets (e.g. Label).
	// [SelectableAll] forces all display widgets to be selectable,
	// [SelectableNone] forces them to be non-selectable, and
	// [SelectablePerWidget] (the default) defers to each widget's own setting.
	//
	// Since: 2.8
	SetSelectableMode(SelectableMode)

	// Icon returns the window icon, this is used in various ways
	// depending on operating system.
	// Most commonly this is displayed on the window border or task switcher.
	Icon() Resource

	// SetIcon sets the icon resource used for this window.
	// If none is set should return the application icon.
	SetIcon(Resource)

	// MainMenu gets the content of the window's top level menu.
	MainMenu() *MainMenu

	// SetMainMenu adds a top level menu to this window.
	// The way this is rendered will depend on the loaded driver.
	SetMainMenu(*MainMenu)

	// SetOnClosed sets a function that runs when the window is closed.
	SetOnClosed(func())

	// SetCloseIntercept sets a function that runs instead of closing if defined.
	// [Window.Close] should be called explicitly in the interceptor to close the window.
	//
	// Since: 1.4
	SetCloseIntercept(func())

	// Show the window on screen.
	Show()
	// Hide the window from the user.
	// This will not destroy the window or cause the app to exit.
	Hide()
	// Close the window.
	// If it is the "master" window the app will Quit.
	// If it is the only open window and no menu is set via [desktop.App]
	// SetSystemTrayMenu the app will also Quit.
	Close()

	// Content returns the content of this window.
	Content() core.CanvasObject
	// SetContent sets the content of this window.
	SetContent(core.CanvasObject)
	// Canvas returns the canvas context to render in the window.
	// This can be useful to set a key handler for the window, for example.
	Canvas() core.Canvas
}

// MasterWindow is an optional interface that a [Window] can implement
// to indicate that closing it should exit the app.
type MasterWindow interface {
	SetMaster()
}

// DroppableWindow is an optional interface that a [Window] can implement
// to receive dropped items.
type DroppableWindow interface {
	// SetOnDropped allows setting a window-wide callback to receive dropped items.
	SetOnDropped(func(geo.Position, []URI))
}

// DropEvent carries the items dropped onto a [Droppable] target along with the
// drop position.
//
// Since: 2.19
type DropEvent struct {
	// AbsolutePosition is the drop position relative to the window canvas.
	AbsolutePosition geo.Position
	// Position is the drop position relative to the receiving object.
	Position geo.Position
	// URIs are the dropped items. On desktop these are file URIs (the platform
	// backend delivers dropped file paths).
	URIs []URI
}

// Droppable is an optional interface that a [CanvasObject] can implement to
// receive items dropped onto it from outside the application — for example
// files dragged from the desktop file manager.
//
// The driver routes a drop to the deepest Droppable under the pointer; a drop
// not claimed by any Droppable falls back to the window-wide
// [DroppableWindow.SetOnDropped] callback, which therefore keeps working
// unchanged.
//
// A Droppable may additionally implement [DropHoverable] to receive
// drag-enter/over/leave hover feedback while a drag is in progress over it.
//
// Since: 2.19
type Droppable interface {
	// Dropped is called when one or more items are dropped onto this object.
	Dropped(*DropEvent)
}

// DragHoverEvent carries the pointer position while a drag from outside the
// application is in progress over a [DropHoverable].
//
// It intentionally carries only the position, not the dragged items: the
// desktop drag protocols do not make the payload available before the drop, so
// the items arrive only with the final [DropEvent].
//
// Since: 2.19
type DragHoverEvent struct {
	// AbsolutePosition is the pointer position relative to the window canvas.
	AbsolutePosition geo.Position
	// Position is the pointer position relative to the receiving object.
	Position geo.Position
}

// DropHoverable is an optional companion to [Droppable] that a [CanvasObject]
// can implement to receive hover feedback while an external drag is in progress
// over it, before the drop — for example to highlight a drop zone.
//
// The driver routes hover to the deepest DropHoverable under the pointer:
// [DropHoverable.DragEnter] when the drag first reaches the object,
// [DropHoverable.DragOver] as it moves within, and [DropHoverable.DragLeave]
// when it moves off the object, leaves the window, or the drag ends without a
// drop on this object. A drop is still delivered through [Droppable.Dropped]
// (after which DragLeave also fires so the object can clear its highlight).
//
// Drag-motion is delivered only on desktop backends whose native drag protocol
// exposes it (X11 XDND, macOS and Windows OLE); where it is unavailable the object still
// receives [Droppable.Dropped], just without the hover phases.
//
// Since: 2.19
type DropHoverable interface {
	Droppable

	// DragEnter is called when an external drag first reaches this object.
	DragEnter(*DragHoverEvent)
	// DragOver is called as an external drag moves within this object.
	DragOver(*DragHoverEvent)
	// DragLeave is called when an external drag moves off this object, leaves
	// the window, or ends.
	DragLeave()
}

// ShowAndRun shows a window and starts the application event loop.
// This is a convenience function equivalent to calling w.Show() followed by app.Run().
func ShowAndRun(w Window) {
	w.Show()
	CurrentApp().Run()
}
