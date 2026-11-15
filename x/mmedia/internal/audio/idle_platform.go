package audio

import "runtime"

// suspendWhenIdle is whether idleSuspend suspends the device on this platform.
var suspendWhenIdle = suspendsWhenIdle(runtime.GOOS)

// suspendsWhenIdle reports the platforms where an idle device is suspended:
// Android, where an idle device's stream would be reopened in a phone call's
// audio mode (#1052), and iOS, which shows ⏸ on the lock screen for as long
// as the app's audio queue runs, even playing silence (#1242).
func suspendsWhenIdle(goos string) bool {
	return goos == "android" || goos == "ios"
}
