package cell

import (
	"testing"
	"time"
)

// TextForValue returns the value formatted as the cell shows it (untruncated),
// so a screen reader announces what is visible.
func TestTextForValue(t *testing.T) {
	// The built-in text-like renderers satisfy the optional interface.
	var _ AccessibleValueTexter = (*NumberRenderer)(nil)
	var _ AccessibleValueTexter = (*DateRenderer)(nil)
	var _ AccessibleValueTexter = (*TextRenderer)(nil)

	if got := (&NumberRenderer{Format: "%.2f"}).TextForValue(3.14159); got != "3.14" {
		t.Errorf("NumberRenderer{%%.2f}.TextForValue = %q, want %q", got, "3.14")
	}
	if got := NewNumberRenderer().TextForValue(1000); got != "1000" {
		t.Errorf("default NumberRenderer.TextForValue = %q, want %q", got, "1000")
	}

	ts := time.Date(2026, 7, 7, 13, 30, 0, 0, time.UTC)
	if got := NewDateRenderer().TextForValue(ts); got != "2026-07-07" {
		t.Errorf("DateRenderer.TextForValue = %q, want %q", got, "2026-07-07")
	}
	if got := (&DateRenderer{Format: "Jan 2, 2006"}).TextForValue(ts); got != "Jul 7, 2026" {
		t.Errorf("DateRenderer{custom}.TextForValue = %q, want %q", got, "Jul 7, 2026")
	}

	if got := NewTextRenderer().TextForValue("hi"); got != "hi" {
		t.Errorf("TextRenderer.TextForValue = %q, want %q", got, "hi")
	}

	// nil values render empty, no panic.
	if got := NewDateRenderer().TextForValue(nil); got != "" {
		t.Errorf("DateRenderer.TextForValue(nil) = %q, want empty", got)
	}
	if got := NewNumberRenderer().TextForValue(nil); got != "" {
		t.Errorf("NumberRenderer.TextForValue(nil) = %q, want empty", got)
	}
}
