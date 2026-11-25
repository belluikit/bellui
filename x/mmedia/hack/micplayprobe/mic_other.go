//go:build !ios

package main

import "errors"

func micPermission() bool { return false }

func micStart() error { return errors.New("micplayprobe captures only on iOS") }

func micStop() {}

func readMic() micStats { return micStats{} }

func sessionCategory() string { return "n/a" }

func sessionOutputs() string { return "n/a" }
