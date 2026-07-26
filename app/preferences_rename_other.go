//go:build !windows && !wasm

package app

import "os"

// atomicRename moves oldpath onto newpath. On POSIX filesystems os.Rename is
// already atomic and succeeds even when the target is open, so no retry is
// needed.
func atomicRename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}
