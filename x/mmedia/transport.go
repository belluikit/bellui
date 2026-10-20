package mmedia

import (
	"sync"

	"github.com/belluikit/bellui/x/mmedia/internal/pipeline"
)

// transport orders a player's Play, Pause, Stop and Close against the restart
// a looping player makes when its clip ends (#527). The restart runs on its
// own goroutine (#502), so without this a Pause just as the clip ended found
// the pipeline not playing and did nothing, and the restart then played the
// clip again.
//
// wantPlay records whether the last call asked for playback. A restart that
// finds it false leaves the player stopped at the end; a Pause or Stop that
// arrives during a restart waits for it, then applies to the restarted
// pipeline.
//
// No pipeline callback takes mu, so holding it across pipe.Stop, which waits
// for the pipeline's goroutines, cannot deadlock.
type transport struct {
	mu       sync.Mutex
	wantPlay bool
}

func (t *transport) play(pipe *pipeline.Pipeline) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.wantPlay = true
	pipe.Play()
}

func (t *transport) pause(pipe *pipeline.Pipeline) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.wantPlay = false
	pipe.Pause()
}

func (t *transport) stop(pipe *pipeline.Pipeline) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.wantPlay = false
	pipe.Stop()
}

func (t *transport) close(pipe *pipeline.Pipeline) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.wantPlay = false
	pipe.Close()
}

// restart plays pipe again from the start if playback is still wanted, and
// reports whether it did. A looping player calls it, off the pipeline's
// goroutine, when its clip ends.
func (t *transport) restart(pipe *pipeline.Pipeline) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.wantPlay {
		return false
	}
	pipe.Stop()
	pipe.Play()
	return true
}
