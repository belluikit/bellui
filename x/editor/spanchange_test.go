package editor

import (
	"reflect"
	"testing"
)

func TestTextInRange(t *testing.T) {
	text := "alpha\nbeta\ngamma"
	cases := []struct {
		name string
		r    TextRange
		want string
	}{
		{"single-line mid", TextRange{0, 1, 0, 4}, "lph"},
		{"single-line full", TextRange{1, 0, 1, 4}, "beta"},
		{"multi-line", TextRange{0, 2, 2, 3}, "pha\nbeta\ngam"},
		{"end-col clamped", TextRange{1, 0, 1, 99}, "beta"},
		{"empty range", TextRange{1, 2, 1, 2}, ""},
		{"row out of range", TextRange{9, 0, 9, 1}, ""},
	}
	for _, tc := range cases {
		if got := textInRange(text, tc.r); got != tc.want {
			t.Errorf("%s: textInRange = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestClassifySpanChanges(t *testing.T) {
	// Snapshot taken at open time (name → value).
	snapshot := map[string]string{"a": "1", "b": "2", "c": "3"}

	// Current document: a unchanged, b changed, d new; c is gone (not in spans).
	current := "1\nX\nY"
	spans := []NamedSpan{
		{Name: "a", Range: TextRange{0, 0, 0, 1}}, // "1" — unchanged
		{Name: "b", Range: TextRange{1, 0, 1, 1}}, // "X" — modified
		{Name: "d", Range: TextRange{2, 0, 2, 1}}, // "Y" — added
	}

	got := ClassifySpanChanges(snapshot, current, spans)
	want := []SpanChange{
		{Name: "b", Kind: SpanModified},
		{Name: "d", Kind: SpanAdded},
		{Name: "c", Kind: SpanRemoved},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ClassifySpanChanges =\n  %+v\nwant\n  %+v", got, want)
	}
}

func TestClassifySpanChanges_NoChanges(t *testing.T) {
	snapshot := map[string]string{"a": "secret"}
	current := "secret"
	spans := []NamedSpan{{Name: "a", Range: TextRange{0, 0, 0, 6}}}
	if got := ClassifySpanChanges(snapshot, current, spans); len(got) != 0 {
		t.Errorf("unchanged spans should yield no changes, got %+v", got)
	}
}

func TestClassifySpanChanges_RemovedSortedDeterministic(t *testing.T) {
	// Several removed names: result order is sorted (Go map iteration is random).
	snapshot := map[string]string{"z": "1", "a": "2", "m": "3"}
	got := ClassifySpanChanges(snapshot, "", nil)
	want := []SpanChange{
		{Name: "a", Kind: SpanRemoved},
		{Name: "m", Kind: SpanRemoved},
		{Name: "z", Kind: SpanRemoved},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("removed entries should be sorted by name, got %+v", got)
	}
}

func TestClassifySpanChanges_EmptyInputs(t *testing.T) {
	if got := ClassifySpanChanges(nil, "", nil); len(got) != 0 {
		t.Errorf("empty inputs should yield no changes, got %+v", got)
	}
}
