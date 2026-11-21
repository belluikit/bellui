//go:build !windows

package commands

import "syscall"

// processAlive reports whether a process with this PID exists. Run reaps the
// fake cloudflared it started, so a stopped one does not linger as a zombie.
func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
