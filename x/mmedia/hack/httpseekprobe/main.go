// Command httpseekprobe is the live-verify probe for #1098: a seek near the
// end of an MP3 played over http asks the server for the target's bytes
// when the file is CBR, instead of reading the file forward, and a seek a
// server refuses fails in words, not as FFmpeg's number.
//
// It plays cbr.mp3 and vbr.mp3 from serve (see serve/main.go), from a server
// that honours Range and one that ignores it. Each leg runs three times:
// Play, then 2 s later a seek to 3 s before the end, then it waits for the
// end or an error. Make the files with FFmpeg, 12 minutes each:
//
//	ffmpeg -f lavfi -i anoisesrc=d=720:c=pink:a=0.3 -ac 2 -b:a 256k cbr.mp3
//	ffmpeg -f lavfi -i anoisesrc=d=720:c=pink:a=0.3 -ac 2 -q:a 2 vbr.mp3
//
// On a device:
//
//	bellui package -os android/arm64 -app-id io.bellui.httpseekprobe
//	adb install -r httpseekprobe.apk
//	adb logcat -s Bellui:I | grep httpseekprobe
//
// PASS: every run ends within a few seconds of its seek, at about the
// target, and serve's log shows a ranged request near the end of cbr.mp3 on
// :8090. A failed seek must not show a number. VBR on :8090 may read forward:
// it lands exactly that way, which is the owner's pick on #1098.
package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/mmedia"
)

type leg struct{ name, url string }

var legs = []leg{
	{"cbr-range", "http://127.0.0.1:8090/cbr.mp3"},
	{"cbr-norange", "http://127.0.0.1:8091/cbr.mp3"},
	{"vbr-range", "http://127.0.0.1:8090/vbr.mp3"},
	{"vbr-norange", "http://127.0.0.1:8091/vbr.mp3"},
}

// rawCode is FFmpeg's number in an error message, such as "-38".
var rawCode = regexp.MustCompile(`-\d{2,}`)

func run(l leg, n int) string {
	type result struct {
		err error
		at  time.Time
	}
	done := make(chan result, 1)
	var seekAt time.Time
	var firstAfter time.Duration = -1
	updates := make(chan time.Duration, 64)
	pb, err := mmedia.NewAudioPlaybackURL(fmt.Sprintf("%s?leg=%s-%d", l.url, l.name, n),
		mmedia.WithVolume(0.05),
		mmedia.WithOnTimeUpdate(func(cur, _ time.Duration) {
			select {
			case updates <- cur:
			default:
			}
		}),
		mmedia.WithOnEnd(func() { done <- result{at: time.Now()} }),
		mmedia.WithOnError(func(err error) { done <- result{err: err, at: time.Now()} }),
	)
	if err != nil {
		return "FAIL open: " + err.Error()
	}
	defer pb.Close()
	pb.Play()
	time.Sleep(2 * time.Second)
	target := pb.Duration() - 3*time.Second
	seekAt = time.Now()
	pb.Seek(target)
	timeout := time.After(30 * time.Second)
	for {
		select {
		case cur := <-updates:
			if firstAfter < 0 && !seekAt.IsZero() && cur >= target-time.Second {
				firstAfter = cur
			}
		case r := <-done:
			wall := r.at.Sub(seekAt).Round(10 * time.Millisecond)
			if r.err != nil {
				verdict := "FAIL"
				if !rawCode.MatchString(r.err.Error()) {
					verdict = "ERR-READABLE"
				}
				return fmt.Sprintf("%s error %v after %v: %q", verdict, target, wall, r.err.Error())
			}
			verdict := "PASS"
			if wall > 6*time.Second || firstAfter < 0 {
				verdict = "FAIL"
			}
			return fmt.Sprintf("%s seek %v: first update %v, ended %v after the seek", verdict, target, firstAfter, wall)
		case <-timeout:
			return fmt.Sprintf("FAIL seek %v: no end within 30s (first update %v)", target, firstAfter)
		}
	}
}

func main() {
	a := app.New()
	w := a.NewWindow("httpseekprobe")
	status := widget.NewLabel("httpseekprobe: running")
	w.SetContent(status)
	go func() {
		for _, l := range legs {
			for n := 1; n <= 3; n++ {
				msg := fmt.Sprintf("httpseekprobe %s #%d %s", l.name, n, run(l, n))
				log.Print(msg)
				bellui.Do(func() { status.SetText(msg) })
			}
		}
		log.Print("httpseekprobe DONE")
		time.Sleep(time.Second)
		os.Exit(0)
	}()
	bellui.ShowAndRun(w)
}
