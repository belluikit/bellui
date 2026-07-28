package tiles

import (
	"sync"
	"testing"
)

// A cache is shared by URL across every map using that tile server, while the
// repaint seam is per chart, so tile arrival has to fan out to all of them.
func TestSetListener_FanOut(t *testing.T) {
	c := NewCache("", nil)

	var mu sync.Mutex
	var calls []string
	record := func(name string) func() {
		return func() {
			mu.Lock()
			defer mu.Unlock()
			calls = append(calls, name)
		}
	}

	c.SetListener("a", record("a"))
	c.SetListener("b", record("b"))
	c.notifyLoaded()

	mu.Lock()
	got := append([]string(nil), calls...)
	mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("expected both listeners to fire, got %v", got)
	}
}

// Registering again under the same key replaces rather than accumulates, so a
// chart re-rendering every frame cannot grow the listener set without bound.
func TestSetListener_ReplacesByKey(t *testing.T) {
	c := NewCache("", nil)

	var mu sync.Mutex
	count := 0
	for i := 0; i < 10; i++ {
		c.SetListener("chart", func() { mu.Lock(); count++; mu.Unlock() })
	}
	c.notifyLoaded()

	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Errorf("expected 1 call from the single surviving registration, got %d", count)
	}
}

func TestSetListener_NilRemoves(t *testing.T) {
	c := NewCache("", nil)

	fired := false
	c.SetListener("chart", func() { fired = true })
	c.SetListener("chart", nil)
	c.notifyLoaded()

	if fired {
		t.Error("a removed listener must not fire")
	}
}

// The constructor callback still works alongside registered listeners.
func TestNotifyLoaded_IncludesConstructorCallback(t *testing.T) {
	fired := false
	c := NewCache("", func() { fired = true })
	c.notifyLoaded()

	if !fired {
		t.Error("the NewCache callback must still fire")
	}
}

// A nil listener callback must not be stored, so notifyLoaded cannot panic on
// a series rendered without a repaint seam.
func TestNotifyLoaded_NoListeners(t *testing.T) {
	c := NewCache("", nil)
	c.SetListener("chart", nil)
	c.notifyLoaded() // must not panic
}

func TestHasListener(t *testing.T) {
	c := NewCache("", nil)
	if c.HasListener("chart") {
		t.Error("a fresh cache has no listeners")
	}
	c.SetListener("chart", func() {})
	if !c.HasListener("chart") {
		t.Error("expected the registration to be visible")
	}
	c.SetListener("chart", nil)
	if c.HasListener("chart") {
		t.Error("expected the registration to be gone")
	}
}
