//go:build !windows

package source

import "syscall"

// errnoENOSYS is C's ENOSYS, which FFmpeg negates into AVERROR(ENOSYS): 38 on
// Linux and Android, 78 on macOS and iOS.
const errnoENOSYS = int(syscall.ENOSYS)
