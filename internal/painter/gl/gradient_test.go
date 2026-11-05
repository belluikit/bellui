//go:build (gl || (!darwin && !linux && !windows && !wasm)) && !android && !ios && !mobile && !wasm && !no_glfw

package gl

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"runtime"
	"testing"

	"github.com/belluikit/glfw"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// mainCalls carries work to the main thread, where macOS requires GLFW and
// the context it makes current to live.
var mainCalls = make(chan func())

// TestMain runs the tests on another goroutine and keeps the main thread
// serving onMain.
func TestMain(m *testing.M) {
	runtime.LockOSThread()
	done := make(chan int)
	go func() { done <- m.Run() }()
	for {
		select {
		case f := <-mainCalls:
			f()
		case code := <-done:
			os.Exit(code)
		}
	}
}

func onMain(f func()) {
	ran := make(chan struct{})
	mainCalls <- func() { defer close(ran); f() }
	<-ran
}

const offscreenDim = 96

// offscreenCanvas is the least of a canvas the painter reads while drawing.
type offscreenCanvas struct {
	core.Canvas
}

func (offscreenCanvas) Size() geo.Size { return geo.NewSquareSize(offscreenDim) }
func (offscreenCanvas) Scale() float32 { return 1 }

// noContext stands in for the driver: the test keeps the context current.
type noContext struct{}

func (noContext) RunWithContext(f func()) { f() }
func (noContext) RescaleContext()         {}
func (noContext) Context() any            { return nil }

// offscreenPainter is a GL painter drawing into an offscreenDim² texture,
// in the context of a hidden GLFW window. Its methods run on the main thread.
type offscreenPainter struct {
	*painter
	window *glfw.Window
	fbo    Framebuffer
}

// newOffscreenPainter makes the painter, or skips the test where no GL
// context can be had (no display, no driver).
func newOffscreenPainter(t *testing.T) *offscreenPainter {
	t.Helper()
	var o *offscreenPainter
	var err error
	onMain(func() {
		if err = glfw.Init(); err != nil {
			return
		}
		glfw.WindowHint(glfw.Visible, glfw.False)
		contextHints()
		var w *glfw.Window
		if w, err = glfw.CreateWindow(offscreenDim, offscreenDim, "gl painter test", nil, nil); err != nil {
			return
		}
		w.MakeContextCurrent()
		p := NewPainter(offscreenCanvas{}, noContext{}).(*painter)
		p.Init()
		o = &offscreenPainter{painter: p, window: w}

		tex := p.ctx.CreateTexture()
		p.ctx.BindTexture(texture2D, tex)
		p.ctx.TexImage2D(texture2D, 0, offscreenDim, offscreenDim, colorFormatRGBA, unsignedByte, nil)
		o.fbo = p.ctx.CreateFramebuffer()
		p.ctx.BindFramebuffer(glFramebuffer, o.fbo)
		p.ctx.FramebufferTexture2D(glFramebuffer, colorAttachment0, tex, 0)
		if s := p.ctx.CheckFramebufferStatus(glFramebuffer); s != framebufferComplete {
			err = fmt.Errorf("offscreen framebuffer incomplete: status 0x%x", s)
		}
	})
	if err != nil {
		t.Skipf("no GL context for an offscreen painter: %v", err)
	}
	require.False(t, o.InitFailed(), "the painter's programs did not build")
	t.Cleanup(func() { onMain(func() { o.window.Destroy(); glfw.Terminate() }) })
	return o
}

// paint clears the target to opaque black, draws obj at (10, 10) and reads
// the pixels back, top row first.
func (o *offscreenPainter) paint(obj core.CanvasObject) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, offscreenDim, offscreenDim))
	onMain(func() {
		p := o.painter
		p.ctx.BindFramebuffer(glFramebuffer, o.fbo)
		p.ctx.Viewport(0, 0, offscreenDim, offscreenDim)
		p.ctx.ClearColor(0, 0, 0, 1)
		p.ctx.Clear(bitColorBuffer)
		p.Paint(obj, geo.NewPos(10, 10), offscreenCanvas{}.Size())
		p.ctx.Flush()
		rows := make([]uint8, len(img.Pix))
		p.ctx.ReadPixels(0, 0, offscreenDim, offscreenDim, colorFormatRGBA, unsignedByte, rows)
		stride := offscreenDim * 4
		for y := range offscreenDim {
			copy(img.Pix[y*stride:(y+1)*stride], rows[(offscreenDim-1-y)*stride:])
		}
	})
	return img
}

// paintGradient draws obj with the gradient shader or with the CPU texture
// reference.
func (o *offscreenPainter) paintGradient(obj core.CanvasObject, shader bool) *image.RGBA {
	prev := gpuGradients
	gpuGradients = shader
	defer func() { gpuGradients = prev }()
	return o.paint(obj)
}

// TestGradientShaderMatchesGenerate pins #957: the GL painter's gradient
// shader draws what the CPU texture (canvas Generate) draws, for the cases
// the WebGPU painter's test of the same name covers (#956).
func TestGradientShaderMatchesGenerate(t *testing.T) {
	o := newOffscreenPainter(t)
	require.NotNil(t, o.gradientProgram.uniforms, "the gradient program did not build")
	red := color.NRGBA{R: 255, A: 255}
	blue := color.NRGBA{B: 255, A: 255}
	soft := color.NRGBA{R: 255, G: 255, B: 255, A: 0x24}

	cases := map[string]func() core.CanvasObject{}
	for _, a := range []float64{0, 45, 90, 135, 180, 225, 270, 315, 30} {
		cases["linear angle "+fmtAngle(a)] = func() core.CanvasObject {
			g := canvas.NewLinearGradient(red, blue, a)
			g.Resize(geo.NewSize(70, 50))
			return g
		}
	}
	cases["radial"] = func() core.CanvasObject {
		g := canvas.NewRadialGradient(red, blue)
		g.Resize(geo.NewSize(70, 70))
		return g
	}
	cases["radial offset"] = func() core.CanvasObject {
		g := canvas.NewRadialGradient(red, blue)
		g.CenterOffsetX, g.CenterOffsetY = 0.2, -0.3
		g.Resize(geo.NewSize(70, 50))
		return g
	}
	cases["radial translucent to clear"] = func() core.CanvasObject {
		g := canvas.NewRadialGradient(soft, color.Transparent)
		g.Resize(geo.NewSize(70, 70))
		return g
	}
	cases["linear three stops"] = func() core.CanvasObject {
		g := canvas.NewLinearGradient(nil, nil, 270)
		g.Stops = []canvas.GradientStop{{Color: red, Position: 0}, {Color: color.NRGBA{G: 255, A: 255}, Position: 0.3}, {Color: blue, Position: 1}}
		g.Resize(geo.NewSize(70, 40))
		return g
	}

	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			ref := o.paintGradient(mk(), false)
			got := o.paintGradient(mk(), true)
			require.Greater(t, ink(ref), 100, "the reference drew nothing")
			if n := diffPixels(ref, got, 3); n > 0 {
				x, y := firstDiff(ref, got, 3)
				t.Errorf("%d pixels differ from Generate by more than 3/255; first at (%d,%d): reference %v, shader %v",
					n, x, y, ref.RGBAAt(x, y), got.RGBAAt(x, y))
			}
		})
	}
}

func fmtAngle(a float64) string {
	return string(rune('0'+int(a)/100%10)) + string(rune('0'+int(a)/10%10)) + string(rune('0'+int(a)%10))
}

// ink counts the pixels that are not the cleared black.
func ink(img *image.RGBA) int {
	n := 0
	for i := 0; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 0 || img.Pix[i+1] != 0 || img.Pix[i+2] != 0 {
			n++
		}
	}
	return n
}

// diffPixels counts the pixels whose colour differs by more than tol.
func diffPixels(a, b *image.RGBA, tol int) int {
	n := 0
	for i := 0; i < len(a.Pix); i += 4 {
		for c := range 3 {
			if d := int(a.Pix[i+c]) - int(b.Pix[i+c]); d > tol || -d > tol {
				n++
				break
			}
		}
	}
	return n
}

func firstDiff(a, b *image.RGBA, tol int) (int, int) {
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			p, q := a.RGBAAt(x, y), b.RGBAAt(x, y)
			for _, d := range []int{int(p.R) - int(q.R), int(p.G) - int(q.G), int(p.B) - int(q.B)} {
				if d > tol || -d > tol {
					return x, y
				}
			}
		}
	}
	return -1, -1
}
