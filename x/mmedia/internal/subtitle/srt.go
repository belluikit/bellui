package subtitle

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// ParseSRT parses an SRT subtitle file from a reader.
//
// SRT format:
//
//	1
//	00:01:23,456 --> 00:01:25,789
//	First line of text
//	Second line of text
//
//	2
//	...
func ParseSRT(r io.Reader) (*Track, error) {
	scanner := bufio.NewScanner(r)
	track := &Track{}

	for {
		cue, err := parseSRTCue(scanner)
		if err != nil {
			break
		}
		if cue != nil {
			track.Cues = append(track.Cues, *cue)
		}
	}

	if len(track.Cues) == 0 {
		return nil, fmt.Errorf("srt: no cues found")
	}

	return track, nil
}

func parseSRTCue(scanner *bufio.Scanner) (*Cue, error) {
	// Skip blank lines.
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			// This should be the cue index.
			idx, err := strconv.Atoi(line)
			if err != nil {
				continue // Skip non-numeric lines.
			}

			cue := &Cue{Index: idx}

			// Read timestamp line.
			if !scanner.Scan() {
				return nil, io.EOF
			}
			tsLine := strings.TrimSpace(scanner.Text())
			start, end, err := parseSRTTimestamps(tsLine)
			if err != nil {
				return nil, err
			}
			cue.Start = start
			cue.End = end

			// Read text lines until blank line or EOF.
			var textLines []string
			for scanner.Scan() {
				line := scanner.Text()
				if strings.TrimSpace(line) == "" {
					break
				}
				textLines = append(textLines, line)
			}
			cue.Text = stripSRTFormatting(strings.Join(textLines, "\n"))

			return cue, nil
		}
	}
	return nil, io.EOF
}

// parseSRTTimestamps parses "00:01:23,456 --> 00:01:25,789".
func parseSRTTimestamps(line string) (start, end time.Duration, err error) {
	parts := strings.Split(line, "-->")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("srt: invalid timestamp line: %q", line)
	}

	start, err = parseSRTTime(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, err
	}
	end, err = parseSRTTime(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, err
	}

	return start, end, nil
}

// parseSRTTime parses "00:01:23,456" or "00:01:23.456".
func parseSRTTime(s string) (time.Duration, error) {
	// Normalize comma to period.
	s = strings.Replace(s, ",", ".", 1)

	var h, m, sec, ms int
	// Try "HH:MM:SS.mmm" format.
	n, err := fmt.Sscanf(s, "%d:%d:%d.%d", &h, &m, &sec, &ms)
	if err != nil || n < 3 {
		// Try without milliseconds.
		n, err = fmt.Sscanf(s, "%d:%d:%d", &h, &m, &sec)
		if err != nil || n < 3 {
			return 0, fmt.Errorf("srt: invalid time: %q", s)
		}
	}

	return time.Duration(h)*time.Hour +
		time.Duration(m)*time.Minute +
		time.Duration(sec)*time.Second +
		time.Duration(ms)*time.Millisecond, nil
}

// stripSRTFormatting removes HTML-like tags from SRT text.
// Common tags: <b>, <i>, <u>, <font color="...">.
func stripSRTFormatting(s string) string {
	var result strings.Builder
	inTag := false
	for _, ch := range s {
		if ch == '<' {
			inTag = true
			continue
		}
		if ch == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(ch)
		}
	}
	return result.String()
}
