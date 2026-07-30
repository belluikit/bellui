package mmedia

import (
	"image"
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"

	"github.com/belluikit/bellui/x/mmedia/internal/source"
)

// scrubPreviewWidth is the on-screen width of the preview thumbnail. The height
// follows from the video's aspect ratio once the first frame has been decoded;
// until then this is what the placeholder occupies.
const (
	scrubPreviewWidth  = 160
	scrubPreviewHeight = 90 // 16:9, replaced once the real aspect is known
)

// scrubPreview shows the frame under the pointer while the seek bar is hovered.
//
// The decode happens on its own goroutine and is coalesced: a request that
// arrives while one is in flight replaces the pending position rather than
// queueing behind it, so a pointer sweeping across the bar produces a handful of
// decodes rather than one per pixel, and the previously decoded frame stays on
// screen in the meantime. That matters more than raw decode speed — the seek
// dominates, and there is no point decoding positions the pointer has already
// left.
type scrubPreview struct {
	thumb *source.Thumbnailer

	img   *canvas.Image
	label *canvas.Text
	obj   *bellui.Container

	mu sync.Mutex
	// pending is the position most recently asked for, and pendingSet whether
	// there is one. Together they are the coalescing: only the newest survives.
	pending    time.Duration
	pendingSet bool
	running    bool
	closed     bool
	// aspectKnown stops the widget being resized on every frame once the real
	// video geometry has been seen.
	aspectKnown bool
	// onResize is called when the placeholder aspect is replaced by the real
	// one, so the controls can be told the new size.
	onResize func(geo.Size)
}

// newScrubPreview builds the preview over url, which must be something FFmpeg
// can open a second time. Nothing is opened until the first frame is requested.
func newScrubPreview(url string) *scrubPreview {
	p := &scrubPreview{thumb: source.NewThumbnailer(url)}

	// A placeholder image rather than an empty one: the preview appears on the
	// first hover, before any frame can possibly have been decoded, and a
	// zero-sized image there would make it flash into place a moment later.
	p.img = canvas.NewImageFromImage(image.NewRGBA(image.Rect(0, 0, 16, 9)))
	p.img.FillMode = canvas.ImageFillStretch
	p.img.ScaleMode = canvas.ImageScaleFastest

	p.label = canvas.NewText("00:00", theme.Color(theme.ColorNameForeground))
	p.label.Alignment = bellui.TextAlignCenter
	p.label.TextSize = theme.Size(theme.SizeNameCaptionText)

	bg := canvas.NewRectangle(theme.Color(theme.ColorNameOverlayBackground))
	p.obj = container.NewStack(
		bg,
		container.New(layout.NewBorderLayout(nil, p.label, nil, nil), p.label, p.img),
	)
	p.obj.Hide()
	return p
}

// object is the thing the transport controls float above the seek bar.
func (p *scrubPreview) object() core.CanvasObject { return p.obj }

// size is the space the preview asks for.
func (p *scrubPreview) size() geo.Size {
	return geo.NewSize(scrubPreviewWidth, scrubPreviewHeight+p.label.MinSize().Height)
}

// request asks for the frame at pos. It returns immediately: the decode happens
// on a worker goroutine, and the currently shown frame stays up until a new one
// is ready.
func (p *scrubPreview) request(pos time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.pending, p.pendingSet = pos, true
	if p.running {
		// A decode is already in flight. It will pick this up when it finishes,
		// which is the coalescing: positions the pointer has already left are
		// never decoded.
		return
	}
	p.running = true
	go p.decodeLoop()
}

// setTime updates the timestamp under the thumbnail. Done separately from the
// frame so the label tracks the pointer exactly while the image lags a decode
// behind it — the reverse would look like the pointer was sticking.
func (p *scrubPreview) setTime(pos time.Duration) {
	text := formatDuration(pos)
	bellui.Do(func() {
		if p.label.Text == text {
			return
		}
		p.label.Text = text
		p.label.Refresh()
	})
}

// decodeLoop drains the pending position until there is none left, then stops.
func (p *scrubPreview) decodeLoop() {
	for {
		p.mu.Lock()
		if p.closed || !p.pendingSet {
			p.running = false
			p.mu.Unlock()
			return
		}
		pos := p.pending
		p.pendingSet = false
		p.mu.Unlock()

		frame, err := p.thumb.FrameAt(pos)
		if err != nil {
			// A preview that cannot decode one position is not worth reporting
			// through the player's error path — that is for playback failing.
			// The previous frame stays up and the next hover tries again.
			continue
		}
		p.show(frame)
	}
}

// show puts a decoded frame on screen, from the decode goroutine.
func (p *scrubPreview) show(frame *image.RGBA) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	// The first frame settles the aspect ratio, which the placeholder only
	// guessed at. Doing it once rather than per frame keeps a 16:9 video from
	// re-laying out the controls on every pointer move.
	var resized *geo.Size
	if !p.aspectKnown {
		if w, h := p.thumb.Size(); w > 0 && h > 0 {
			p.aspectKnown = true
			s := geo.NewSize(scrubPreviewWidth,
				scrubPreviewWidth*float32(h)/float32(w)+p.label.MinSize().Height)
			resized = &s
		}
	}
	onResize := p.onResize
	p.mu.Unlock()

	bellui.Do(func() {
		p.img.Image = frame
		p.img.Refresh()
		if resized != nil && onResize != nil {
			onResize(*resized)
		}
	})
}

// close releases the preview decoder. Requests after it are dropped rather than
// reopening the media.
func (p *scrubPreview) close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.pendingSet = false
	p.mu.Unlock()

	p.thumb.Close()
}
