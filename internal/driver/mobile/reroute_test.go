//go:build !wasm

package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/internal/driver/mobile/event/key"
	"github.com/belluikit/bellui/internal/textinput"
	"github.com/belluikit/bellui/widget"
)

// The one-writer IME reroute (pre-rebrand tracker #1061) ignores the redundant native-queue echo of
// a hardware key the Android IME already applied through its InputConnection —
// but only for prose fields where Gboard is the writer, never for shortcuts,
// non-text keys, unfocused input, or password/number fields (where the native
// queue is the sole delivery). imeOwnsNativeKey is the platform-independent
// decision core.
func TestIMEOwnsNativeKey(t *testing.T) {
	bound := func(kt mobile.KeyboardType) *driver {
		d := &driver{textInput: textinput.NewSession(keyboardStateBackend{}), keyboardType: kt}
		d.textInput.Bind(widget.NewEntry())
		return d
	}
	printable := key.Event{Rune: 'a', Native: true, Direction: key.DirPress}
	del := key.Event{Code: key.CodeDeleteBackspace, Native: true, Direction: key.DirPress}

	t.Run("printable in prose field is the IME's", func(t *testing.T) {
		assert.True(t, bound(mobile.DefaultKeyboard).imeOwnsNativeKey(printable))
		assert.True(t, bound(mobile.SingleLineKeyboard).imeOwnsNativeKey(printable))
	})
	t.Run("backspace in prose field is the IME's", func(t *testing.T) {
		assert.True(t, bound(mobile.DefaultKeyboard).imeOwnsNativeKey(del))
	})
	t.Run("password and number fields keep the native key", func(t *testing.T) {
		assert.False(t, bound(mobile.PasswordKeyboard).imeOwnsNativeKey(printable))
		assert.False(t, bound(mobile.PasswordKeyboard).imeOwnsNativeKey(del))
		assert.False(t, bound(mobile.NumberKeyboard).imeOwnsNativeKey(printable))
	})
	t.Run("no focused editor keeps the native key", func(t *testing.T) {
		d := &driver{textInput: textinput.NewSession(keyboardStateBackend{}), keyboardType: mobile.DefaultKeyboard}
		assert.False(t, d.imeOwnsNativeKey(printable), "unbound session")
		assert.False(t, (&driver{}).imeOwnsNativeKey(printable), "nil session")
	})
	t.Run("shortcut echoes are suppressed in every field type", func(t *testing.T) {
		// The IME forwards Ctrl/Alt/Meta shortcuts via keyboardShortcut (pre-rebrand tracker #1077),
		// so the native echo is redundant and must be dropped — in prose AND
		// password/number fields — or the shortcut dispatches twice.
		for _, kt := range []mobile.KeyboardType{mobile.DefaultKeyboard, mobile.SingleLineKeyboard, mobile.PasswordKeyboard, mobile.NumberKeyboard} {
			for _, mod := range []key.Modifiers{key.ModControl, key.ModAlt, key.ModMeta} {
				e := printable
				e.Modifiers = mod
				assert.True(t, bound(kt).imeOwnsNativeKey(e), "keyboard %v modifier %v", kt, mod)
			}
		}
		// Shift alone is part of ordinary typing, still the IME's (prose only).
		shifted := printable
		shifted.Modifiers = key.ModShift
		assert.True(t, bound(mobile.DefaultKeyboard).imeOwnsNativeKey(shifted))
		assert.False(t, bound(mobile.PasswordKeyboard).imeOwnsNativeKey(shifted), "shift+text still native in password")
	})
	t.Run("non-text and non-native keys reach bellui", func(t *testing.T) {
		d := bound(mobile.DefaultKeyboard)
		tab := key.Event{Rune: '\t', Code: key.CodeTab, Native: true, Direction: key.DirPress}
		assert.False(t, d.imeOwnsNativeKey(tab), "tab is not printable")
		enter := key.Event{Code: key.CodeReturnEnter, Native: true, Direction: key.DirPress}
		assert.False(t, d.imeOwnsNativeKey(enter))
		arrow := key.Event{Code: key.CodeLeftArrow, Native: true, Direction: key.DirPress}
		assert.False(t, d.imeOwnsNativeKey(arrow))
		nonNative := key.Event{Rune: 'a', Direction: key.DirPress} // IME's own keyboardTyped
		assert.False(t, d.imeOwnsNativeKey(nonNative))
	})
}
