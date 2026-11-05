package audio

import "math"

// levelWindow is how much audio one level measurement covers: short enough to
// follow a beat, long enough to hold several periods of a bass note.
const levelWindowMs = 20

// levelHistory is how many windows are kept: 2 s, more than the player holds
// pulled and unplayed, so the window being heard is always among them.
const levelHistory = 100

// levels measures the RMS of the audio the player pulls, one window at a
// time, keyed by where each window ends in the pulled audio. Level asks for
// the window at the played position, so it follows what is heard rather than
// what was last pulled, which runs a chunk (150 ms) or more ahead.
//
// Its methods are called with the puller's lock held.
type levels struct {
	window int // bytes per window, whole 16-bit samples

	start int64   // offset in pulled audio where the open window began
	sum   float64 // of squared samples in the open window, in [-1, 1]
	count int     // samples in the open window
	odd   []byte  // a sample split between two pulls

	ends [levelHistory]int64
	rms  [levelHistory]float32
	n    int // closed windows, newest at (n-1) % levelHistory
}

func newLevels(bytesPerSec int) levels {
	w := bytesPerSec * levelWindowMs / 1000
	w -= w % 2
	return levels{window: max(w, 2)}
}

// add measures b, the audio a pull took from the ring, which begins at offset
// at in the pulled audio.
func (l *levels) add(at int64, b []byte) {
	if len(l.odd) == 1 && len(b) > 0 {
		l.sample(int16(uint16(l.odd[0]) | uint16(b[0])<<8))
		l.odd = l.odd[:0]
		b = b[1:]
		at++
	}
	for i := 0; i+1 < len(b); i += 2 {
		l.sample(int16(uint16(b[i]) | uint16(b[i+1])<<8))
		if l.count*2 >= l.window {
			end := at + int64(i) + 2
			l.ends[l.n%levelHistory] = end
			l.rms[l.n%levelHistory] = float32(math.Sqrt(l.sum / float64(l.count)))
			l.n++
			l.start, l.sum, l.count = end, 0, 0
		}
	}
	if len(b)%2 == 1 {
		l.odd = append(l.odd[:0], b[len(b)-1])
	}
}

func (l *levels) sample(s int16) {
	v := float64(s) / 32768
	l.sum += v * v
	l.count++
}

// at is the RMS, 0 to 1, of the window holding offset played in the pulled
// audio: the oldest window ending after it. Past every closed window, it is
// the newest one, the audio just heard. Before any window has closed, it is
// the open window's so far, or 0.
func (l *levels) at(played int64) float32 {
	oldest := max(0, l.n-levelHistory)
	for i := oldest; i < l.n; i++ {
		if l.ends[i%levelHistory] > played {
			return l.rms[i%levelHistory]
		}
	}
	if l.n > 0 {
		return l.rms[(l.n-1)%levelHistory]
	}
	if l.count > 0 {
		return float32(math.Sqrt(l.sum / float64(l.count)))
	}
	return 0
}

func (l *levels) reset() {
	*l = levels{window: l.window, odd: l.odd[:0]}
}
