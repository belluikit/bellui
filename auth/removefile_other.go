//go:build !windows

package auth

import "syscall"

// removeFile deletes the file at path. Unlike os.Remove, it never removes a
// directory.
func removeFile(path string) error {
	return syscall.Unlink(path)
}
