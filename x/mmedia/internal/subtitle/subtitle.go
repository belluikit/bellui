// Package subtitle provides subtitle parsing and rendering for SRT, WebVTT,
// and MP4-embedded subtitle formats.
package subtitle

import "time"

// Cue represents a single subtitle entry with timing and text.
type Cue struct {
	Index    int
	Start    time.Duration
	End      time.Duration
	Text     string
	Settings string // WebVTT position/alignment settings (optional)
}

// Track holds a list of subtitle cues.
type Track struct {
	Cues     []Cue
	Language string // ISO 639 language code (if known)
	Title    string
}

// CueAt returns the active cue at the given time, or nil if none.
func (t *Track) CueAt(pos time.Duration) *Cue {
	for i := range t.Cues {
		if pos >= t.Cues[i].Start && pos <= t.Cues[i].End {
			return &t.Cues[i]
		}
	}
	return nil
}

// CuesAt returns all active cues at the given time (for overlapping subtitles).
func (t *Track) CuesAt(pos time.Duration) []Cue {
	var result []Cue
	for _, c := range t.Cues {
		if pos >= c.Start && pos <= c.End {
			result = append(result, c)
		}
	}
	return result
}
