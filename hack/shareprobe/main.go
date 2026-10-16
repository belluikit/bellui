// Command shareprobe verifies the mobile intent/share integration:
//   - pre-rebrand tracker #711 share-out: "Share Text" / "Share File" hand content to the OS share
//     sheet via bellui.ShareText / bellui.ShareFile (Android ACTION_SEND chooser via
//     BelluiFileProvider for files / iOS UIActivityViewController).
//   - pre-rebrand tracker #622 open-with: a registered bellui.OpenHandlerApp receives file-association
//     and custom-URL-scheme ("shareprobe://") launches; the received payload is
//     shown in the UI. Drive it on Android with:
//     adb shell am start -a android.intent.action.VIEW \
//     -d "shareprobe://hello" io.bellui.shareprobe
package main

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("shareprobe")
	status := widget.NewLabel("tap a button")
	opened := widget.NewLabel("open-with: (none yet)")

	// pre-rebrand tracker #622 — receive "open with" file / deep-link launches.
	if oh, ok := a.(bellui.OpenHandlerApp); ok {
		oh.SetOnFileOpened(func(u bellui.URI) {
			opened.SetText("FILE opened: " + u.String())
		})
		oh.SetOnURLOpened(func(u *url.URL) {
			opened.SetText("URL opened: " + u.String())
		})
	}

	shareText := widget.NewButton("Share Text", func() {
		if err := bellui.ShareText("Hello from bellui #711 — shared via the OS share sheet"); err != nil {
			status.SetText("TEXT ERROR: " + err.Error())
		} else {
			status.SetText("text shared — the OS chooser should have opened")
		}
	})

	shareFile := widget.NewButton("Share File", func() {
		path := filepath.Join(os.TempDir(), "bellui-share-demo.txt")
		if err := os.WriteFile(path, []byte("shared file contents from bellui #711\n"), 0o600); err != nil {
			status.SetText("WRITE ERROR: " + err.Error())
			return
		}
		if err := bellui.ShareFile(storage.NewFileURI(path)); err != nil {
			status.SetText("FILE ERROR: " + err.Error())
		} else {
			status.SetText("file shared — the OS chooser should have opened")
		}
	})

	w.SetContent(container.NewVBox(
		widget.NewLabel("Share + open-with probe (#711/#622)"),
		shareText, shareFile, status, opened,
	))
	w.Resize(geo.NewSize(400, 320))

	// iOS has no adb-style touch injection, so drive the two flows on a timer:
	//   t+3s — self-deliver a custom-scheme URL. iOS routes shareprobe:// back to
	//          this app's application:openURL: (it is the registered handler), so
	//          this exercises the exact pre-rebrand tracker #622 receiving path end-to-end.
	//   t+9s — auto-open the share sheet for a file (pre-rebrand tracker #711).
	if runtime.GOOS == "ios" {
		time.AfterFunc(3*time.Second, func() {
			bellui.Do(func() {
				if u, err := url.Parse("shareprobe://ios-selftest?x=1"); err == nil {
					_ = bellui.OpenURL(u)
				}
			})
		})
		time.AfterFunc(9*time.Second, func() {
			bellui.Do(func() {
				path := filepath.Join(os.TempDir(), "bellui-share-demo.txt")
				if err := os.WriteFile(path, []byte("shared file contents from bellui #711 (iOS)\n"), 0o600); err != nil {
					status.SetText("WRITE ERROR: " + err.Error())
					return
				}
				if err := bellui.ShareFile(storage.NewFileURI(path)); err != nil {
					status.SetText("FILE ERROR: " + err.Error())
				} else {
					status.SetText("auto ShareFile fired — chooser should have opened")
				}
			})
		})
	}

	bellui.ShowAndRun(w)
}
