package mmedia

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/belluikit/goffmpeg"
)

// TestCaptureFormatMatchesPlatform locks the GOOS mapping. Only the running
// platform's arm can be asserted, but that is the one that matters: getting it
// wrong means libavformat probes a device selector as a file path.
func TestCaptureFormatMatchesPlatform(t *testing.T) {
	want := map[string]string{
		"darwin":  formatAVFoundation,
		"windows": formatDShow,
	}[runtime.GOOS]
	if want == "" {
		want = formatV4L2 // linux and the BSDs
	}
	if got := captureFormat(); got != want {
		t.Errorf("captureFormat() on %s = %q, want %q", runtime.GOOS, got, want)
	}
}

// TestResolveCaptureDeviceSelectors covers every platform's selector rules from
// any host: resolveCaptureDevice takes the format explicitly, so none of this is
// gated on the machine running the test. Only the paths that need no enumeration
// are here — a bare v4l2 name has to be looked up against real hardware.
func TestResolveCaptureDeviceSelectors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		format string
		device string
		want   string
	}{
		{"avfoundation index passes through", formatAVFoundation, "0", "0"},
		{"avfoundation name passes through", formatAVFoundation, "FaceTime HD Camera", "FaceTime HD Camera"},
		{"dshow name gains the video prefix", formatDShow, "Logitech BRIO", "video=Logitech BRIO"},
		{"dshow video prefix is kept", formatDShow, "video=Logitech BRIO", "video=Logitech BRIO"},
		{"dshow audio prefix is respected", formatDShow, "audio=Microphone", "audio=Microphone"},
		{"v4l2 node passes through", formatV4L2, "/dev/video0", "/dev/video0"},
		{"surrounding space is ignored", formatDShow, "  Logitech BRIO  ", "video=Logitech BRIO"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveCaptureDevice(tc.format, tc.device)
			if err != nil {
				t.Fatalf("resolveCaptureDevice(%q, %q): %v", tc.format, tc.device, err)
			}
			if got != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}
}

// TestDefaultCaptureDeviceOnMacOS is the positional-selector case: avfoundation
// cannot enumerate, so the default must come from the documented "0" rather than
// from a device list.
func TestDefaultCaptureDeviceOnMacOS(t *testing.T) {
	got, err := defaultCaptureDevice(formatAVFoundation)
	if err != nil {
		t.Fatalf("defaultCaptureDevice: %v", err)
	}
	if got != "0" {
		t.Errorf("= %q, want %q — avfoundation has no enumeration to fall back on", got, "0")
	}
}

// TestResolveCaptureIndexOnMacOS likewise: the one platform that cannot
// enumerate is the one that takes an index natively, so no lookup is involved.
func TestResolveCaptureIndexOnMacOS(t *testing.T) {
	got, err := resolveCaptureIndex(formatAVFoundation, 2)
	if err != nil {
		t.Fatalf("resolveCaptureIndex: %v", err)
	}
	if got != "2" {
		t.Errorf("= %q, want %q", got, "2")
	}
}

func TestResolveCaptureIndexRejectsNegative(t *testing.T) {
	if _, err := resolveCaptureIndex(formatAVFoundation, -1); err == nil {
		t.Error("a negative device index must be rejected")
	}
}

// TestFirstVideoDeviceSkipsNonVideo is the Linux trap: a UVC camera exposes a
// metadata node beside its real one, and opening that blocks indefinitely rather
// than failing — so a device known not to carry video must never be chosen.
func TestFirstVideoDeviceSkipsNonVideo(t *testing.T) {
	devs := []goffmpeg.DeviceInfo{
		{
			Name: "/dev/video1", Description: "metadata",
			MediaKnown: true, MediaTypes: nil,
		},
		{
			Name: "/dev/video0", Description: "camera",
			MediaKnown: true, MediaTypes: []goffmpeg.MediaType{goffmpeg.MediaTypeVideo},
		},
	}

	got, ok := firstVideoDevice(devs, false)
	if !ok {
		t.Fatal("expected the camera to be found")
	}
	if got.Name != "/dev/video0" {
		t.Errorf("chose %q, want the video-capable node /dev/video0", got.Name)
	}
}

// TestFirstVideoDevicePrefersDefault checks the platform's own nomination wins
// over enumeration order.
func TestFirstVideoDevicePrefersDefault(t *testing.T) {
	video := []goffmpeg.MediaType{goffmpeg.MediaTypeVideo}
	devs := []goffmpeg.DeviceInfo{
		{Name: "first", MediaKnown: true, MediaTypes: video},
		{Name: "nominated", MediaKnown: true, MediaTypes: video, Default: true},
	}

	got, ok := firstVideoDevice(devs, true)
	if !ok {
		t.Fatal("expected the nominated default to be found")
	}
	if got.Name != "nominated" {
		t.Errorf("chose %q, want the platform default", got.Name)
	}
	// With no nomination the same list falls back to enumeration order.
	devs[1].Default = false
	if _, ok := firstVideoDevice(devs, true); ok {
		t.Error("defaultOnly must not match when nothing is nominated")
	}
	if got, _ := firstVideoDevice(devs, false); got.Name != "first" {
		t.Errorf("fallback chose %q, want the first video device", got.Name)
	}
}

// TestFirstVideoDeviceTrustsUnknownMedia covers the deliberate asymmetry in
// goffmpeg's HasMedia: a device whose media could not be determined must be
// tried, since guessing "no" would hide a working camera.
func TestFirstVideoDeviceTrustsUnknownMedia(t *testing.T) {
	devs := []goffmpeg.DeviceInfo{{Name: "mystery", MediaKnown: false}}
	got, ok := firstVideoDevice(devs, false)
	if !ok || got.Name != "mystery" {
		t.Error("a device of unknown media must be treated as a candidate")
	}
}

func TestCaptureDeviceLabel(t *testing.T) {
	if got := (CaptureDevice{Name: "/dev/video0", Description: "Logitech BRIO"}).Label(); got != "Logitech BRIO" {
		t.Errorf("Label = %q, want the description", got)
	}
	if got := (CaptureDevice{Name: "/dev/video0"}).Label(); got != "/dev/video0" {
		t.Errorf("Label = %q, want the selector as a fallback", got)
	}
}

// TestWithCaptureOptionsIsRecorded guards the option plumbing; the string itself
// is libavdevice's contract and is not validated here.
func TestWithCaptureOptionsIsRecorded(t *testing.T) {
	o := buildOptions([]Option{WithCaptureOptions("framerate=30,pixel_format=nv12")})
	if !strings.Contains(o.captureOptions, "framerate=30") {
		t.Errorf("captureOptions = %q, want the forwarded device options", o.captureOptions)
	}
}

// TestCaptureAttempts covers the whole of the framerate fallback policy (#95).
// It is a pure function precisely so this needs no camera: the decision is which
// opens to try and in what order, and that is what is asserted.
func TestCaptureAttempts(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options string
		want    []string
	}{
		{
			// The zero-config case the issue is about. The first attempt is the
			// caller's own (empty) options, so a device that accepts FFmpeg's
			// default still opens on the first try and pays nothing.
			name:    "no options falls back through the ladder",
			options: "",
			want:    []string{"", "framerate=60", "framerate=30", "framerate=25"},
		},
		{
			// An explicit rate is honoured exactly once. Retrying would replace a
			// deliberate choice with a guess and hide the failure the caller needs.
			name:    "an explicit framerate is tried once and not overridden",
			options: "framerate=30",
			want:    []string{"framerate=30"},
		},
		{
			name:    "an explicit framerate anywhere in the list still counts",
			options: "pixel_format=nv12,framerate=24",
			want:    []string{"pixel_format=nv12,framerate=24"},
		},
		{
			// Other device options are preserved on every rung: falling back on
			// the framerate must not drop the pixel format with it.
			name:    "other options are carried onto every rung",
			options: "pixel_format=nv12",
			want: []string{
				"pixel_format=nv12",
				"pixel_format=nv12,framerate=60",
				"pixel_format=nv12,framerate=30",
				"pixel_format=nv12,framerate=25",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := captureAttempts(tc.options)
			if len(got) != len(tc.want) {
				t.Fatalf("captureAttempts(%q) = %q, want %q", tc.options, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("attempt %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestHasDeviceOption pins the parse the fallback keys off. It mirrors
// av_dict_parse_string, which is what actually consumes the string.
func TestHasDeviceOption(t *testing.T) {
	for _, tc := range []struct {
		options string
		key     string
		want    bool
	}{
		{"", "framerate", false},
		{"framerate=30", "framerate", true},
		{"pixel_format=nv12", "framerate", false},
		{"pixel_format=nv12,framerate=30", "framerate", true},
		{" framerate = 30 ", "framerate", true},
		{"FrameRate=30", "framerate", true},
		// A bare word sets nothing: with no "=" there is no value, and FFmpeg
		// would not read it as one either.
		{"framerate", "framerate", false},
		// The key must match whole, not as a prefix or suffix of another.
		{"max_framerate=30", "framerate", false},
	} {
		if got := hasDeviceOption(tc.options, tc.key); got != tc.want {
			t.Errorf("hasDeviceOption(%q, %q) = %v, want %v", tc.options, tc.key, got, tc.want)
		}
	}
}

// TestCaptureOpenErrorNamesTheAttempts checks the failure an application
// actually sees. FFmpeg prints the device's supported modes to its own log and
// goffmpeg does not expose it (belluikit/goffmpeg#5), so naming the rates
// already ruled out is the most this side can honestly say.
func TestCaptureOpenErrorNamesTheAttempts(t *testing.T) {
	cause := errors.New(`failed to open device "avfoundation" "0": -1`)

	err := captureOpenError(formatAVFoundation, "0", captureAttempts(""), cause)
	if !errors.Is(err, cause) {
		t.Error("the underlying open failure must stay unwrapped-to")
	}
	for _, want := range []string{"framerate=60", "framerate=30", "framerate=25", "WithCaptureOptions"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}

	// A caller who named a rate made one attempt, so there is no ladder to
	// report and the error stays the plain one.
	single := captureOpenError(formatAVFoundation, "0", captureAttempts("framerate=30"), cause)
	if strings.Contains(single.Error(), "also tried") {
		t.Errorf("error %q lists fallbacks that were never attempted", single)
	}
}
