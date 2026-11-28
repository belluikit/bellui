// Command pushprobe is the live-verify harness for iOS remote push (#1517,
// bellui.PushApp): permission, the APNs device token, a tapped push on a cold
// launch and while running, and a push that arrives in the foreground.
//
// At start it asks for permission and registers, and prints each answer. The
// tapped-push callback is set two seconds after the app starts, so a tap that
// launched the app has to be held for it. A foreground push is shown when its
// payload has "show": true, and hidden otherwise.
//
// Every event is a "pushprobe:" line on stdout (`xcrun simctl launch
// --console`, `devicectl device process launch --console`) and in
// $TMPDIR/pushprobe.log inside the app's container.
//
// Simulator:
//
//	bellui package -os iossimulator -app-id com.trendvidia.pushprobe
//	xcrun simctl install booted pushprobe.app
//	xcrun simctl launch --console booted com.trendvidia.pushprobe
//	xcrun simctl push booted com.trendvidia.pushprobe payload.apns
//
// payload.apns: {"aps":{"alert":{"title":"T","body":"B"},"sound":"default"},"show":true,"data":{"k":"v"}}
//
// With -local (`xcrun simctl launch … com.trendvidia.pushprobe -local`) it
// also sends one of the app's own rich notifications three seconds after
// start, with a body tap and a "Reply" action. Those taps reach the app's
// callbacks through the same app delegate since #1517, and must not reach
// the push callbacks.
//
// A device build needs a provisioning profile with Push Notifications, and a
// push sent through APNs's sandbox with the app's key.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/widget"
)

func main() {
	local := flag.Bool("local", false, "send a rich local notification 3s after start")
	asks := flag.Int("asks", 1, "how many permission requests to make at once at start")
	flag.Parse()
	a := app.NewWithID("com.trendvidia.pushprobe")
	w := a.NewWindow("Push probe")

	logFile, _ := os.OpenFile(filepath.Join(os.TempDir(), "pushprobe.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	lines := widget.NewLabel("")
	lines.Wrapping = bellui.TextWrapWord
	var shown []string
	logf := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		line := time.Now().Format("15:04:05.000") + " pushprobe: " + msg
		fmt.Println(line)
		if logFile != nil {
			fmt.Fprintln(logFile, line)
		}
		bellui.Do(func() {
			shown = append(shown, msg)
			if len(shown) > 14 {
				shown = shown[1:]
			}
			lines.SetText(strings.Join(shown, "\n"))
		})
	}
	describe := func(n *bellui.PushNotification) string {
		data, _ := json.Marshal(n.Payload)
		return fmt.Sprintf("title=%q body=%q payload=%s", n.Title, n.Body, data)
	}

	_, isPush := a.(bellui.PushApp)
	logf("start pid=%d pushapp=%v", os.Getpid(), isPush)

	ask := func() {
		bellui.RequestNotificationPermission(func(granted bool, err error) {
			logf("permission granted=%v err=%v", granted, err)
		})
	}
	register := func() {
		bellui.RegisterForPush(func(token string) {
			logf("token %s (%d hex)", token, len(token))
		}, func(err error) {
			logf("register error: %v", err)
		})
	}
	bellui.SetOnPushReceived(func(n *bellui.PushNotification) bool {
		show, _ := n.Payload["show"].(bool)
		logf("received show=%v %s", show, describe(n))
		return show
	})
	// Late on purpose: a tap that launched the app arrives before this, and
	// must be held for it.
	time.AfterFunc(2*time.Second, func() {
		bellui.Do(func() {
			bellui.SetOnPushOpened(func(n *bellui.PushNotification) {
				logf("opened %s", describe(n))
			})
			logf("opened callback set")
		})
	})

	w.SetContent(container.NewBorder(
		container.NewHBox(
			widget.NewButton("Ask permission", ask),
			widget.NewButton("Register", register),
		), nil, nil, nil,
		container.NewVScroll(lines),
	))
	a.Lifecycle().SetOnStarted(func() {
		for range *asks {
			ask()
		}
		register()
	})
	if *local {
		time.AfterFunc(3*time.Second, func() {
			rich := bellui.SendNotificationWithOptions(&bellui.NotificationOptions{
				Title:    "LocalRich",
				Content:  "the app's own notification",
				OnTapped: func() { logf("local tapped") },
				Actions: []bellui.NotificationAction{
					{Label: "Reply", Callback: func() { logf("local action Reply") }},
				},
			})
			logf("local sent rich=%v", rich)
		})
	}
	bellui.ShowAndRun(w)
}
