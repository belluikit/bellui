package lang

import (
	"github.com/belluikit/bellui/internal/driver/mobile/app"
	"github.com/belluikit/bellui/internal/driver/mobile/mobileinit"

	"github.com/jeandeaual/go-locale"
)

func initRuntime() {
	locale.SetRunOnJVM(app.RunOnJVM)
}

// systemLocalesReady is false until the JVM is attached, which happens in
// JNI_OnLoad, after package init has run. Asking for the locales earlier
// failed with "no current JVM" and logged an error on every launch (#892);
// the toolkit asks again once the JVM is there ([ResolveSystemLocale], #282).
var systemLocalesReady = mobileinit.HasJVM
