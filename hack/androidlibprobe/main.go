// Command androidlibprobe live-verifies `bellui package` support for Android
// library dependencies in android/libs (#672): jars and AARs dexed into the
// APK, AAR resources linked with an R class per library package, and AAR
// manifests merged.
//
// It calls android/LibProbe.java two seconds after launch, which uses
// androidx.core (ContextCompat.getColor on a color defined in the core AAR,
// getString on an AAR string through androidx.core.R) and androidx.collection
// (a plain jar), and prints one line to stdout (logcat tag Bellui):
//
//	androidlibprobe ok color=#ff9e9e9e overflow=999+ arrayset=2
//
// Those values are androidx.core 1.3.2's own; any other value, an "error"
// line, or a crash (NoClassDefFoundError, Resources$NotFoundException) means
// a library was not dexed or its resources were not linked.
//
// Build and run (from this directory):
//
//	./fetch.sh
//	CGO_LDFLAGS= ANDROID_HOME=~/Library/Android/sdk ANDROID_NDK_HOME=~/Library/Android/sdk/ndk/26.3.11579264 \
//	  bellui package --target android/arm64 --app-id com.trendvidia.androidlibprobe
//	adb install -r androidlibprobe.apk
//	adb logcat -c && adb shell am start -n com.trendvidia.androidlibprobe/org.golang.app.GoNativeActivity
//	adb logcat -d -s Bellui | grep androidlibprobe
package main

import (
	"log"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("androidlibprobe")
	label := widget.NewLabel("androidlibprobe: waiting…")
	w.SetContent(label)

	go func() {
		time.Sleep(2 * time.Second)
		line := "androidlibprobe " + runProbe()
		log.Println(line)
		bellui.Do(func() { label.SetText(line) })
	}()
	bellui.ShowAndRun(w)
}
