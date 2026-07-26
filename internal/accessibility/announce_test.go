//go:build accessibility

package accessibility

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnnounce_FansOutToListeners(t *testing.T) {
	type call struct {
		text      string
		assertive bool
	}
	var first, second []call
	removeFirst := AddAnnouncementListener(func(text string, assertive bool) {
		first = append(first, call{text, assertive})
	})
	defer removeFirst()
	removeSecond := AddAnnouncementListener(func(text string, assertive bool) {
		second = append(second, call{text, assertive})
	})
	defer removeSecond()

	Announce("saved", false)
	Announce("disk full", true)

	want := []call{{"saved", false}, {"disk full", true}}
	assert.Equal(t, want, first)
	assert.Equal(t, want, second)
}

func TestAnnounce_EmptyTextIsDropped(t *testing.T) {
	calls := 0
	remove := AddAnnouncementListener(func(string, bool) { calls++ })
	defer remove()

	Announce("", true)
	assert.Zero(t, calls)
}

func TestAddAnnouncementListener_RemoveIsPerRegistration(t *testing.T) {
	calls := 0
	fn := func(string, bool) { calls++ }

	removeA := AddAnnouncementListener(fn)
	removeB := AddAnnouncementListener(fn)

	removeA()
	removeA() // removing twice must not disturb the other registration
	Announce("still here", false)
	assert.Equal(t, 1, calls)

	removeB()
	Announce("gone", false)
	assert.Equal(t, 1, calls)
}
