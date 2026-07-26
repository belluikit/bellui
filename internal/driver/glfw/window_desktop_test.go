//go:build !wasm

package glfw

import (
	"testing"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/glfw"
	"github.com/stretchr/testify/assert"
)

var keyCodeMap = map[glfw.Key]event.KeyName{
	// non-printable
	glfw.KeyEscape:    event.KeyEscape,
	glfw.KeyEnter:     event.KeyReturn,
	glfw.KeyTab:       event.KeyTab,
	glfw.KeyBackspace: event.KeyBackspace,
	glfw.KeyInsert:    event.KeyInsert,
	glfw.KeyDelete:    event.KeyDelete,
	glfw.KeyRight:     event.KeyRight,
	glfw.KeyLeft:      event.KeyLeft,
	glfw.KeyDown:      event.KeyDown,
	glfw.KeyUp:        event.KeyUp,
	glfw.KeyPageUp:    event.KeyPageUp,
	glfw.KeyPageDown:  event.KeyPageDown,
	glfw.KeyHome:      event.KeyHome,
	glfw.KeyEnd:       event.KeyEnd,

	glfw.KeySpace:   event.KeySpace,
	glfw.KeyKPEnter: event.KeyEnter,

	// functions
	glfw.KeyF1:  event.KeyF1,
	glfw.KeyF2:  event.KeyF2,
	glfw.KeyF3:  event.KeyF3,
	glfw.KeyF4:  event.KeyF4,
	glfw.KeyF5:  event.KeyF5,
	glfw.KeyF6:  event.KeyF6,
	glfw.KeyF7:  event.KeyF7,
	glfw.KeyF8:  event.KeyF8,
	glfw.KeyF9:  event.KeyF9,
	glfw.KeyF10: event.KeyF10,
	glfw.KeyF11: event.KeyF11,
	glfw.KeyF12: event.KeyF12,

	// numbers - lookup by code to avoid AZERTY using the symbol name instead of number
	glfw.Key0:   event.Key0,
	glfw.KeyKP0: event.Key0,
	glfw.Key1:   event.Key1,
	glfw.KeyKP1: event.Key1,
	glfw.Key2:   event.Key2,
	glfw.KeyKP2: event.Key2,
	glfw.Key3:   event.Key3,
	glfw.KeyKP3: event.Key3,
	glfw.Key4:   event.Key4,
	glfw.KeyKP4: event.Key4,
	glfw.Key5:   event.Key5,
	glfw.KeyKP5: event.Key5,
	glfw.Key6:   event.Key6,
	glfw.KeyKP6: event.Key6,
	glfw.Key7:   event.Key7,
	glfw.KeyKP7: event.Key7,
	glfw.Key8:   event.Key8,
	glfw.KeyKP8: event.Key8,
	glfw.Key9:   event.Key9,
	glfw.KeyKP9: event.Key9,

	// desktop
	glfw.KeyLeftShift:    desktop.KeyShiftLeft,
	glfw.KeyRightShift:   desktop.KeyShiftRight,
	glfw.KeyLeftControl:  desktop.KeyControlLeft,
	glfw.KeyRightControl: desktop.KeyControlRight,
	glfw.KeyLeftAlt:      desktop.KeyAltLeft,
	glfw.KeyRightAlt:     desktop.KeyAltRight,
	glfw.KeyLeftSuper:    desktop.KeySuperLeft,
	glfw.KeyRightSuper:   desktop.KeySuperRight,
	glfw.KeyMenu:         desktop.KeyMenu,
	glfw.KeyPrintScreen:  desktop.KeyPrintScreen,
	glfw.KeyCapsLock:     desktop.KeyCapsLock,
}

func TestGlfwKeyToKeyName(t *testing.T) {
	for key, value := range keyCodeMap {
		translated := glfwKeyToKeyName(key)
		assert.Equal(t, value, translated)
	}

	invalid := glfwKeyToKeyName(glfw.Key(-1))
	assert.Equal(t, event.KeyUnknown, invalid)
}

func TestConvertASCII(t *testing.T) {
	for i := 0; i <= 'Z'-'A'; i++ {
		translated := convertASCII(glfw.KeyA + glfw.Key(i))
		expected := event.KeyName(rune(event.KeyA[0] + byte(i)))
		assert.Equal(t, expected, translated)
	}

	invalid := convertASCII(glfw.Key(-1))
	assert.Equal(t, event.KeyUnknown, invalid)
}

var keyNameMapSpecialCharacters = map[string]event.KeyName{
	"'": event.KeyApostrophe,
	",": event.KeyComma,
	"-": event.KeyMinus,
	".": event.KeyPeriod,
	"/": event.KeySlash,
	"*": event.KeyAsterisk,
	"`": event.KeyBackTick,

	";": event.KeySemicolon,
	"+": event.KeyPlus,
	"=": event.KeyEqual,

	"[":  event.KeyLeftBracket,
	"\\": event.KeyBackslash,
	"]":  event.KeyRightBracket,
}

func TestKeyCodeToKeyName(t *testing.T) {
	for key, value := range keyNameMapSpecialCharacters {
		translated := keyCodeToKeyName(key)
		assert.Equal(t, value, translated)
	}

	for i := rune(0); i <= 'z'-'a'; i++ {
		translated := keyCodeToKeyName(string('a' + i))
		expected := event.KeyName(rune(event.KeyA[0]) + i)
		assert.Equal(t, expected, translated)
	}

	invalid := keyCodeToKeyName("@")
	assert.Equal(t, event.KeyUnknown, invalid)

	invalid = keyCodeToKeyName("invalid")
	assert.Equal(t, event.KeyUnknown, invalid)
}
