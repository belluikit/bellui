// Command iosframeworkprobe live-verifies the ios/ packaging conventions of
// `bellui package -os ios` (#672) on a device:
//
//   - ios/Info.plist merges a non-string value: UIBackgroundModes = [audio];
//   - ios/Entitlements.plist signs in a keychain access group;
//   - ios/Frameworks/Probe.xcframework, a dynamic framework, is linked,
//     embedded in the app's Frameworks/ and signed. The app calls into it
//     through cgo, so a launch that prints the line below proves dyld found
//     and loaded the embedded framework.
//
// It prints one greppable line to stdout and shows it on screen:
//
//	iosframeworkprobe probe_answer=42 background_modes=[audio]
//
// Build the framework (it is not committed), then package from this
// directory:
//
//	./build-framework.sh
//	bellui package -os ios --app-id com.trendvidia.iosframeworkprobe \
//	    --cert "Apple Development: Benedicto Franco Jr. (D68Z8W925S)"
//
// Inspect the bundle without a device:
//
//	plutil -p iosframeworkprobe.app/Info.plist | grep -A2 UIBackgroundModes
//	codesign -d --entitlements - --xml iosframeworkprobe.app | plutil -p -
//	ls iosframeworkprobe.app/Frameworks
//	otool -L iosframeworkprobe.app/main | grep Probe
//
// Run on a device:
//
//	xcrun devicectl device install app --device <UDID> iosframeworkprobe.app
//	xcrun devicectl device process launch --console --device <UDID> com.trendvidia.iosframeworkprobe
package main

import (
	"fmt"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("iosframeworkprobe")
	line := fmt.Sprintf("iosframeworkprobe probe_answer=%d background_modes=%v", probeAnswer(), backgroundModes())
	fmt.Println(line)
	w.SetContent(widget.NewLabel(line))
	bellui.ShowAndRun(w)
}
