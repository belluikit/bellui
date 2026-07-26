// Package undoredo defines the native undo/redo invocation event used by the
// mobile driver to carry a platform-initiated undo or redo — the iPad soft
// keyboard's undo/redo bar keys, the three-finger swipe gestures or
// shake-to-undo, all of which drive the first responder's NSUndoManager — from
// the platform layer to the focused editor (#1134). The native side holds only
// proxy undo actions (the Flutter FlutterUndoManagerPlugin model): state and
// invocations cross this seam, never the edit history, which stays owned by
// the bellui editor.
package undoredo

// Event reports a native undo (Redo == false) or redo (Redo == true)
// invocation to be applied to the focused editor.
type Event struct {
	Redo bool
}
