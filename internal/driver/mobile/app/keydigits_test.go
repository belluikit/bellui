package app

import "testing"

// A native digit key event is dropped as an echo only while a text keyboard
// is up: with the keys keyboard it is the only copy of the digit (#824).
func TestNativeDigitIsEcho(t *testing.T) {
	for _, c := range []struct {
		r    rune
		kb   KeyboardType
		echo bool
	}{
		{'5', DefaultKeyboard, true},
		{'0', SingleLineKeyboard, true},
		{'9', NumberKeyboard, true},
		{'5', KeysKeyboard, false},
		{'a', DefaultKeyboard, false},
		{'a', KeysKeyboard, false},
	} {
		if got := nativeDigitIsEcho(c.r, c.kb); got != c.echo {
			t.Errorf("digit %q with keyboard %d: echo %v, want %v", c.r, c.kb, got, c.echo)
		}
	}
}
