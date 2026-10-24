package util

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func batchCommand(bat string, args []string) (*exec.Cmd, error) {
	line, err := batchCmdLine(bat, args)
	if err != nil {
		return nil, err
	}
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	}
	cmd := exec.Command(comspec)
	// Args stays as exec.Command set it; CmdLine replaces the line built from
	// it. The first token is the program name, as Windows passes it.
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `"` + comspec + `" ` + line}
	return cmd, nil
}
