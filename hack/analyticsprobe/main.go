// Command analyticsprobe live-verifies the analytics battery model (pre-rebrand tracker #699,
// follow-up to pre-rebrand tracker #526/#527) on a real device. Instead of a real backend +
// mitmproxy, it wires a custom Sink that logs every batch to logcat with its
// size and timestamp — so the batch CADENCE (batched, not per-event) is visible
// directly:
//
//	adb logcat | grep analyticsprobe
//
// Checks, all observable in the log:
//   - Consent gate: fire events BEFORE "Enable consent" → NO batch is sent.
//   - Batching by size: WithBatchSize(30); "Fire 25" twice → one batch, not 50.
//   - Explicit Flush.
//   - Background-edge flush: press Home → Attach's SetOnExitedForeground fires a
//     batch (session_end included) — the workhorse on mobile.
//   - Auto-capture (pre-rebrand tracker #527): taps become "ui.activate" and coalesce into batches.
//   - Spool drain: events fired before consent are spooled; relaunch + enable
//     drains them oldest-first.
package main

import (
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/analytics"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// logSink records each delivered batch to logcat. This is the observation point:
// one line per Sink.Send == one network request in a real sink.
type logSink struct {
	batches atomic.Int64
	events  atomic.Int64
}

func (s *logSink) Send(batch []Event) error {
	n := s.batches.Add(1)
	s.events.Add(int64(len(batch)))
	names := make([]string, 0, len(batch))
	for i, e := range batch {
		if i >= 4 {
			names = append(names, "…")
			break
		}
		if e.Name != "" {
			names = append(names, e.Name)
		} else {
			names = append(names, string(e.Kind))
		}
	}
	log.Printf("analyticsprobe: BATCH #%d size=%d names=%v (total events sent=%d)", n, len(batch), names, s.events.Load())
	return nil
}

// Event is analytics.Event; aliased so the Sink signature reads cleanly.
type Event = analytics.Event

func main() {
	a := app.NewWithID("io.bellui.analyticsprobe")
	w := a.NewWindow("Analytics cadence probe (#699)")

	sink := &logSink{}
	tracker := analytics.New(
		sink,
		analytics.WithBatchSize(30),
		analytics.WithInterval(60*time.Second),
	)
	analytics.Attach(tracker, a.Lifecycle()) // background-edge flush + session events
	analytics.SetDefault(tracker)
	analytics.EnableAutoCapture(tracker) // taps → ui.activate

	fired := 0
	status := widget.NewLabel("consent: OFF — fire events, watch for NO batch until enabled")
	status.Wrapping = bellui.TextWrapWord
	refresh := func(msg string) {
		bellui.Do(func() {
			status.SetText(fmt.Sprintf("%s\nfired=%d  batches=%d  sent=%d",
				msg, fired, sink.batches.Load(), sink.events.Load()))
		})
	}

	fire := func(n int) {
		for i := 0; i < n; i++ {
			fired++
			tracker.Track("btn.fire", map[string]any{"i": fired})
		}
		log.Printf("analyticsprobe: fired %d events (cumulative=%d) — batch on 30-boundary only", n, fired)
		refresh(fmt.Sprintf("fired %d events", n))
	}

	w.SetContent(container.NewVBox(
		widget.NewLabelWithStyle("Analytics batch-cadence probe (#699)", bellui.TextAlignCenter, bellui.TextStyle{Bold: true}),
		status,
		widget.NewSeparator(),
		widget.NewButton("Enable consent (SetEnabled true)", func() {
			analytics.SetEnabled(true)
			log.Print("analyticsprobe: consent ENABLED — spool drains, batching begins")
			refresh("consent ENABLED")
		}),
		widget.NewButton("Fire 25 events", func() { fire(25) }),
		widget.NewButton("Fire 25 more (cross 30-boundary)", func() { fire(25) }),
		widget.NewButton("Flush now", func() {
			log.Print("analyticsprobe: explicit Flush()")
			go func() { _ = analytics.Flush(); refresh("flushed") }()
		}),
		widget.NewButton("Disable consent (drops queue+spool)", func() {
			analytics.SetEnabled(false)
			log.Print("analyticsprobe: consent DISABLED")
			refresh("consent DISABLED")
		}),
		widget.NewLabel("Then press Android HOME → background-edge flush fires a batch.\nTaps on these buttons also auto-capture as ui.activate."),
	))
	w.Resize(geo.NewSize(680, 760))
	log.Print("analyticsprobe: ready (consent OFF by construction)")
	bellui.ShowAndRun(w)
}
