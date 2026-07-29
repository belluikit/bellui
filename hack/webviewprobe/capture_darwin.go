//go:build darwin

package main

import (
	"fmt"
	"os/exec"
)

// captureScreen reads the composited screen with macOS screencapture.
//
// It needs Screen Recording permission for the terminal running the probe
// (System Settings > Privacy & Security > Screen Recording); without it the
// capture comes back blank or desktop-only, and every measurement built on it
// would quietly report a hole where there is none.
func captureScreen(r screenshotRegion, path string) error {
	region := fmt.Sprintf("%d,%d,%d,%d", r.x, r.y, r.w, r.h)
	if out, err := exec.Command("screencapture", "-x", "-R", region, path).CombinedOutput(); err != nil {
		return fmt.Errorf("screencapture: %v (%s)", err, out)
	}
	return nil
}
