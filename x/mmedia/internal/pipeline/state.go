package pipeline

import "sync"

// State represents the current playback state.
type State int

const (
	StateStopped State = iota
	StatePlaying
	StatePaused
)

// StateMachine manages playback state transitions thread-safely.
type StateMachine struct {
	mu       sync.RWMutex
	state    State
	onChange func(State)
}

func NewStateMachine(onChange func(State)) *StateMachine {
	return &StateMachine{
		state:    StateStopped,
		onChange: onChange,
	}
}

func (sm *StateMachine) Get() State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

func (sm *StateMachine) Set(s State) {
	sm.mu.Lock()
	prev := sm.state
	sm.state = s
	sm.mu.Unlock()

	if prev != s && sm.onChange != nil {
		sm.onChange(s)
	}
}

func (sm *StateMachine) IsPlaying() bool {
	return sm.Get() == StatePlaying
}

func (sm *StateMachine) IsPaused() bool {
	return sm.Get() == StatePaused
}

func (sm *StateMachine) IsStopped() bool {
	return sm.Get() == StateStopped
}
