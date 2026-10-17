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
	assert.Equal(t, 0, s.id(a, touch.TypeBegin, true))
	assert.Equal(t, 1, s.id(b, touch.TypeBegin, false))

	assert.Equal(t, 0, s.id(a, touch.TypeEnd, false), "A lifts")
	assert.Equal(t, 1, s.id(b, touch.TypeMove, false), "B keeps its id")
	assert.Equal(t, 0, s.id(c, touch.TypeBegin, false), "a new finger takes the freed slot")
	assert.Equal(t, 1, s.id(b, touch.TypeCancel, false), "a cancelled touch frees its slot too")
	assert.Equal(t, 1, s.find(0), "B's slot is free again")
}

// A touch whose end UIKit never reported must not hold its slot forever: that
// was fyne-io/fyne#2407's "out of touchIDs" panic.
func TestTouchSlots_LostEndsAreReclaimed(t *testing.T) {
	var s touchSlots
	s.id(0xa0, touch.TypeBegin, true) // its end is never reported

	assert.Equal(t, 0, s.id(0xb0, touch.TypeBegin, true),
		"a gesture that starts with no finger down frees the leftover slot")

	for i := range len(s) - 1 { // fill the other slots with more lost ends
		s.id(uintptr(0x100+i), touch.TypeBegin, false)
	}
	assert.Equal(t, -1, s.find(0), "every slot is taken")
	assert.NotPanics(t, func() {
		assert.Equal(t, 0, s.id(0xffff, touch.TypeBegin, false), "a full table is reclaimed, not a panic")
	})
}
