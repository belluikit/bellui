package interact

import "testing"

func TestEventBus_On(t *testing.T) {
	bus := NewEventBus()
	called := false
	bus.On(EventClick, func(evt interface{}) {
		called = true
	})

	bus.Emit(DataPointEvent{Event: Event{Type: EventClick}, SeriesName: "test"})
	if !called {
		t.Error("handler not called")
	}
}

func TestEventBus_TypeFilter(t *testing.T) {
	bus := NewEventBus()
	clickCount := 0
	hoverCount := 0
	bus.On(EventClick, func(evt interface{}) { clickCount++ })
	bus.On(EventHover, func(evt interface{}) { hoverCount++ })

	bus.Emit(DataPointEvent{Event: Event{Type: EventClick}})
	bus.Emit(DataPointEvent{Event: Event{Type: EventHover}})
	bus.Emit(DataPointEvent{Event: Event{Type: EventClick}})

	if clickCount != 2 {
		t.Errorf("click count = %d, want 2", clickCount)
	}
	if hoverCount != 1 {
		t.Errorf("hover count = %d, want 1", hoverCount)
	}
}

func TestEventBus_SeriesFilter(t *testing.T) {
	bus := NewEventBus()
	called := false
	idx := 1
	bus.OnFiltered(EventFilter{SeriesIndex: &idx, EventTypes: []EventType{EventClick}}, func(evt interface{}) {
		called = true
	})

	// Emit for series 0 — should not match
	bus.Emit(DataPointEvent{Event: Event{Type: EventClick}, SeriesIndex: 0})
	if called {
		t.Error("should not match series 0")
	}

	// Emit for series 1 — should match
	bus.Emit(DataPointEvent{Event: Event{Type: EventClick}, SeriesIndex: 1})
	if !called {
		t.Error("should match series 1")
	}
}

func TestEventBus_Off(t *testing.T) {
	bus := NewEventBus()
	count := 0
	id := bus.On(EventClick, func(evt interface{}) { count++ })

	bus.Emit(DataPointEvent{Event: Event{Type: EventClick}})
	bus.Off(id)
	bus.Emit(DataPointEvent{Event: Event{Type: EventClick}})

	if count != 1 {
		t.Errorf("count = %d, want 1 (handler should be removed)", count)
	}
}
