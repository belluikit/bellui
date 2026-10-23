package mmedia_test

import (
	"testing"
	"time"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/mmedia"
)

// An app can build a subtitle track itself and hand it to LoadTrack (#339).
// LoadTrack took *subtitle.Track from an internal package, so code outside
// this module could not name its argument and the method was uncallable.
// This test lives in an external package for exactly that reason.
func TestSubtitleOverlay_LoadTrackFromApp(t *testing.T) {
	test.NewTempApp(t)

	so := mmedia.NewSubtitleOverlay()
	w := test.NewTempWindow(t, so)
	_ = w

	so.LoadTrack(&mmedia.SubtitleTrack{
		Language: "en",
		Cues: []mmedia.SubtitleCue{
			{Index: 1, Start: time.Second, End: 2 * time.Second, Text: "first"},
			{Index: 2, Start: 3 * time.Second, End: 4 * time.Second, Text: "second"},
		},
	})

	shown := func() string {
		for _, o := range test.LaidOutObjects(so) {
			if txt, ok := o.(*canvas.Text); ok && txt.Text != "" {
				return txt.Text
			}
		}
		return ""
	}

	so.SetPosition(1500 * time.Millisecond)
	if got := shown(); got != "first" {
		t.Errorf("at 1.5s the overlay shows %q, want %q", got, "first")
	}
	so.SetPosition(2500 * time.Millisecond)
	if got := shown(); got != "" {
		t.Errorf("between cues the overlay shows %q, want nothing", got)
	}
	so.SetPosition(3500 * time.Millisecond)
	if got := shown(); got != "second" {
		t.Errorf("at 3.5s the overlay shows %q, want %q", got, "second")
	}
}
