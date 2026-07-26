//go:build windows

package app

import (
	"os"
	"time"
)

// renameRetries and renameRetryDelay bound how long atomicRename waits for a
// transient lock on the target to clear before giving up (~250ms total).
const (
	renameRetries    = 25
	renameRetryDelay = 10 * time.Millisecond
)

// atomicRename moves oldpath onto newpath, retrying on Windows. Unlike POSIX,
// os.Rename (MoveFileEx) fails with "Access is denied" while another handle has
// the target open — which happens transiently when a directory watcher
// (fsnotify) or antivirus reads the file as it is replaced. The replacement is
// still effectively atomic; we just wait briefly for the foreign handle to close.
func atomicRename(oldpath, newpath string) error {
	var err error
	for i := 0; i < renameRetries; i++ {
		if err = os.Rename(oldpath, newpath); err == nil {
			return nil
		}
		time.Sleep(renameRetryDelay)
	}
	return err
}
