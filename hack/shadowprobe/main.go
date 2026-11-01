// Command shadowprobe compares the software painter's canvas.BoxShadow with
// the GPU painter's, on the same canvas (#876). It shows opaque black shadows
// at three blur radii, with a corner radius and an offset, over white, then
// paints that one window canvas twice: through the window's GPU painter
// (Canvas.Capture), and through software.NewPainter. Every pixel inside the
// shadows' painted extents is compared.
//
//	go run ./hack/shadowprobe         # the default (WebGPU) painter
//	go run -tags gl ./hack/shadowprobe
//	BELLUI_SCALE=2 go run ./hack/shadowprobe
//
// PASS: no channel differs by more than 10/255. Most of the difference is the
// GPU's: its blur approximates the Gaussian with a 9-tap kernel and halvings
// that internal/painter.PlanBlur documents as up to about a tenth of sigma off,
// while the software painter computes it exactly. The limit was 8, from that
// bound on a straight edge. At scale 2, 20 corner pixels of the blur-12 shadow
// differed by 9. There, a fine-grid integral of the Gaussian over the rounded
// rect gave grey 58.6: the software painter drew 58 and the GPU 67. So 10 is
// the GPU's own error at a corner.
// FAIL: anything more. Before #876 the software painter drew no shadow at all,
// so the difference was the whole shadow, 255 at its middle.
package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"reflect"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter/software"
)

const maxDiff = 10

func main() {
	a := app.New()
	w := a.NewWindow("shadowprobe (#876)")
	w.SetPadded(false)

	bg := canvas.NewRectangle(color.White)
	bg.Resize(geo.NewSize(600, 260))
	objs := []core.CanvasObject{bg}
	var shadows []*canvas.BoxShadow
	for i, blur := range []float32{4, 12, 24} {
		s := canvas.NewBoxShadow(color.Black, blur)
		s.CornerRadius = 12
		s.OffsetY = 4
		s.Move(geo.NewPos(float32(60+i*180), 70))
		s.Resize(geo.NewSize(120, 100))
		shadows = append(shadows, s)
		objs = append(objs, s)
	}
	content := container.NewWithoutLayout(objs...)
	w.SetContent(content)
	w.Resize(geo.NewSize(600, 260))

	go func() {
		time.Sleep(1500 * time.Millisecond) // mapped and painted
		var gpu, sw image.Image
		var scale float32
		bellui.DoAndWait(func() {
			c := w.Canvas()
			scale = c.Scale()
			// Which painter the window has, as the evidence the GPU path ran.
			if m := reflect.ValueOf(c).MethodByName("Painter"); m.IsValid() {
				fmt.Printf("window painter: %v\n", m.Call(nil)[0].Elem().Type())
			}
			gpu = c.Capture()
			sw = software.NewPainter().Paint(c)
		})
		fail := compare(gpu, sw, shadows, scale)
		fmt.Printf("shadowprobe DONE fail=%d\n", fail)
		bellui.Do(a.Quit)
		time.Sleep(300 * time.Millisecond)
		os.Exit(min(fail, 1))
	}()
	bellui.ShowAndRun(w)
}

func compare(gpu, sw image.Image, shadows []*canvas.BoxShadow, scale float32) int {
	fmt.Printf("gpu %v, software %v, scale %.2f\n", gpu.Bounds(), sw.Bounds(), scale)
	if gpu.Bounds().Size() != sw.Bounds().Size() {
		fmt.Println("FAIL: the two images differ in size")
		return 1
	}
	fail := 0
	for _, s := range shadows {
		spread := s.BlurRadius * 2
		px := func(v float32) int { return int(v * scale) }
		box := image.Rect(px(s.Position().X+s.OffsetX-spread), px(s.Position().Y+s.OffsetY-spread),
			px(s.Position().X+s.OffsetX+s.Size().Width+spread), px(s.Position().Y+s.OffsetY+s.Size().Height+spread)).
			Intersect(gpu.Bounds())
		worst, sum, n, over := 0, 0, 0, 0
		var at image.Point
		for y := box.Min.Y; y < box.Max.Y; y++ {
			for x := box.Min.X; x < box.Max.X; x++ {
				d := diff(gpu.At(x, y), sw.At(x, y))
				sum += d
				n++
				if d > maxDiff {
					over++
				}
				if d > worst {
					worst, at = d, image.Pt(x, y)
				}
			}
		}
		// The profile along the middle row, from the padded edge inwards.
		y := (box.Min.Y + box.Max.Y) / 2
		var prof []string
		for x := box.Min.X; x < box.Min.X+px(spread*2) && x < box.Max.X; x += max(1, px(spread)/6) {
			prof = append(prof, fmt.Sprintf("%d/%d", grey(gpu.At(x, y)), grey(sw.At(x, y))))
		}
		fmt.Printf("  worst pixel %v: gpu %d, software %d\n", at, grey(gpu.At(at.X, at.Y)), grey(sw.At(at.X, at.Y)))
		verdict := "PASS"
		if worst > maxDiff {
			verdict = "FAIL"
			fail++
		}
		fmt.Printf("%s blur %.0f: max diff %d at %v, %d px over %d, mean %.2f over %d px; row %d gpu/software: %v\n",
			verdict, s.BlurRadius, worst, at, over, maxDiff, float64(sum)/float64(n), n, y, prof)
	}
	return fail
}

func diff(a, b color.Color) int {
	p := color.NRGBAModel.Convert(a).(color.NRGBA)
	q := color.NRGBAModel.Convert(b).(color.NRGBA)
	d := 0
	for _, v := range []int{int(p.R) - int(q.R), int(p.G) - int(q.G), int(p.B) - int(q.B)} {
		d = max(d, abs(v))
	}
	return d
}

func grey(c color.Color) int { return int(color.GrayModel.Convert(c).(color.Gray).Y) }

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
