package editor

import "testing"

func TestDeclaredNamesSets(t *testing.T) {
	s := DeclaredNames{
		Annotations: []string{"a", "b", ""}, // empty names dropped
		Types:       []string{"T"},
		Functions:   nil, // empty category -> nil set
	}.Sets()

	if !s.Annotations["a"] || !s.Annotations["b"] {
		t.Error("declared annotations not in set")
	}
	if s.Annotations[""] {
		t.Error("empty name should be dropped")
	}
	if len(s.Annotations) != 2 {
		t.Errorf("annotation set size = %d, want 2", len(s.Annotations))
	}
	if !s.Types["T"] {
		t.Error("declared type not in set")
	}
	if s.Functions != nil {
		t.Error("empty Functions should yield a nil set")
	}
	// A nil set is safe to index (matches nothing).
	if s.Functions["anything"] {
		t.Error("nil set must match nothing")
	}
}

func TestDeclaredNamesZeroValue(t *testing.T) {
	var s DeclaredNameSets // zero value
	if s.Annotations["x"] || s.Types["x"] || s.Functions["x"] {
		t.Error("zero-value sets must match nothing")
	}
}
