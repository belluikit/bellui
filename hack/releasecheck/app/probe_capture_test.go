package main

import (
	"bufio"
	"bytes"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/belluikit/bellui/test"
)

// A window that gives no frame fails the capture and the run carries on.
// The nil capture went to png.Encode and aborted the whole run, hiding every
// check after it (#1415).
func TestProbeShot_NoFrameFailsTheCapture(t *testing.T) {
	test.NewTempApp(t)
	var report bytes.Buffer
	tries := 0
	p := &probe{
		dir:  t.TempDir(),
		out:  bufio.NewWriter(&report),
		grab: func() image.Image { tries++; return nil },
	}
	p.shot("blank", "a capture of nothing")
	p.out.Flush()

	if tries != captureTries {
		t.Errorf("asked for a frame %d times, want %d", tries, captureTries)
	}
	if p.fails != 1 || p.shots != 0 {
		t.Errorf("fails=%d shots=%d, want 1 and 0", p.fails, p.shots)
	}
	if !strings.Contains(report.String(), "FAIL: capture blank") {
		t.Errorf("the report does not name the failed capture:\n%s", report.String())
	}
	if _, err := os.Stat(filepath.Join(p.dir, "blank.png")); err == nil {
		t.Error("an empty capture was written to disk")
	}
}

// A frame that comes late, after a nil or two, is used as if it came first.
func TestProbeShot_LateFrameIsKept(t *testing.T) {
	test.NewTempApp(t)
	var report bytes.Buffer
	tries := 0
	p := &probe{
		dir: t.TempDir(),
		out: bufio.NewWriter(&report),
		grab: func() image.Image {
			tries++
			if tries < 3 {
				return nil
			}
			return image.NewRGBA(image.Rect(0, 0, 4, 4))
		},
	}
	p.shot("late", "a frame on the third try")
	p.out.Flush()

	if p.fails != 0 || p.shots != 1 {
		t.Errorf("fails=%d shots=%d, want 0 and 1:\n%s", p.fails, p.shots, report.String())
	}
	if _, err := os.Stat(filepath.Join(p.dir, "late.png")); err != nil {
		t.Errorf("the capture was not written: %v", err)
	}
}
