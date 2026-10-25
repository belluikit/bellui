// Package component provides the timeline playback controller.
package component

import (
	"time"

	"github.com/belluikit/bellui"
)

// TimelineConfig configures timeline playback through a sequence of chart states.
type TimelineConfig struct {
	// Interval between auto-advance steps (default 2s).
	Interval time.Duration
	// AutoPlay starts playback automatically (default false).
	AutoPlay bool
	// Loop restarts from the beginning after reaching the end (default false).
	Loop bool
}

// TimelineState manages playback state.
type TimelineState struct {
	Config  TimelineConfig
	Current int  // current step index
	Total   int  // total number of steps
	Playing bool // auto-advancing
	ticker  *time.Ticker
	stopCh  chan struct{}
	onStep  func(index int) // callback to apply step
}

// NewTimelineState creates a timeline state with the given step count and callback.
func NewTimelineState(total int, cfg TimelineConfig, onStep func(int)) *TimelineState {
	if cfg.Interval <= 0 {
		cfg.Interval = 2 * time.Second
	}
	return &TimelineState{
		Config: cfg,
		Total:  total,
		onStep: onStep,
	}
}

// Play starts auto-advancing through steps. Each step, and its callback, runs
// on the UI thread; call Play, Stop and the step methods from there too.
func (ts *TimelineState) Play() {
	if ts.Playing || ts.Total <= 0 {
		return
	}
	ts.Playing = true
	ticker := time.NewTicker(ts.Config.Interval)
	stop := make(chan struct{})
	ts.ticker, ts.stopCh = ticker, stop
	go func() {
		for {
			select {
			case <-ticker.C:
				// Advance on the UI thread: onStep updates the chart, and the
				// app reads Current and Playing there too. The goroutine used
				// to call Next itself (#343).
				bellui.Do(func() {
					if ts.stopCh != stop { // stopped, or restarted, since this tick
						return
					}
					ts.Next()
					if !ts.Config.Loop && ts.Current >= ts.Total-1 {
						ts.Stop()
					}
				})
			case <-stop:
				return
			}
		}
	}()
}

// Stop halts auto-advance.
func (ts *TimelineState) Stop() {
	ts.Playing = false
	if ts.ticker != nil {
		ts.ticker.Stop()
		ts.ticker = nil
	}
	if ts.stopCh != nil {
		// Closing rather than sending: a send the goroutine was not waiting
		// for was dropped, and the goroutine then waited forever.
		close(ts.stopCh)
		ts.stopCh = nil
	}
}

// Next advances to the next step.
func (ts *TimelineState) Next() {
	ts.Current++
	if ts.Current >= ts.Total {
		if ts.Config.Loop {
			ts.Current = 0
		} else {
			ts.Current = ts.Total - 1
			return
		}
	}
	if ts.onStep != nil {
		ts.onStep(ts.Current)
	}
}

// Prev goes back one step.
func (ts *TimelineState) Prev() {
	ts.Current--
	if ts.Current < 0 {
		ts.Current = 0
	}
	if ts.onStep != nil {
		ts.onStep(ts.Current)
	}
}

// GoTo jumps to a specific step.
func (ts *TimelineState) GoTo(index int) {
	if index < 0 {
		index = 0
	}
	if index >= ts.Total {
		index = ts.Total - 1
	}
	ts.Current = index
	if ts.onStep != nil {
		ts.onStep(ts.Current)
	}
}
