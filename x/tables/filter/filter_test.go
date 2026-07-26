package filter

import "testing"

func TestTextFilter_Contains(t *testing.T) {
	f := &TextFilter{Mode: TextContains, Value: "ell"}
	if !f.Matches("hello") {
		t.Error("expected 'hello' to match contains 'ell'")
	}
	if f.Matches("world") {
		t.Error("expected 'world' to not match contains 'ell'")
	}
}

func TestTextFilter_Equals(t *testing.T) {
	f := &TextFilter{Mode: TextEquals, Value: "hello"}
	if !f.Matches("hello") {
		t.Error("expected exact match")
	}
	if f.Matches("Hello") {
		t.Error("expected case-sensitive mismatch")
	}
}

func TestTextFilter_StartsWith(t *testing.T) {
	f := &TextFilter{Mode: TextStartsWith, Value: "he"}
	if !f.Matches("hello") {
		t.Error("expected startsWith match")
	}
	if f.Matches("world") {
		t.Error("expected startsWith mismatch")
	}
}

func TestTextFilter_EndsWith(t *testing.T) {
	f := &TextFilter{Mode: TextEndsWith, Value: "lo"}
	if !f.Matches("hello") {
		t.Error("expected endsWith match")
	}
	if f.Matches("help") {
		t.Error("expected endsWith mismatch")
	}
}

func TestTextFilter_NonString(t *testing.T) {
	f := &TextFilter{Mode: TextContains, Value: "test"}
	if f.Matches(42) {
		t.Error("expected non-string to not match")
	}
	if f.Matches(nil) {
		t.Error("expected nil to not match")
	}
}

func TestNumberFilter_Equals(t *testing.T) {
	f := &NumberFilter{Mode: NumberEquals, Value: 42}
	if !f.Matches(42.0) {
		t.Error("expected 42.0 to match")
	}
	if !f.Matches(42) {
		t.Error("expected int 42 to match")
	}
	if f.Matches(43.0) {
		t.Error("expected 43 to not match")
	}
}

func TestNumberFilter_GreaterThan(t *testing.T) {
	f := &NumberFilter{Mode: NumberGreaterThan, Value: 10}
	if !f.Matches(11.0) {
		t.Error("expected 11 > 10")
	}
	if f.Matches(10.0) {
		t.Error("expected 10 not > 10")
	}
	if f.Matches(9.0) {
		t.Error("expected 9 not > 10")
	}
}

func TestNumberFilter_InRange(t *testing.T) {
	f := &NumberFilter{Mode: NumberInRange, Value: 10, To: 20}
	if !f.Matches(15.0) {
		t.Error("expected 15 in [10,20]")
	}
	if !f.Matches(10.0) {
		t.Error("expected 10 in [10,20] (inclusive)")
	}
	if !f.Matches(20.0) {
		t.Error("expected 20 in [10,20] (inclusive)")
	}
	if f.Matches(9.0) {
		t.Error("expected 9 not in [10,20]")
	}
	if f.Matches(21.0) {
		t.Error("expected 21 not in [10,20]")
	}
}

func TestSetFilter(t *testing.T) {
	f := &SetFilter{Values: map[string]bool{"a": true, "c": true}}
	if !f.Matches("a") {
		t.Error("expected 'a' to match")
	}
	if f.Matches("b") {
		t.Error("expected 'b' to not match")
	}
	if !f.Matches("c") {
		t.Error("expected 'c' to match")
	}
}

func TestSetFilter_Empty(t *testing.T) {
	f := &SetFilter{Values: map[string]bool{}}
	if !f.Matches("anything") {
		t.Error("empty set filter should pass everything")
	}
}
