package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/internal/driver/mobile/event/key"
)

type shortcutRecorder struct {
	belluicanvas.Rectangle
	shortcuts []core.Shortcut
	keys      []*event.KeyEvent
}

func (r *shortcutRecorder) FocusGained()                  {}
func (r *shortcutRecorder) FocusLost()                    {}
func (r *shortcutRecorder) TypedRune(rune)                {}
func (r *shortcutRecorder) TypedKey(ev *event.KeyEvent)   { r.keys = append(r.keys, ev) }
func (r *shortcutRecorder) TypedShortcut(s core.Shortcut) { r.shortcuts = append(r.shortcuts, s) }

// Hardware-keyboard editing shortcuts forwarded from the Android IME (pre-rebrand tracker #1077)
// reach the focused Shortcutable as the matching bellui.Shortcut, and a bare
// Ctrl/Cmd combo does not also fire TypedKey. A Shift-mixed combo is not a
// built-in shortcut and still reaches TypedKey (pre-rebrand tracker #1042).
func Test_mobileDriver_HardwareShortcut(t *testing.T) {
	d := &driver{}
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	rec := &shortcutRecorder{}
	c.SetContent(rec)
	c.Focus(rec)

	d.typeDownCanvas(c, 0, key.CodeA, key.ModControl)
	require.Len(t, rec.shortcuts, 1)
	assert.Equal(t, "SelectAll", rec.shortcuts[0].ShortcutName())
	assert.Empty(t, rec.keys, "a shortcut combo must not also fire TypedKey")

	d.typeDownCanvas(c, 0, key.CodeC, key.ModControl)
	d.typeDownCanvas(c, 0, key.CodeV, key.ModControl)
	d.typeDownCanvas(c, 0, key.CodeX, key.ModControl)
	require.Len(t, rec.shortcuts, 4)
	assert.Equal(t, "Copy", rec.shortcuts[1].ShortcutName())
	assert.Equal(t, "Paste", rec.shortcuts[2].ShortcutName())
	assert.Equal(t, "Cut", rec.shortcuts[3].ShortcutName())

	// Cmd/Meta is a shortcut modifier too (future iOS hardware keyboards).
	d.typeDownCanvas(c, 0, key.CodeA, key.ModMeta)
	require.Len(t, rec.shortcuts, 5)
	assert.Equal(t, "SelectAll", rec.shortcuts[4].ShortcutName())

	// Shift+Ctrl+A is not a built-in shortcut → reaches TypedKey instead.
	d.typeDownCanvas(c, 0, key.CodeA, key.ModControl|key.ModShift)
	assert.Len(t, rec.shortcuts, 5, "no new shortcut for a Shift-mixed combo")
	require.Len(t, rec.keys, 1)
	assert.Equal(t, event.KeyA, rec.keys[0].Name)

	// A bare key is unaffected.
	d.typeDownCanvas(c, 'b', key.CodeB, 0)
	assert.Len(t, rec.shortcuts, 5)
}
