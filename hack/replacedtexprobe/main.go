// Command replacedtexprobe checks #1082 on a GPU: a replaced screen's image,
// and the texture the painter made for it, are let go of with the walk of the
// frame that shows the new screen, with no frame after it.
//
// It shows a 2048x2048 image under a RepaintBoundary for 2 s, so the painter
// draws it into a texture, replaces the content with a label, and then sits
// idle, collecting garbage every 250 ms and printing
//
//	REPLACED frames=N
//	RESULT image=freed|KEPT after=... frames-since-replace=M
//
// image=freed means the image is unreachable, which on WebGPU it can only be
// once the painter's Free released its texture and dropped the cache entry
// that held it. frames-since-replace counts the frames begun after the
// replace: the frame that shows the label is one, so 1 means nothing after it
// was needed. Before #1082 the texture cache kept the image until the first
// frame at least a minute after it was last drawn, so an idle window printed
// KEPT at the 20 s limit.
//
//	go run ./hack/replacedtexprobe
//
// On Android, package it and read logcat for "replacedtexprobe".
package main

import (
	"image"
	"image/color"
	"log"
	"runtime"
	"time"
	"weak"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/gpu"
	"github.com/belluikit/bellui/widget"
)

const limit = 20 * time.Second

func main() {
	log.SetPrefix("replacedtexprobe: ")
	a := app.NewWithID("com.bellui.replacedtexprobe")
	w := a.NewWindow("replacedtexprobe")

	pix := image.NewNRGBA(image.Rect(0, 0, 2048, 2048))
	for i := range pix.Pix {
		pix.Pix[i] = byte(i)
	}
	img := canvas.NewImageFromImage(pix)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(geo.NewSquareSize(200))
	weakImg := weak.Make(img)
	w.SetContent(container.NewRepaintBoundary(container.NewVBox(widget.NewLabel("screen A"), img)))
	pix, img = nil, nil
	w.Resize(geo.NewSize(400, 400))

	go func() {
		time.Sleep(2 * time.Second)
		var at uint64
		bellui.DoAndWait(func() {
			at = frames(w.Canvas())
			w.SetContent(container.NewCenter(canvas.NewText("screen B", color.NRGBA{R: 200, A: 255})))
			log.Printf("REPLACED frames=%d", at)
		})
		start := time.Now()
		for time.Since(start) < limit {
			time.Sleep(250 * time.Millisecond)
			runtime.GC()
			runtime.GC()
			if weakImg.Value() == nil {
				report("freed", start, w, at)
				return
			}
		}
		report("KEPT", start, w, at)
	}()
	bellui.ShowAndRun(w)
}

func report(result string, start time.Time, w bellui.Window, at uint64) {
	bellui.DoAndWait(func() {
		log.Printf("RESULT image=%s after=%v frames-since-replace=%d", result,
			time.Since(start).Round(time.Millisecond), frames(w.Canvas())-at)
	})
}

func frames(c core.Canvas) uint64 {
	p := c.(interface{ Painter() gpu.Painter }).Painter()
	if f, ok := p.(interface{ FramesBegun() uint64 }); ok {
		return f.FramesBegun()
	}
	return 0
}
