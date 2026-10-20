//go:build !partialrepaint && !nopartialrepaint && !mobile

package build

import (
	"runtime"
	"testing"
)

// TestPartialRepaintDefault pins where partial repaint is on without a tag
// (#496): on desktop, where it is validated pixel for pixel against a full
// repaint, and off on mobile and the web, where it is not yet.
func TestPartialRepaintDefault(t *testing.T) {
	desktop := map[string]bool{"darwin": true, "linux": true, "windows": true, "freebsd": true, "openbsd": true, "netbsd": true}
	if want := desktop[runtime.GOOS]; partialRepaintDefault != want {
		t.Errorf("partial repaint default on %s = %v, want %v", runtime.GOOS, partialRepaintDefault, want)
	}
}

// TestPartialRepaintAllowed pins the run-time switch: BELLUI_PARTIAL_REPAINT
// turns partial repaint off, and never on.
func TestPartialRepaintAllowed(t *testing.T) {
	for v, want := range map[string]bool{"": true, "1": true, "on": true, "0": false, "false": false, "off": false, "no": false} {
		if got := partialRepaintAllowed(v); got != want {
			t.Errorf("BELLUI_PARTIAL_REPAINT=%q leaves it on = %v, want %v", v, got, want)
		}
	}
}
