// Command filedialogprobe live-verifies the platform-native file dialogs
// (pre-rebrand tracker #514), default-on since #226.
//
// It builds a small window with buttons that open a native File-Open,
// Folder-Open and File-Save panel (the native backends start enabled; the
// "Native pickers" check toggles them), logging the chosen URI to stdout.
// On macOS the panels are NSOpenPanel / NSSavePanel; on Linux/BSD the xdg
// portal; on Windows the Common Item Dialog (IFileOpenDialog /
// IFileSaveDialog); on iOS/Android the system document pickers.
//
//	go run ./hack/filedialogprobe            # button UI
//	go run ./hack/filedialogprobe open       # auto-open a File-Open panel
//	go run ./hack/filedialogprobe folder     # auto-open a Folder-Open panel
//	go run ./hack/filedialogprobe save       # auto-open a File-Save panel
//	go run ./hack/filedialogprobe multiple   # auto-open a multi-select File-Open panel
//	go run ./hack/filedialogprobe windowed   # windowed canvas dialog (frame X + resize corner)
//
// The button UI carries a "Native pickers" check bound to
// dialog.UseNativeFileDialogs: unchecking it must serve every request with
// the canvas dialog — on iOS/Android too (#267), which is how #259's
// long-press selection mode is reached on-device.
package main

import (
	"fmt"
	"os"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/dialog"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/widget"
)

func main() {
	dialog.UseNativeFileDialogs(true)

	a := app.NewWithID("com.trendvidia.filedialogprobe")
	w := a.NewWindow("Native File Dialog Probe")

	imgFilter := storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"})

	openFile := func() {
		d := dialog.NewFileOpen(func(r bellui.URIReadCloser, err error) {
			if err != nil {
				fmt.Println("open error:", err)
				return
			}
			if r == nil {
				fmt.Println("open: cancelled")
				return
			}
			fmt.Println("open:", r.URI())
			_ = r.Close()
		}, w)
		// The close report must fire on every backend — canvas, desktop native
		// (#227), and the mobile/wasm picker overrides (#233) — immediately
		// before the result callback prints.
		d.SetOnClosed(func() { fmt.Println("open: onClosed fired") })
		d.Show()
	}
	openFileFiltered := func() {
		d := dialog.NewFileOpen(func(r bellui.URIReadCloser, err error) {
			if r == nil {
				fmt.Println("open(filtered): cancelled or error:", err)
				return
			}
			fmt.Println("open(filtered):", r.URI())
			_ = r.Close()
		}, w)
		d.SetFilter(imgFilter)
		d.Show()
	}
	openFolder := func() {
		d := dialog.NewFolderOpen(func(l bellui.ListableURI, err error) {
			if l == nil {
				fmt.Println("folder: cancelled or error:", err)
				return
			}
			fmt.Println("folder:", l)
		}, w)
		d.SetOnClosed(func() { fmt.Println("folder: onClosed fired") })
		d.Show()
	}
	saveFile := func() {
		d := dialog.NewFileSave(func(wr bellui.URIWriteCloser, err error) {
			if wr == nil {
				fmt.Println("save: cancelled or error:", err)
				return
			}
			fmt.Println("save:", wr.URI())
			_ = wr.Close()
		}, w)
		d.SetFileName("untitled.txt")
		// On wasm the save "dialog" is the download offered at writer close, so
		// this fires after the save: line, not before it (#233).
		d.SetOnClosed(func() { fmt.Println("save: onClosed fired") })
		d.Show()
	}

	openMultiple := func() {
		dialog.ShowFileOpenMultiple(func(readers []bellui.URIReadCloser, err error) {
			if err != nil {
				fmt.Println("open(multiple) error:", err)
				return
			}
			fmt.Printf("open(multiple): %d file(s)\n", len(readers))
			for _, r := range readers {
				fmt.Println("   ", r.URI())
				_ = r.Close()
			}
		}, w)
	}

	// The canvas dialogs default to windowed mode, framed by an InnerWindow —
	// on mobile the frame's close (X) button is the #254 target and the
	// bottom-right resize corner is the #263 target. The result logs either
	// way, so a dead X shows up as a dialog that will not leave; a dead
	// corner as a frame that will not grow.
	windowedDialog := func() {
		d := dialog.NewConfirm("Windowed dialog probe",
			"1. Drag the bottom-right corner — the frame must resize (#263).\n"+
				"2. Tap the frame's X button — the dialog must dismiss\nand log confirmed=false (#254).",
			func(ok bool) {
				fmt.Printf("windowed dialog closed: confirmed=%v\n", ok)
			}, w)
		d.Show()
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "open":
			openFile()
		case "folder":
			openFolder()
		case "save":
			saveFile()
		case "multiple":
			openMultiple()
		case "windowed":
			windowedDialog()
		}
	}

	// See the package doc: every button below must be served by the canvas
	// dialog while this is unchecked, on iOS/Android too (#267).
	nativeCheck := widget.NewCheck("Native pickers (UseNativeFileDialogs)", func(on bool) {
		dialog.UseNativeFileDialogs(on)
		fmt.Println("UseNativeFileDialogs:", on)
	})
	nativeCheck.SetChecked(true)

	w.SetContent(container.NewVBox(
		nativeCheck,
		widget.NewButton("Open File", openFile),
		widget.NewButton("Open File (image filter)", openFileFiltered),
		widget.NewButton("Open Multiple (Ctrl/Cmd+click, Shift+click)", openMultiple),
		widget.NewButton("Open Folder", openFolder),
		widget.NewButton("Save File", saveFile),
		widget.NewButton("Windowed Dialog (frame X button)", windowedDialog),
	))
	w.Resize(geo.NewSize(320, 240))
	w.Show()
	a.Run()
}
