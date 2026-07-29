package subtitle

import (
	"strings"
	"testing"
	"time"
)

func TestParseSRT_Basic(t *testing.T) {
	input := `1
00:00:01,000 --> 00:00:03,000
Hello World

2
00:00:05,000 --> 00:00:07,500
Second subtitle
with two lines

`
	track, err := ParseSRT(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseSRT error: %v", err)
	}

	if len(track.Cues) != 2 {
		t.Fatalf("expected 2 cues, got %d", len(track.Cues))
	}

	c := track.Cues[0]
	if c.Index != 1 {
		t.Errorf("cue 0 index = %d, expected 1", c.Index)
	}
	if c.Start != 1*time.Second {
		t.Errorf("cue 0 start = %v, expected 1s", c.Start)
	}
	if c.End != 3*time.Second {
		t.Errorf("cue 0 end = %v, expected 3s", c.End)
	}
	if c.Text != "Hello World" {
		t.Errorf("cue 0 text = %q, expected %q", c.Text, "Hello World")
	}

	c = track.Cues[1]
	if c.Text != "Second subtitle\nwith two lines" {
		t.Errorf("cue 1 text = %q", c.Text)
	}
	if c.Start != 5*time.Second {
		t.Errorf("cue 1 start = %v", c.Start)
	}
	if c.End != 7500*time.Millisecond {
		t.Errorf("cue 1 end = %v", c.End)
	}
}

func TestParseSRT_HTMLStripping(t *testing.T) {
	input := `1
00:00:01,000 --> 00:00:02,000
<b>Bold</b> and <i>italic</i>

`
	track, err := ParseSRT(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if track.Cues[0].Text != "Bold and italic" {
		t.Errorf("HTML stripping: got %q", track.Cues[0].Text)
	}
}

func TestParseSRTTime(t *testing.T) {
	tests := []struct {
		input  string
		expect time.Duration
	}{
		{"00:00:00,000", 0},
		{"00:00:01,000", time.Second},
		{"00:01:00,000", time.Minute},
		{"01:00:00,000", time.Hour},
		{"00:00:01,500", 1500 * time.Millisecond},
		{"01:23:45,678", 1*time.Hour + 23*time.Minute + 45*time.Second + 678*time.Millisecond},
		{"00:00:01.500", 1500 * time.Millisecond}, // period instead of comma
	}
	for _, tc := range tests {
		got, err := parseSRTTime(tc.input)
		if err != nil {
			t.Errorf("parseSRTTime(%q) error: %v", tc.input, err)
			continue
		}
		if got != tc.expect {
			t.Errorf("parseSRTTime(%q) = %v, expected %v", tc.input, got, tc.expect)
		}
	}
}

func TestTrack_CueAt(t *testing.T) {
	track := &Track{
		Cues: []Cue{
			{Start: 1 * time.Second, End: 3 * time.Second, Text: "first"},
			{Start: 5 * time.Second, End: 7 * time.Second, Text: "second"},
		},
	}

	if c := track.CueAt(0); c != nil {
		t.Error("expected nil at t=0")
	}
	if c := track.CueAt(2 * time.Second); c == nil || c.Text != "first" {
		t.Errorf("expected 'first' at t=2s, got %v", c)
	}
	if c := track.CueAt(4 * time.Second); c != nil {
		t.Error("expected nil at t=4s (gap)")
	}
	if c := track.CueAt(6 * time.Second); c == nil || c.Text != "second" {
		t.Errorf("expected 'second' at t=6s, got %v", c)
	}
}

func TestParseWebVTT_Basic(t *testing.T) {
	input := `WEBVTT

00:00:01.000 --> 00:00:03.000
Hello World

00:05.000 --> 00:07.500
No hours
`
	track, err := ParseWebVTT(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseWebVTT error: %v", err)
	}

	if len(track.Cues) != 2 {
		t.Fatalf("expected 2 cues, got %d", len(track.Cues))
	}

	if track.Cues[0].Start != time.Second {
		t.Errorf("cue 0 start = %v", track.Cues[0].Start)
	}
	if track.Cues[1].Start != 5*time.Second {
		t.Errorf("cue 1 start = %v (MM:SS format)", track.Cues[1].Start)
	}
}

func TestParseWebVTT_SkipsNotes(t *testing.T) {
	input := `WEBVTT

NOTE This is a comment

00:00:01.000 --> 00:00:02.000
Actual subtitle
`
	track, err := ParseWebVTT(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(track.Cues) != 1 {
		t.Fatalf("expected 1 cue (note skipped), got %d", len(track.Cues))
	}
}
