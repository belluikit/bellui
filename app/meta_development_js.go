//go:build js

package app

import "os"

// getProjectPath returns the working directory directly on js/wasm, where
// os.Executable() is never implemented (it always returns an error). The
// desktop variant logs that error; here it would fire on every app launch in
// the browser, so we skip the lookup entirely.
func getProjectPath() string {
	work, _ := os.Getwd()
	return work
}
