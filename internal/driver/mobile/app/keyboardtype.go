package app

// KeyboardType represents the type of a keyboard
type KeyboardType int32

const (
	// DefaultKeyboard is the keyboard with default input style and "return" return key
	DefaultKeyboard KeyboardType = iota
	// SingleLineKeyboard is the keyboard with default input style and "Done" return key
	SingleLineKeyboard
	// NumberKeyboard is the keyboard with number input style and "Done" return key
	NumberKeyboard
)

// KeysKeyboard is the keyboard for a focused widget that takes keys but is not
// a text editor, such as a chart waiting for a label. The IME is told there is
// no text field (Android TYPE_NULL), so it sends key presses instead of
// composing and committing text, which only an editor can take (#666). It has
// no mobile.KeyboardType counterpart: the driver picks it, after the public
// codes 0–3.
const KeysKeyboard KeyboardType = 4
