package widget

// AnimatedGif is adapted from the fyne-x project (fyne.io/x/fyne, widget/gif.go),
// Copyright (C) 2020 Fyne.io developers and community, BSD-3-Clause license.
// The port replaces the frame-driving goroutine with a bellui.Animation frame
// ticker on the main goroutine and adds renderer lifecycle handling
// (Destroy stops the ticker, CreateRenderer re-arms it on re-show).

import (
	"bytes"
	"image"
	"image/draw"
	"image/gif"
	"io"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/layoutgen"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/theme"
)

// AnimatedGif widget shows a Gif image with many frames.
// All methods must be called from the main (UI) goroutine, like other widgets;
// frames are advanced by a per-frame animation tick, so playback pauses with
// the canvas and needs no background goroutine.
//
// Since: 1.0
type AnimatedGif struct {
	BaseWidget
	min geo.Size

	src            *gif.GIF
	dst            *canvas.Image
	buffer         *image.NRGBA
	noDisposeIndex int
	remaining      int
	frameIndex     int
	nextFrameAt    time.Time

	// playing is the desired state (Start called, not yet stopped or
	// completed); ticker is the live frame driver. They differ while the
	// renderer is destroyed: eviction of a hidden widget stops the ticker,
	// CreateRenderer re-arms it from playing.
	playing bool
	ticker  *bellui.Animation
}

// NewAnimatedGif creates a new widget loaded to show the specified image.
// If there is an error loading the image it will be returned in the error value.
func NewAnimatedGif(u bellui.URI) (*AnimatedGif, error) {
	ret := newGif()

	return ret, ret.Load(u)
}

// NewAnimatedGifFromResource creates a new widget loaded to show the specified image resource.
// If there is an error loading the image it will be returned in the error value.
func NewAnimatedGifFromResource(r bellui.Resource) (*AnimatedGif, error) {
	ret := newGif()

	return ret, ret.LoadResource(r)
}

// CreateRenderer loads the widget renderer for this widget. This is an internal requirement for Bellui.
func (g *AnimatedGif) CreateRenderer() core.WidgetRenderer {
	// Re-arm playback stopped by a renderer Destroy: eviction also hits
	// live, temporarily-hidden widgets, and on re-show no other seam fires.
	// Idempotent when the ticker is already running.
	if g.playing {
		g.startTicker()
	}
	return &gifRenderer{gif: g}
}

// Load is used to change the gif file shown.
// It will change the loaded content and prepare the new frames for animation.
// Any running animation is stopped.
func (g *AnimatedGif) Load(u bellui.URI) error {
	g.stopPlayback()
	g.src = nil
	g.dst.Image = nil
	g.dst.Refresh()

	if u == nil {
		return nil
	}

	read, err := storage.Reader(u)
	if err != nil {
		return err
	}
	defer read.Close()

	return g.load(read)
}

// LoadResource is used to change the gif resource shown.
// It will change the loaded content and prepare the new frames for animation.
// Any running animation is stopped.
func (g *AnimatedGif) LoadResource(r bellui.Resource) error {
	g.stopPlayback()
	g.src = nil
	g.dst.Image = nil
	g.dst.Refresh()

	if r == nil || len(r.Content()) == 0 {
		return nil
	}
	return g.load(bytes.NewReader(r.Content()))
}

func (g *AnimatedGif) load(read io.Reader) error {
	pix, err := gif.DecodeAll(read)
	if err != nil {
		return err
	}
	g.src = pix
	g.dst.Image = pix.Image[0]
	g.dst.Refresh()

	return nil
}

// MinSize returns the minimum size that this GIF can occupy.
// Because gif images are measured in pixels we cannot use the dimensions, so this defaults to 0x0.
// You can set a minimum size if required using SetMinSize.
func (g *AnimatedGif) MinSize() geo.Size {
	return g.min
}

// SetMinSize sets the smallest possible size that this AnimatedGif should be drawn at.
// Be careful not to set this based on pixel sizes as that will vary based on output device.
func (g *AnimatedGif) SetMinSize(min geo.Size) {
	if g.min != min {
		layoutgen.Changed()
	}
	g.min = min
}

// Start begins the animation. The speed of the transition is controlled by the loaded gif file.
func (g *AnimatedGif) Start() {
	if g.playing || g.src == nil || len(g.src.Image) == 0 {
		return
	}
	g.playing = true

	buffer := image.NewNRGBA(g.dst.Image.Bounds())
	g.buffer = buffer
	g.dst.Image = buffer
	g.noDisposeIndex = -1
	g.drawFrame(0, gif.DisposalNone)
	g.frameIndex = 0
	g.nextFrameAt = time.Now().Add(g.frameDelay(0))

	switch g.src.LoopCount {
	case -1: // don't loop
		g.remaining = 1
	case 0: // loop forever
		g.remaining = -1
	default:
		g.remaining = g.src.LoopCount + 1
	}

	g.startTicker()
}

// Stop will request that the animation stops running, the last frame will remain visible.
func (g *AnimatedGif) Stop() {
	g.stopPlayback()
}

func (g *AnimatedGif) stopPlayback() {
	g.playing = false
	g.stopTicker()
}

// startTicker arms the per-frame driver; a no-op when it is already running.
func (g *AnimatedGif) startTicker() {
	if g.ticker != nil {
		return
	}
	if a := bellui.CurrentApp(); a == nil || a.Driver() == nil {
		return
	}
	anim := &bellui.Animation{
		Duration:    time.Second, // irrelevant: Tick fires every frame while running
		RepeatCount: bellui.AnimationRepeatForever,
		Curve:       bellui.AnimationLinear,
		Tick:        func(float32) { g.tick() },
	}
	g.ticker = anim
	anim.Start()
}

func (g *AnimatedGif) stopTicker() {
	if g.ticker == nil {
		return
	}
	g.ticker.Stop()
	g.ticker = nil
}

// tick runs once per rendered frame and advances the gif when the current
// frame's delay has elapsed. Advancing is bounded to one full cycle per tick;
// falling further behind (e.g. re-shown after a long eviction) resynchronizes
// to now instead of replaying the backlog.
func (g *AnimatedGif) tick() {
	if !g.playing || g.src == nil {
		g.stopTicker()
		return
	}
	now := time.Now()
	for steps := 0; g.playing && !now.Before(g.nextFrameAt); steps++ {
		if steps >= len(g.src.Image) {
			g.nextFrameAt = now.Add(g.frameDelay(g.frameIndex))
			break
		}
		g.advance()
	}
}

// advance composites the next frame and schedules the one after it, counting
// down the remaining loops on wrap-around (the last frame stays visible).
func (g *AnimatedGif) advance() {
	next := g.frameIndex + 1
	if next == len(g.src.Image) {
		if g.remaining > -1 { // don't underflow int
			g.remaining--
		}
		if g.remaining == 0 {
			g.stopPlayback()
			return
		}
		next = 0
	}

	dispose := byte(gif.DisposalNone)
	if next > 0 {
		dispose = g.src.Disposal[next-1]
	}
	g.drawFrame(next, dispose)
	g.frameIndex = next
	g.nextFrameAt = g.nextFrameAt.Add(g.frameDelay(next))
}

func (g *AnimatedGif) frameDelay(index int) time.Duration {
	if index >= len(g.src.Delay) {
		return 0
	}
	return time.Duration(g.src.Delay[index]) * 10 * time.Millisecond
}

func (g *AnimatedGif) drawFrame(index int, dispose byte) {
	dst := g.buffer
	frame := g.src.Image[index]
	bounds := dst.Bounds()
	switch dispose {
	case gif.DisposalNone:
		// Do not dispose old frame, draw new frame over old
		draw.Draw(dst, bounds, frame, bounds.Min, draw.Over)
		// will be used in case of disposalPrevious
		g.noDisposeIndex = index - 1
	case gif.DisposalBackground:
		// clear with background then render new frame Over it.
		// We must blank out the previous in case there is transparent content on the next frame.
		draw.Draw(dst, bounds, image.NewUniform(theme.ColorForWidget(theme.ColorNameBackground, g)), bounds.Min, draw.Src)
		draw.Draw(dst, bounds, frame, bounds.Min, draw.Src)
	case gif.DisposalPrevious:
		// restore frame with previous image then render new over it
		if g.noDisposeIndex >= 0 {
			draw.Draw(dst, bounds, g.src.Image[g.noDisposeIndex], bounds.Min, draw.Src)
			draw.Draw(dst, bounds, frame, bounds.Min, draw.Over)
		} else {
			// there was no previous graphic, render background instead?
			draw.Draw(dst, bounds, frame, bounds.Min, draw.Src)
		}
	default:
		// Disposal = Unspecified/Reserved, simply draw new frame over previous
		draw.Draw(dst, bounds, frame, bounds.Min, draw.Over)
	}

	g.dst.Refresh()
}

func newGif() *AnimatedGif {
	ret := &AnimatedGif{}
	ret.ExtendBaseWidget(ret)
	ret.dst = &canvas.Image{}
	ret.dst.FillMode = canvas.ImageFillContain
	return ret
}

type gifRenderer struct {
	gif *AnimatedGif
}

// Destroy stops the frame ticker so a discarded widget's animation closure
// does not keep it alive and ticking. Destroy also fires on cache eviction of
// a live, temporarily-hidden widget, so the desired playing state is kept and
// CreateRenderer re-arms the ticker on re-show.
func (g *gifRenderer) Destroy() {
	g.gif.stopTicker()
}

func (g *gifRenderer) Layout(size geo.Size) {
	g.gif.dst.Resize(size)
}

func (g *gifRenderer) MinSize() geo.Size {
	return g.gif.MinSize()
}

func (g *gifRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{g.gif.dst}
}

func (g *gifRenderer) Refresh() {
	g.gif.dst.Refresh()
}
