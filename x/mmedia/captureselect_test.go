package mmedia

import (
	"errors"
	"strings"
	"testing"
)

// None of these open a device. That is the point: the picker's whole job is to
// decide what to show and what to report, and every one of those decisions is
// reachable without hardware. Probing itself is covered only at its guard rails
// — an actual probe lights the camera, which no unit test should do.

func TestCaptureLabelMarksThePlatformDefault(t *testing.T) {
	plain := CaptureDevice{Name: "0", Description: "Camera 1"}
	if got := captureLabel(plain); got != "Camera 1" {
		t.Errorf("label = %q, want the bare description", got)
	}

	// The marker earns its place on a probed list, where the names are
	// synthesised and nothing else distinguishes one entry from another.
	def := CaptureDevice{Name: "0", Description: "Camera 0", Default: true}
	if got := captureLabel(def); !strings.Contains(got, "Camera 0") ||
		!strings.Contains(got, "default") {
		t.Errorf("label = %q, want the description marked as the default", got)
	}
}

// TestMergeCurrentKeepsAnUndiscoverableDevice is the in-use case: a probe cannot
// open a camera this process already holds, so discovery reports it as absent.
// Without this the picker would drop the camera it is currently displaying.
func TestMergeCurrentKeepsAnUndiscoverableDevice(t *testing.T) {
	found := []CaptureDevice{{Name: "1", Description: "Camera 1"}}
	current := CaptureDevice{Name: "0", Description: "Camera 0"}

	got := mergeCurrent(found, &current)
	if len(got) != 2 {
		t.Fatalf("merged list = %v, want the in-use device kept", got)
	}
	if got[0].Name != "1" || got[1].Name != "0" {
		t.Errorf("merged list = %v, want discovery order with the missing device appended", got)
	}
}

// TestMergeCurrentDoesNotDuplicate: a device that *was* found keeps its place
// rather than being appended a second time.
func TestMergeCurrentDoesNotDuplicate(t *testing.T) {
	found := []CaptureDevice{
		{Name: "0", Description: "Camera 0"},
		{Name: "1", Description: "Camera 1"},
	}
	current := CaptureDevice{Name: "0", Description: "Camera 0"}

	got := mergeCurrent(found, &current)
	if len(got) != 2 {
		t.Fatalf("merged list = %v, want no duplicate", got)
	}
	if got[0].Name != "0" {
		t.Errorf("merged list = %v, want discovery order preserved", got)
	}
}

func TestMergeCurrentWithNoCurrentDevice(t *testing.T) {
	found := []CaptureDevice{{Name: "0"}}
	if got := mergeCurrent(found, nil); len(got) != 1 {
		t.Errorf("merged list = %v, want the discovered list unchanged", got)
	}
}

func TestSelectedLabelFindsTheCurrentDevice(t *testing.T) {
	devs := []CaptureDevice{
		{Name: "0", Description: "Camera 0", Default: true},
		{Name: "1", Description: "Camera 1"},
	}
	if got, want := selectedLabel(devs, &devs[1]), "Camera 1"; got != want {
		t.Errorf("selectedLabel = %q, want %q", got, want)
	}
	// The default marker is part of the label, so the selection has to carry it
	// or Select.SetSelected would match nothing.
	if got := selectedLabel(devs, &devs[0]); !strings.Contains(got, "default") {
		t.Errorf("selectedLabel = %q, want the marked label the dropdown holds", got)
	}
	if got := selectedLabel(devs, nil); got != "" {
		t.Errorf("selectedLabel = %q with no current device, want empty", got)
	}
	absent := CaptureDevice{Name: "9"}
	if got := selectedLabel(devs, &absent); got != "" {
		t.Errorf("selectedLabel = %q for an unlisted device, want empty", got)
	}
}

// TestCaptureSelectReportsOnlyUserChoices is the fork's SetX/SetXSilently rule
// applied to a selection: populating the list must not look like a choice, or
// every Reload would switch the camera.
func TestCaptureSelectReportsOnlyUserChoices(t *testing.T) {
	var chosen []CaptureDevice
	s := NewCaptureSelect(func(d CaptureDevice) { chosen = append(chosen, d) })

	devs := []CaptureDevice{
		{Name: "0", Description: "Camera 0", HasVideo: true},
		{Name: "1", Description: "Camera 1", HasVideo: true},
	}
	s.publish(devs, nil, false)
	if len(chosen) != 0 {
		t.Fatalf("populating the list reported %d choice(s), want none", len(chosen))
	}

	// SetCurrent records a decision already taken elsewhere; it must not echo it
	// back as a fresh one.
	s.SetCurrent(devs[0])
	if len(chosen) != 0 {
		t.Fatalf("SetCurrent reported %d choice(s), want none", len(chosen))
	}

	// A user picking from the dropdown is the one thing that does report.
	s.chosen(captureLabel(devs[1]))
	if len(chosen) != 1 || chosen[0].Name != "1" {
		t.Fatalf("user choice reported %v, want Camera 1 once", chosen)
	}
}

// TestCaptureSelectIgnoresAnUnknownLabel guards the label→device mapping: a
// dropdown entry with no device behind it must report nothing rather than a
// zero-valued device, which would resolve to the platform default and switch
// the camera out from under the user.
func TestCaptureSelectIgnoresAnUnknownLabel(t *testing.T) {
	var chosen []CaptureDevice
	s := NewCaptureSelect(func(d CaptureDevice) { chosen = append(chosen, d) })
	s.publish([]CaptureDevice{{Name: "0", Description: "Camera 0"}}, nil, false)

	s.chosen("Some Other Camera")
	if len(chosen) != 0 {
		t.Errorf("an unmatched label reported %v, want nothing", chosen)
	}
}

// TestCaptureSelectRecordsDiscoveryFailure: "no camera is plugged in" and "this
// platform would not say" are different answers, and an application showing the
// second as the first is lying to its user.
func TestCaptureSelectRecordsDiscoveryFailure(t *testing.T) {
	s := NewCaptureSelect(nil)
	boom := errors.New("enumeration exploded")

	s.publish(nil, boom, false)
	if !errors.Is(s.Err(), boom) {
		t.Errorf("Err = %v, want the discovery failure", s.Err())
	}
	if len(s.Devices()) != 0 {
		t.Errorf("Devices = %v, want empty after a failed discovery", s.Devices())
	}

	s.publish([]CaptureDevice{{Name: "0"}}, nil, false)
	if s.Err() != nil {
		t.Errorf("Err = %v, want it cleared by a successful discovery", s.Err())
	}
}

// TestCaptureSelectKeepListPreservesScanResults covers the sequencing that
// caused the widget to need the flag at all: on a platform that cannot
// enumerate, a Reload after a Scan must not wipe what the Scan found.
func TestCaptureSelectKeepListPreservesScanResults(t *testing.T) {
	s := NewCaptureSelect(nil)
	scanned := []CaptureDevice{{Name: "0", Description: "Camera 0", HasVideo: true}}
	s.publish(scanned, nil, false)

	s.publish(nil, nil, true) // what Reload does where enumeration is unsupported

	if got := s.Devices(); len(got) != 1 || got[0].Name != "0" {
		t.Errorf("Devices = %v, want the scanned list kept", got)
	}
}

func TestCaptureSelectDevicesIsACopy(t *testing.T) {
	s := NewCaptureSelect(nil)
	s.publish([]CaptureDevice{{Name: "0", Description: "Camera 0"}}, nil, false)

	got := s.Devices()
	got[0].Name = "tampered"
	if s.Devices()[0].Name != "0" {
		t.Error("Devices handed out its own slice — a caller can rewrite the picker")
	}
}

// TestProbeCaptureDevicesRejectsAnUnboundedScan. Every index is a real device
// open, so there is no sensible unbounded form and guessing a number for the
// caller would be a decision they did not make.
func TestProbeCaptureDevicesRejectsAnUnboundedScan(t *testing.T) {
	for _, max := range []int{0, -1} {
		if _, err := ProbeCaptureDevices(max); err == nil {
			t.Errorf("ProbeCaptureDevices(%d) succeeded, want a rejected bound", max)
		}
	}
}

// TestVideoOnlyFilters is the shared filter behind both discovery routes: dshow
// enumerates microphones alongside cameras, and a picker must not offer them.
func TestVideoOnlyFilters(t *testing.T) {
	all := []CaptureDevice{
		{Name: "cam", HasVideo: true},
		{Name: "mic", HasAudio: true},
		{Name: "both", HasVideo: true, HasAudio: true},
	}
	got := videoOnly(all)
	if len(got) != 2 || got[0].Name != "cam" || got[1].Name != "both" {
		t.Errorf("videoOnly = %v, want the two video-capable devices in order", got)
	}
}

// TestSetCaptureDeviceRefusesANonCapturePlayer. Doing nothing would leave a
// picker looking as though it had worked.
func TestSetCaptureDeviceRefusesANonCapturePlayer(t *testing.T) {
	p, err := NewMediaPlayerURL(urlTestMedia)
	if err != nil {
		t.Fatalf("NewMediaPlayerURL: %v", err)
	}
	defer p.Close()

	if err := p.SetCaptureDevice("0"); !errors.Is(err, ErrNotACapturePlayer) {
		t.Errorf("SetCaptureDevice on a file player = %v, want ErrNotACapturePlayer", err)
	}
	if got := p.CaptureDevice(); got != "" {
		t.Errorf("CaptureDevice = %q on a file player, want empty", got)
	}
}
