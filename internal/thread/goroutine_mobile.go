//go:build android || ios || mobile

package thread

// IsMainGoroutine returns true if called from the main goroutine.
// On mobile, if the main goroutine ID has not been set yet (zero), it returns true
// to avoid blocking during early initialization.
func IsMainGoroutine() bool {
	routineID := mainGoroutineID.Load()
	return routineID == 0 || GoroutineID() == routineID
}
