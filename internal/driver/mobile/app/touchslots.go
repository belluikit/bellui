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
type touchSlots [11]uintptr

// reset frees every slot. The iOS side calls it once when touches begin with
// no other touch down, before any of them is given an id: anything still held
// then is a touch whose end was lost. Calling it per touch instead gave two
// fingers that landed in the same event the same id.
func (s *touchSlots) reset() { *s = touchSlots{} }

// id returns the id of the touch ptr for an event of type t. An ended or
// cancelled touch's slot is freed.
func (s *touchSlots) id(ptr uintptr, t touch.Type) int {
	id := s.find(ptr)
	if id < 0 {
		id = s.find(0)
		if id < 0 {
			// Every slot holds a touch, which cannot all be down: reclaim them
			// rather than panic.
			*s = touchSlots{}
			id = 0
		}
		s[id] = ptr
	}

	if t == touch.TypeEnd || t == touch.TypeCancel {
		s[id] = 0
	}
	return id
}

func (s *touchSlots) find(ptr uintptr) int {
	for i, v := range s {
		if v == ptr {
			return i
		}
	}
	return -1
}
