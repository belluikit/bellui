package app

import (
	"testing"

	"github.com/belluikit/bellui/internal/driver/mobile/event/lifecycle"
)

// TestActivityStage pins how Android's onPause/onStop/onResume move the
// lifecycle (#673): pause reports the background at once, nothing raises the
// stage before there is a surface, and a resume over a surviving window
// returns to focused.
func TestActivityStage(t *testing.T) {
	const (
		dead    = lifecycle.StageDead
		alive   = lifecycle.StageAlive
		visible = lifecycle.StageVisible
		focused = lifecycle.StageFocused
	)
	for _, tc := range []struct {
		name        string
		e           activityEvent
		cur         lifecycle.Stage
		windowReady bool
		to          lifecycle.Stage
		ok          bool
	}{
		{"pause while focused leaves the foreground", activityPause, focused, true, visible, true},
		{"pause after the window died stays alive", activityPause, alive, false, alive, false},
		{"pause before the first frame stays dead", activityPause, dead, false, dead, false},
		{"stop while visible hides", activityStop, visible, true, alive, true},
		{"stop straight from focused hides", activityStop, focused, true, alive, true},
		{"stop after the window died is a no-op", activityStop, alive, false, alive, false},
		{"resume over a surviving window refocuses", activityResume, visible, true, focused, true},
		{"resume after a stop with the window alive refocuses", activityResume, alive, true, focused, true},
		{"resume without a window waits for the redraw", activityResume, alive, false, alive, false},
		{"resume while focused is a no-op", activityResume, focused, true, focused, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			to, ok := activityStage(tc.e, tc.cur, tc.windowReady)
			if to != tc.to || ok != tc.ok {
				t.Errorf("activityStage(%v, %v, %v) = %v, %v; want %v, %v", tc.e, tc.cur, tc.windowReady, to, ok, tc.to, tc.ok)
			}
		})
	}
}
