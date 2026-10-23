//go:build !ios && !android

package main

import "errors"

func startAudio() error { return errors.New("bgaudioprobe only plays audio on iOS") }

func stopAudio() {}

func audioStats() stats { return stats{state: "n/a"} }
