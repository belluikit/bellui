//go:build wasm

package glfw

import (
	"regexp"
	"strings"
	"syscall/js"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
)

var (
	isMobile bool
	isMacOS  bool

	setCursor func(name string)

	blurDummyEntry  func(...any) js.Value
	focusDummyEntry func(...any) js.Value

	// imeComposing is true between an IME compositionstart and compositionend.
	// While set, charInput drops glfw-js character events so composed text is
	// not inserted twice (once via the composition events, once as plain runes).
	imeComposing bool
)

func init() {
	navigator := js.Global().Get("navigator")
	isMobile = regexp.MustCompile("Android|iPhone|iPad|iPod").
		MatchString(navigator.Get("userAgent").String())
	isMacOS = strings.Contains(navigator.Get("platform").String(), "Mac")

	document := js.Global().Get("document")
	style := document.Get("body").Get("style")
	setStyleProperty := style.Get("setProperty").Call("bind", style)
	setCursor = func(name string) {
		setStyleProperty.Invoke("cursor", name)
	}

	dummyEntry := document.Call("getElementById", "dummyEntry")
	if dummyEntry.IsNull() {
		return
	}

	blurDummyEntry = dummyEntry.Get("blur").Call("bind", dummyEntry).Invoke
	focusDummyEntry = dummyEntry.Get("focus").Call("bind", dummyEntry).Invoke

	connectComposition(dummyEntry)
}

// connectComposition wires the browser IME composition (preedit) events on the
// hidden input to the focused widget via the active window. The browser
// positions the candidate window against the focused input automatically, so no
// caret geometry needs to be exported.
func connectComposition(dummyEntry js.Value) {
	addListener := func(name string, fn func(data string)) {
		cb := js.FuncOf(func(_ js.Value, args []js.Value) any {
			data := ""
			if len(args) > 0 {
				if d := args[0].Get("data"); !d.IsNull() && !d.IsUndefined() {
					data = d.String()
				}
			}
			fn(data)
			return nil
		})
		dummyEntry.Call("addEventListener", name, cb)
	}

	addListener("compositionstart", func(string) {
		imeComposing = true
	})
	addListener("compositionupdate", func(data string) {
		runOnMainAsync(func() {
			if w := composeTargetWindow(); w != nil {
				w.processCompose(&event.PreeditEvent{Text: data, CursorPos: len([]rune(data))})
			}
		})
	})
	addListener("compositionend", func(data string) {
		imeComposing = false
		// Clear the hidden input so the next composition starts empty.
		dummyEntry.Set("value", "")
		runOnMainAsync(func() {
			if w := composeTargetWindow(); w != nil {
				w.processComposeCommit(data)
			}
		})
	})
}

// composeTargetWindow resolves the window IME composition should reach.
// curWindow is set by the browser focus callback — but a page that loads
// already focused never gets a focus transition, leaving curWindow nil until
// the user switches away and back; composition silently dropped its events in
// that state (#674 wasm leg). The web target has exactly one real window, so
// fall back to it.
func composeTargetWindow() *window {
	if curWindow != nil {
		return curWindow
	}
	d, ok := bellui.CurrentApp().Driver().(*gLDriver)
	if !ok {
		return nil
	}
	if ws := d.windowList(); len(ws) > 0 {
		if w, ok := ws[0].(*window); ok {
			return w
		}
	}
	return nil
}

func (*glDevice) IsMobile() bool {
	return isMobile
}

func (*glDevice) SystemScaleForWindow(w bellui.Window) float32 {
	// Get the scale information from the web browser directly
	return float32(js.Global().Get("devicePixelRatio").Float())
}

func (*glDevice) hideVirtualKeyboard() {
	if blurDummyEntry == nil {
		return
	}
	blurDummyEntry()
}

func (*glDevice) showVirtualKeyboard() {
	if focusDummyEntry == nil {
		return
	}
	focusDummyEntry()
}

func connectKeyboard(c *glCanvas) {
	c.OnFocus = handleKeyboard
	c.OnUnfocus = hideVirtualKeyboard
}

func isMacOSRuntime() bool {
	return isMacOS // Value depends on which OS the browser is running on.
}
