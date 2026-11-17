package source

// errnoENOSYS is the C runtime's ENOSYS, which FFmpeg negates into
// AVERROR(ENOSYS). syscall.ENOSYS on Windows is an invented Go value, not the
// CRT's (goffmpeg's errnoEAGAIN explains the same trap).
const errnoENOSYS = 40
