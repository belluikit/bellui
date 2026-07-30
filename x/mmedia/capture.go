package mmedia

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/belluikit/goffmpeg"

	"github.com/belluikit/bellui/x/mmedia/internal/pipeline"
	"github.com/belluikit/bellui/x/mmedia/internal/source"
)

// captureBinding is what a capture player remembers about the device it opened,
// so SetCaptureDevice can resolve a new selector the same way and reopen with
// the same device options. A player built from a file, reader or url has none.
type captureBinding struct {
	format  string
	url     string
	options string
}

// ErrNoCaptureDevice reports that the platform enumerated no device able to
// provide video. It is distinct from an enumeration failure: the list was
// obtained and nothing in it was a camera.
var ErrNoCaptureDevice = errors.New("mmedia: no video capture device found")

// Input device format names. These are libavdevice's, not ours, but naming them
// keeps the per-platform mapping in one readable place.
const (
	formatAVFoundation = "avfoundation" // macOS
	formatDShow        = "dshow"        // Windows
	formatV4L2         = "v4l2"         // Linux, and the BSDs via V4L compatibility
)

// CaptureDevice is one capture device the platform can see.
type CaptureDevice struct {
	// Name is the selector to pass to NewCapturePlayer. It is what the platform
	// wants, which is not always readable — a DirectShow unique name, or a
	// /dev/video node.
	Name string
	// Description is the human-friendly name ("Logitech BRIO"), where the
	// platform provides one. It is not guaranteed unique across identical
	// cameras, so prefer Name as a selector and Description as a label.
	Description string
	// Default reports whether the platform nominated this as its default.
	Default bool
	// HasVideo and HasAudio report what the device can provide. A platform that
	// cannot determine this answers true for both, because guessing "no" would
	// hide a working camera.
	HasVideo bool
	HasAudio bool
}

// Label is the name to show a user: the description where there is one, else the
// raw selector.
func (d CaptureDevice) Label() string {
	if d.Description != "" {
		return d.Description
	}
	return d.Name
}

// CaptureDevices lists the capture devices this platform can see, video and
// audio alike. Use VideoCaptureDevices for the camera-only list a picker wants.
//
// Not every platform can enumerate. macOS/avfoundation has no enumeration
// callback at all and reports goffmpeg.ErrDeviceListUnsupported; cameras there
// are selected positionally instead, which is what NewCapturePlayerIndex and the
// default device are for. Treat that error as "cannot list", never as "no
// camera".
func CaptureDevices() ([]CaptureDevice, error) {
	return captureDevices(captureFormat())
}

// VideoCaptureDevices is CaptureDevices restricted to devices that can provide
// video, in enumeration order. This is the list NewCapturePlayerIndex indexes.
func VideoCaptureDevices() ([]CaptureDevice, error) {
	all, err := CaptureDevices()
	if err != nil {
		return nil, err
	}
	return videoOnly(all), nil
}

// defaultProbeDepth is how far ProbeCaptureDevices scans when a caller asks a
// picker to find cameras without naming a bound. Four covers a laptop's
// built-in camera plus a few externals, and costs four device opens.
const defaultProbeDepth = 4

// ProbeCaptureDevices discovers cameras on a platform that cannot enumerate
// them, by opening indices 0, 1, 2 … up to max and reporting which answered.
//
// On a platform that *can* enumerate this defers to VideoCaptureDevices and
// opens nothing: real names beat synthesised ones, and there is no reason to pay
// for a probe when the list is free. macOS/avfoundation is the case this exists
// for — it has no enumeration callback at all, so a picker there has nothing to
// populate from, and index is the only selector the format accepts.
//
// This is an explicit call rather than something a widget does implicitly,
// because of what it costs:
//
//   - Every index is a real device open. It takes roughly a second and it turns
//     the camera indicator on. Never call this from a repaint, a layout pass or
//     anything that runs more than once; cache the result.
//   - A device already in use does not answer. Opening a camera another
//     application holds fails, so an in-use device is reported as absent — and
//     that includes one this process has open. A picker that wants to keep
//     showing the camera currently being displayed has to remember it itself;
//     CaptureSelect.SetCurrent is that.
//   - A probed device has no name. Name carries the positional selector and
//     Description is synthesised ("Camera 0"), because the platform never told
//     us anything better. Index 0 is marked Default, matching the documented
//     avfoundation default.
//
// Indices are probed independently rather than stopping at the first gap, so a
// device list with a hole in it still reports what is past it.
//
// Two limits are worth stating plainly, because both were seen on hardware while
// this was written and neither can be fixed from here:
//
//   - Not everything that opens is a camera. avfoundation's positional list
//     continues past the cameras into screen-capture inputs, and an open
//     succeeds for those exactly as it does for a camera — on the machine this
//     was developed on, "Camera 2" was the 3840x2160 display. Nothing in the
//     open tells the two apart, so an application showing this list should say
//     "found by scanning" rather than "cameras". Distinguishing them needs the
//     device kind from the backend: belluikit/goffmpeg#5.
//   - An index is not a stable identifier. Across two runs minutes apart, the
//     device at index 0 changed from a 1920x1080 camera to a 4096x2160 one —
//     which is what a Continuity Camera appearing does. Treat a probed selector
//     as valid for this session, not as something to persist and reopen later.
//
// Neither is a reason to avoid probing on macOS; there is no alternative there.
// They are reasons not to present its results as more than they are.
func ProbeCaptureDevices(max int) ([]CaptureDevice, error) {
	if max <= 0 {
		return nil, fmt.Errorf("mmedia: ProbeCaptureDevices needs a positive bound, got %d", max)
	}
	format := captureFormat()

	// Prefer real enumeration wherever it exists. Only ErrDeviceListUnsupported
	// means "this format cannot list" — every other failure is a genuine error
	// and probing past it would bury the cause.
	devs, err := captureDevices(format)
	if err == nil {
		return videoOnly(devs), nil
	}
	if !errors.Is(err, goffmpeg.ErrDeviceListUnsupported) {
		return nil, err
	}

	found := make([]CaptureDevice, 0, max)
	for i := 0; i < max; i++ {
		url := strconv.Itoa(i)
		src, err := openCaptureSource(format, url, "")
		if err != nil {
			continue
		}
		src.Close()
		found = append(found, CaptureDevice{
			Name:        url,
			Description: fmt.Sprintf("Camera %d", i),
			Default:     i == 0,
			HasVideo:    true,
		})
	}
	if len(found) == 0 {
		return nil, ErrNoCaptureDevice
	}
	return found, nil
}

// videoOnly filters an enumerated list down to the cameras a picker wants.
func videoOnly(all []CaptureDevice) []CaptureDevice {
	out := make([]CaptureDevice, 0, len(all))
	for _, d := range all {
		if d.HasVideo {
			out = append(out, d)
		}
	}
	return out
}

// NewCapturePlayer opens a live capture device — a camera — as a player, with the
// same widget, transport controls and options as the playback constructors.
//
// device is a device name, or on macOS optionally a positional index as a string;
// pass "" for the platform default. The libavdevice format is inferred from the
// platform, and platform-specific selector syntax is supplied for you: a
// DirectShow name needs a "video=" prefix, which is added if absent, and a
// friendly name under v4l2 is resolved to its /dev/video node by enumeration.
//
// If the device refuses the framerate FFmpeg asks for by default — a camera
// whose only mode is 1080p60 does, since libavdevice's default is 29.97 and
// there is no way to ask for "whatever the device offers" — the open is retried
// at a few common rates. Naming a framerate through WithCaptureOptions turns
// that off and the single explicit attempt is reported as it happens; see
// openCaptureWithFallback for why the ladder exists at all.
//
// The resulting player is live: Duration is 0, the seek bar is disabled and shows
// "live", Seek does nothing, and WithLoop is inert. A device that stops producing
// media — unplugged, or claimed by another application — is reported through
// WithOnError wrapping pipeline.ErrCaptureStopped, not through WithOnEnd.
//
// Stop releases the device, so the camera's indicator light goes out and other
// applications can claim it; Play reacquires it. Close releases it permanently.
func NewCapturePlayer(device string, opts ...Option) (*MediaPlayer, error) {
	format := captureFormat()
	url, err := resolveCaptureDevice(format, device)
	if err != nil {
		return nil, err
	}
	return newCapturePlayer(format, url, opts)
}

// NewCapturePlayerIndex opens the index-th video capture device, counting from
// zero over VideoCaptureDevices.
//
// On macOS this is the platform's own positional selector, since avfoundation
// cannot enumerate and index is the only thing it accepts. Everywhere else the
// index is resolved against the enumerated list, so it is stable only for as long
// as that list is — prefer a name from VideoCaptureDevices for anything the user
// chose and you intend to remember.
func NewCapturePlayerIndex(index int, opts ...Option) (*MediaPlayer, error) {
	format := captureFormat()
	url, err := resolveCaptureIndex(format, index)
	if err != nil {
		return nil, err
	}
	return newCapturePlayer(format, url, opts)
}

func newCapturePlayer(format, url string, opts []Option) (*MediaPlayer, error) {
	o := buildOptions(opts)
	src, err := openCaptureSource(format, url, o.captureOptions)
	if err != nil {
		return nil, err
	}
	mp, err := newMediaPlayerFromPipeline(
		pipeline.NewPipelineForSource(src, pipeline.Config{Downmix: o.downmix}), o,
	)
	if err != nil {
		return nil, err
	}
	// Remembered so SetCaptureDevice can resolve a new selector against the same
	// format and reopen with the same device options.
	mp.capture = &captureBinding{format: format, url: url, options: o.captureOptions}
	return mp, nil
}

// openCaptureSource opens a capture device as a source, through the framerate
// fallback. This is the form SetCaptureDevice needs: a swap replaces the source
// inside a running pipeline rather than building a new one.
func openCaptureSource(format, url, options string) (source.Source, error) {
	return openCaptureWithFallback(format, url, options,
		func(attempt string) (source.Source, error) {
			return source.NewFFmpegSourceDevice(format, url, attempt)
		})
}

// captureFramerateLadder is the framerates a zero-config open falls back
// through when the device refuses FFmpeg's own default. See captureAttempts.
//
// Short on purpose. Every rung is a real device open — it lights the camera
// indicator and costs a few hundred milliseconds — so a long ladder would turn
// a fast, clear failure into a slow one. 60 and 30 are the rates a fixed-mode
// camera actually offers, and 25 covers PAL-region hardware; a device that
// matches none of those is one the caller has to configure explicitly.
var captureFramerateLadder = []string{"60", "30", "25"}

// openCaptureWithFallback opens a capture device, retrying at a few common
// framerates when the caller named none and the device refused the default.
//
// Generic over what an open produces because there are two callers that need
// the same policy over different results: a player is built from a whole
// pipeline, and a device switch replaces only the source inside a running one.
//
// The retry is not speculative tidiness. libavdevice's framerate is an
// AVOption with a *default*, not an absent value: avfoundation declares
// {.str = "ntsc"}, so every open asks for 29.97 whether or not anyone said so,
// and a camera whose only mode is 1920x1080@60 rejects the open outright —
// which is the whole of #95. Confirmed against goffmpeg v0.4.0: its
// decoder_open_device parses only the options string we hand it and adds
// nothing, so the rate is FFmpeg's choice rather than the binding's.
//
// "Just do not set a framerate" is therefore not available. There is no way to
// unset an AVOption default, and the obvious spelling is rejected before it
// reaches the device: framerate=0 fails in av_parse_video_rate, which requires
// a positive rate ("Unable to parse \"framerate\" option value \"0\" as video
// rate"). Retrying is the remedy that exists.
//
// goffmpeg reports every failure as an opaque -1 with no sentinel, so there is
// nothing to test for and the ladder is walked on any error. A device that is
// simply absent therefore costs four failed opens rather than one — about a
// second, and each fails as fast as the first, which is a fair price for a path
// that until now just did not work.
//
// pixel_format has the same shape of default and deliberately gets no ladder:
// avfoundation warns and substitutes one the device does support ("Selected
// pixel format (yuv420p) is not supported by the input device. […] Overriding
// selected pixel format to use uyvy422 instead") rather than failing, so there
// is nothing to recover from. Verified on the same camera that reproduces #95.
func openCaptureWithFallback[T any](format, url, options string,
	open func(attempt string) (T, error),
) (T, error) {
	attempts := captureAttempts(options)

	var first error
	for _, attempt := range attempts {
		// The rung that succeeds is the one the source keeps: the attempt string
		// reaches source.NewFFmpegSourceDevice, whose reopen closure captures it.
		// So the Stop/Play release-and-reacquire cycle reuses the configuration
		// that worked rather than falling back through the ladder again — and,
		// more to the point, rather than reacquiring with the default rate this
		// device already refused.
		opened, err := open(attempt)
		if err == nil {
			return opened, nil
		}
		// The first failure is the one worth reporting: it is the open the
		// caller actually asked for, and the rest are our guesses at a fix.
		if first == nil {
			first = err
		}
	}
	var zero T
	return zero, captureOpenError(format, url, attempts, first)
}

// captureAttempts is the ordered list of libavdevice option strings to try for
// one capture open. It is the whole of the fallback policy, kept as a pure
// function so the decision is testable without a camera.
//
// The caller's own options always go first, so a device that works answers on
// the first open and nothing below ever runs. A caller who named a framerate
// gets that one attempt and no more: overriding an explicit choice would hide
// the very failure they need to see, and "framerate=30 did not work so we
// silently gave you 60" is not a contract worth having.
func captureAttempts(options string) []string {
	attempts := []string{options}
	if hasDeviceOption(options, "framerate") {
		return attempts
	}
	for _, rate := range captureFramerateLadder {
		attempts = append(attempts, joinDeviceOptions(options, "framerate="+rate))
	}
	return attempts
}

// hasDeviceOption reports whether a libavdevice option string sets key.
//
// The parse mirrors av_dict_parse_string's, which is what actually consumes
// this string: split on commas, then on the first "=". There is no escaping in
// that format, so neither is there any here.
func hasDeviceOption(options, key string) bool {
	for _, pair := range strings.Split(options, ",") {
		k, _, found := strings.Cut(pair, "=")
		if !found {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(k), key) {
			return true
		}
	}
	return false
}

// joinDeviceOptions appends one "key=value" to an option string, which may be
// empty.
func joinDeviceOptions(options, pair string) string {
	if options == "" {
		return pair
	}
	return options + "," + pair
}

// captureOpenError reports a failed open, naming what was tried.
//
// The information a user needs — "supported modes: 1920x1080@60" — is printed
// by FFmpeg to its own log and is not reachable from here: goffmpeg exposes
// neither the log nor the device's mode list, so an application sees only -1.
// Listing the attempts is what can honestly be said from this side, and it at
// least tells an app which framerates are already ruled out. Capturing the real
// mode list is belluikit/goffmpeg#5.
func captureOpenError(format, url string, attempts []string, err error) error {
	base := fmt.Errorf("mmedia: opening capture device %q via %s: %w", url, format, err)
	if len(attempts) < 2 {
		return base
	}
	tried := make([]string, 0, len(attempts)-1)
	for _, a := range attempts[1:] {
		tried = append(tried, strconv.Quote(a))
	}
	return fmt.Errorf("%w (also tried %s; if the device needs a rate none of "+
		"those cover, pass it with WithCaptureOptions)",
		base, strings.Join(tried, ", "))
}

// CaptureFormat reports the libavdevice input format this platform captures
// through: "avfoundation" on macOS, "dshow" on Windows, "v4l2" elsewhere.
//
// The capture entry points apply it for you; it is exported because the accepted
// WithCaptureOptions keys are the format's own, so an app offering the user any
// device configuration needs to know which format it is talking to.
func CaptureFormat() string { return captureFormat() }

// captureFormat is the libavdevice input format for this platform.
//
// This mapping is the reason the capture entry points exist at all. goffmpeg is a
// thin FFmpeg binding and correctly takes the format name; making every app carry
// the per-platform matrix would be a poor contract for a widget toolkit.
func captureFormat() string {
	switch runtime.GOOS {
	case "darwin":
		return formatAVFoundation
	case "windows":
		return formatDShow
	default:
		return formatV4L2
	}
}

func captureDevices(format string) ([]CaptureDevice, error) {
	infos, err := goffmpeg.ListInputSources(format)
	if err != nil {
		return nil, err
	}
	out := make([]CaptureDevice, 0, len(infos))
	for _, i := range infos {
		out = append(out, CaptureDevice{
			Name:        i.Name,
			Description: i.Description,
			Default:     i.Default,
			HasVideo:    i.HasMedia(goffmpeg.MediaTypeVideo),
			HasAudio:    i.HasMedia(goffmpeg.MediaTypeAudio),
		})
	}
	return out, nil
}

// resolveCaptureDevice turns a caller's device selector into the url the format
// expects. Kept separate from captureFormat, and taking the format explicitly, so
// every platform's rules are testable from any host.
func resolveCaptureDevice(format, device string) (string, error) {
	device = strings.TrimSpace(device)
	if device == "" {
		return defaultCaptureDevice(format)
	}

	switch format {
	case formatAVFoundation:
		// Accepts either a positional index or a device name, as given.
		return device, nil
	case formatDShow:
		return dshowSelector(device), nil
	default:
		// v4l2 takes a device node. An absolute path is one already; anything
		// else is a friendly name that has to be looked up.
		if strings.HasPrefix(device, "/") {
			return device, nil
		}
		return lookupCaptureDevice(format, device)
	}
}

// dshowSelector adds the media-kind prefix DirectShow selectors carry and
// enumeration omits. An already-prefixed selector is passed through, so a caller
// who knows dshow can name an audio device too.
func dshowSelector(name string) string {
	if strings.HasPrefix(name, "video=") || strings.HasPrefix(name, "audio=") {
		return name
	}
	return "video=" + name
}

// deviceURL applies any format-specific decoration to an enumerated device name.
func deviceURL(format, name string) string {
	if format == formatDShow {
		return dshowSelector(name)
	}
	return name
}

func defaultCaptureDevice(format string) (string, error) {
	if format == formatAVFoundation {
		// avfoundation cannot enumerate, and "0" is its documented default video
		// device — which is why it takes a positional selector at all.
		return "0", nil
	}

	devs, err := goffmpeg.ListInputSources(format)
	if err != nil {
		if format == formatV4L2 {
			// Enumeration was unavailable rather than empty (an old prebuilt
			// libgoffmpeg under purego exports no enumeration symbols). The
			// conventional first node is a better answer than refusing to open
			// anything.
			return "/dev/video0", nil
		}
		return "", err
	}

	// Prefer the platform's nominated default, then the first camera. Devices
	// whose media is *known* to exclude video must be skipped rather than tried:
	// on Linux a UVC camera exposes a metadata node beside its real one, and
	// opening that blocks indefinitely instead of failing.
	if d, ok := firstVideoDevice(devs, true); ok {
		return deviceURL(format, d.Name), nil
	}
	if d, ok := firstVideoDevice(devs, false); ok {
		return deviceURL(format, d.Name), nil
	}
	return "", ErrNoCaptureDevice
}

func firstVideoDevice(devs []goffmpeg.DeviceInfo, defaultOnly bool) (goffmpeg.DeviceInfo, bool) {
	for _, d := range devs {
		if !d.HasMedia(goffmpeg.MediaTypeVideo) {
			continue
		}
		if defaultOnly && !d.Default {
			continue
		}
		return d, true
	}
	return goffmpeg.DeviceInfo{}, false
}

// lookupCaptureDevice resolves a friendly name to a platform selector, matching
// the description first and then the raw name, both case-insensitively.
func lookupCaptureDevice(format, name string) (string, error) {
	devs, err := goffmpeg.ListInputSources(format)
	if err != nil {
		return "", fmt.Errorf("mmedia: resolving capture device %q: %w", name, err)
	}
	want := strings.ToLower(name)
	for _, d := range devs {
		if !d.HasMedia(goffmpeg.MediaTypeVideo) {
			continue
		}
		if strings.ToLower(d.Description) == want || strings.ToLower(d.Name) == want {
			return deviceURL(format, d.Name), nil
		}
	}
	return "", fmt.Errorf("mmedia: no video capture device named %q", name)
}

func resolveCaptureIndex(format string, index int) (string, error) {
	if index < 0 {
		return "", fmt.Errorf("mmedia: capture device index %d is negative", index)
	}
	if format == formatAVFoundation {
		// Positional selection is native here, and this is the one platform that
		// cannot enumerate, so there is nothing to resolve against.
		return strconv.Itoa(index), nil
	}

	devs, err := goffmpeg.ListInputSources(format)
	if err != nil {
		return "", fmt.Errorf("mmedia: resolving capture device index %d: %w", index, err)
	}
	seen := 0
	for _, d := range devs {
		if !d.HasMedia(goffmpeg.MediaTypeVideo) {
			continue
		}
		if seen == index {
			return deviceURL(format, d.Name), nil
		}
		seen++
	}
	if seen == 0 {
		return "", ErrNoCaptureDevice
	}
	return "", fmt.Errorf("mmedia: capture device index %d out of range (%d video device(s))", index, seen)
}
