package bellui

import "testing"

// DoRunsInline is false with no app, as for any driver that does not say
// otherwise (#1356).
func TestDoRunsInlineWithoutApp(t *testing.T) {
	prev := CurrentApp()
	SetCurrentApp(nil)
	t.Cleanup(func() { SetCurrentApp(prev) })
	if DoRunsInline() {
		t.Fatal("DoRunsInline with no app reported true")
	}
}
