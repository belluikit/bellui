package widget

import "syscall"

// hideTestFile marks the file hidden: Windows uses the hidden file attribute
// rather than the dot-prefix convention.
func hideTestFile(path string) error {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return syscall.SetFileAttributes(p, syscall.FILE_ATTRIBUTE_HIDDEN)
}
