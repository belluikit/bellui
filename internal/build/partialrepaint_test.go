//go:build !partialrepaint && !nopartialrepaint && !mobile

package build

import (
	"runtime"
	"testing"
)

// TestPartialRepaintDefault pins where partial repaint is on without a tag.
// It is on where it was validated pixel for pixel against a full repaint:
// desktop (#496), and Android and iOS with the WebGPU painter (#593). It is
// off on the web (#648) and with the GL painter on mobile, which are not
// validated yet.
func TestPartialRepaintDefault(t *testing.T) {
	desktop := map[string]bool{"darwin": true, "linux": true, "windows": true, "freebsd": true, "openbsd": true, "netbsd": true}
	phone := map[string]bool{"android": true, "ios": true}
	want := desktop[runtime.GOOS] || (phone[runtime.GOOS] && !glPainterBuild)
	if partialRepaintDefault != want {
		t.Errorf("partial repaint default on %s (gl tag %v) = %v, want %v", runtime.GOOS, glPainterBuild, partialRepaintDefault, want)
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
