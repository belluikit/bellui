// Package codecset says which of goffmpeg's codec sets the build links, so a
// test whose fixture only the media set opens (MP4, M4A, HLS) can skip in a
// goffmpeg_novideo build, which links the audio set: MP3 and WAV
// (goffmpeg#41).
package codecset

import "testing"

// NeedsMedia skips t in a build that links goffmpeg's audio set.
func NeedsMedia(t testing.TB) {
	t.Helper()
	if !Media {
		t.Skip("needs goffmpeg's media codec set (MP4, M4A, HLS); a goffmpeg_novideo build links the audio set, MP3 and WAV")
	}
}
