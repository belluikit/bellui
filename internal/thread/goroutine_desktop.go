//go:build !android && !ios && !mobile

package thread

// IsMainGoroutine returns true if called from the main goroutine.
func IsMainGoroutine() bool {
	return GoroutineID() == mainGoroutineID.Load()
}
