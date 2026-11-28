package scroll

import "testing"

// The iOS bridge (app/darwin_ios.m, SCROLL_PHASE_*) and Android's both send
// these values as plain ints.
func TestPhaseValues(t *testing.T) {
	for want, p := range []Phase{PhaseNone, PhaseBegan, PhaseChanged, PhaseEnded} {
		if int(p) != want {
			t.Errorf("phase %d is %d, which the platform bridges do not send", want, p)
		}
	}
}
