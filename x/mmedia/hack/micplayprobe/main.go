// Command micplayprobe live-checks #1188 on an iPhone or iPad: the first song
// x/mmedia plays in a process must play, and must not silence a microphone
// capture that is already running.
//
// x/mmedia sets the shared audio session when it opens the audio device, once
// per process. It used to ask for Playback unconditionally. belluikit/player's
// capture puts the session in Record, and with the capture running iOS refuses
// the change to Playback (561017449, '!pri'): the session stays in Record, which
// has no output, and the song fails to open with "oto: AudioQueueStart failed".
// Measured with this probe on an iPad (10th gen, 2026-11-25), twice out of two,
// against bellui main 1532e4a5. Had the change gone through, Playback has no
// input and the capture would have gone silent instead (belluikit/player#184).
//
// The probe runs one scripted timeline and needs no taps after the microphone
// permission prompt:
//
//	0 s   start a capture, with Player's session code (mic_ios.m)
//	3 s   open the first x/mmedia playback of the process: a 440 Hz tone
//	9 s   stop the capture, as Player does when a recording ends
//	12 s  close the playback and print the verdict
//
// Every half second it logs one line:
//
//	micplayprobe t=4.5 phase=song cat=PlayAndRecord bufs=3 rms=0.0412 engine=true pos=1.5s
//
// bufs is how many microphone buffers arrived in the last half second (2-3 at
// the 200 ms tap size); 0 means the capture stopped delivering. rms is their
// level: digital silence is 0, a quiet room around 0.001-0.01, and the tone
// from the speaker well above that. pos is where the song is, and it must keep
// moving.
//
// PASS: in every window between 1 s after the song starts and the capture's
// end, the microphone delivered buffers with a level above digital silence,
// and the song advanced. FAIL says what broke: the song did not open, or the
// window where the capture went silent or the song stopped. After the capture
// stops the probe also reports whether the song kept playing; that half is
// Player's release path, recorded rather than judged.
//
// Package from this directory and run on a device:
//
//	bellui package -os ios --app-id com.trendvidia.micplayprobe \
//	    --cert "Apple Development: Benedicto Franco Jr. (D68Z8W925S)"
//	xcrun devicectl device install app --device <UDID> micplayprobe.app
//	xcrun devicectl device process launch --console --terminate-existing \
//	    --device <UDID> com.trendvidia.micplayprobe
//
// With -nocapture (pass it after the bundle id) the probe only plays: the
// session must end up in Playback, as before #1188, and the song must advance.
//
// The result is also on screen, for when the console connection drops.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/mmedia"
)

const (
	window      = 500 * time.Millisecond
	songAt      = 3 * time.Second
	stopMicAt   = 9 * time.Second
	endAt       = 12 * time.Second
	settle      = time.Second // after the song starts, before windows are judged
	silentBelow = 1e-5        // rms under this is digital silence
)

type micStats struct {
	bufs, frames  uint64
	sumSq         float64
	configChanges int
	running       bool
}

func main() {
	noCapture := flag.Bool("nocapture", false, "only play: check the session is Playback when nothing records")
	flag.Parse()
	a := app.New()
	w := a.NewWindow("micplayprobe")
	status := widget.NewLabel("starting…")
	status.Wrapping = bellui.TextWrapWord
	w.SetContent(container.NewVBox(widget.NewLabel("micplayprobe (#1188)"), status))

	show := func(s string) { bellui.Do(func() { status.SetText(s) }) }
	a.Lifecycle().SetOnStarted(func() {
		go func() {
			run := run
			if *noCapture {
				run = runNoCapture
			}
			v := run()
			log.Println("micplayprobe " + v)
			show(v)
		}()
	})
	bellui.ShowAndRun(w)
}

func run() string {
	if !micPermission() {
		return "INCONCLUSIVE: microphone permission denied; allow it in Settings and relaunch"
	}
	log.Printf("micplayprobe start cat=%s", sessionCategory())
	if err := micStart(); err != nil {
		return "INCONCLUSIVE: capture did not start: " + err.Error()
	}
	log.Printf("micplayprobe capture=started cat=%s", sessionCategory())

	start := time.Now()
	var pb *mmedia.AudioPlayback
	var fails []string
	var ambient, during []float64
	var lastPos, posAtStop time.Duration
	songStalls, afterStalls := 0, 0
	micStopped := false
	prev := readMic()
	for tick := time.NewTicker(window); ; {
		<-tick.C
		t := time.Since(start)

		if pb == nil && t >= songAt {
			var err error
			pb, err = mmedia.NewAudioPlayback(bytes.NewReader(tone(15 * time.Second)))
			if err != nil {
				micStop()
				return "FAIL: the first song did not open during the capture: " + err.Error()
			}
			pb.Play()
			log.Printf("micplayprobe song=started cat=%s outputs=%s", sessionCategory(), sessionOutputs())
		}
		if !micStopped && t >= stopMicAt {
			micStop()
			micStopped = true
			if pb != nil {
				posAtStop = pb.CurrentTime()
			}
			log.Printf("micplayprobe capture=stopped cat=%s", sessionCategory())
		}

		cur := readMic()
		bufs := cur.bufs - prev.bufs
		rms := 0.0
		if n := cur.frames - prev.frames; n > 0 {
			rms = math.Sqrt((cur.sumSq - prev.sumSq) / float64(n))
		}
		prev = cur

		phase := "before"
		var pos time.Duration
		switch {
		case micStopped:
			phase = "after"
		case pb != nil:
			phase = "song"
		}
		if pb != nil {
			pos = pb.CurrentTime()
		}
		log.Printf("micplayprobe t=%.1f phase=%s cat=%s bufs=%d rms=%.4f engine=%v configChanges=%d pos=%v",
			t.Seconds(), phase, sessionCategory(), bufs, rms, cur.running, cur.configChanges, pos.Round(100*time.Millisecond))

		switch {
		case phase == "before" && t > window:
			ambient = append(ambient, rms)
		case phase == "song" && t >= songAt+settle:
			during = append(during, rms)
			if bufs == 0 || rms < silentBelow {
				fails = append(fails, fmt.Sprintf("t=%.1f bufs=%d rms=%.6f", t.Seconds(), bufs, rms))
			}
			if pos <= lastPos {
				songStalls++
			}
		case phase == "after" && pos <= lastPos:
			afterStalls++
		}
		lastPos = pos

		if t >= endAt {
			tick.Stop()
			break
		}
	}
	posAtEnd := pb.CurrentTime()
	pb.Close()

	after := "song kept playing after the capture stopped"
	if afterStalls > 0 || posAtEnd <= posAtStop {
		after = fmt.Sprintf("song STALLED after the capture stopped (%d windows, pos %v -> %v)", afterStalls, posAtStop, posAtEnd)
	}
	summary := fmt.Sprintf("ambient rms %.4f, rms during song %.4f; %s; final cat=%s",
		mean(ambient), mean(during), after, sessionCategory())
	switch {
	case len(during) == 0:
		return "INCONCLUSIVE: no window was judged; " + summary
	case len(fails) > 0:
		return fmt.Sprintf("FAIL: the capture went silent once the song started (%s); %s", strings.Join(fails, ", "), summary)
	case songStalls > 0:
		return fmt.Sprintf("FAIL: the song did not advance during the capture (%d windows); %s", songStalls, summary)
	}
	return "PASS: the capture kept delivering audio through the first song; " + summary
}

// runNoCapture plays the first song with no capture running.
func runNoCapture() string {
	log.Printf("micplayprobe start cat=%s", sessionCategory())
	pb, err := mmedia.NewAudioPlayback(bytes.NewReader(tone(15 * time.Second)))
	if err != nil {
		return "FAIL: playback did not open: " + err.Error()
	}
	defer pb.Close()
	pb.Play()
	cat, outs := sessionCategory(), sessionOutputs()
	log.Printf("micplayprobe song=started cat=%s outputs=%s", cat, outs)
	time.Sleep(500 * time.Millisecond)
	from := pb.CurrentTime()
	time.Sleep(3 * time.Second)
	to := pb.CurrentTime()
	switch {
	case cat != "Playback":
		return fmt.Sprintf("FAIL: no capture running, yet the session is %s, not Playback", cat)
	case to-from < 2*time.Second:
		return fmt.Sprintf("FAIL: the song advanced %v in 3 s", to-from)
	}
	return fmt.Sprintf("PASS: no capture: session Playback, outputs %s, song advanced %v in 3 s", outs, (to - from).Round(10*time.Millisecond))
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// tone is a mono 48 kHz 16-bit WAV of a 440 Hz sine at a third of full scale.
func tone(d time.Duration) []byte {
	const rate = 48000
	n := int(d.Seconds() * rate)
	var b bytes.Buffer
	le := binary.LittleEndian
	b.WriteString("RIFF")
	_ = binary.Write(&b, le, uint32(36+2*n))
	b.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(rate), uint32(2 * rate), uint16(2), uint16(16)} {
		_ = binary.Write(&b, le, v)
	}
	b.WriteString("data")
	_ = binary.Write(&b, le, uint32(2*n))
	for i := 0; i < n; i++ {
		_ = binary.Write(&b, le, int16(math.Sin(2*math.Pi*440*float64(i)/rate)*math.MaxInt16/3))
	}
	return b.Bytes()
}
