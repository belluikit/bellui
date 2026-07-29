//go:build !darwin && !windows

package main

import "errors"

// captureScreen has no implementation here, which is the honest answer rather
// than an inconvenience: -auto measures the hole punch by comparing bellui's
// surface against the composited screen, and without the second capture there
// is nothing to compare. The interactive checklist in the package comment still
// applies — and on Linux there is no backend to check it against anyway (#60).
func captureScreen(screenshotRegion, string) error {
	return errors.New("no screen capture on this platform")
}
