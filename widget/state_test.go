package widget

import (
	"testing"

	"github.com/belluikit/bellui/test"
)

func TestState_SetGet(t *testing.T) {
	w := NewLabel("test")
	s := NewState[string](w, "hello")

	if s.Get() != "hello" {
		t.Errorf("expected 'hello', got %q", s.Get())
	}

	s.Set("world")
	if s.Get() != "world" {
		t.Errorf("expected 'world', got %q", s.Get())
	}
}

func TestState_NoRefreshOnSameValue(t *testing.T) {
	w := NewLabel("test")
	s := NewState[int](w, 42)

	// Setting same value should not trigger refresh.
	s.Set(42)
	if s.Get() != 42 {
		t.Errorf("expected 42, got %d", s.Get())
	}
}

func TestState_WithCallback(t *testing.T) {
	_ = test.NewTempApp(t)

	var called bool
	var received int
	w := NewLabel("test")
	s := NewStateWithCallback[int](w, 0, func(v int) {
		called = true
		received = v
	})

	s.Set(5)
	if !called {
		t.Error("callback should have been called")
	}
	if received != 5 {
		t.Errorf("expected callback with 5, got %d", received)
	}
}
