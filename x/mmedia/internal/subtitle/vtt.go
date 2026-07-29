package subtitle

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"
)

// ParseWebVTT parses a WebVTT subtitle file from a reader.
//
// WebVTT format:
//
//	WEBVTT
//
//	00:01:23.456 --> 00:01:25.789 position:50% align:center
//	First line of text
//	Second line of text
//
//	NOTE This is a comment
func ParseWebVTT(r io.Reader) (*Track, error) {
	scanner := bufio.NewScanner(r)
	track := &Track{}

	// First line must be "WEBVTT" (possibly with BOM or trailing text).
	if !scanner.Scan() {
		return nil, fmt.Errorf("vtt: empty file")
	}
	firstLine := strings.TrimSpace(scanner.Text())
	// Strip BOM if present.
	firstLine = strings.TrimPrefix(firstLine, "\xEF\xBB\xBF")
	if !strings.HasPrefix(firstLine, "WEBVTT") {
		return nil, fmt.Errorf("vtt: missing WEBVTT header")
	}

	index := 1
	for {
		cue, err := parseVTTCue(scanner, index)
		if err != nil {
			break
		}
		if cue != nil {
			track.Cues = append(track.Cues, *cue)
			index++
		}
	}

	if len(track.Cues) == 0 {
		return nil, fmt.Errorf("vtt: no cues found")
	}

	return track, nil
}

func parseVTTCue(scanner *bufio.Scanner, index int) (*Cue, error) {
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Skip NOTE blocks.
		if strings.HasPrefix(line, "NOTE") {
			for scanner.Scan() {
				if strings.TrimSpace(scanner.Text()) == "" {
					break
				}
			}
			continue
		}

		// Skip STYLE blocks.
		if strings.HasPrefix(line, "STYLE") {
			for scanner.Scan() {
				if strings.TrimSpace(scanner.Text()) == "" {
					break
				}
			}
			continue
		}

		// Check if this line is a timestamp or a cue identifier.
		if strings.Contains(line, "-->") {
			return parseVTTTimestampLine(scanner, line, index)
		}

		// Could be a cue identifier — next line should be timestamp.
		if scanner.Scan() {
			nextLine := strings.TrimSpace(scanner.Text())
			if strings.Contains(nextLine, "-->") {
				return parseVTTTimestampLine(scanner, nextLine, index)
			}
		}
	}
	return nil, io.EOF
}

func parseVTTTimestampLine(scanner *bufio.Scanner, tsLine string, index int) (*Cue, error) {
	cue := &Cue{Index: index}

	// Split timestamp and optional settings.
	// "00:01:23.456 --> 00:01:25.789 position:50% align:center"
	parts := strings.SplitN(tsLine, "-->", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("vtt: invalid timestamp: %q", tsLine)
	}

	startStr := strings.TrimSpace(parts[0])
	rightSide := strings.TrimSpace(parts[1])

	// Right side may have settings after the end timestamp.
	endParts := strings.Fields(rightSide)
	if len(endParts) == 0 {
		return nil, fmt.Errorf("vtt: missing end timestamp")
	}
	endStr := endParts[0]
	if len(endParts) > 1 {
		cue.Settings = strings.Join(endParts[1:], " ")
	}

	var err error
	cue.Start, err = parseVTTTime(startStr)
	if err != nil {
		return nil, err
	}
	cue.End, err = parseVTTTime(endStr)
	if err != nil {
		return nil, err
	}

	// Read text lines.
	var textLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			break
		}
		textLines = append(textLines, line)
	}
	cue.Text = stripVTTFormatting(strings.Join(textLines, "\n"))

	return cue, nil
}

// parseVTTTime parses "00:01:23.456" or "01:23.456" (hours optional in WebVTT).
func parseVTTTime(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)

	parts := strings.Split(s, ":")
	switch len(parts) {
	case 3:
		// HH:MM:SS.mmm
		return parseSRTTime(s)
	case 2:
		// MM:SS.mmm (no hours)
		return parseSRTTime("00:" + s)
	default:
		return 0, fmt.Errorf("vtt: invalid time: %q", s)
	}
}

// stripVTTFormatting removes WebVTT tags (<b>, <i>, <c.classname>, <v Speaker>, etc).
func stripVTTFormatting(s string) string {
	return stripSRTFormatting(s)
}
