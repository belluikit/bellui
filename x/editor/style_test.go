package editor

import (
	"testing"
)

func TestHighlighterFunc(t *testing.T) {
	called := false
	hf := HighlighterFunc(func(line string, state int) ([]StyleRun, int) {
		called = true
		return []StyleRun{{Text: line}}, state + 1
	})

	runs, newState := hf.Highlight("hello", 0)
	if !called {
		t.Error("HighlighterFunc should have been called")
	}
	if len(runs) != 1 || runs[0].Text != "hello" {
		t.Errorf("unexpected runs: %v", runs)
	}
	if newState != 1 {
		t.Errorf("newState = %d, want 1", newState)
	}
}

func TestStyleRunDefaults(t *testing.T) {
	r := StyleRun{Text: "x"}
	if r.ColorName != "" {
		t.Error("default ColorName should be empty")
	}
	if r.Bold || r.Italic || r.Underline {
		t.Error("default style flags should be false")
	}
	if r.ForegroundColor != nil || r.BackgroundColor != nil {
		t.Error("default colors should be nil")
	}
}
