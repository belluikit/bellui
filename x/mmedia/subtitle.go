package mmedia

import (
	"io"
	"strings"
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/mmedia/internal/subtitle"
)

// SubtitleOverlay is a Bellui widget that renders timed subtitle text
// over a video player. It supports SRT and WebVTT formats.
type SubtitleOverlay struct {
	widget.BaseWidget

	mu       sync.Mutex
	track    *subtitle.Track
	label    *canvas.Text
	position time.Duration
}

// NewSubtitleOverlay creates a new subtitle overlay widget.
func NewSubtitleOverlay() *SubtitleOverlay {
	so := &SubtitleOverlay{}
	so.label = canvas.NewText("", theme.Color(theme.ColorNameForeground))
	so.label.TextSize = 20
	so.label.Alignment = bellui.TextAlignCenter
	so.label.TextStyle = bellui.TextStyle{Bold: true}
	so.ExtendBaseWidget(so)
	return so
}

// LoadSRT loads subtitles from an SRT file.
func (so *SubtitleOverlay) LoadSRT(r io.Reader) error {
	track, err := subtitle.ParseSRT(r)
	if err != nil {
		return err
	}
	so.mu.Lock()
	so.track = track
	so.mu.Unlock()
	return nil
}

// LoadWebVTT loads subtitles from a WebVTT file.
func (so *SubtitleOverlay) LoadWebVTT(r io.Reader) error {
	track, err := subtitle.ParseWebVTT(r)
	if err != nil {
		return err
	}
	so.mu.Lock()
	so.track = track
	so.mu.Unlock()
	return nil
}

// LoadTrack sets the subtitle track directly.
func (so *SubtitleOverlay) LoadTrack(track *subtitle.Track) {
	so.mu.Lock()
	so.track = track
	so.mu.Unlock()
}

// SetPosition updates the current playback position and refreshes the subtitle.
func (so *SubtitleOverlay) SetPosition(pos time.Duration) {
	so.mu.Lock()
	so.position = pos
	track := so.track
	so.mu.Unlock()

	if track == nil {
		so.updateText("")
		return
	}

	cues := track.CuesAt(pos)
	if len(cues) == 0 {
		so.updateText("")
		return
	}

	// Join multiple simultaneous cues.
	var texts []string
	for _, c := range cues {
		texts = append(texts, c.Text)
	}
	so.updateText(strings.Join(texts, "\n"))
}

func (so *SubtitleOverlay) updateText(text string) {
	if so.label.Text != text {
		so.label.Text = text
		so.label.Refresh()
	}
}

// CreateRenderer returns the Bellui widget renderer.
func (so *SubtitleOverlay) CreateRenderer() core.WidgetRenderer {
	bg := canvas.NewRectangle(theme.Color(theme.ColorNameShadow))
	bg.SetMinSize(geo.NewSize(0, 0))

	content := container.New(
		layout.NewStackLayout(),
		bg,
		container.NewPadded(so.label),
	)

	return widget.NewSimpleRenderer(content)
}

// --- Integration with MediaPlayer ---

// MediaPlayerWithSubtitles creates a MediaPlayer with a subtitle overlay.
// The subtitle position auto-updates with playback time.
func MediaPlayerWithSubtitles(player *MediaPlayer, subtitleOverlay *SubtitleOverlay) *bellui.Container {
	// Wire time updates to the subtitle overlay.
	player.pipe.SetOnTime(func(current, total time.Duration) {
		bellui.Do(func() {
			player.controls.SetTime(current, total)
			subtitleOverlay.SetPosition(current)
		})
	})

	// Stack the subtitle overlay on top of the video, controls at the bottom.
	// The video sits in an aspect-preserving container because a streaming
	// raster is always drawn stretched.
	vw, vh := player.pipe.VideoSize()
	video := container.New(&aspectLayout{width: vw, height: vh}, player.surface.Object())
	videoWithSubs := container.NewStack(
		video,
		container.NewVBox(
			layout.NewSpacer(),
			subtitleOverlay,
		),
	)

	return container.NewBorder(
		nil,
		player.controls,
		nil,
		nil,
		videoWithSubs,
	)
}
