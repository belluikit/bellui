package mmedia

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/belluikit/goffmpeg"

	"github.com/belluikit/bellui/x/mmedia/internal/pipeline"
)

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
	out := make([]CaptureDevice, 0, len(all))
	for _, d := range all {
		if d.HasVideo {
			out = append(out, d)
		}
	}
	return out, nil
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
	pipe, err := pipeline.NewPipelineDevice(format, url, o.captureOptions,
		pipeline.Config{Downmix: o.downmix})
	if err != nil {
		return nil, fmt.Errorf("mmedia: opening capture device %q via %s: %w", url, format, err)
	}
	return newMediaPlayerFromPipeline(pipe, o)
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
