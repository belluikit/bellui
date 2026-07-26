package cell

import "github.com/belluikit/bellui/core"

// EditorFactory creates and manages inline cell editors.
type EditorFactory interface {
	// CreateEditor returns a new editor CanvasObject.
	CreateEditor() core.CanvasObject
	// BeginEdit prepares the editor with the current cell value.
	BeginEdit(editor core.CanvasObject, value any, params Params)
	// EndEdit extracts the edited value. Returns (newValue, cancelled).
	EndEdit(editor core.CanvasObject) (any, bool)
}
