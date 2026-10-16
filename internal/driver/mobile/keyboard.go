package mobile

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/internal/driver/mobile/app"
)

func (d *device) hideVirtualKeyboard() {
	if drv, ok := bellui.CurrentApp().Driver().(*driver); ok {
		if drv.app == nil { // not yet running
			return
		}

		drv.app.HideVirtualKeyboard()
		d.keyboardShown = false

		// Detach the text-input mirror; nothing should be pushed while no field
		// is focused (pre-rebrand tracker #770). The next focus rebinds via showVirtualKeyboard.
		if drv.textInput != nil {
			drv.textInput.Unbind()
		}
	}
}

func (d *device) handleKeyboard(obj event.Focusable) {
	isDisabled := false
	if disWid, ok := obj.(core.Disableable); ok {
		isDisabled = disWid.Disabled()
	}
	if obj != nil && !isDisabled {
		if keyb, ok := obj.(mobile.Keyboardable); ok {
			d.showVirtualKeyboard(keyb.Keyboard())
		} else {
			d.showVirtualKeyboard(mobile.DefaultKeyboard)
		}
	} else {
		d.hideVirtualKeyboard()
	}
}

func (d *device) showVirtualKeyboard(keyboard mobile.KeyboardType) {
	if drv, ok := bellui.CurrentApp().Driver().(*driver); ok {
		if drv.app == nil { // not yet running
			bellui.LogError("Cannot show keyboard before app is running", nil)
			return
		}

		d.keyboardShown = true
		// Remember the field type so the one-writer IME reroute only suppresses
		// native text-key echoes for prose fields, where Gboard writes through
		// the InputConnection (pre-rebrand tracker #1061). Password/number fields have no IC writer.
		drv.keyboardType = keyboard
		drv.app.ShowVirtualKeyboard(app.KeyboardType(keyboard))

		// Bind the newly focused field to the text-input mirror and push its real
		// content into the native IME shadow (pre-rebrand tracker #572 → pre-rebrand tracker #770 seam). Bind forces the
		// next Sync to push even when a field with the same text as the previous
		// focus (e.g. another empty entry) is focused; a non-texter focus unbinds.
		//
		// When the bound editor's identity changes, tell the native surface FIRST
		// (ordered ahead of the seed push on the same op queue): the IME session
		// state it holds — composing word, echo filters — belongs to the previous
		// field, and the seed push for the new field must never be swallowed by
		// it or leave the old session running (pre-rebrand tracker #966: one Gboard editing session
		// spanning two fields sprayed the password across them).
		if drv.textInput != nil {
			if w := drv.currentWindow(); w != nil {
				if c, ok := w.Canvas().(*canvas); ok {
					if drv.textInput.Bind(c.Focused()) {
						app.KeyboardEditorChanged()
					}
					drv.syncKeyboardState()
				}
			}
		}
	}
}
