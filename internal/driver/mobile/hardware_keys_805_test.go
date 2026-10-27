//go:build !wasm

package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/internal/driver/mobile/event/key"
)

// The keys a hardware keyboard's navigation and editing keys arrive as reach
// a focused widget that takes keys but no text, each press once. Android
// routes the arrows and Forward-Delete from the shadow EditText into this
// path (#805); Forward-Delete had no key name, so it was dropped here even
// once it arrived.
func TestDispatchKeyEvent_HardwareNavigationKeysReachAKeyTarget(t *testing.T) {
	d := &driver{}
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	rec := newKeyRecorder()
	c.SetContent(rec)
	c.Focus(rec)

	for _, code := range []key.Code{
		key.CodeLeftArrow, key.CodeRightArrow, key.CodeUpArrow, key.CodeDownArrow,
		key.CodeDeleteForward,
	} {
		d.dispatchKeyEvent(c, key.Event{Code: code, Direction: key.DirPress})
		d.dispatchKeyEvent(c, key.Event{Code: code, Direction: key.DirRelease})
	}
	assert.Equal(t, []event.KeyName{
		event.KeyLeft, event.KeyRight, event.KeyUp, event.KeyDown, event.KeyDelete,
	}, rec.keys)
}
