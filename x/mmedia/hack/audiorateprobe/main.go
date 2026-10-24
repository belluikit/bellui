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
// It is also the live check for #707's channel policy. A 5.1 clip first must
// leave the device stereo, and so be downmixed:
//
//	ffmpeg -f lavfi -i sine=frequency=440:sample_rate=48000:duration=4 -af "pan=5.1|c0=c0|c1=c0|c2=c0|c3=c0|c4=c0|c5=c0" -c:a aac s51.m4a
//	go run . s51.m4a b.m4a
//
// and -channels n opts in to an n-channel device (SetAudioDeviceChannels):
//
//	go run . -channels 6 s51.m4a b.m4a
//
// PASS: speed within 1% of 1.000, and the device opened with 2 channels, or
// with -channels n, n.
// FAIL: anything else. Before #507 the 48 kHz clip played at 0.918x after a
// 44.1 kHz one; before #506's fix a mono clip first opened a mono device;
// before #707's a 5.1 clip first opened a 6-channel device.
package main

import (
	"flag"
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
	channels := flag.Int("channels", 0, "opt in to a device with this many channels (SetAudioDeviceChannels); 0 keeps the default, stereo")
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: audiorateprobe [-channels n] <first-clip> <second-clip>")
		os.Exit(2)
	}
	want := 2
	if *channels > 0 {
		if err := mmedia.SetAudioDeviceChannels(*channels); err != nil {
			fmt.Println("FAIL: SetAudioDeviceChannels", err)
			os.Exit(1)
		}
		want = *channels
	}
	a := app.New()
	w := a.NewWindow("audiorateprobe (#506)")
	go func() {
		code := run(flag.Arg(0), flag.Arg(1), want)
		bellui.Do(a.Quit)
		os.Exit(code)
	}()
	bellui.ShowAndRun(w)
}

func run(first, second string, want int) int {
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
	fmt.Printf("audiorateprobe device=%d Hz x %d (want %d) after %s\n", audio.DeviceSampleRate, channels, want, first)

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

	if math.Abs(speed-1) > 0.01 || channels != want {
		fmt.Println("FAIL")
		return 1
	}
	fmt.Println("PASS")
	return 0
}
