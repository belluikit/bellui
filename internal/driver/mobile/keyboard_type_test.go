//go:build !wasm

package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/internal/driver/mobile/app"
	"github.com/belluikit/bellui/widget"
)

// numberKeys takes keys, is not an editor, and asks for the number keyboard.
type numberKeys struct{ *keyRecorder }

func (numberKeys) Keyboard() mobile.KeyboardType { return mobile.NumberKeyboard }

// A focused widget that takes keys but is not an editor gets the keys keyboard,
// on which the IME sends key presses instead of committing text that only an
// editor can take (#666). Editors, and widgets that name a keyboard, keep theirs.
func TestNativeKeyboard(t *testing.T) {
	multi := widget.NewMultiLineEntry()
	single := widget.NewEntry()
	password := widget.NewPasswordEntry()

	for name, tt := range map[string]struct {
		kind   mobile.KeyboardType
		native app.KeyboardType
		got    func() (mobile.KeyboardType, app.KeyboardType)
	}{
		"non-editor": {mobile.DefaultKeyboard, app.KeysKeyboard, func() (mobile.KeyboardType, app.KeyboardType) {
			return nativeKeyboard(newKeyRecorder())
		}},
		"non-editor naming a keyboard": {mobile.NumberKeyboard, app.NumberKeyboard, func() (mobile.KeyboardType, app.KeyboardType) {
			return nativeKeyboard(numberKeys{newKeyRecorder()})
		}},
		"multi-line entry": {mobile.DefaultKeyboard, app.DefaultKeyboard, func() (mobile.KeyboardType, app.KeyboardType) {
			return nativeKeyboard(multi)
		}},
		"single-line entry": {mobile.SingleLineKeyboard, app.SingleLineKeyboard, func() (mobile.KeyboardType, app.KeyboardType) {
			return nativeKeyboard(single)
		}},
		"password entry": {mobile.PasswordKeyboard, app.KeyboardType(mobile.PasswordKeyboard), func() (mobile.KeyboardType, app.KeyboardType) {
			return nativeKeyboard(password)
		}},
		"rich entry": {mobile.DefaultKeyboard, app.DefaultKeyboard, func() (mobile.KeyboardType, app.KeyboardType) {
			return nativeKeyboard(widget.NewRichEntry())
		}},
	} {
		t.Run(name, func(t *testing.T) {
			kind, native := tt.got()
			assert.Equal(t, tt.kind, kind)
			assert.Equal(t, tt.native, native)
		})
	}
}
