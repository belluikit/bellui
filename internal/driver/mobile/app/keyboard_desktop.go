//go:build !android && !ios

package app

// SetKeyboardState is the desktop/host no-op counterpart of the Android IME
// state mirror (#572). The mobile driver package compiles on the host for
// tests and `go vet`, so the symbol must exist on every platform; only Android
// has a shadow EditText to mirror into (iOS drives composition through the
// input view's marked text — see darwin_ios.go).
func SetKeyboardState(text string, selStart, selEnd int) {}

// KeyboardEditorChanged is the desktop/host no-op counterpart of the Android
// editor-change signal (#966); only Android holds cross-push IME session state
// (composing flag, echo filters) that must be reset when the bound editor's
// identity changes.
func KeyboardEditorChanged() {}

// SetKeyboardUndoState is the desktop/host no-op counterpart of the iOS
// NSUndoManager state mirror (#1134); only iOS surfaces native undo/redo UI
// (keyboard bar keys, gestures) driven by the first responder's undo manager.
func SetKeyboardUndoState(canUndo, canRedo bool) {}
