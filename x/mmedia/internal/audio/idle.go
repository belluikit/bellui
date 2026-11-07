package audio

import (
	"sync"

	"github.com/belluikit/bellui"
)

// suspender is what idleSuspend needs of the device: *oto.Context's Suspend
// and Resume.
type suspender interface {
	Suspend() error
	Resume() error
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
// Only Android needs it (suspendWhenIdle). Elsewhere a suspend has no such
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
}
