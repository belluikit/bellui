package pipeline

import (
	"encoding/binary"
	"image"
	"sync"
	"testing"
	"time"

	"github.com/belluikit/bellui/x/mmedia/internal/source"
)

// clipSource is a finite 30 fps video-only stream. Each frame carries its own
// index in its first four bytes, so a test can tell which frame was shown.
type clipSource struct {
	vcb    source.VideoCallback
	frames int
	next   int
}

func (s *clipSource) HasVideo() bool          { return true }
func (s *clipSource) HasAudio() bool          { return false }
func (s *clipSource) VideoSize() (int, int)   { return 16, 16 }
func (s *clipSource) Duration() time.Duration { return time.Duration(s.frames) * time.Second / 30 }
func (s *clipSource) Framerate() float64      { return 30 }
func (s *clipSource) SampleRate() int         { return 0 }
func (s *clipSource) Channels() int           { return 0 }
func (s *clipSource) SetVideoCallback(f source.VideoCallback) {
	s.vcb = f
}
func (s *clipSource) SetAudioCallback(source.AudioCallback) {}
func (s *clipSource) SetAudioChannels(int) error            { return nil }
func (s *clipSource) SetAudioSampleRate(int) error          { return nil }
func (s *clipSource) Decode(time.Duration) {
	if s.next >= s.frames || s.vcb == nil {
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	binary.LittleEndian.PutUint32(img.Pix, uint32(s.next))
	s.vcb(img, time.Duration(s.next)*time.Second/30)
	s.next++
}
func (s *clipSource) Seek(pos time.Duration) { s.next = int(pos * 30 / time.Second) }
func (s *clipSource) Rewind()                { s.next = 0 }
func (s *clipSource) HasEnded() bool         { return s.next >= s.frames }
func (s *clipSource) Close()                 {}

// frameIndex reads the index a clipSource frame carries.
func frameIndex(img *image.RGBA) int { return int(binary.LittleEndian.Uint32(img.Pix)) }

// TestBackwardSeekShowsTheTargetPromptly pins #501: after a seek back, the
// display used to wait for the clock to come round to the frame it was holding
// from the old position, and then showed frames from before the seek. The
// picture froze for as long as the seek went back.
func TestBackwardSeekShowsTheTargetPromptly(t *testing.T) {
	p := newPipelineForSource(&clipSource{frames: 300}, Config{})
	defer p.Close()

	type shown struct {
		at    time.Time
		index int
	}
	var mu sync.Mutex
	var log []shown
	p.SetOnFrame(func(img *image.RGBA) {
		mu.Lock()
		log = append(log, shown{time.Now(), frameIndex(img)})
		mu.Unlock()
	})

	p.Play()
	time.Sleep(2 * time.Second) // about frame 60

	// Back to frame 3. The old display froze until the clock came round to the
	// frame it held, about 1.9 s; the margins below leave room for -race.
	seekAt := time.Now()
	p.Seek(100 * time.Millisecond)
	time.Sleep(time.Second)

	mu.Lock()
	defer mu.Unlock()
	var after []shown
	for _, s := range log {
		if s.at.After(seekAt) {
			after = append(after, s)
		}
	}
	if len(after) == 0 {
		t.Fatal("no frame shown in the second after a backward seek")
	}
	if wait := after[0].at.Sub(seekAt); wait > 500*time.Millisecond {
		t.Errorf("first frame after the seek came %v later; want within 500 ms", wait)
	}
	// The target itself comes first. A seek that discarded frames decoded at
	// the target showed a later one, held until it was due (#513).
	if after[0].index != 3 {
		t.Errorf("first frame after the seek is frame %d; want the target, frame 3", after[0].index)
	}
	for _, s := range after {
		if s.index > 3+40 { // the target plus a second at 30 fps, with room
			t.Errorf("frame %d, from before the seek, was shown after it", s.index)
		}
	}
}
