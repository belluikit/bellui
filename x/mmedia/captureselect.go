package mmedia

import (
	"errors"
	"sync"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/goffmpeg"
)

// Labels the picker shows. Kept together so the wording is visible in one place
// rather than scattered through the widget.
const (
	captureSelectPlaceholder = "Select a camera"
	captureSelectEmpty       = "No camera found"
	captureSelectScan        = "Scan for cameras"
	captureSelectScanning    = "Scanning…"
)

// CaptureSelect is a camera picker: a dropdown of the capture devices this
// platform can see, and — only where the platform cannot enumerate them — a
// button to go and find them.
//
// It is populated from VideoCaptureDevices, which is free and can be refreshed
// as often as you like: cameras are hot-pluggable, so Reload exists to be
// called when one appears. Probing is the opposite, and is why the button is a
// button: on macOS/avfoundation there is no enumeration at all, so the only way
// to discover a camera is to open one, which takes about a second per index and
// lights the indicator. Making that a user action rather than something the
// widget does on its own is the whole reason the two are separate calls.
//
// Choosing a device calls OnChosen. Setting the selection programmatically —
// Reload, Scan, SetCurrent — never does; only a user's choice reports, which is
// the fork's SetX/SetXSilently rule applied to a selection rather than a value.
//
// What a scanned list can and cannot promise is ProbeCaptureDevices's business
// and is documented there. The part that reaches the user: on macOS the entries
// are positional, synthesised and not all of them are cameras — the platform's
// list runs on into screen-capture inputs and an open cannot tell them apart. An
// application that wants to be straight with its user should label the scanned
// results as what a scan found rather than as its cameras.
type CaptureSelect struct {
	widget.BaseWidget

	// OnChosen is called with the device the user picked. It runs on the main
	// goroutine, so anything slow — MediaPlayer.SetCaptureDevice among them —
	// belongs in a goroutine of its own.
	OnChosen func(CaptureDevice)

	// ProbeDepth is how many indices Scan tries when this platform cannot
	// enumerate. Zero means defaultProbeDepth.
	ProbeDepth int

	sel     *widget.Select
	scanBtn *widget.Button

	mu sync.Mutex
	// devices is the list behind the dropdown, index-aligned with sel.Options.
	devices []CaptureDevice
	// current is the device in use, kept in the list even when discovery cannot
	// see it. See SetCurrent.
	current *CaptureDevice
	// quiet suppresses OnChosen while the selection is being set from code.
	quiet bool
	// probes reports whether this platform needs the Scan button at all.
	probes bool
	// err records why the last discovery attempt found nothing.
	err error
}

// NewCaptureSelect creates a camera picker and populates it from the devices
// this platform can enumerate.
//
// On a platform that cannot enumerate — macOS — the list starts empty and the
// Scan button appears. Nothing is opened until the user presses it.
func NewCaptureSelect(onChosen func(CaptureDevice)) *CaptureSelect {
	s := &CaptureSelect{OnChosen: onChosen}

	s.sel = widget.NewSelect(nil, s.chosen)
	s.sel.PlaceHolder = captureSelectPlaceholder

	s.scanBtn = widget.NewButton(captureSelectScan, func() {
		// A probe is a series of real device opens. Off the main goroutine, or
		// the UI freezes for as long as the scan takes.
		s.scanBtn.Disable()
		s.scanBtn.SetText(captureSelectScanning)
		go func() {
			s.Scan()
			bellui.Do(func() {
				s.scanBtn.SetText(captureSelectScan)
				s.scanBtn.Enable()
			})
		}()
	})

	s.Reload()
	s.ExtendBaseWidget(s)
	return s
}

// Reload re-reads the platform's device list.
//
// Cheap: it enumerates and opens nothing, so it is reasonable to call on a
// window gaining focus or on a hot-plug notification. On a platform that cannot
// enumerate it changes nothing and leaves the list to Scan.
//
// It blocks for as long as enumeration takes, which is short but is a platform
// call. The selection is preserved if the device is still there.
func (s *CaptureSelect) Reload() {
	devs, err := VideoCaptureDevices()
	if errors.Is(err, goffmpeg.ErrDeviceListUnsupported) {
		// Not a failure and not an empty list: this platform simply cannot say.
		// Leave whatever Scan found in place and offer the button.
		s.mu.Lock()
		s.probes = true
		s.mu.Unlock()
		s.publish(nil, nil, true)
		return
	}
	s.publish(devs, err, false)
}

// Scan discovers cameras by opening them, for platforms where that is the only
// way. See ProbeCaptureDevices for what it costs — this is the same call, and
// the same warnings apply.
//
// On a platform that can enumerate this is exactly Reload and opens nothing.
//
// It blocks. The Scan button runs it in a goroutine; a caller driving it
// directly should do the same.
func (s *CaptureSelect) Scan() {
	depth := s.ProbeDepth
	if depth <= 0 {
		depth = defaultProbeDepth
	}
	devs, err := ProbeCaptureDevices(depth)
	s.publish(devs, err, false)
}

// SetCurrent marks a device as the one in use, keeping it in the list and
// selected even when discovery cannot see it.
//
// This is not cosmetic. A probe cannot open a device another application already
// holds, and that includes the camera this process is displaying — so without
// this, scanning while a camera is live drops it from its own picker. Call it
// with the device the player was opened on.
//
// Does not call OnChosen: it records a choice already made.
func (s *CaptureSelect) SetCurrent(d CaptureDevice) {
	s.mu.Lock()
	s.current = &d
	devs := append([]CaptureDevice(nil), s.devices...)
	s.mu.Unlock()
	s.publish(devs, nil, false)
}

// Devices returns the picker's current list, in the order shown.
func (s *CaptureSelect) Devices() []CaptureDevice {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]CaptureDevice(nil), s.devices...)
}

// Err reports why the last Reload or Scan produced no devices, or nil.
//
// Worth distinguishing from an empty list: "no camera is plugged in" and "this
// platform refused to tell us" are different answers and deserve different
// words in an application's UI.
func (s *CaptureSelect) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// publish installs a discovered list and refreshes the dropdown.
//
// keepList is for the one case where discovery has nothing to say rather than
// nothing to show: a platform that cannot enumerate must not have Scan's results
// wiped by the Reload that follows.
func (s *CaptureSelect) publish(devs []CaptureDevice, err error, keepList bool) {
	s.mu.Lock()
	if !keepList {
		s.devices = mergeCurrent(devs, s.current)
		s.err = err
	}
	labels := make([]string, 0, len(s.devices))
	for _, d := range s.devices {
		labels = append(labels, captureLabel(d))
	}
	selected := selectedLabel(s.devices, s.current)
	empty := len(labels) == 0
	s.mu.Unlock()

	bellui.Do(func() {
		s.mu.Lock()
		s.quiet = true
		s.mu.Unlock()

		s.sel.PlaceHolder = captureSelectPlaceholder
		if empty {
			s.sel.PlaceHolder = captureSelectEmpty
		}
		s.sel.SetOptions(labels)
		if selected != "" {
			s.sel.SetSelected(selected)
		}

		s.mu.Lock()
		s.quiet = false
		s.mu.Unlock()
		s.Refresh()
	})
}

// mergeCurrent keeps the in-use device in a freshly discovered list, appending
// it when discovery missed it. Ordering is discovery's, so a device that *was*
// found keeps its place rather than being moved.
func mergeCurrent(devs []CaptureDevice, current *CaptureDevice) []CaptureDevice {
	out := append([]CaptureDevice(nil), devs...)
	if current == nil {
		return out
	}
	for _, d := range out {
		if d.Name == current.Name {
			return out
		}
	}
	return append(out, *current)
}

// selectedLabel is the label to show as selected: the in-use device where there
// is one and it is listed.
func selectedLabel(devs []CaptureDevice, current *CaptureDevice) string {
	if current == nil {
		return ""
	}
	for _, d := range devs {
		if d.Name == current.Name {
			return captureLabel(d)
		}
	}
	return ""
}

// captureLabel is what one device reads as in the dropdown. The platform's own
// nomination is marked, since on a list of synthesised "Camera 0"/"Camera 1"
// names it is the only thing distinguishing them.
func captureLabel(d CaptureDevice) string {
	if d.Default {
		return d.Label() + "  (default)"
	}
	return d.Label()
}

// chosen maps the dropdown's text back to a device and reports it.
func (s *CaptureSelect) chosen(label string) {
	s.mu.Lock()
	quiet := s.quiet
	var picked *CaptureDevice
	for i := range s.devices {
		if captureLabel(s.devices[i]) == label {
			picked = &s.devices[i]
			break
		}
	}
	if !quiet && picked != nil {
		d := *picked
		s.current = &d
	}
	s.mu.Unlock()

	if quiet || picked == nil || s.OnChosen == nil {
		return
	}
	s.OnChosen(*picked)
}

// CreateRenderer returns the Bellui renderer for this widget.
func (s *CaptureSelect) CreateRenderer() core.WidgetRenderer {
	s.mu.Lock()
	probes := s.probes
	s.mu.Unlock()

	// The Scan button is offered only where enumeration cannot answer. On Linux
	// and Windows it would be a button that duplicates Reload at the price of
	// opening every camera in turn.
	var content *bellui.Container
	if probes {
		content = container.NewBorder(nil, nil, nil, s.scanBtn, s.sel)
	} else {
		content = container.NewStack(s.sel)
	}
	return widget.NewSimpleRenderer(content)
}

// MinSize keeps the dropdown wide enough for a device name.
func (s *CaptureSelect) MinSize() geo.Size {
	min := s.sel.MinSize()
	if min.Width < 220 {
		min.Width = 220
	}
	s.mu.Lock()
	probes := s.probes
	s.mu.Unlock()
	if probes {
		min.Width += s.scanBtn.MinSize().Width
	}
	return min
}

var _ core.Widget = (*CaptureSelect)(nil)
