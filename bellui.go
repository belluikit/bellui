// Package bellui describes the objects and components available to any Bellui app.
// These can all be created, manipulated and tested without rendering (for speed).
// Your main package should use the app package to create an application with
// a default driver that will render your UI.
//
// A simple application may look like this:
//
//	package main
//
//	import "github.com/belluikit/bellui/app"
//	import "github.com/belluikit/bellui/container"
//	import "github.com/belluikit/bellui/widget"
//
//	func main() {
//		a := app.New()
//		w := a.NewWindow("Hello")
//
//		hello := widget.NewLabel("Hello Bellui!")
//		w.SetContent(container.NewVBox(
//			hello,
//			widget.NewButton("Hi!", func() {
//				hello.SetText("Welcome :)")
//			}),
//		))
//
//		bellui.ShowAndRun(w)
//	}
//
// # Package Organization
//
// The root bellui package defines application-level interfaces and types.
// Fundamental types are in sub-packages:
//
//   - [github.com/belluikit/bellui/geo] — Geometry: Size, Position, Delta
//   - [github.com/belluikit/bellui/event] — Input events and behavioral interfaces (Tappable, Draggable, Focusable)
//   - [github.com/belluikit/bellui/core] — Core interfaces: CanvasObject, Widget, Canvas, Layout
//   - [github.com/belluikit/bellui/app] — Application lifecycle and preferences
//   - [github.com/belluikit/bellui/canvas] — Drawing primitives (text, shapes, images)
//   - [github.com/belluikit/bellui/container] — Layout containers (VBox, HBox, Border, Split, Grid)
//   - [github.com/belluikit/bellui/widget] — UI widgets (Button, Entry, Label, List, etc.)
//   - [github.com/belluikit/bellui/dialog] — Modal dialogs
//   - [github.com/belluikit/bellui/layout] — Layout managers
//   - [github.com/belluikit/bellui/theme] — Theming system, colors, icons, typography
//   - [github.com/belluikit/bellui/data/binding] — Type-safe data binding
//   - [github.com/belluikit/bellui/storage] — URI handling and file system access
//   - [github.com/belluikit/bellui/test] — Test utilities and assertions
//
// # This Package
//
// Interfaces: [App], [Window], [Driver], [Theme]
//
// Shortcuts: [Shortcutable], [KeyboardShortcut], [ShortcutHandler]
//
// Text: [TextStyle], [TextAlign], [TextWrap], [TextTruncation]
//
// Threading: [Do], [DoAndWait] — schedule work on the main goroutine.
// Most widget methods (SetText, Enable, etc.) handle this internally.
package bellui
