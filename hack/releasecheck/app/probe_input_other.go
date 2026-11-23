//go:build !darwin || ios

package main

import "errors"

// newProbeInput has no way to post input here: the probe drives the macOS
// desktop only.
func newProbeInput() (probeInput, error) {
	return nil, errors.New("the probe posts input through AppKit, so it runs on macOS only")
}
