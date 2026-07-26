package core

import "testing"

func TestOnActivation_NilByDefault(t *testing.T) {
	// Importing core (or any package that does not install a consumer) must
	// leave the seam nil, so an activation costs a single nil check.
	if OnActivation != nil {
		t.Fatal("OnActivation should be nil unless a consumer installs it")
	}
	// NotifyActivation with no consumer is a no-op and must not panic.
	NotifyActivation(nil, false)
	NotifyActivation(&mockNonAccessible{}, false)
}

func TestNotifyActivation_FiresOncePerCall(t *testing.T) {
	var gotObj CanvasObject
	var gotKeyboard bool
	calls := 0
	OnActivation = func(obj CanvasObject, viaKeyboard bool) {
		calls++
		gotObj = obj
		gotKeyboard = viaKeyboard
	}
	t.Cleanup(func() { OnActivation = nil })

	obj := &mockNonAccessible{}
	NotifyActivation(obj, true)
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
	if gotObj != obj {
		t.Fatal("consumer received the wrong object")
	}
	if !gotKeyboard {
		t.Fatal("viaKeyboard flag not propagated")
	}

	NotifyActivation(obj, false)
	if calls != 2 {
		t.Fatalf("expected 2 calls after second activation, got %d", calls)
	}
}

func TestNotifyActivation_NilObjectIgnored(t *testing.T) {
	calls := 0
	OnActivation = func(CanvasObject, bool) { calls++ }
	t.Cleanup(func() { OnActivation = nil })

	NotifyActivation(nil, false)
	if calls != 0 {
		t.Fatalf("a nil object must not invoke the consumer, got %d calls", calls)
	}
}
