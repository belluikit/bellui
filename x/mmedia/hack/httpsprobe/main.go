// Command httpsprobe is the live-verify probe for #871: an https URL plays and
// seeks through x/mmedia's URL constructors on Android, whose FFmpeg has no
// TLS, by way of the Go reader in internal/source. It runs anywhere; off
// Android it takes the Go path on purpose, so a desktop run checks the same code.
//
//	go run .                       # the built-in legs
//	go run . <https-url> [video]   # one leg of your own
//
// On a device, package it from this directory and read logcat (tag Bellui):
//
//	bellui package --target android/arm64 --app-id com.trendvidia.httpsprobe
//
// Each leg opens a player, plays to 1 s of media, seeks to 75% and plays on,
// then opens a second player and seeks straight after Play
// (belluikit/player#148). It counts the bytes the reader fetched, which is
// both the evidence that the Go path ran and the check that playback started
// before the whole file had arrived.
//
// The local leg needs a server on the host, reached through adb reverse; see
// serve.go's header. Without one it is skipped.
//
// PASS: every leg that ran prints PASS, then "httpsprobe DONE fail=0".
// FAIL: any FAIL line. Before #871 every https leg failed at open on Android
// with "failed to open decoder: -1".
package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/core"

	"github.com/belluikit/bellui/x/mmedia"
	"github.com/belluikit/bellui/x/mmedia/internal/source"
)

type leg struct {
	name  string
	url   string
	video bool
	// local trusts any certificate: the host's server is self-signed. Every
	// other leg uses the system's roots, which is part of what is checked.
	local bool
	// wantErr is part of the error a leg must fail with, for a server no
	// reader can play from.
	wantErr string
}

var legs = []leg{
	{name: "mp3-ranges", url: "https://www.soundhelix.com/examples/mp3/SoundHelix-Song-1.mp3"},
	// This server ignores Range and sends no length (2026-11-01), so the file,
	// index at the end, cannot play; FFmpeg's own https fails on it too.
	{name: "mp4-unsized-server", url: "https://filesamples.com/samples/video/mp4/sample_640x360.mp4", video: true, wantErr: "sent no length"},
	{name: "local-mp4-index-at-end", url: "https://localhost:8443/big.mp4", video: true, local: true},
}

// counter counts the requests and response bytes the reader makes, and
// remembers the largest total size a response named.
type counter struct {
	base     http.RoundTripper
	requests atomic.Int64
	bytes    atomic.Int64
	size     atomic.Int64
	mu       sync.Mutex
	ranges   []string
}

func (c *counter) RoundTrip(r *http.Request) (*http.Response, error) {
	c.requests.Add(1)
	c.mu.Lock()
	c.ranges = append(c.ranges, strings.TrimPrefix(r.Header.Get("Range"), "bytes="))
	c.mu.Unlock()
	resp, err := c.base.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	if _, total, ok := strings.Cut(resp.Header.Get("Content-Range"), "/"); ok {
		if n, err := strconv.ParseInt(total, 10, 64); err == nil {
			c.size.Store(n)
		}
	} else if resp.StatusCode == http.StatusOK && resp.ContentLength > 0 {
		c.size.Store(resp.ContentLength)
	}
	resp.Body = &countingBody{ReadCloser: resp.Body, n: &c.bytes}
	return resp, nil
}

type countingBody struct {
	io.ReadCloser
	n *atomic.Int64
}

func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.n.Add(int64(n))
	return n, err
}

// player is what the probe needs of AudioPlayer and MediaPlayer.
type player interface {
	core.CanvasObject
	Play()
	Seek(time.Duration)
	Duration() time.Duration
	CurrentTime() time.Duration
	Close()
}

func main() {
	if len(os.Args) > 1 {
		legs = []leg{{name: "arg", url: os.Args[1], video: len(os.Args) > 2 && os.Args[2] == "video"}}
	}
	a := app.New()
	w := a.NewWindow("httpsprobe (#871)")
	go func() {
		fail := 0
		for _, l := range legs {
			if !run(w, l) {
				fail++
			}
		}
		fmt.Printf("httpsprobe DONE fail=%d\n", fail)
		bellui.Do(a.Quit)
		time.Sleep(500 * time.Millisecond)
		os.Exit(min(fail, 1))
	}()
	bellui.ShowAndRun(w)
}

func run(w bellui.Window, l leg) bool {
	if l.local {
		c, err := net.DialTimeout("tcp", "localhost:8443", time.Second)
		if err != nil {
			fmt.Printf("LEG %s SKIP: no local server (%v)\n", l.name, err)
			return true
		}
		c.Close()
	}
	base := http.DefaultTransport.(*http.Transport).Clone()
	if l.local {
		base.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // the host's self-signed server
	}
	cnt := &counter{base: base}
	restore := source.UseGoHTTPSForProbes(&http.Client{Transport: cnt})
	defer restore()

	failf := func(format string, args ...any) bool {
		msg := fmt.Sprintf(format, args...)
		if l.wantErr != "" && strings.Contains(msg, l.wantErr) {
			fmt.Printf("LEG %s PASS: failed as it must: %s\n", l.name, msg)
			return true
		}
		fmt.Printf("LEG %s FAIL: %s\n", l.name, msg)
		return false
	}

	errs := make(chan error, 4)
	onErr := mmedia.WithOnError(func(err error) {
		select {
		case errs <- err:
		default:
		}
	})
	open := func() (player, time.Duration, error) {
		start := time.Now()
		var p player
		var err error
		bellui.DoAndWait(func() {
			if l.video {
				p, err = mmedia.NewMediaPlayerURL(l.url, onErr)
			} else {
				p, err = mmedia.NewAudioPlayerURL(l.url, onErr)
			}
			if err == nil {
				w.SetContent(p)
			}
		})
		return p, time.Since(start), err
	}

	p, openTook, err := open()
	if err != nil {
		return failf("open: %v (requests %d)", err, cnt.requests.Load())
	}
	dur := p.Duration()
	fmt.Printf("LEG %s open %v duration=%v requests=%d bytes=%d size=%d\n",
		l.name, openTook.Round(time.Millisecond), dur, cnt.requests.Load(), cnt.bytes.Load(), cnt.size.Load())

	playStart := time.Now()
	bellui.DoAndWait(p.Play)
	if err := reach(p, time.Second, errs); err != nil {
		p.Close()
		return failf("play: %v", err)
	}
	fetched, size := cnt.bytes.Load(), cnt.size.Load()
	fmt.Printf("LEG %s at 1s of media after %v: fetched %d of %d bytes (%.1f%%)\n",
		l.name, time.Since(playStart).Round(time.Millisecond), fetched, size, pct(fetched, size))
	if size > 0 && fetched >= size {
		p.Close()
		return failf("the whole file was fetched before 1s of media played")
	}

	if dur > 0 {
		target := dur * 3 / 4
		seekStart := time.Now()
		bellui.DoAndWait(func() { p.Seek(target) })
		seekTook := time.Since(seekStart)
		if err := reach(p, target+time.Second, errs); err != nil {
			p.Close()
			return failf("after a seek to %v: %v", target, err)
		}
		fmt.Printf("LEG %s seek to %v: Seek returned in %v, playing on at %v\n",
			l.name, target, seekTook.Round(time.Millisecond), p.CurrentTime().Round(10*time.Millisecond))
	} else {
		fmt.Printf("LEG %s no duration: the stream is forward-only, seek not tried\n", l.name)
	}
	bellui.DoAndWait(p.Close)

	// belluikit/player#148: a seek straight after Play.
	if dur > 0 {
		p, _, err = open()
		if err != nil {
			return failf("reopen: %v", err)
		}
		target := dur - 5*time.Second
		bellui.DoAndWait(func() {
			p.Play()
			p.Seek(target)
		})
		err := reach(p, target+time.Second, errs)
		bellui.DoAndWait(p.Close)
		if err != nil {
			return failf("seek straight after play to %v: %v", target, err)
		}
		fmt.Printf("LEG %s seek straight after play to %v: ok\n", l.name, target)
	}

	if l.wantErr != "" {
		return failf("played; want an error containing %q", l.wantErr)
	}
	cnt.mu.Lock()
	fmt.Printf("LEG %s PASS: %d requests, %d bytes fetched of %d; ranges %v\n",
		l.name, cnt.requests.Load(), cnt.bytes.Load(), cnt.size.Load(), cnt.ranges)
	cnt.mu.Unlock()
	return true
}

// reach waits for playback to pass pos, failing on an error or after 30 s.
func reach(p player, pos time.Duration, errs <-chan error) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-errs:
			return err
		default:
		}
		var now time.Duration
		bellui.DoAndWait(func() { now = p.CurrentTime() })
		if now >= pos {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	var now time.Duration
	bellui.DoAndWait(func() { now = p.CurrentTime() })
	return fmt.Errorf("stuck at %v, short of %v", now, pos)
}

func pct(n, of int64) float64 {
	if of <= 0 {
		return 0
	}
	return 100 * float64(n) / float64(of)
}
