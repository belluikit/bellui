// Command filedialogprobe live-verifies the opt-in native file dialogs (#514).
//
// It builds a small window with buttons that open a native File-Open,
// Folder-Open and File-Save panel (dialog.UseNativeFileDialogs is enabled),
// logging the chosen URI to stdout. On macOS the panels are NSOpenPanel /
// NSSavePanel; on Linux/BSD the xdg portal; on Windows the Common Item Dialog
// (IFileOpenDialog / IFileSaveDialog).
//
//	go run ./hack/filedialogprobe            # button UI
//	go run ./hack/filedialogprobe open       # auto-open a File-Open panel
//	go run ./hack/filedialogprobe folder     # auto-open a Folder-Open panel
//	go run ./hack/filedialogprobe save       # auto-open a File-Save panel
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
		dialog.ShowFileOpen(func(r bellui.URIReadCloser, err error) {
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
		dialog.ShowFolderOpen(func(l bellui.ListableURI, err error) {
			if l == nil {
				fmt.Println("folder: cancelled or error:", err)
				return
			}
			fmt.Println("folder:", l)
		}, w)
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
		}
	}

	w.SetContent(container.NewVBox(
		widget.NewButton("Open File", openFile),
		widget.NewButton("Open File (image filter)", openFileFiltered),
		widget.NewButton("Open Multiple (Ctrl/Cmd+click, Shift+click)", openMultiple),
		widget.NewButton("Open Folder", openFolder),
		widget.NewButton("Save File", saveFile),
	))
	w.Resize(geo.NewSize(320, 240))
	w.Show()
	a.Run()
}
