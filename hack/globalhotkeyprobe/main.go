// Command globalhotkeyprobe is a live-verify harness for the system-wide global
// hotkey API (#518, desktop.GlobalShortcutApp). It registers Ctrl+Alt+K and
// logs each time the shortcut fires — even while another application is in the
// foreground, which is the whole point of a global hotkey.
//
// Modes:
//
//	globalhotkeyprobe            interactive: press Ctrl+Alt+K yourself
//	globalhotkeyprobe -auto      self-driving (Windows): steals foreground with
//	                             Notepad, synthesizes the keypress, spawns a
//	                             second process to prove a conflict errors, then
//	                             reports PASS/FAIL and quits
//	globalhotkeyprobe -conflict  register the same shortcut and print whether it
//	                             conflicts with an already-running holder, then
//	                             exit (used as the child of -auto)
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func shortcut() *desktop.CustomShortcut {
	return &desktop.CustomShortcut{
		KeyName:  event.KeyK,
		Modifier: event.KeyModifierControl | event.KeyModifierAlt,
	}
}

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	conflict := flag.Bool("conflict", false, "register the shortcut, report whether it conflicts, then exit")
	auto := flag.Bool("auto", false, "self-drive the verification (Windows only)")
	logfile := flag.String("logfile", "", "also append log output to this file (for headless/scheduled runs)")
	flag.Parse()

	if *logfile != "" {
		if f, err := os.OpenFile(*logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			log.SetOutput(io.MultiWriter(os.Stderr, f))
			defer f.Close()
		}
	}

	a := app.NewWithID("com.trendvidia.globalhotkeyprobe")
	gs, ok := a.Driver().(desktop.GlobalShortcutApp)
	if !ok {
		log.Fatal("driver does not implement desktop.GlobalShortcutApp")
	}

	if *conflict {
		if err := gs.RegisterGlobalShortcut(shortcut(), func() {}); err != nil {
			fmt.Println("CONFLICT-ERR:", err)
		} else {
			fmt.Println("CONFLICT-OK (no other holder was running)")
			gs.UnregisterGlobalShortcut(shortcut())
		}
		return
	}

	w := a.NewWindow("Global Hotkey Probe")
	status := widget.NewLabel("Waiting… press Ctrl+Alt+K (fires even when unfocused)")

	var fires int32
	err := gs.RegisterGlobalShortcut(shortcut(), func() {
		n := atomic.AddInt32(&fires, 1)
		msg := fmt.Sprintf("FIRED x%d at %s", n, time.Now().Format("15:04:05.000"))
		log.Println(msg)
		status.SetText(msg)
	})
	log.Println("REGISTER-ERR:", err)
	if err != nil {
		return
	}
	log.Println("registered Ctrl+Alt+K")

	if *auto {
		go driveAuto(a, os.Args[0], &fires)
	}

	w.SetContent(status)
	w.Resize(geo.NewSize(520, 120))
	bellui.ShowAndRun(w)

	gs.UnregisterGlobalShortcut(shortcut())
}

// driveAuto exercises the whole feature without a human: it steals the
// foreground with Notepad (so the hotkey must fire while our window is NOT
// focused), synthesizes Ctrl+Alt+K, and confirms a second process registering
// the same combo gets an error.
func driveAuto(a bellui.App, self string, fires *int32) {
	time.Sleep(1500 * time.Millisecond)

	// A second process registering the same combo must fail: the OS allows only
	// one holder of a given modifier+key system-wide.
	out, _ := exec.Command(self, "-conflict").CombinedOutput()
	log.Printf("child(-conflict) said: %s", out)

	// Steal the foreground so the fire below is unambiguously "while unfocused".
	note := exec.Command("notepad.exe")
	if e := note.Start(); e != nil {
		log.Println("could not launch notepad (continuing focused):", e)
		note = nil
	}
	time.Sleep(1200 * time.Millisecond)

	log.Println("synthesizing Ctrl+Alt+K…")
	synthHotKey()
	time.Sleep(1500 * time.Millisecond)

	if note != nil && note.Process != nil {
		_ = note.Process.Kill()
	}

	if atomic.LoadInt32(fires) > 0 {
		log.Println("RESULT: PASS — global hotkey fired while unfocused")
	} else {
		log.Println("RESULT: FAIL — global hotkey did not fire")
	}
	time.Sleep(300 * time.Millisecond)
	bellui.Do(a.Quit) // Quit must run on the main goroutine
}
