//go:build darwin && !ci

// Package realloop_test runs widgets under a real app and its draw loop. The
// glfw package's own tests run with the test driver as the current app, so a
// change that asks the app's windows for a frame cannot be seen there.
package realloop_test

import (
	"os"
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

var testApp bellui.App

func TestMain(m *testing.M) {
	testApp = app.New()
	code := make(chan int, 1)
	go func() {
		code <- m.Run()
		testApp.Quit()
	}()
	testApp.Run() // the draw loop needs the process's main goroutine
	os.Exit(<-code)
}

// TestShownLaterIsLaidOut pins player#278: an object hidden before its
// window's first frame was never placed, so no canvas knows it and its
// refresh reaches no window. Shown later, no frame ran to lay it out, and it
// sat at size 0 in its row: a button's centred label hung off the row's left
// edge. A widget and a container behave alike. The window shows nothing else
// that changes, so nothing else asks for a frame.
func TestShownLaterIsLaidOut(t *testing.T) {
	for _, c := range []struct {
		name string
		make func() (show core.CanvasObject, sized *widget.Button)
	}{
		{"widget", func() (core.CanvasObject, *widget.Button) {
			b := widget.NewButton("Older", nil)
			return b, b
		}},
		{"container", func() (core.CanvasObject, *widget.Button) {
			b := widget.NewButton("Older", nil)
			return container.NewHBox(b), b
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			newer := widget.NewButton("Newer", nil)
			older, sized := c.make()
			var w bellui.Window
			bellui.DoAndWait(func() {
				newer.Hide()
				older.Hide()
				w = testApp.NewWindow("shown later")
				w.SetContent(container.NewVBox(container.NewHBox(newer, older), widget.NewLabel("list")))
				w.Resize(geo.NewSize(360, 200))
				w.Show()
			})
			defer bellui.DoAndWait(w.Hide)     // closing the last window quits the app
			time.Sleep(300 * time.Millisecond) // frames with older hidden

			bellui.DoAndWait(older.Show)
			var size, min geo.Size
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				time.Sleep(50 * time.Millisecond)
				bellui.DoAndWait(func() { size, min = sized.Size(), sized.MinSize() })
				if size == min {
					return
				}
			}
			t.Fatalf("Older is %v after it was shown, want its min size %v: no frame laid it out", size, min)
		})
	}
}
