package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/internal/driver/mobile/event/touch"
)

// id delivers an event on the touch ptr and returns the id it went out under,
// failing if the table had to reclaim anything to make room.
func (s *touchSlots) id(t *testing.T, ptr uintptr, typ touch.Type) int {
	t.Helper()
	out := s.events(ptr, typ, 0, 0)
	if !assert.Len(t, out, 1, "no id was reclaimed") {
		return -1
	}
	return int(out[0].Sequence)
}

// A finger still down keeps its id when another lifts (#392). Clearing every
// slot on any end gave it a new id, so it arrived as a touch that had never
// gone down and could pan or tap.
func TestTouchSlots_SurvivingTouchKeepsItsID(t *testing.T) {
	var s touchSlots
	const a, b, c = 0xa0, 0xb0, 0xc0
	assert.Equal(t, 0, s.id(t, a, touch.TypeBegin))
	assert.Equal(t, 1, s.id(t, b, touch.TypeBegin))

	assert.Equal(t, 0, s.id(t, a, touch.TypeEnd), "A lifts")
	assert.Equal(t, 1, s.id(t, b, touch.TypeMove), "B keeps its id")
	assert.Equal(t, 0, s.id(t, c, touch.TypeBegin), "a new finger takes the freed slot")
	assert.Equal(t, 1, s.id(t, b, touch.TypeCancel), "a cancelled touch frees its slot too")
	assert.Equal(t, 1, s.find(0), "B's slot is free again")
}

// The event itself keeps its type and position.
func TestTouchSlots_EventCarriesTypeAndPosition(t *testing.T) {
	var s touchSlots
	assert.Equal(t, []touch.Event{{X: 3, Y: 4, Sequence: 0, Type: touch.TypeBegin}},
		s.events(0xa0, touch.TypeBegin, 3, 4))
}

// A touch whose end UIKit never reported must not hold its slot forever: that
// was fyne-io/fyne#2407's "out of touchIDs" panic.
func TestTouchSlots_LostEndsAreReclaimed(t *testing.T) {
	var s touchSlots
	s.id(t, 0xa0, touch.TypeBegin) // its end is never reported

	s.reset() // the next gesture starts with no finger down
	assert.Equal(t, 0, s.id(t, 0xb0, touch.TypeBegin), "the leftover slot was freed")

	for i := range len(s) - 1 { // fill the other slots with more lost ends
		s.id(t, uintptr(0x100+i), touch.TypeBegin)
	}
	assert.Equal(t, -1, s.find(0), "every slot is taken")
	assert.NotPanics(t, func() {
		out := s.events(0xffff, touch.TypeBegin, 0, 0)
		assert.Equal(t, touch.TypeBegin, out[len(out)-1].Type)
		assert.Equal(t, touch.Sequence(0), out[len(out)-1].Sequence, "a full table is reclaimed, not a panic")
	})
}

// A reclaimed id is cancelled, so the driver and canvas forget the touch they
// still hold as down. Without it, a stale contact stays counted, and the next
// one-finger drag pairs with it into a pinch around a finger that is gone.
func TestTouchSlots_ReclaimedIDsAreCancelled(t *testing.T) {
	var s touchSlots
	assert.Empty(t, s.reset(), "nothing held, nothing cancelled")

	s.id(t, 0xa0, touch.TypeBegin)
	s.id(t, 0xb0, touch.TypeBegin)
	s.id(t, 0xc0, touch.TypeBegin)
	s.id(t, 0xa0, touch.TypeEnd) // only A's end is reported

	assert.Equal(t, []touch.Event{
		{Sequence: 1, Type: touch.TypeCancel},
		{Sequence: 2, Type: touch.TypeCancel},
	}, s.reset(), "B and C were lost")

	for i := range len(s) {
		s.id(t, uintptr(0x100+i), touch.TypeBegin)
	}
	out := s.events(0xffff, touch.TypeBegin, 0, 0)
	if assert.Len(t, out, len(s)+1, "a cancel per reclaimed id, then the begin") {
		for i, e := range out[:len(s)] {
			assert.Equal(t, touch.Event{Sequence: touch.Sequence(i), Type: touch.TypeCancel}, e)
		}
		assert.Equal(t, touch.Event{Sequence: 0, Type: touch.TypeBegin}, out[len(s)])
	}
}

// Two fingers that land in the same event get different ids. Measured on an
// iPad: the free-stale-slots step ran per touch, so the second finger's begin
// freed the first finger's slot, both were id 0, and the first finger came
// back as id 1 with a 539 px jump (#392).
func TestTouchSlots_TwoFingersLandingTogether(t *testing.T) {
	var s touchSlots
	s.reset() // once for the event, as darwin_ios.m does
	a := s.id(t, 0xa0, touch.TypeBegin)
	b := s.id(t, 0xb0, touch.TypeBegin)
	assert.NotEqual(t, a, b)
	assert.Equal(t, a, s.id(t, 0xa0, touch.TypeMove), "A keeps its id")
	assert.Equal(t, b, s.id(t, 0xb0, touch.TypeMove), "B keeps its id")
}
