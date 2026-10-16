package bellui

import (
	"sync"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
)

// Shortcutable describes any [core.CanvasObject] that can respond to shortcut commands.
type Shortcutable interface {
	TypedShortcut(core.Shortcut)
}

// ShortcutHandler is a default implementation of the shortcut handler
// for [core.CanvasObject].
type ShortcutHandler struct {
	entry sync.Map // map[string]func(core.Shortcut)
}

// TypedShortcut handle the registered shortcut
func (sh *ShortcutHandler) TypedShortcut(shortcut core.Shortcut) {
	val, ok := sh.entry.Load(shortcut.ShortcutName())
	if !ok {
		return
	}

	f := val.(func(core.Shortcut))
	f(shortcut)
}

// AddShortcut register a handler to be executed when the shortcut action is triggered
func (sh *ShortcutHandler) AddShortcut(shortcut core.Shortcut, handler func(shortcut core.Shortcut)) {
	sh.entry.Store(shortcut.ShortcutName(), handler)
}

// RemoveShortcut removes a registered shortcut
func (sh *ShortcutHandler) RemoveShortcut(shortcut core.Shortcut) {
	sh.entry.Delete(shortcut.ShortcutName())
}

// KeyboardShortcut describes a shortcut meant to be triggered by a keyboard action.
type KeyboardShortcut interface {
	core.Shortcut
	Key() event.KeyName
	Mod() event.KeyModifier
}

var _ KeyboardShortcut = (*ShortcutPaste)(nil)

// ShortcutPaste describes a shortcut paste action.
type ShortcutPaste struct {
	Clipboard Clipboard
}

// Key returns the [event.KeyName] for this shortcut.
func (se *ShortcutPaste) Key() event.KeyName {
	return event.KeyV
}

// Mod returns the [event.KeyModifier] for this shortcut.
func (se *ShortcutPaste) Mod() event.KeyModifier {
	return event.KeyModifierShortcutDefault
}

// ShortcutName returns the shortcut name
func (se *ShortcutPaste) ShortcutName() string {
	return "Paste"
}

var _ KeyboardShortcut = (*ShortcutCopy)(nil)

// ShortcutCopy describes a shortcut copy action.
type ShortcutCopy struct {
	Clipboard Clipboard
}

// Key returns the [event.KeyName] for this shortcut.
func (se *ShortcutCopy) Key() event.KeyName {
	return event.KeyC
}

// Mod returns the [event.KeyModifier] for this shortcut.
func (se *ShortcutCopy) Mod() event.KeyModifier {
	return event.KeyModifierShortcutDefault
}

// ShortcutName returns the shortcut name
func (se *ShortcutCopy) ShortcutName() string {
	return "Copy"
}

var _ KeyboardShortcut = (*ShortcutCut)(nil)

// ShortcutCut describes a shortcut cut action.
type ShortcutCut struct {
	Clipboard Clipboard
}

// Key returns the [event.KeyName] for this shortcut.
func (se *ShortcutCut) Key() event.KeyName {
	return event.KeyX
}

// Mod returns the [event.KeyModifier] for this shortcut.
func (se *ShortcutCut) Mod() event.KeyModifier {
	return event.KeyModifierShortcutDefault
}

// ShortcutName returns the shortcut name
func (se *ShortcutCut) ShortcutName() string {
	return "Cut"
}

var _ KeyboardShortcut = (*ShortcutSelectAll)(nil)

// ShortcutSelectAll describes a shortcut selectAll action.
type ShortcutSelectAll struct{}

// Key returns the [event.KeyName] for this shortcut.
func (se *ShortcutSelectAll) Key() event.KeyName {
	return event.KeyA
}

// Mod returns the [event.KeyModifier] for this shortcut.
func (se *ShortcutSelectAll) Mod() event.KeyModifier {
	return event.KeyModifierShortcutDefault
}

// ShortcutName returns the shortcut name
func (se *ShortcutSelectAll) ShortcutName() string {
	return "SelectAll"
}

var _ KeyboardShortcut = (*ShortcutUndo)(nil)

// ShortcutUndo describes a shortcut undo action.
//
// Since: 1.0
type ShortcutUndo struct{}

// Key returns the [event.KeyName] for this shortcut.
func (se *ShortcutUndo) Key() event.KeyName {
	return event.KeyZ
}

// Mod returns the [event.KeyModifier] for this shortcut.
func (se *ShortcutUndo) Mod() event.KeyModifier {
	return event.KeyModifierShortcutDefault
}

// ShortcutName returns the shortcut name
func (se *ShortcutUndo) ShortcutName() string {
	return "Undo"
}

var _ KeyboardShortcut = (*ShortcutRedo)(nil)

// ShortcutRedo describes a shortcut redo action.
//
// Since: 1.0
type ShortcutRedo struct{}

// Key returns the [event.KeyName] for this shortcut.
func (se *ShortcutRedo) Key() event.KeyName {
	return event.KeyY
}

// Mod returns the [event.KeyModifier] for this shortcut.
func (se *ShortcutRedo) Mod() event.KeyModifier {
	return event.KeyModifierShortcutDefault
}

// ShortcutName returns the shortcut name
func (se *ShortcutRedo) ShortcutName() string {
	return "Redo"
}
