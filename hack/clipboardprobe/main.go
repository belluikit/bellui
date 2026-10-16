// Command clipboardprobe live-verifies the clipboard flavor interfaces
// (pre-rebrand tracker #513) against the real system clipboard.
//
// CLI mode (desktop): run one operation and exit, e.g.
//
//	go run ./hack/clipboardprobe caps
//	go run ./hack/clipboardprobe set-html '<b>bold</b>' 'bold'
//	go run ./hack/clipboardprobe get-image /tmp/out.png
//
// Managed CLI mode (Linux): --run executes the operation inside the running
// app — clipboard access there needs the initialized GLFW loop, Wayland
// additionally needs the window focused, and after a set the process must
// stay alive to serve the X11 selection / Wayland data source:
//
//	go run ./hack/clipboardprobe --run set-image   # serves until killed
//	go run ./hack/clipboardprobe --run get-image /tmp/out.png
//
// UI mode (no args): a button-per-operation window, needed on wasm where
// clipboard access must happen inside a user gesture.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()
	args := os.Args[1:]
	if len(args) > 1 && args[0] == "--run" {
		runManaged(a, args[1:])
		return
	}
	if len(args) > 0 {
		os.Exit(runCLI(a.Clipboard(), args[0], args[1:]))
	}
	runUI(a)
}

// runManaged executes one CLI operation while the app runs. The delay before
// the operation leaves time for an external actor (a window manager script,
// a click) to focus the window, which Wayland requires for both selection
// reads and writes. Get operations quit when done; set operations keep the
// app alive serving the selection until the process is killed.
func runManaged(a bellui.App, args []string) {
	w := a.NewWindow("Clipboard Probe")
	w.Resize(geo.NewSize(240, 120))
	w.SetContent(widget.NewLabel("clipboardprobe --run " + strings.Join(args, " ")))

	code := 0
	a.Lifecycle().SetOnStarted(func() {
		go func() {
			time.Sleep(3 * time.Second)
			bellui.DoAndWait(func() {
				code = runCLI(a.Clipboard(), args[0], args[1:])
			})
			if strings.HasPrefix(args[0], "set-") && code == 0 {
				return // keep serving the selection
			}
			bellui.Do(a.Quit)
		}()
	})
	bellui.ShowAndRun(w)
	os.Exit(code)
}

func capabilities(cb bellui.Clipboard) string {
	var caps []string
	if _, ok := cb.(bellui.ImageClipboard); ok {
		caps = append(caps, "image")
	}
	if _, ok := cb.(bellui.FileClipboard); ok {
		caps = append(caps, "files")
	}
	if _, ok := cb.(bellui.HTMLClipboard); ok {
		caps = append(caps, "html")
	}
	if _, ok := cb.(bellui.MultiFlavorClipboard); ok {
		caps = append(caps, "multi")
	}
	if len(caps) == 0 {
		return "text only"
	}
	return strings.Join(caps, ", ")
}

// testImage draws a red/green/blue/white 64x64 quadrant image.
func testImage() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			c := color.NRGBA{A: 255}
			switch {
			case x < 32 && y < 32:
				c.R = 255
			case x >= 32 && y < 32:
				c.G = 255
			case x < 32:
				c.B = 255
			default:
				c.R, c.G, c.B = 255, 255, 255
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func runCLI(cb bellui.Clipboard, op string, args []string) int {
	fail := func(format string, a ...any) int {
		fmt.Fprintf(os.Stderr, format+"\n", a...)
		return 1
	}

	switch op {
	case "caps":
		fmt.Println(capabilities(cb))

	case "set-text":
		cb.SetContent(strings.Join(args, " "))

	case "get-text":
		fmt.Println(cb.Content())

	case "set-html":
		if len(args) != 2 {
			return fail("usage: set-html <html> <plain>")
		}
		if !setHTML(cb, args[0], args[1]) {
			return fail("HTML clipboard unsupported")
		}

	case "get-html":
		c, ok := cb.(bellui.HTMLClipboard)
		if !ok {
			return fail("HTML clipboard unsupported")
		}
		html, ok := c.HTML()
		if !ok {
			return fail("no HTML on clipboard")
		}
		fmt.Println(html)

	case "set-image":
		c, ok := cb.(bellui.ImageClipboard)
		if !ok {
			return fail("image clipboard unsupported")
		}
		img := testImage()
		if len(args) == 1 {
			f, err := os.Open(args[0])
			if err != nil {
				return fail("open: %v", err)
			}
			defer f.Close()
			img, err = png.Decode(f)
			if err != nil {
				return fail("decode: %v", err)
			}
		}
		c.SetImage(img)

	case "get-image":
		c, ok := cb.(bellui.ImageClipboard)
		if !ok {
			return fail("image clipboard unsupported")
		}
		img, ok := c.Image()
		if !ok {
			return fail("no image on clipboard")
		}
		fmt.Printf("%dx%d\n", img.Bounds().Dx(), img.Bounds().Dy())
		if len(args) == 1 {
			f, err := os.Create(args[0])
			if err != nil {
				return fail("create: %v", err)
			}
			defer f.Close()
			if err := png.Encode(f, img); err != nil {
				return fail("encode: %v", err)
			}
		}

	case "set-files":
		c, ok := cb.(bellui.FileClipboard)
		if !ok {
			return fail("file clipboard unsupported")
		}
		files := make([]bellui.URI, 0, len(args))
		for _, p := range args {
			files = append(files, storage.NewFileURI(p))
		}
		c.SetFiles(files)

	case "set-multi", "set-multi-th", "set-multi-ti":
		if !setMultiOp(cb, op, args) {
			return fail("multi-flavor clipboard unsupported")
		}

	case "get-files":
		c, ok := cb.(bellui.FileClipboard)
		if !ok {
			return fail("file clipboard unsupported")
		}
		files := c.Files()
		if files == nil {
			return fail("no files on clipboard")
		}
		for _, f := range files {
			fmt.Println(f.Path())
		}

	default:
		return fail("unknown op %q (caps|set-text|get-text|set-html|get-html|set-image|get-image|set-files|get-files|set-multi|set-multi-th|set-multi-ti)", op)
	}
	return 0
}

// setMultiOp writes one multi-flavor transaction (pre-rebrand tracker #583). "set-multi" is the
// full set — text + HTML + image, plus a file per given path; the pairwise
// variants isolate which flavor combination a paste target mishandles (used
// to diagnose the iOS Notes empty-paste of the three-flavor item, pre-rebrand tracker #1144).
func setMultiOp(cb bellui.Clipboard, op string, paths []string) bool {
	c, ok := cb.(bellui.MultiFlavorClipboard)
	if !ok {
		return false
	}

	var content bellui.ClipboardContent
	switch op {
	case "set-multi-th": // text + HTML only
		content = bellui.ClipboardContent{
			Text: "th: combined flavors from clipboard probe",
			HTML: "<b>th combined</b> flavors from <i>clipboard probe</i>",
		}
	case "set-multi-ti": // text + image only
		content = bellui.ClipboardContent{
			Text:  "ti: combined flavors from clipboard probe",
			Image: testImage(),
		}
	default:
		content = bellui.ClipboardContent{
			Text:  "combined flavors from clipboard probe",
			HTML:  "<b>combined</b> flavors from <i>clipboard probe</i>",
			Image: testImage(),
		}
		for _, p := range paths {
			content.Files = append(content.Files, storage.NewFileURI(p))
		}
	}
	c.SetFlavors(content)
	return true
}

func setHTML(cb bellui.Clipboard, html, plain string) bool {
	c, ok := cb.(bellui.HTMLClipboard)
	if !ok {
		return false
	}
	c.SetHTML(html, plain)
	return true
}

func runUI(a bellui.App) {
	w := a.NewWindow("Clipboard Probe")
	cb := a.Clipboard()

	status := widget.NewLabel("capabilities: " + capabilities(cb))
	status.Wrapping = bellui.TextWrapWord
	preview := canvas.NewImageFromImage(image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	preview.FillMode = canvas.ImageFillContain
	preview.SetMinSize(geo.NewSize(128, 128))

	report := func(format string, args ...any) {
		status.SetText(fmt.Sprintf(format, args...))
	}

	buttons := container.NewGridWithColumns(
		2,
		widget.NewButton("Set image", func() {
			if ok := bellui.SetClipboardImage(testImage()); !ok {
				report("image unsupported")
				return
			}
			report("wrote 64x64 test image")
		}),
		widget.NewButton("Get image", func() {
			img, ok := bellui.ClipboardImage()
			if !ok {
				report("no image on clipboard")
				return
			}
			preview.Image = img
			preview.Refresh()
			report("read %dx%d image", img.Bounds().Dx(), img.Bounds().Dy())
		}),
		widget.NewButton("Set HTML", func() {
			if ok := bellui.SetClipboardHTML("<b>bold</b> and <i>italic</i> from bellui", "bold and italic from bellui"); !ok {
				report("HTML unsupported")
				return
			}
			report("wrote HTML + plain fallback")
		}),
		widget.NewButton("Get HTML", func() {
			html, ok := bellui.ClipboardHTML()
			if !ok {
				report("no HTML on clipboard")
				return
			}
			report("HTML: %s", html)
		}),
		widget.NewButton("Set files", func() {
			tmp, err := os.CreateTemp("", "clipboardprobe-*.txt")
			if err != nil {
				report("tempfile: %v", err)
				return
			}
			tmp.WriteString("clipboard probe test file\n")
			tmp.Close()
			if ok := bellui.SetClipboardFiles([]bellui.URI{storage.NewFileURI(tmp.Name())}); !ok {
				report("files unsupported")
				return
			}
			report("wrote file list: %s", tmp.Name())
		}),
		widget.NewButton("Get files", func() {
			files := bellui.ClipboardFiles()
			if files == nil {
				report("no files on clipboard")
				return
			}
			paths := make([]string, 0, len(files))
			for _, f := range files {
				paths = append(paths, f.Path())
			}
			report("files: %s", strings.Join(paths, ", "))
		}),
		widget.NewButton("Set multi", func() {
			ok := bellui.SetClipboardFlavors(bellui.ClipboardContent{
				Text:  "combined flavors from clipboard probe",
				HTML:  "<b>combined</b> flavors from <i>clipboard probe</i>",
				Image: testImage(),
			})
			if !ok {
				report("multi-flavor unsupported")
				return
			}
			report("wrote text + HTML + image in one transaction")
		}),
		widget.NewButton("Set text", func() {
			cb.SetContent("plain text from clipboard probe")
			report("wrote plain text")
		}),
		widget.NewButton("Get text", func() {
			report("text: %s", cb.Content())
		}),
	)

	w.SetContent(container.NewBorder(nil, container.NewVBox(status, preview), nil, nil, buttons))
	w.Resize(geo.NewSize(480, 420))
	bellui.ShowAndRun(w)
}
