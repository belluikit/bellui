package app

import "github.com/belluikit/bellui/internal/driver/mobile/event/touch"

// touchSlots gives each iOS touch that is down a small id that stays the same
// for as long as the touch does. UIKit identifies a touch by its UITouch
// pointer; the position in the array is the id, the value the pointer, and 0
// a free slot. It is widely reported that the iPhone can handle up to 5
// simultaneous touches, the iPad 11.
//
// Every slot used to be cleared when any touch ended (fyne-io/fyne#2407, which
// panicked "out of touchIDs" when UIKit never reported some ends). That gave a
// finger still down a new id on its next event, so it arrived as a touch that
// had never gone down and could pan or tap (#392). Now only the ended touch's
// slot is freed, and the slots of touches whose end was never reported are
// reclaimed instead: when a gesture starts with no other finger down, and as a
// last resort when every slot is taken.
//
// A reclaimed id is reported as a cancel. The driver and the canvas still hold
// that touch as down (a contact a pinch can pair with, a window it routes to),
// and only an end or a cancel lets them forget it.
type touchSlots [11]uintptr

// reset frees every slot and returns a cancel for each id that was held. The
// iOS side calls it once when touches begin with no other touch down, before
// any of them is given an id: anything still held then is a touch whose end
// was lost. Calling it per touch instead gave two fingers that landed in the
// same event the same id.
func (s *touchSlots) reset() []touch.Event {
	var lost []touch.Event
	for i, v := range s {
		if v != 0 {
			lost = append(lost, touch.Event{Sequence: touch.Sequence(i), Type: touch.TypeCancel})
		}
	}
	*s = touchSlots{}
	return lost
}

// events returns what to deliver for an event of type t on the touch ptr at
// (x, y): a cancel for each id reclaimed to make room for it, then the event
// itself under the touch's id. An ended or cancelled touch's slot is freed.
func (s *touchSlots) events(ptr uintptr, t touch.Type, x, y float32) []touch.Event {
	var out []touch.Event
	id := s.find(ptr)
	if id < 0 {
		id = s.find(0)
		if id < 0 {
			// Every slot holds a touch, which cannot all be down: reclaim them
			// rather than panic.
			out = s.reset()
			id = 0
		}
		s[id] = ptr
	}

	if t == touch.TypeEnd || t == touch.TypeCancel {
		s[id] = 0
	}
	return append(out, touch.Event{X: x, Y: y, Sequence: touch.Sequence(id), Type: t})
}

func (s *touchSlots) find(ptr uintptr) int {
	for i, v := range s {
		if v == ptr {
			return i
		}
	}
	return -1
}
