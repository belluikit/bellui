package audio

import (
	"sync"

	"github.com/belluikit/bellui"
)

// suspender is what idleSuspend needs of the device: *oto.Context's Suspend,
// Resume and Flush.
type suspender interface {
	Suspend() error
	Resume() error
	Flush() error
}

// idleSuspend suspends the device while no Output in the process is playing,
// and resumes it before one plays again.
//
// On Android this keeps a phone call from leaving the music on a call-mode
// stream (#1052). The call's routing change disconnects the AAudio stream.
// oto reopens a stream that disconnects while it plays at once, so the new
// stream opened during the call and kept its thin voice-call sound after it.
// A suspended stream is paused, and oto opens a new one only at Resume, which
// is the next Play: after the call, when the player resumes.
//
// On iOS the lock screen shows ⏸ while the app's audio queue runs, even when
// it plays only silence, so after a pause the next lock-screen press is
// another pause (#1242). A suspended queue is paused, which iOS reads as not
// playing, and it starts again from the background.
//
// Only those two need it (suspendWhenIdle). Elsewhere a suspend has no such
// reason, and costs a restart of the device on every Play.
type idleSuspend struct {
	mu        sync.Mutex
	dev       suspender // nil until the device opens
	playing   int       // Outputs started and not paused or stopped
	suspended bool
}

// play counts an Output that started playing, resuming the device if it was
// suspended. It runs before the Output's player plays.
func (s *idleSuspend) play() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.playing++
	if s.suspended {
		s.suspended = false
		if err := s.dev.Resume(); err != nil {
			bellui.LogError("x/mmedia: resuming the audio device", err)
		}
	}
}

// idle counts an Output that stopped playing, and suspends the device when it
// was the last one.
func (s *idleSuspend) idle() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.playing--
	if s.playing > 0 || s.dev == nil || !suspendWhenIdle {
		return
	}
	if err := s.dev.Suspend(); err != nil {
		bellui.LogError("x/mmedia: suspending the audio device", err)
		return
	}
	s.suspended = true
	// A suspended device keeps the sound it was sent and has not played, and
	// plays it at the next Resume, before what plays next. The Outputs let the
	// device play their own out first (Output.settle), so it is silence, unless
	// the device did not say what it had played; it is discarded either way
	// (#1228).
	if err := s.dev.Flush(); err != nil {
		bellui.LogError("x/mmedia: discarding the audio device's queued sound", err)
	}
}
