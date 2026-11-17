package source

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// An MP3 over http seeks one of two ways (#1098). By default FFmpeg reads the
// file forward from its start to the target, which lands exactly but downloads
// everything before it: 23 MB for a seek near the end of a 12-minute song.
// With fflags=fastseek it computes the byte position and makes one ranged
// request. That is exact for a CBR file. For a VBR file it uses the file's
// 100-entry Xing table, and the seek lands up to 1% of the song early, with
// the position shown off by a second or more until the next seek. Measured on
// macOS with goffmpeg v0.13.0: CBR 717.04 s for 717 s, VBR 711.58 s.
//
// FFmpeg does not say which kind a file is, so sniffCBR reads the start of the
// file itself, and only a CBR file is opened with fast seek (owner's pick on
// #1098). A VBR file, and anything not known to be CBR, seeks as before.

// fastSeekOptions is what an open adds to seek a CBR MP3 by its byte position.
const fastSeekOptions = "fflags=fastseek"

// sniffLimit bounds what sniffCBR reads from a server that ignores Range: it
// has to read through any ID3 tag (cover art can be large) to reach the first
// frame.
const sniffLimit = 4 << 20

// sniffFrames is how many frames sniffCBR reads past the first. A file with no
// Info, Xing or VBRI tag is taken for CBR only if they all share its bitrate.
const sniffFrames = 16

// sniffSpan covers the first frame and sniffFrames more at the largest Layer
// III frame, 1441 bytes (320 kbit/s at 32 kHz).
const sniffSpan = 32 << 10

// sniffCBR reports whether the MP3 at url is CBR. Any failure, and anything
// that is not an MP3, answers false: the file then seeks as it always has.
func sniffCBR(ctx context.Context, client *http.Client, url string) bool {
	const headLen = 64 << 10
	head, err := fetchRange(ctx, client, url, 0, headLen)
	if err != nil {
		return false
	}
	off := id3Size(head)
	if len(head) == headLen && off+sniffSpan > len(head) {
		// The file goes on, and the frames to read lie past what was read:
		// behind a large tag, such as one with cover art.
		if head, err = fetchRange(ctx, client, url, int64(off), sniffSpan); err != nil {
			return false
		}
		off = 0
	}
	return mp3IsCBR(head[min(off, len(head)):])
}

// fetchRange reads n bytes of url from off, by a ranged request. A server that
// ignores Range answers with the whole file, which is read through to off.
func fetchRange(ctx context.Context, client *http.Client, url string, off int64, n int) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+int64(n)-1))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusPartialContent:
	case http.StatusOK:
		if off+int64(n) > sniffLimit {
			return nil, fmt.Errorf("mmedia: the first frame is beyond %d bytes", sniffLimit)
		}
		if _, err := io.CopyN(io.Discard, resp.Body, off); err != nil {
			return nil, err
		}
	default:
		return nil, &statusError{url: url, code: resp.StatusCode}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, int64(n)))
	if err != nil {
		return nil, err
	}
	return b, nil
}

// id3Size is the length of the ID3v2 tag at the start of b, or 0.
func id3Size(b []byte) int {
	if len(b) < 10 || !bytes.Equal(b[:3], []byte("ID3")) {
		return 0
	}
	n := int(b[6]&0x7f)<<21 | int(b[7]&0x7f)<<14 | int(b[8]&0x7f)<<7 | int(b[9]&0x7f)
	n += 10
	if b[5]&0x10 != 0 { // a footer follows the tag
		n += 10
	}
	return n
}

// mp3Frame is what a Layer III frame header says.
type mp3Frame struct {
	mpeg1   bool
	mono    bool
	bitrate int // kbit/s
	length  int // bytes, header included
}

var (
	mp3Bitrates1 = [15]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320}
	mp3Bitrates2 = [15]int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160}
	mp3Rates     = [3]int{44100, 48000, 32000}
	errNotAFrame = errors.New("mmedia: not a Layer III frame")
)

// parseMP3Frame reads the Layer III frame header at the start of b.
func parseMP3Frame(b []byte) (mp3Frame, error) {
	if len(b) < 4 || b[0] != 0xff || b[1]&0xe0 != 0xe0 {
		return mp3Frame{}, errNotAFrame
	}
	version := b[1] >> 3 & 3 // 3 MPEG-1, 2 MPEG-2, 0 MPEG-2.5
	layer := b[1] >> 1 & 3   // 1 is Layer III
	bri := int(b[2] >> 4)
	sri := int(b[2] >> 2 & 3)
	// A free-format frame (index 0) does not say its length.
	if version == 1 || layer != 1 || bri == 0 || bri == 15 || sri == 3 {
		return mp3Frame{}, errNotAFrame
	}
	f := mp3Frame{mpeg1: version == 3, mono: b[3]>>6 == 3}
	rate := mp3Rates[sri]
	per := 144 // bytes per kbit/s at 1 Hz, for 1152 samples
	f.bitrate = mp3Bitrates1[bri]
	switch version {
	case 2:
		rate /= 2
		per, f.bitrate = 72, mp3Bitrates2[bri]
	case 0:
		rate /= 4
		per, f.bitrate = 72, mp3Bitrates2[bri]
	}
	f.length = per*f.bitrate*1000/rate + int(b[2]>>1&1)
	return f, nil
}

// mp3IsCBR reports whether the MP3 whose first frame starts b is CBR. An Info
// tag says CBR and a Xing or VBRI tag says VBR, as they say to FFmpeg. With no
// tag the file is CBR when its first frames all share one bitrate.
func mp3IsCBR(b []byte) bool {
	f, err := parseMP3Frame(b)
	if err != nil {
		return false
	}
	side := 17 // MPEG-1 stereo is 32, MPEG-2 mono 9
	switch {
	case f.mpeg1 && !f.mono:
		side = 32
	case !f.mpeg1 && f.mono:
		side = 9
	}
	if tag := b[min(len(b), 4+side):]; len(tag) >= 4 {
		switch string(tag[:4]) {
		case "Info":
			return true
		case "Xing":
			return false
		}
	}
	if len(b) >= 40 && string(b[36:40]) == "VBRI" {
		return false
	}
	pos := 0
	for range sniffFrames {
		pos += f.length
		next, err := parseMP3Frame(b[min(len(b), pos):])
		if err != nil || next.bitrate != f.bitrate {
			return false
		}
	}
	return true
}
