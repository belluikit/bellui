// Command bgposprobe live-checks #846 on iOS: after the app returns from the
// background, x/mmedia's time updates must not jump past the track's end, and
// playback must resume where the audio stopped.
//
// It plays a generated 12 s tone with the default audio session and logs
// every time update and every lifecycle change to the console (Bellui:):
//
//	bgposprobe t=5.103 pos=5.1s of 12s
//	bgposprobe t=5.210 lifecycle=background
//
// t is wall seconds since the probe started. Send it to the background a few
// seconds in (Home, or launch another app with devicectl), wait ~15 s, and
// bring it back. PASS: no pos is ever past "of", and the first pos after
// lifecycle=foreground is within a second of the last pos before
// lifecycle=background. FAIL: a pos past the duration, or one that jumped by
// the time spent in the background.
//
// Measured on an iPad (10th gen, 2026-11-04), Settings launched over the
// probe with devicectl at about 7 s and the probe brought back 15 s later:
// before the fix, the last update before the gap read 7.1 s and the first
// after it 21.1 s of 12 s, for about a second; after it, 7.7 s then 7.7 s,
// and 8.0 s then 8.0 s, with no update past 12 s.
package main

import (
	"bytes"
	"encoding/binary"
	"log"
	"math"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/mmedia"
)

const (
	rate    = 44100
	seconds = 12
)

// tone is a 16-bit mono WAV of a quiet 440 Hz sine.
func tone() []byte {
	n := rate * seconds
	var b bytes.Buffer
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	b.WriteString("RIFF")
	w(uint32(36 + n*2))
	b.WriteString("WAVEfmt ")
	w(uint32(16))
	w(uint16(1))
	w(uint16(1))
	w(uint32(rate))
	w(uint32(rate * 2))
	w(uint16(2))
	w(uint16(16))
	b.WriteString("data")
	w(uint32(n * 2))
	for i := 0; i < n; i++ {
		w(int16(3000 * math.Sin(2*math.Pi*440*float64(i)/rate)))
	}
	return b.Bytes()
}

func main() {
	start := time.Now()
	logf := func(format string, args ...any) {
		log.Printf("bgposprobe t=%.3f "+format, append([]any{time.Since(start).Seconds()}, args...)...)
	}

	a := app.New()
	w := a.NewWindow("bgposprobe")
	a.Lifecycle().SetOnEnteredForeground(func() { logf("lifecycle=foreground") })
	a.Lifecycle().SetOnExitedForeground(func() { logf("lifecycle=background") })

	status := widget.NewLabel("bgposprobe")
	p, err := mmedia.NewAudioPlayer(
		bytes.NewReader(tone()),
		mmedia.WithAutoPlay(),
		mmedia.WithOnTimeUpdate(func(cur, total time.Duration) {
			logf("pos=%.1fs of %.0fs", cur.Seconds(), total.Seconds())
			bellui.Do(func() { status.SetText(cur.Round(100 * time.Millisecond).String()) })
		}),
		mmedia.WithOnEnd(func() { logf("end") }),
		mmedia.WithOnError(func(err error) { logf("error=%v", err) }),
	)
	if err != nil {
		log.Fatalf("bgposprobe open: %v", err)
	}
	w.SetContent(container.NewVBox(status, p))
	bellui.ShowAndRun(w)
}
