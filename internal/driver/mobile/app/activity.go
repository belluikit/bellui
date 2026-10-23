// Copyright 2026 The Bellui Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import "github.com/belluikit/bellui/internal/driver/mobile/event/lifecycle"

// activityEvent is an Activity callback that moves the lifecycle without
// touching the native window (#673).
type activityEvent int

const (
	activityPause activityEvent = iota
	activityStop
	activityResume
)

// activityStage is the lifecycle stage an Activity callback moves the app to,
// given the current stage and whether the window has a surface to draw on.
// Pause and stop only ever lower the stage: a pause after the window is
// already gone must not raise it back to visible. Resume raises to focused
// only when the window survived the pause (a translucent activity covered
// this one); otherwise the next onNativeWindowRedrawNeeded raises it, once
// there is a surface to paint. ok is false when the stage does not change.
func activityStage(e activityEvent, cur lifecycle.Stage, windowReady bool) (to lifecycle.Stage, ok bool) {
	switch e {
	case activityPause:
		to = lifecycle.StageVisible
	case activityStop:
		to = lifecycle.StageAlive
	case activityResume:
		if !windowReady {
			return cur, false
		}
		return lifecycle.StageFocused, cur != lifecycle.StageFocused
	}
	if cur <= to {
		return cur, false
	}
	return to, true
}
