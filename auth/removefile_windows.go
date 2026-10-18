package auth

import "syscall"

// removeFile deletes the file at path. Unlike os.Remove, it never removes a
// directory.
func removeFile(path string) error {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return syscall.DeleteFile(p)
}
