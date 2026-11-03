//go:build !android

package lang

func initRuntime() {}

// systemLocalesReady: the system can be asked at any time off Android. A
// variable so the tests can take the Android path.
var systemLocalesReady = func() bool { return true }
