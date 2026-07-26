package common

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
)

func TestBuiltinKeyboardShortcut(t *testing.T) {
	clip := test.NewClipboard()
	name := func(k event.KeyName) string {
		sc := BuiltinKeyboardShortcut(k, clip)
		if sc == nil {
			return ""
		}
		return sc.ShortcutName()
	}

	assert.Equal(t, "SelectAll", name(event.KeyA))
	assert.Equal(t, "Copy", name(event.KeyC))
	assert.Equal(t, "Copy", name(event.KeyInsert))
	assert.Equal(t, "Cut", name(event.KeyX))
	assert.Equal(t, "Paste", name(event.KeyV))
	assert.Equal(t, "Undo", name(event.KeyZ))
	assert.Equal(t, "Redo", name(event.KeyY))

	// Non-shortcut keys map to nil.
	assert.Nil(t, BuiltinKeyboardShortcut(event.KeyB, clip))
	assert.Nil(t, BuiltinKeyboardShortcut(event.KeyReturn, clip))
	assert.Nil(t, BuiltinKeyboardShortcut(event.KeyUnknown, clip))

	// Clipboard shortcuts carry the passed clipboard.
	if sc, ok := BuiltinKeyboardShortcut(event.KeyC, clip).(*bellui.ShortcutCopy); assert.True(t, ok) {
		assert.Equal(t, clip, sc.Clipboard)
	}
}
