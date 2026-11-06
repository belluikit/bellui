//go:build darwin && !ci

// Package quitloop_test quits a real app while other goroutines post work to
// its main loop. An app runs once per process, so each run is a child process
// of the test.
package quitloop_test

import (
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
)

// childEnv marks the process as a child that runs one app and quits it.
const childEnv = "BELLUI_QUITLOOP_CHILD"

func TestMain(m *testing.M) {
	if os.Getenv(childEnv) == "1" {
		runAndQuit()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runAndQuit runs an app whose OnStopped and a few goroutines post work to the
// main loop, and quits it once the loop is running.
func runAndQuit() {
	a := app.New()
	var stop atomic.Bool
	a.Lifecycle().SetOnStopped(func() {})
	a.Lifecycle().SetOnStarted(func() {
		// Each Do wakes the loop, and posted back to back from four goroutines
		// the wakes outran the loop, so Quit never ran (#1023): pace them.
		for range 4 {
			go func() {
				for !stop.Load() {
					bellui.Do(func() {})
					time.Sleep(20 * time.Microsecond)
				}
			}()
		}
		go func() {
			time.Sleep(20 * time.Millisecond)
			bellui.Do(a.Quit)
		}()
	})
	a.Run()
	// Let the goroutines and OnStopped post past the shutdown before exiting.
	time.Sleep(50 * time.Millisecond)
	stop.Store(true)
}

// TestDoWhileQuitting pins #1021: a Do landing after the driver terminated
// GLFW woke the loop through glfw.PostEmptyEvent, which panics once GLFW is
// terminated ("NotInitialized: The GLFW library is not initialized").
func TestDoWhileQuitting(t *testing.T) {
	runs := 40
	if testing.Short() {
		runs = 5
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for i := range runs {
		cmd := exec.CommandContext(t.Context(), exe, "-test.run=^$")
		cmd.WaitDelay = time.Second
		cmd.Env = append(os.Environ(), childEnv+"=1")
		out, err := cmd.CombinedOutput()
		if err != nil || strings.Contains(string(out), "panic") {
			t.Fatalf("run %d of %d: %v\n%s", i+1, runs, err, out)
		}
	}
	t.Logf("%d runs quit with work posted, no panic", runs)
}
