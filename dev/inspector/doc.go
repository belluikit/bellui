// Package inspector provides an opt-in, read-only runtime widget-tree
// inspector for debugging Bellui layouts — the human-facing counterpart to the
// accessibility introspection layer.
//
// Call [Show] with the window you want to inspect:
//
//	func main() {
//	    a := app.New()
//	    w := a.NewWindow("My App")
//	    w.SetContent(buildUI())
//
//	    if os.Getenv("BELLUI_INSPECT") == "1" {
//	        inspector.Show(w) // opens the inspector in its own window
//	    }
//	    bellui.ShowAndRun(w)
//	}
//
// The inspector window shows:
//
//   - a tree pane: the live object tree of the target (the same walk used by
//     the painter and hit-test, descending both containers and widget-renderer
//     internals), rebuilt on demand via the Refresh button — there is no
//     ticker, so a closed inspector costs nothing;
//   - a detail pane: Position/Size/MinSize (dp), absolute bounds, visibility,
//     clip bounds, scroll offset, direction-scope and theme-override presence,
//     and the accessibility snapshot (role/label/value) of the selected node;
//   - a bounds highlight drawn as an overlay on the target when a node is
//     selected;
//   - a pick mode (the Pick button): hover the target to preview the object
//     under the cursor, click to select it in the tree, Escape to cancel.
//
// Everything the inspector adds to the target is removed when the inspector
// window closes.
//
// This package is heavy relative to core and is only linked into a binary that
// imports it — mirroring theme/cjk. Phase 1 is desktop-only (its own window);
// mobile and wasm are single-window and out of scope.
package inspector
