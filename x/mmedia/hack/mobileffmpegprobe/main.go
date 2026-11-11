// Command mobileffmpegprobe live-checks #1122 on a phone: an x/mmedia app
// packaged by bellui package -os android or -os ios must carry goffmpeg's
// FFmpeg shared libraries and load them from its own package.
//
// It plays an embedded 4 s MP3 (a 440 Hz tone) and logs to the console
// (Bellui:, or logcat on Android):
//
//	mobileffmpegprobe PASS: played 4.0s of 4.0s in 4.1s wall
//
//	bellui package -os android -app-id io.bellui.mobileffmpegprobe
//	adb install -r mobileffmpegprobe.apk
//	adb logcat -s GoLog | grep mobileffmpegprobe
//
// Build with -tags goffmpeg_novideo for the audio set, and check that the
// package carries only the *-goffmpeg-audio libraries.
//
// PASS: the MP3 plays to its end, its media time within 10% of wall time.
// FAIL: an error opening or playing it. Packaged without the libraries, the
// app never reaches main: Android's linker stops it at launch with
// `dlopen failed: library "libavformat-goffmpeg.so" not found`, and iOS's
// dyld with `Library not loaded: @rpath/libavformat-goffmpeg.framework/…`.
//
// Measured 2026-11-11 with the bellui of #1122 (packaged before it, the app
// failed both ways above): PASS, 4 s in 4.0–4.3 s wall, on a moto g power 5G
// (debug, --release, the bundletool APK of a bellui release bundle, and
// goffmpeg_novideo) and on an iPad Pro simulator (media and audio sets).
package main

import (
	"bytes"
	_ "embed"
	"log"
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/mmedia"
)

// tone.mp3: ffmpeg -f lavfi -i sine=frequency=440:sample_rate=44100:duration=4 -ac 1 -c:a libmp3lame -b:a 32k tone.mp3
//
//go:embed tone.mp3
var tone []byte

func main() {
	a := app.New()
	w := a.NewWindow("mobileffmpegprobe")
	status := widget.NewLabel("mobileffmpegprobe: playing")
	w.SetContent(status)
	report := func(msg string) {
		log.Print("mobileffmpegprobe " + msg)
		bellui.Do(func() { status.SetText(msg) })
	}

	var start time.Time
	var last atomic.Int64 // media time of the last update, ns; set and read on the pipeline's goroutines
	pb, err := mmedia.NewAudioPlayback(
		bytes.NewReader(tone),
		mmedia.WithOnTimeUpdate(func(cur, _ time.Duration) { last.Store(int64(cur)) }),
		mmedia.WithOnEnd(func() {
			wall, played := time.Since(start), time.Duration(last.Load())
			verdict := "PASS"
			if r := played.Seconds() / wall.Seconds(); r < 0.9 || r > 1.1 || played < 3500*time.Millisecond {
				verdict = "FAIL"
			}
			report(verdict + ": played " + played.Round(100*time.Millisecond).String() + " of 4s in " +
				wall.Round(100*time.Millisecond).String() + " wall")
		}),
		mmedia.WithOnError(func(err error) { report("FAIL: " + err.Error()) }),
	)
	if err != nil {
		report("FAIL: open: " + err.Error())
	} else {
		defer pb.Close()
		start = time.Now()
		pb.Play()
	}
	bellui.ShowAndRun(w)
}
