package glfw

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strconv"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/build"
)

// partialVerifier checks partial repaint against a full repaint on the real GPU
// path, which the software-painter golden suite cannot reach (#496). It is
// enabled by BELLUI_PARTIAL_VERIFY=N in a build with the partialrepaint tag, and
// is otherwise inert.
//
// Every N painted frames it captures the window as partial repaint left it,
// makes the next frame a full repaint (Canvas.MarkFullRepaint), captures that,
// and compares the two. Both captures run in one main-thread call, so nothing
// can change the scene between them: any pixel that differs is one partial
// repaint got wrong. A check also corrects what it found, so a wrong pixel is
// reported by the first check after the frame that left it.
//
// A check whose two captures differ takes a second full repaint; if that
// differs from the first, the scene changed during the check (a video drawing
// from another goroutine) and the check counts as unstable, not as a mismatch.
//
// It logs each mismatch with the differing pixel count and their bounding box,
// and, when BELLUI_PARTIAL_VERIFY_DIR names a directory, saves the first few as
// PNGs (partial, full and a diff mask). The driver's shutdown prints a summary.
type partialVerifier struct {
	enabled bool
	every   int
	dir     string

	busy                     bool
	frames, checks, failures int
	differing                int // pixels, over all checks
	// unstable counts checks whose two full repaints differed: the scene
	// changed during the check, so it proves nothing.
	unstable int
}

// maxSavedMismatches bounds the PNGs a run writes.
const maxSavedMismatches = 5

var verifier = newPartialVerifier()

func newPartialVerifier() *partialVerifier {
	v := os.Getenv("BELLUI_PARTIAL_VERIFY")
	if v == "" {
		return &partialVerifier{}
	}
	if !build.PartialRepaint {
		bellui.LogError("BELLUI_PARTIAL_VERIFY: ignored, this build has no partialrepaint tag", nil)
		return &partialVerifier{}
	}
	every, err := strconv.Atoi(v)
	if err != nil || every <= 0 {
		every = 1
	}
	dir := os.Getenv("BELLUI_PARTIAL_VERIFY_DIR")
	bellui.LogError(fmt.Sprintf("BELLUI_PARTIAL_VERIFY: comparing partial repaint with a full repaint every %d frames", every), nil)
	return &partialVerifier{enabled: true, every: every, dir: dir}
}

// afterFrame runs after w presented a frame.
func (v *partialVerifier) afterFrame(w *window) {
	if !v.enabled || v.busy || !w.visible {
		return
	}
	v.frames++
	if v.frames%v.every != 0 {
		return
	}
	v.busy = true // the captures repaint, which would call this again
	defer func() { v.busy = false }()

	partial := w.canvas.Capture()
	w.canvas.MarkFullRepaint()
	full := w.canvas.Capture()
	if partial == nil || full == nil || partial.Bounds().Empty() {
		return
	}
	v.checks++
	n, box, maxDelta := diffImages(partial, full)
	if n == 0 {
		return
	}
	// Something drawing from another goroutine (a video, a streaming raster)
	// can change the scene between the captures. A second full repaint tells
	// that apart from a partial repaint error: it should match the first.
	w.canvas.MarkFullRepaint()
	again := w.canvas.Capture()
	if m, _, _ := diffImages(full, again); m != 0 {
		v.unstable++
		return
	}
	v.failures++
	v.differing += n
	msg := fmt.Sprintf("BELLUI_PARTIAL_VERIFY: frame %d: %d pixels differ from a full repaint, within %v, max channel delta %d",
		v.frames, n, box, maxDelta)
	if v.dir != "" && v.failures <= maxSavedMismatches {
		if err := saveMismatch(v.dir, v.frames, partial, full); err != nil {
			msg += fmt.Sprintf(" (saving failed: %v)", err)
		} else {
			msg += fmt.Sprintf(" (saved in %s)", v.dir)
		}
	}
	bellui.LogError(msg, nil)
}

// Flush prints the run's summary. Safe to call when disabled.
func (v *partialVerifier) Flush() {
	if v == nil || !v.enabled {
		return
	}
	bellui.LogError(fmt.Sprintf("BELLUI_PARTIAL_VERIFY: summary: frames=%d checks=%d mismatches=%d unstable=%d differing_pixels=%d",
		v.frames, v.checks, v.failures, v.unstable, v.differing), nil)
}

// diffImages counts the pixels that differ between a and b, which must have the
// same bounds, and returns their bounding box and the largest channel delta.
// Differently sized images differ everywhere.
func diffImages(a, b image.Image) (n int, box image.Rectangle, maxDelta uint8) {
	if a.Bounds() != b.Bounds() {
		return a.Bounds().Dx() * a.Bounds().Dy(), a.Bounds(), 255
	}
	ra, rb := toRGBA(a), toRGBA(b)
	r := ra.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		pa := ra.Pix[ra.PixOffset(r.Min.X, y):ra.PixOffset(r.Max.X, y)]
		pb := rb.Pix[rb.PixOffset(r.Min.X, y):rb.PixOffset(r.Max.X, y)]
		for i := 0; i < len(pa); i += 4 {
			d := maxChannelDelta(pa[i:i+4], pb[i:i+4])
			if d == 0 {
				continue
			}
			n++
			maxDelta = max(maxDelta, d)
			p := image.Pt(r.Min.X+i/4, y)
			box = box.Union(image.Rectangle{Min: p, Max: p.Add(image.Pt(1, 1))})
		}
	}
	return n, box, maxDelta
}

func maxChannelDelta(a, b []uint8) uint8 {
	var m uint8
	for i := range a {
		d := a[i] - b[i]
		if b[i] > a[i] {
			d = b[i] - a[i]
		}
		m = max(m, d)
	}
	return m
}

func toRGBA(img image.Image) *image.RGBA {
	if r, ok := img.(*image.RGBA); ok {
		return r
	}
	r := image.NewRGBA(img.Bounds())
	draw.Draw(r, r.Bounds(), img, img.Bounds().Min, draw.Src)
	return r
}

// saveMismatch writes the partial and full captures and a mask of where they
// differ (white) into dir.
func saveMismatch(dir string, frame int, partial, full image.Image) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	pa, fu := toRGBA(partial), toRGBA(full)
	mask := image.NewGray(pa.Bounds())
	for y := pa.Rect.Min.Y; y < pa.Rect.Max.Y; y++ {
		for x := pa.Rect.Min.X; x < pa.Rect.Max.X; x++ {
			i := pa.PixOffset(x, y)
			if maxChannelDelta(pa.Pix[i:i+4], fu.Pix[i:i+4]) != 0 {
				mask.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	for name, img := range map[string]image.Image{"partial": pa, "full": fu, "diff": mask} {
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("frame%06d-%s.png", frame, name)))
		if err != nil {
			return err
		}
		err = png.Encode(f, img)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	return nil
}
