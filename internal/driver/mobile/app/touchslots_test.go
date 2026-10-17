package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/internal/driver/mobile/event/touch"
)

// A finger still down keeps its id when another lifts (#392). Clearing every
// slot on any end gave it a new id, so it arrived as a touch that had never
// gone down and could pan or tap.
func TestTouchSlots_SurvivingTouchKeepsItsID(t *testing.T) {
	var s touchSlots
	const a, b, c = 0xa0, 0xb0, 0xc0
	assert.Equal(t, 0, s.id(a, touch.TypeBegin))
	assert.Equal(t, 1, s.id(b, touch.TypeBegin))

	assert.Equal(t, 0, s.id(a, touch.TypeEnd), "A lifts")
	assert.Equal(t, 1, s.id(b, touch.TypeMove), "B keeps its id")
	assert.Equal(t, 0, s.id(c, touch.TypeBegin), "a new finger takes the freed slot")
	assert.Equal(t, 1, s.id(b, touch.TypeCancel), "a cancelled touch frees its slot too")
	assert.Equal(t, 1, s.find(0), "B's slot is free again")
}

// A touch whose end UIKit never reported must not hold its slot forever: that
// was fyne-io/fyne#2407's "out of touchIDs" panic.
func TestTouchSlots_LostEndsAreReclaimed(t *testing.T) {
	var s touchSlots
	s.id(0xa0, touch.TypeBegin) // its end is never reported

	s.reset() // the next gesture starts with no finger down
	assert.Equal(t, 0, s.id(0xb0, touch.TypeBegin), "the leftover slot was freed")

	for i := range len(s) - 1 { // fill the other slots with more lost ends
		s.id(uintptr(0x100+i), touch.TypeBegin)
	}
	assert.Equal(t, -1, s.find(0), "every slot is taken")
	assert.NotPanics(t, func() {
		assert.Equal(t, 0, s.id(0xffff, touch.TypeBegin), "a full table is reclaimed, not a panic")
	})
}

// Two fingers that land in the same event get different ids. Measured on an
// iPad: the free-stale-slots step ran per touch, so the second finger's begin
// freed the first finger's slot, both were id 0, and the first finger came
// back as id 1 with a 539 px jump (#392).
func TestTouchSlots_TwoFingersLandingTogether(t *testing.T) {
	var s touchSlots
	s.reset() // once for the event, as darwin_ios.m does
	a := s.id(0xa0, touch.TypeBegin)
	b := s.id(0xb0, touch.TypeBegin)
	assert.NotEqual(t, a, b)
	assert.Equal(t, a, s.id(0xa0, touch.TypeMove), "A keeps its id")
	assert.Equal(t, b, s.id(0xb0, touch.TypeMove), "B keeps its id")
}
