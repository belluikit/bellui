package common

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
)

// BuiltinKeyboardShortcut returns the standard editing shortcut bound to a key
// pressed with the platform shortcut modifier held (Control, or Super on
// macOS), or nil when the key is not one of the built-ins. Clipboard shortcuts
// (Copy/Cut/Paste) are given clip. Shared by the desktop (glfw) and mobile
// drivers so both map the same keys to the same shortcuts (#1077).
func BuiltinKeyboardShortcut(key event.KeyName, clip bellui.Clipboard) core.Shortcut {
	switch key {
	case event.KeyZ: // undo
		return &bellui.ShortcutUndo{}
	case event.KeyY: // redo
		return &bellui.ShortcutRedo{}
	case event.KeyV: // paste
		return &bellui.ShortcutPaste{Clipboard: clip}
	case event.KeyC, event.KeyInsert: // copy
		return &bellui.ShortcutCopy{Clipboard: clip}
	case event.KeyX: // cut
		return &bellui.ShortcutCut{Clipboard: clip}
	case event.KeyA: // select all
		return &bellui.ShortcutSelectAll{}
	}
	return nil
}
