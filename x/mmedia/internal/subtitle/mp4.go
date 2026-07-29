package subtitle

import (
	"encoding/binary"
	"strings"
	"time"
)

// ParseTx3g extracts subtitle text from an MP4 tx3g (3GPP Timed Text) sample.
// Each sample is a length-prefixed UTF-8 string, optionally followed by style atoms.
func ParseTx3g(data []byte) string {
	if len(data) < 2 {
		return ""
	}
	textLen := int(binary.BigEndian.Uint16(data[0:2]))
	if textLen <= 0 || 2+textLen > len(data) {
		return ""
	}
	return string(data[2 : 2+textLen])
}

// ParseWVTT extracts subtitle text from an MP4 wvtt (WebVTT in ISO BMFF) sample.
// Each sample contains one or more VTTCueBox (vttc) boxes.
func ParseWVTT(data []byte) string {
	var texts []string
	off := 0
	for off+8 <= len(data) {
		boxSize := int(binary.BigEndian.Uint32(data[off:]))
		boxType := string(data[off+4 : off+8])
		if boxSize < 8 || off+boxSize > len(data) {
			break
		}
		if boxType == "vttc" {
			text := parseVTTC(data[off+8 : off+boxSize])
			if text != "" {
				texts = append(texts, text)
			}
		}
		off += boxSize
	}
	return strings.Join(texts, "\n")
}

func parseVTTC(data []byte) string {
	off := 0
	for off+8 <= len(data) {
		boxSize := int(binary.BigEndian.Uint32(data[off:]))
		boxType := string(data[off+4 : off+8])
		if boxSize < 8 || off+boxSize > len(data) {
			break
		}
		if boxType == "payl" {
			// Payload box contains the cue text.
			return string(data[off+8 : off+boxSize])
		}
		off += boxSize
	}
	return ""
}

// BuildTrackFromMP4Samples builds a subtitle Track from a series of
// MP4 subtitle samples with their timestamps and durations.
type MP4SubtitleSample struct {
	Data     []byte
	PTS      time.Duration
	Duration time.Duration
	Codec    string // "tx3g" or "wvtt"
}

// BuildTrackFromMP4Samples creates a Track from MP4 subtitle samples.
func BuildTrackFromMP4Samples(samples []MP4SubtitleSample) *Track {
	track := &Track{}

	for i, s := range samples {
		var text string
		switch s.Codec {
		case "tx3g", "text":
			text = ParseTx3g(s.Data)
		case "wvtt":
			text = ParseWVTT(s.Data)
		default:
			text = ParseTx3g(s.Data) // default to tx3g
		}

		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		track.Cues = append(track.Cues, Cue{
			Index: i + 1,
			Start: s.PTS,
			End:   s.PTS + s.Duration,
			Text:  text,
		})
	}

	return track
}
