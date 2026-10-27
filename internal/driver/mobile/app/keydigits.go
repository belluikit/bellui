package app

// nativeDigitIsEcho reports whether a digit key event from the native input
// queue duplicates text the keyboard delivers on its own. For a text field
// Gboard sends a digit both as text (textChanged) and as a key event, so the
// key event is dropped. The keys keyboard of a focused non-editor (TYPE_NULL)
// sends only key events: there the key event is the digit, and dropping it
// lost every digit typed into, say, a chart's text label (#824).
func nativeDigitIsEcho(r rune, kb KeyboardType) bool {
	return r >= '0' && r <= '9' && kb != KeysKeyboard
}
