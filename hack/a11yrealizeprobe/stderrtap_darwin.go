//go:build darwin

package main

import (
	"os"
	"syscall"
)

// tapStderr redirects fd 2 into a pipe and returns the read end plus an echo
// writer for the original stderr, so NSLog/stderr trace lines can be both
// shown in the on-device console pane and passed through to an attached
// devicectl console. darwin/iOS only (GOOS=ios satisfies the darwin tag) —
// the dup2 dance has no portable equivalent and the pane only matters for the
// on-device screen-reader passes.
func tapStderr() (r *os.File, echo func(string), ok bool) {
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, nil, false
	}
	orig, _ := syscall.Dup(2)
	syscall.Dup2(int(pw.Fd()), 2)
	echo = func(line string) {
		if orig > 0 {
			syscall.Write(orig, []byte(line+"\n"))
		}
	}
	return pr, echo, true
}
