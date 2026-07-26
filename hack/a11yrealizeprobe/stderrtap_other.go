//go:build !darwin

package main

import "os"

// tapStderr is unavailable off darwin/iOS: the probe's on-device trace pane
// stays empty and the desktop/UIA/AT-SPI drive paths are unaffected.
func tapStderr() (*os.File, func(string), bool) { return nil, nil, false }
