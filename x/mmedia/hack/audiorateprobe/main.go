// Command audiorateprobe is the live-verify probe for #506: once a process has
// played audio, a later stream at another sample rate must still play at
// normal speed, and a mono stream first must not leave the device mono.
//
// Make the two clips with FFmpeg:
//
//	ffmpeg -f lavfi -i sine=frequency=440:sample_rate=44100:duration=4 -ac 1 -c:a aac a.m4a
//	ffmpeg -f lavfi -i sine=frequency=1000:sample_rate=48000:duration=10 -ac 2 -c:a aac b.m4a
//	go run . a.m4a b.m4a
//
// It plays the first clip for a second and closes it, then plays the second
// and reports media time per wall second over four seconds of playback.
//
// PASS: speed within 1% of 1.000, and the device opened with 2 channels.
// FAIL: anything else. Before #507 the 48 kHz clip played at 0.918x after a
// 44.1 kHz one; before #506's fix a mono clip first opened a mono device.
package main

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"

	"github.com/belluikit/bellui/x/mmedia"
	"github.com/belluikit/bellui/x/mmedia/internal/audio"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: audiorateprobe <first-clip> <second-clip>")
		os.Exit(2)
	}
	a := app.New()
	w := a.NewWindow("audiorateprobe (#506)")
	go func() {
		code := run(os.Args[1], os.Args[2])
		bellui.Do(a.Quit)
		os.Exit(code)
	}()
	bellui.ShowAndRun(w)
}

func run(first, second string) int {
	p1, err := mmedia.NewAudioPlayerURL(first)
	if err != nil {
		fmt.Println("FAIL: open", first, err)
		return 1
	}
	p1.Play()
	time.Sleep(time.Second)
	p1.Close()

	// The device is open now; a new Output reports the format it has.
	out, err := audio.NewOutput(1)
	if err != nil {
		fmt.Println("FAIL: audio device", err)
		return 1
	}
	channels := out.Channels()
	fmt.Printf("audiorateprobe device=%d Hz x %d after %s\n", audio.DeviceSampleRate, channels, first)

	p2, err := mmedia.NewAudioPlayerURL(second)
	if err != nil {
		fmt.Println("FAIL: open", second, err)
		return 1
	}
	defer p2.Close()
	p2.Play()
	time.Sleep(time.Second) // past start-up
	m0, w0 := p2.CurrentTime(), time.Now()
	time.Sleep(4 * time.Second)
	m1, w1 := p2.CurrentTime(), time.Now()
	speed := (m1 - m0).Seconds() / w1.Sub(w0).Seconds()
	fmt.Printf("audiorateprobe media %v..%v over %v wall: speed=%.3f\n", m0, m1, w1.Sub(w0).Round(time.Millisecond), speed)

	if math.Abs(speed-1) > 0.01 || channels != 2 {
		fmt.Println("FAIL")
		return 1
	}
	fmt.Println("PASS")
	return 0
}
