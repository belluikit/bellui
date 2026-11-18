// Command assetprobe live-verifies that an app reads both kinds of APK asset
// `bellui package` writes (#1285): a text asset, now deflated, and a media
// asset, still stored.
//
// gen.sh writes assets/THIRD_PARTY_NOTICES.txt (a real app's notices when
// given one, else license text from the Go module cache) and
// assets/media/tone.ogg, and prints each file's size and CRC-32. Two seconds
// after launch the probe reads both through the asset API twice, from Java
// (AssetManager.open) and from native code (AAsset_read), tries openFd on
// each, and prints one line to stdout (logcat tag Bellui):
//
//	assetprobe ok notices java=<size>/<crc> native=<size>/<crc> fd=compressed tone java=<size>/<crc> native=<size>/<crc> fd=<length>
//
// The sizes and CRCs must match what gen.sh printed. fd=compressed for the
// notices shows the entry really is deflated on the device (openFd refuses a
// compressed entry); a number for the tone shows a stored asset can still be
// opened as a file descriptor, which media playback needs.
//
// Build and run (from this directory):
//
//	./gen.sh [notices.txt]
//	CGO_LDFLAGS= ANDROID_HOME=~/Library/Android/sdk ANDROID_NDK_HOME=~/Library/Android/sdk/ndk/26.3.11579264 \
//	  bellui package --target android/arm64 --app-id com.trendvidia.assetprobe
//	unzip -lv assetprobe.apk | grep assets/
//	adb install -r assetprobe.apk
//	adb logcat -c && adb shell am start -n com.trendvidia.assetprobe/org.golang.app.GoNativeActivity
//	adb logcat -d -s Bellui | grep assetprobe
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
	w := a.NewWindow("assetprobe")
	label := widget.NewLabel("assetprobe: waiting…")
	label.Wrapping = bellui.TextWrapWord
	w.SetContent(label)

	go func() {
		time.Sleep(2 * time.Second)
		line := "assetprobe " + runProbe()
		log.Println(line)
		bellui.Do(func() { label.SetText(line) })
	}()
	bellui.ShowAndRun(w)
}
