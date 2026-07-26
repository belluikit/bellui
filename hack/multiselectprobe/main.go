package main

import (
	"log"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/dialog"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// #716 live probe: opens a native NSOpenPanel with multi-select enabled and logs
// the chosen files. Pick two or more and confirm the count/paths.
func main() {
	dialog.UseNativeFileDialogs(true)
	a := app.New()
	w := a.NewWindow("multiselect probe")
	open := func() {
		dialog.ShowFileOpenMultiple(func(readers []bellui.URIReadCloser, err error) {
			log.Printf("multi-select callback: %d files, err=%v", len(readers), err)
			for _, r := range readers {
				log.Printf("  %s", r.URI())
				_ = r.Close()
			}
		}, w)
	}
	w.SetContent(widget.NewButton("Open files…", open))
	w.Resize(geo.NewSize(360, 200))
	log.Println("probe up — click the button, pick multiple files")
	open() // present immediately for the screenshot
	bellui.ShowAndRun(w)
}
