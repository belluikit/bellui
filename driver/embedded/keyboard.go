package embedded

import "github.com/belluikit/bellui/event"

// KeyDirection specifies the press/release of a key event
//
// Since: 2.7
type KeyDirection uint8

const (
	// KeyPressed specifies that a key was pushed down.
	//
	// Since: 2.7
	KeyPressed KeyDirection = iota

	// KeyReleased indicates a key was let back up.
	//
	// Since: 2.7
	KeyReleased
)

// KeyEvent is an event from keyboard actions occurring in an embedded device keyboard.
//
// Since: 2.7
type KeyEvent struct {
	Name      event.KeyName
	Direction KeyDirection

	// Modifiers reports the modifier keys held when the event was generated.
	// The zero value means no modifiers; hosts without modifier information
	// may leave it unset. It is carried through to the Modifier field of the
	// event.KeyEvent delivered to key handlers.
	//
	// Since: 3.0
	Modifiers event.KeyModifier
}

func (d *KeyEvent) isEvent() {}

// CharacterEvent is an event specifying that a character was created by a hardware or virtual keyboard.
//
// Since: 2.7
type CharacterEvent struct {
	Rune rune
}

func (c *CharacterEvent) isEvent() {}
