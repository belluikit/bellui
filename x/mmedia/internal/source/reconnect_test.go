package source

import (
	"bytes"
	"image"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/belluikit/bellui/x/mmedia/internal/codecset"
)

const reconnectMedia = "testdata/sample_stream.mp4" // index at the front

// decodeAll decodes s to its end and counts what it delivered.
func decodeAll(t *testing.T, s *FFmpegSource, within time.Duration) (frames, audioBytes int) {
	t.Helper()
	s.SetVideoCallback(func(*image.RGBA, time.Duration) { frames++ })
	s.SetAudioCallback(func(b []byte) { audioBytes += len(b) })
	done := make(chan struct{})
	go func() {
		defer close(done)
		for !s.HasEnded() {
			s.Decode(0)
		}
	}()
	select {
	case <-done:
	case <-time.After(within):
		t.Fatalf("decoding did not finish within %v", within)
	}
	return frames, audioBytes
}

// failFirst serves data honouring Range, except that the first request sends
// the first half and then fails as fail says.
func failFirst(t *testing.T, data []byte, fail func(http.ResponseWriter, *http.Request)) (*httptest.Server, func() int) {
	var mu sync.Mutex
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n++
		first := n == 1
		mu.Unlock()
		if !first {
			http.ServeContent(w, r, "s.mp4", time.Time{}, bytes.NewReader(data))
			return
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		_, _ = w.Write(data[:len(data)/2])
		w.(http.Flusher).Flush()
		fail(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, func() int { mu.Lock(); defer mu.Unlock(); return n }
}

// TestURLSourceRidesOutANetworkFailure pins the http options of #839: a
// connection that drops, or goes silent without closing, is picked up again
// from where it stopped, and the whole file decodes. Without them a drop ended
// the stream partway and a stall blocked the read forever.
func TestURLSourceRidesOutANetworkFailure(t *testing.T) {
	codecset.NeedsMedia(t)
	data, err := os.ReadFile(reconnectMedia)
	if err != nil {
		t.Fatal(err)
	}
	control, err := NewFFmpegSourceURL(reconnectMedia)
	if err != nil {
		t.Fatal(err)
	}
	wantFrames, wantAudio := decodeAll(t, control, 10*time.Second)
	control.Close()

	for _, tc := range []struct {
		name string
		fail func(http.ResponseWriter, *http.Request)
	}{
		{"dropped", func(w http.ResponseWriter, _ *http.Request) {
			if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
				_ = conn.(*net.TCPConn).Close()
			}
		}},
		{"stalled", func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, requests := failFirst(t, data, tc.fail)
			s, err := NewFFmpegSourceURL(srv.URL + "/s.mp4")
			if err != nil {
				t.Fatalf("NewFFmpegSourceURL: %v", err)
			}
			defer s.Close()
			frames, audio := decodeAll(t, s, 20*time.Second)
			t.Logf("%s: %d frames, %d audio bytes over %d requests; the file has %d and %d",
				tc.name, frames, audio, requests(), wantFrames, wantAudio)
			if err := s.Err(); err != nil {
				t.Fatalf("ended on %v; want it to reconnect and finish", err)
			}
			if requests() < 2 {
				t.Errorf("%d request(s); want a reconnect after the failure", requests())
			}
			if frames != wantFrames || audio != wantAudio {
				t.Errorf("decoded %d frames and %d audio bytes; want the file's %d and %d",
					frames, audio, wantFrames, wantAudio)
			}
		})
	}
}

// dropOne serves data, the whole of it every time and ignoring Range, except
// that request number drop (from 1) is closed after cut bytes. With length
// false it sends no Content-Length, as a live stream does.
func dropOne(t *testing.T, data []byte, drop, cut int, length bool) (*httptest.Server, func() int) {
	var mu sync.Mutex
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		n++
		dropThis := n == drop
		mu.Unlock()
		if length {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		} else {
			w.(http.Flusher).Flush() // chunked
		}
		if !dropThis {
			_, _ = w.Write(data)
			return
		}
		_, _ = w.Write(data[:cut])
		w.(http.Flusher).Flush()
		if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
			_ = conn.(*net.TCPConn).Close()
		}
	}))
	t.Cleanup(srv.Close)
	return srv, func() int { mu.Lock(); defer mu.Unlock(); return n }
}

// coverMP3 is an MP3 because MP3 is frames with no index: bytes from the file's
// start, spliced into the stream, decode as more audio. An MP4 fails to parse
// them, which would hide the splice.
const coverMP3 = "testdata/cover.mp3"

// TestURLSourceFailsADropFromAServerWithoutRange pins #868. A server that
// ignores Range answered a reconnect with the file from byte 0, and FFmpeg
// played that as the continuation: after a seek near the end, the position ran
// on past the duration, with no error. Such a server cannot resume a file, so a
// drop has to end in an error.
func TestURLSourceFailsADropFromAServerWithoutRange(t *testing.T) {
	data, err := os.ReadFile(coverMP3)
	if err != nil {
		t.Fatal(err)
	}
	srv, requests := dropOne(t, data, 1, len(data)/4, true)
	s, err := NewFFmpegSourceURL(srv.URL + "/s.mp3")
	if err != nil {
		t.Fatalf("NewFFmpegSourceURL: %v", err)
	}
	defer s.Close()
	dur := s.Duration()
	s.Seek(dur - 500*time.Millisecond)
	last := 0.0
	for !s.HasEnded() {
		s.Decode(0)
		if s.dec != nil {
			last = max(last, s.dec.Position())
		}
	}
	t.Logf("duration %v, last position %.2fs, %d request(s), error %v", dur, last, requests(), s.Err())
	if last > dur.Seconds()+0.1 {
		t.Errorf("decoded on to %.2fs, past the %v duration: the file's start was spliced in", last, dur)
	}
	if s.Err() == nil {
		t.Error("the drop ended with no error; want it reported")
	}
}

// TestURLSourceFailsADropOfALiveStream is the other half of #868's fix: a
// stream with no length (live radio) keeps reconnect_streamed, without which a
// drop read as a clean end of the stream, so the player said OnEnd. The open
// that finds no duration is request 1; the stream that plays, and drops, is 2.
func TestURLSourceFailsADropOfALiveStream(t *testing.T) {
	song, err := os.ReadFile(coverMP3)
	if err != nil {
		t.Fatal(err)
	}
	// A minute of its audio frames, so the drop comes well after FFmpeg's
	// probe. The ID3 tag, which holds the cover art, would break the join.
	data := bytes.Repeat(mp3Frames(t, song), 20)
	srv, requests := dropOne(t, data, 2, len(data)/2, false)
	s, err := NewFFmpegSourceURL(srv.URL + "/live")
	if err != nil {
		t.Fatalf("NewFFmpegSourceURL: %v", err)
	}
	defer s.Close()
	if d := s.Duration(); d != 0 {
		t.Fatalf("Duration = %v; the test needs a stream FFmpeg cannot size", d)
	}
	_, audio := decodeAll(t, s, 20*time.Second)
	t.Logf("%d audio bytes over %d request(s), error %v", audio, requests(), s.Err())
	if s.Err() == nil {
		t.Error("a dropped live stream ended with no error, which the player reports as OnEnd")
	}
}

// mp3Frames is song without its ID3v2 tag (and an ID3v1 one, if any): the
// MPEG audio frames alone, which can be repeated into a longer stream.
func mp3Frames(t *testing.T, song []byte) []byte {
	t.Helper()
	if len(song) < 10 || string(song[:3]) != "ID3" {
		t.Fatal("want an MP3 with an ID3v2 tag")
	}
	// The tag's size is a syncsafe integer: 7 bits a byte.
	size := int(song[6])<<21 | int(song[7])<<14 | int(song[8])<<7 | int(song[9])
	frames := song[10+size:]
	if n := len(frames); n > 128 && string(frames[n-128:n-125]) == "TAG" {
		frames = frames[:n-128]
	}
	if frames[0] != 0xff || frames[1]&0xe0 != 0xe0 {
		t.Fatalf("no MPEG frame sync after the tag: % x", frames[:2])
	}
	return frames
}

// TestURLSourceReportsASeekTheServerCannotDo is #869 through real FFmpeg, as
// measured: after a drop on a server that ignores Range, FFmpeg cannot seek
// back, and dec.Seek fails with ENOSYS (-78 on macOS, -38 on Linux and
// Android). That error was dropped.
func TestURLSourceReportsASeekTheServerCannotDo(t *testing.T) {
	song, err := os.ReadFile(coverMP3)
	if err != nil {
		t.Fatal(err)
	}
	data := bytes.Repeat(mp3Frames(t, song), 20)
	srv, _ := dropOne(t, data, 1, len(data)/2, true)
	s, err := NewFFmpegSourceURL(srv.URL + "/s.mp3")
	if err != nil {
		t.Fatalf("NewFFmpegSourceURL: %v", err)
	}
	defer s.Close()
	decodeAll(t, s, 20*time.Second)
	if s.Err() == nil {
		t.Fatal("the drop ended with no error; the seek below needs a dropped stream")
	}
	s.Seek(time.Second)
	t.Logf("ended %v, Err %v", s.HasEnded(), s.Err())
	if !s.HasEnded() || s.Err() == nil || !strings.Contains(s.Err().Error(), "seek to 1s failed") {
		t.Fatalf("after a seek back the server cannot serve: ended %v, Err %v; want the seek's error",
			s.HasEnded(), s.Err())
	}
}
