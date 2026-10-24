//go:build !windows

package util

import "os/exec"

// batchCommand runs a batch file as any other program: only Windows routes
// one through cmd.exe. The argument check still applies, so a refusal shows
// up in tests on every OS.
func batchCommand(bat string, args []string) (*exec.Cmd, error) {
	if _, err := batchCmdLine(bat, args); err != nil {
		return nil, err
	}
	return exec.Command(bat, args...), nil
}
