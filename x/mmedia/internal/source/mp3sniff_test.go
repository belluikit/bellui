package source

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/belluikit/goffmpeg"

	"github.com/belluikit/bellui/x/mmedia/internal/codecset"
)

// cbrMedia is CBR with an Info tag (testdata/mklong.sh), 60 s, behind an ID3
// tag.
const cbrMedia = "testdata/long.mp3"

// layer3Frames is n Layer III frames, MPEG-1 44.1 kHz stereo, at the bitrate
// indexes in bri, in turn. A tag ("Info", "Xing") goes into the first frame
// where FFmpeg looks for it.
func layer3Frames(n int, tag string, bri ...byte) []byte {
	var out []byte
	for i := range n {
		b := bri[i%len(bri)]
		h := []byte{0xff, 0xfb, b << 4, 0x00} // MPEG-1 Layer III, no CRC, 44.1 kHz, stereo
		f, err := parseMP3Frame(h)
		if err != nil {
			panic(err)
		}
		frame := make([]byte, f.length)
		copy(frame, h)
		if i == 0 && tag != "" {
			copy(frame[4+32:], tag)
		}
		out = append(out, frame...)
	}
	return out
}

func TestMP3IsCBR(t *testing.T) {
	long, err := os.ReadFile(cbrMedia)
	if err != nil {
		t.Fatal(err)
	}
	vbri := layer3Frames(20, "", 9)
	copy(vbri[36:], "VBRI")
	mono2 := []byte{0xff, 0xf3, 0x90, 0xc0} // MPEG-2 Layer III, 22.05 kHz, 128 kbit/s, mono
	f, err := parseMP3Frame(mono2)
	if err != nil {
		t.Fatal(err)
	}
	mono2 = append(mono2, make([]byte, f.length-4)...)
	copy(mono2[4+9:], "Info")

	for _, c := range []struct {
		name string
		b    []byte
		want bool
	}{
		{"ffmpeg CBR with Info, after its ID3 tag", long[id3Size(long):], true},
		{"Info tag", layer3Frames(20, "Info", 9), true},
		{"Info tag, MPEG-2 mono", mono2, true},
		{"Xing tag", layer3Frames(20, "Xing", 9), false},
		{"VBRI tag", vbri, false},
		{"no tag, one bitrate", layer3Frames(20, "", 9), true},
		{"no tag, bitrates vary", layer3Frames(20, "", 9, 9, 11), false},
		{"no tag, too few frames to tell", layer3Frames(5, "", 9), false},
		{"ID3 tag not skipped", long, false},
		{"Ogg", []byte("OggS\x00\x02" + strings.Repeat("\x00", 64)), false},
		{"empty", nil, false},
	} {
		if got := mp3IsCBR(c.b); got != c.want {
			t.Errorf("%s: mp3IsCBR = %v, want %v", c.name, got, c.want)
		}
	}
}

// mp3Server serves data, honouring Range or not, and records each request's
// Range header (empty for none) and the bytes it sent.
type mp3Server struct {
	*httptest.Server
	mu     sync.Mutex
	ranges []string
	sent   int64
}

func newMP3Server(t *testing.T, data []byte, honourRange bool) *mp3Server {
	s := &mp3Server{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.ranges = append(s.ranges, r.Header.Get("Range"))
		s.mu.Unlock()
		cw := &countWriter{ResponseWriter: w, s: s}
		if honourRange {
			http.ServeContent(cw, r, "a.mp3", time.Time{}, bytes.NewReader(data))
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		_, _ = cw.Write(data)
	}))
	t.Cleanup(s.Close)
	return s
}

type countWriter struct {
	http.ResponseWriter
	s *mp3Server
}

func (w *countWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.s.mu.Lock()
	w.s.sent += int64(n)
	w.s.mu.Unlock()
	return n, err
}

func (s *mp3Server) log() ([]string, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.ranges...), s.sent
}

func TestSniffCBR(t *testing.T) {
	long, err := os.ReadFile(cbrMedia)
	if err != nil {
		t.Fatal(err)
	}
	// Cover art makes an ID3 tag larger than the first read.
	bigTag := make([]byte, 10+300<<10)
	copy(bigTag, "ID3\x04\x00\x00")
	n := len(bigTag) - 10
	bigTag[6], bigTag[7], bigTag[8], bigTag[9] = byte(n>>21&0x7f), byte(n>>14&0x7f), byte(n>>7&0x7f), byte(n&0x7f)
	behindArt := append(bigTag, long[id3Size(long):]...)

	for _, c := range []struct {
		name string
		data []byte
		want bool
	}{
		{"CBR", long, true},
		{"CBR behind a large tag", behindArt, true},
		{"VBR", layer3Frames(400, "Xing", 9, 10), false},
		{"not an MP3", bytes.Repeat([]byte("OggS"), 1<<15), false},
	} {
		for _, honour := range []bool{true, false} {
			srv := newMP3Server(t, c.data, honour)
			if got := sniffCBR(context.Background(), srv.Client(), srv.URL); got != c.want {
				t.Errorf("%s, Range honoured %v: sniffCBR = %v, want %v", c.name, honour, got, c.want)
			}
		}
	}

	missing := httptest.NewServer(http.NotFoundHandler())
	defer missing.Close()
	if sniffCBR(context.Background(), missing.Client(), missing.URL) {
		t.Error("a 404 was taken for a CBR MP3")
	}
}

// TestURLSeekInCBRAsksForTheTarget pins #1098's done-when on the desktop: a
// far seek in a CBR MP3 over http makes a ranged request near the target,
// rather than reading the file forward from its start, and lands on the
// target. On a server that ignores Range it lands too. The file is ten
// minutes of silent 128 kbit/s frames, large enough that reading forward is
// not a short skip FFmpeg makes within the open response.
func TestURLSeekInCBRAsksForTheTarget(t *testing.T) {
	codecset.NeedsMedia(t)
	const frames = 600 * 44100 / 1152
	data := layer3Frames(frames, "Info", 9)
	const target = 500 * time.Second
	for _, honour := range []bool{true, false} {
		t.Run(fmt.Sprintf("range=%v", honour), func(t *testing.T) {
			srv := newMP3Server(t, data, honour)
			s, err := NewFFmpegSourceURL(srv.URL + "/a.mp3")
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			s.Seek(target)
			if err := s.Err(); err != nil || s.HasEnded() {
				t.Fatalf("seek to %v: ended %v, err %v", target, s.HasEnded(), err)
			}
			if pos := time.Duration(s.dec.Position() * float64(time.Second)); pos < target || pos > target+500*time.Millisecond {
				t.Errorf("landed at %v, want %v", pos, target)
			}
			ranges, _ := srv.log()
			t.Logf("requests %q", ranges)
			if !honour {
				return
			}
			near := false
			for _, r := range ranges {
				var from int
				// FFmpeg also asks for the last 128 bytes, for an ID3v1 tag.
				if _, err := fmt.Sscanf(r, "bytes=%d-", &from); err == nil &&
					from > len(data)*8/10 && from < len(data)-1024 {
					near = true
				}
			}
			// What the server sent says little: on loopback a response
			// fills the socket buffers whether or not FFmpeg reads it.
			if !near {
				t.Errorf("no request started near the target (80%% into %d bytes): %q", len(data), ranges)
			}
		})
	}
}

// TestSeekErrorSaysTheServerCannotSeek pins the second half of #1098's
// done-when: a seek a server refuses fails without FFmpeg's number in the
// message, and the code is still there for errors.As.
func TestSeekErrorSaysTheServerCannotSeek(t *testing.T) {
	cause := fmt.Errorf("seek failed: %w", goffmpeg.AVError(-errnoENOSYS))
	err := seekError(717*time.Second, "http://example.com/a.mp3", cause)
	msg := err.Error()
	if strings.Contains(msg, strconv.Itoa(errnoENOSYS)) || !strings.Contains(msg, "server") {
		t.Errorf("message %q: want the server named and no %d", msg, errnoENOSYS)
	}
	var averr goffmpeg.AVError
	if !errors.As(err, &averr) || int(averr) != -errnoENOSYS {
		t.Errorf("errors.As lost the AVError from %v", err)
	}

	other := fmt.Errorf("seek failed: %w", goffmpeg.AVError(-5))
	if got := seekError(time.Second, "http://example.com/a.mp3", other).Error(); got != "mmedia: seek to 1s failed: "+other.Error() {
		t.Errorf("another failure reads %q", got)
	}
}

// TestInterruptEndsTheCBRCheck pins that Interrupt reaches preferFastSeek's
// requests: a stop must not wait on a server that is slow to answer (#884).
func TestInterruptEndsTheCBRCheck(t *testing.T) {
	codecset.NeedsMedia(t)
	data, err := os.ReadFile(cbrMedia)
	if err != nil {
		t.Fatal(err)
	}
	asked := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.UserAgent(), "Go-http-client") {
			asked <- struct{}{}
			<-r.Context().Done() // the check's request: never answered
			return
		}
		http.ServeContent(w, r, "a.mp3", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	s, err := NewFFmpegSourceURL(srv.URL + "/long.mp3")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Seek(30 * time.Second)
	}()
	select {
	case <-asked:
	case <-time.After(5 * time.Second):
		t.Fatal("the seek made no CBR check")
	}
	start := time.Now()
	s.Interrupt()
	select {
	case <-done:
		if d := time.Since(start); d > time.Second {
			t.Errorf("Seek returned %v after Interrupt", d)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Seek did not return after Interrupt")
	}
}
