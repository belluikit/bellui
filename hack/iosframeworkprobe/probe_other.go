//go:build !ios

package main

// The probe only means something in an iOS package; elsewhere it builds
// and reports nothing.
func probeAnswer() int { return -1 }

func backgroundModes() []string { return nil }
