package desktop

import "github.com/belluikit/bellui/event"

// Canvas defines the desktop specific extensions to a core.Canvas.
type Canvas interface {
	OnKeyDown() func(*event.KeyEvent)
	SetOnKeyDown(func(*event.KeyEvent))
	OnKeyUp() func(*event.KeyEvent)
	SetOnKeyUp(func(*event.KeyEvent))
}
