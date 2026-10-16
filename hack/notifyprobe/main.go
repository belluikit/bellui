// Command notifyprobe is a live-verify harness for rich system notifications
// (pre-rebrand tracker #516, bellui.NotifierWithOptionsApp / SendNotificationWithOptions): click
// activation, an inline action button and a per-notification icon.
//
// Tapping "Send notification" posts a notification carrying a body-click
// callback and one "Reply" action button. When the OS delivers either callback
// the probe appends a line to its log file and updates the in-window status
// label — proving the callback arrived on the main goroutine while the app is
// still running.
//
// Usage:
//
//	go run ./hack/notifyprobe          # Linux/Windows (unbundled is fine on Linux)
//	bellui package && open notifyprobe.app   # macOS needs a bundled app to notify
//
// The log path is printed at startup (also under $TMPDIR/bellui-notifyprobe.log).
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

func main() {
	auto := flag.Bool("auto", false, "send one notification on startup and quit after a delay (scripted verify)")
	autoQuit := flag.Duration("quit", 25*time.Second, "in -auto mode, how long to run before quitting")
	flag.Parse()

	logPath := filepath.Join(os.TempDir(), "bellui-notifyprobe.log")
	logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		log.Fatal(err)
	}
	defer logf.Close()
	logger := log.New(logf, "", log.LstdFlags|log.Lmicroseconds)

	a := app.NewWithID("io.bellui.notifyprobe")
	w := a.NewWindow("Notify Probe")

	status := widget.NewLabel("No callback yet — send a notification, then click it.")
	status.Wrapping = bellui.TextWrapWord

	note := func(msg string) {
		logger.Println(msg)
		fmt.Println(msg)
		// Delivered on the main goroutine by the notification layer.
		status.SetText(msg + "\n(" + time.Now().Format("15:04:05.000") + ")")
	}

	count := 0
	sendRich := func() {
		count++
		n := count
		opts := &bellui.NotificationOptions{
			Title:    fmt.Sprintf("Probe notification #%d", n),
			Content:  "Click me, or use the Reply action.",
			Icon:     theme.WarningIcon(),
			OnTapped: func() { note(fmt.Sprintf("BODY CLICK on #%d", n)) },
			Actions: []bellui.NotificationAction{
				{Label: "Reply", Callback: func() { note(fmt.Sprintf("REPLY action on #%d", n)) }},
				{Label: "Later", Callback: func() { note(fmt.Sprintf("LATER action on #%d", n)) }},
			},
		}
		delivered := bellui.SendNotificationWithOptions(opts)
		note(fmt.Sprintf("sent #%d (rich=%v)", n, delivered))
	}

	send := widget.NewButton("Send notification", sendRich)
	plain := widget.NewButton("Send plain (control)", func() {
		bellui.SendNotification(bellui.NewNotification("Plain probe", "No callbacks here."))
		note("sent plain notification")
	})

	fmt.Println("notifyprobe: logging activations to", logPath)
	w.SetContent(container.NewVBox(
		widget.NewLabel("Rich notification probe (#516)"),
		send, plain,
		widget.NewSeparator(),
		status,
	))
	w.Resize(geo.NewSize(420, 260))

	if *auto {
		// Send once shortly after the loop starts, then quit after the delay so a
		// script can collect the log. Uses bellui.Do so the send runs on the main
		// goroutine once the event loop is pumping.
		go func() {
			time.Sleep(1500 * time.Millisecond)
			bellui.Do(sendRich)
			time.Sleep(*autoQuit)
			bellui.Do(func() { a.Quit() })
		}()
	}

	// Mobile has no argv (so -auto is unreachable) and headless button taps are
	// impossible against the GPU surface, so auto-send once a few seconds after
	// launch to make the Android/iOS notification path scriptable from adb.
	if bellui.CurrentDevice().IsMobile() {
		go func() {
			time.Sleep(3 * time.Second)
			bellui.Do(sendRich)
		}()
	}

	bellui.ShowAndRun(w)
}
