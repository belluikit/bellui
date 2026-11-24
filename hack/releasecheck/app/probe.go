package main

import (
	"bufio"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// The probe moves the real mouse pointer while it runs (see probeMouse), so
// leave the mouse alone until it quits.
//
// The probe (go run ./hack/releasecheck/app -probe DIR) walks the release
// checks a script can drive on the macOS desktop, with input posted to the
// window through AppKit so that it takes the path a real click or key does:
// GLFW, then the driver's hit test, focus manager and gestures. It asserts
// what it can read back (status lines, states, focus, positions, pixel widths)
// and saves real-painter captures for the checks judged by eye. It writes
// DIR/report.md and DIR/*.png, then quits. It does not replace a person for
// the screen reader, sound, the OS appearance switch or dragging the window.

type mouseKind int

const (
	mouseMove mouseKind = iota
	mouseDown
	mouseDrag
	mouseUp
)

type probeKey struct {
	code  uint16 // macOS virtual key code
	chars string
}

var (
	keyTab    = probeKey{48, "\t"}
	keyReturn = probeKey{36, "\r"}
	keyEscape = probeKey{53, "\x1b"}
	keyLeft   = probeKey{123, ""}
	keyA      = probeKey{0, "a"}
)

type probeInput interface {
	contentWidth() (float32, error)
	mouse(k mouseKind, x, y float32) // points from the content's top-left
	key(k probeKey)
}

// probing is set while the probe runs, so a link the probe reaches reports
// its tap instead of opening a browser.
var probing bool

type probe struct {
	a     bellui.App
	w     bellui.Window
	tabs  *container.AppTabs
	in    probeInput
	dir   string
	scale float32 // points per dp
	out   *bufio.Writer
	fails int
	shots int
}

// runProbe runs every scenario, writes the report and quits the app.
func runProbe(a bellui.App, w bellui.Window, tabs *container.AppTabs, dir string) {
	probing = true
	// os.Exit, not a.Quit: quitting returns from main before the code is set.
	exit := os.Exit
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "probe:", err)
		exit(2)
	}
	f, err := os.Create(filepath.Join(dir, "report.md"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "probe:", err)
		exit(2)
	}
	p := &probe{a: a, w: w, tabs: tabs, dir: dir, out: bufio.NewWriter(f)}
	defer func() {
		if r := recover(); r != nil {
			p.logf("\n**PROBE ABORTED:** %v\n", r)
			p.fails++
		}
		p.logf("\n%d failed assertion(s), %d capture(s).\n", p.fails, p.shots)
		p.out.Flush()
		f.Close()
		fmt.Printf("probe: %d failed, report %s\n", p.fails, filepath.Join(dir, "report.md"))
		code := 0
		if p.fails > 0 {
			code = 1
		}
		exit(code)
	}()

	time.Sleep(1500 * time.Millisecond) // first frames
	in, err := newProbeInput()
	if err != nil {
		panic(err)
	}
	p.in = in
	cw, err := in.contentWidth()
	if err != nil {
		panic(err)
	}
	var canvasW float32
	bellui.DoAndWait(func() { canvasW = w.Canvas().Size().Width })
	p.scale = cw / canvasW
	p.logf("# Release check probe, macOS desktop\n\nbellui companion app, %s. Window content %.0f pt wide, canvas %.0f dp: %.3f pt per dp.\n",
		time.Now().Format(time.RFC3339), cw, canvasW, p.scale)

	for _, s := range []struct {
		name string
		run  func()
	}{
		{"Input reaches the driver (control)", p.control},
		{"WG-01 / WG-02 / WG-03 Disabled", p.disabled},
		{"WG-07 Chips", p.chips},
		{"RTL-07 Chip rows and layouts", p.layouts},
		{"SM-08 SVG strokes", p.strokes},
		{"TK-13 Swipe cards", p.swipes},
		{"TK-06 Key target", p.keys},
		{"I18N-01 / 03 / 04 Locale", p.locales},
		{"WG-05 Drawer", p.drawer},
		{"SM-06 Theme override", p.override},
		{"SM-07 Window size", p.windowSize},
		{"ED-05 Fast repeated input", p.fast},
	} {
		p.logf("\n## %s\n\n", s.name)
		s.run()
		p.out.Flush()
	}
}

func (p *probe) logf(format string, args ...any) { fmt.Fprintf(p.out, format, args...) }

// expect records an assertion: what was checked and what was read.
func (p *probe) expect(ok bool, what, got string) {
	mark := "PASS"
	if !ok {
		mark = "FAIL"
		p.fails++
	}
	p.logf("- %s: %s (read: %s)\n", mark, what, got)
}

// settle lets the main loop handle what was posted and finish short
// animations.
func (p *probe) settle(d time.Duration) {
	time.Sleep(d)
	bellui.DoAndWait(func() {})
}

func (p *probe) shot(name, note string) {
	var img image.Image
	bellui.DoAndWait(func() { img = p.w.Canvas().Capture() })
	path := filepath.Join(p.dir, name+".png")
	f, err := os.Create(path)
	if err == nil {
		err = png.Encode(f, img)
		f.Close()
	}
	if err != nil {
		p.logf("- capture %s failed: %v\n", name, err)
		return
	}
	p.shots++
	p.logf("- LOOK `%s.png`: %s\n", name, note)
}

func (p *probe) selectTab(name string) {
	bellui.DoAndWait(func() {
		for i, it := range p.tabs.Items {
			if it.Text == name {
				p.tabs.SelectIndex(i)
			}
		}
	})
	p.settle(300 * time.Millisecond)
}

// found is a visible object and where the walk put it on the canvas.
type found struct {
	obj    core.CanvasObject
	pos    geo.Position
	scroll *container.Scroll // the nearest scroll around it, if any
}

// find returns the first visible object, in the content and then the
// overlays, that match accepts.
func (p *probe) find(match func(core.CanvasObject) bool) (found, bool) {
	var res found
	var ok bool
	bellui.DoAndWait(func() {
		roots := []core.CanvasObject{p.w.Canvas().Content()}
		roots = append(roots, p.w.Canvas().Overlays().List()...)
		for _, root := range roots {
			var scrolls []*container.Scroll
			driver.WalkVisibleObjectTree(root, func(o core.CanvasObject, pos, _ geo.Position, _ geo.Size) bool {
				if s, isScroll := o.(*container.Scroll); isScroll {
					scrolls = append(scrolls, s)
				}
				if match(o) {
					res = found{obj: o, pos: pos}
					if len(scrolls) > 0 {
						res.scroll = scrolls[len(scrolls)-1]
					}
					ok = true
					return true
				}
				return false
			}, func(o core.CanvasObject, _ geo.Position, _ core.CanvasObject) {
				if s, isScroll := o.(*container.Scroll); isScroll && len(scrolls) > 0 && scrolls[len(scrolls)-1] == s {
					scrolls = scrolls[:len(scrolls)-1]
				}
			})
			if ok {
				return
			}
		}
	})
	return res, ok
}

func (p *probe) must(what string, match func(core.CanvasObject) bool) found {
	f, ok := p.find(match)
	if !ok {
		panic("not on screen: " + what)
	}
	return f
}

// reveal scrolls f's object into the window and finds it again.
func (p *probe) reveal(what string, match func(core.CanvasObject) bool) found {
	f := p.must(what, match)
	var h float32
	bellui.DoAndWait(func() { h = p.w.Canvas().Size().Height })
	size := f.obj.Size()
	if f.scroll != nil && (f.pos.Y < 0 || f.pos.Y+size.Height > h) {
		bellui.DoAndWait(func() {
			off := f.scroll.Offset
			off.Y += f.pos.Y - h/3
			f.scroll.ScrollToOffset(off)
		})
		p.settle(200 * time.Millisecond)
		f = p.must(what, match)
	}
	return f
}

func (p *probe) moveTo(x, y float32) {
	p.in.mouse(mouseMove, x*p.scale, y*p.scale)
	p.settle(60 * time.Millisecond)
}

func (p *probe) clickAt(x, y float32) {
	p.moveTo(x, y)
	p.in.mouse(mouseDown, x*p.scale, y*p.scale)
	p.settle(40 * time.Millisecond)
	p.in.mouse(mouseUp, x*p.scale, y*p.scale)
	p.settle(250 * time.Millisecond)
}

// centre is where to click f: its middle, or a Check's box, or the middle of
// a Hyperlink's text, since both are laid out across their row and take a
// click only on their box, label or text.
func centre(f found) (float32, float32) {
	s := f.obj.Size()
	switch o := f.obj.(type) {
	case *widget.Check:
		return f.pos.X + s.Height/2, f.pos.Y + s.Height/2
	case *widget.Hyperlink:
		return f.pos.X + o.MinSize().Width/2, f.pos.Y + s.Height/2
	}
	return f.pos.X + s.Width/2, f.pos.Y + s.Height/2
}

// click clicks the middle of the first visible object match accepts,
// scrolling it into view first.
func (p *probe) click(what string, match func(core.CanvasObject) bool) {
	x, y := centre(p.reveal(what, match))
	p.clickAt(x, y)
}

// drag presses at (x, y), moves by (dx, dy) in steps and releases.
func (p *probe) drag(x, y, dx, dy float32) {
	p.moveTo(x, y)
	p.in.mouse(mouseDown, x*p.scale, y*p.scale)
	p.settle(30 * time.Millisecond)
	const steps = 12
	for i := 1; i <= steps; i++ {
		fx, fy := x+dx*float32(i)/steps, y+dy*float32(i)/steps
		p.in.mouse(mouseDrag, fx*p.scale, fy*p.scale)
		time.Sleep(16 * time.Millisecond)
	}
	p.in.mouse(mouseUp, (x+dx)*p.scale, (y+dy)*p.scale)
	p.settle(900 * time.Millisecond)
}

func (p *probe) press(k probeKey) {
	p.in.key(k)
	p.settle(150 * time.Millisecond)
}

// Matchers.

func button(text string) func(core.CanvasObject) bool {
	return func(o core.CanvasObject) bool { b, ok := o.(*widget.Button); return ok && b.Text == text }
}

func check(text string) func(core.CanvasObject) bool {
	return func(o core.CanvasObject) bool { c, ok := o.(*widget.Check); return ok && c.Text == text }
}

func chip(text string) func(core.CanvasObject) bool {
	return func(o core.CanvasObject) bool { c, ok := o.(*widget.Chip); return ok && c.Text == text }
}

func labelPrefix(prefix string) func(core.CanvasObject) bool {
	return func(o core.CanvasObject) bool {
		l, ok := o.(*widget.Label)
		return ok && strings.HasPrefix(l.Text, prefix)
	}
}

// tab matches a tab bar's button by what a screen reader hears.
func tab(text string) func(core.CanvasObject) bool {
	return func(o core.CanvasObject) bool {
		a, ok := o.(core.Accessible)
		return ok && a.AccessibleRole() == core.RoleTab && a.AccessibleLabel() == text
	}
}

func card(title string) func(core.CanvasObject) bool {
	return func(o core.CanvasObject) bool { c, ok := o.(*widget.Card); return ok && c.Title == title }
}

func ofType[T any]() func(core.CanvasObject) bool {
	return func(o core.CanvasObject) bool { _, ok := o.(T); return ok }
}

func (p *probe) text(what string, match func(core.CanvasObject) bool) string {
	f := p.must(what, match)
	var s string
	bellui.DoAndWait(func() { s = f.obj.(*widget.Label).Text })
	return s
}

func (p *probe) variant(v bellui.ThemeVariant) {
	bellui.DoAndWait(func() { p.a.Settings().SetThemeVariant(v) })
	p.settle(400 * time.Millisecond)
}

// Scenarios.

// control proves the input path: a click on an enabled button through
// AppKit, GLFW and the driver runs its tap handler. Without this, a check that
// expects nothing to happen would pass for the wrong reason.
func (p *probe) control() {
	p.selectTab("Disabled")
	status := labelPrefix("Everything below")
	p.click("Button", button("Button"))
	got := p.text("status", func(o core.CanvasObject) bool {
		l, ok := o.(*widget.Label)
		return ok && (strings.HasPrefix(l.Text, "Everything below") || strings.Contains(l.Text, "tapped at"))
	})
	p.expect(strings.HasPrefix(got, "Button tapped"), "a posted click on an enabled Button runs its tap", got)
	if !strings.HasPrefix(got, "Button tapped") {
		panic("posted input does not reach the app; every later result would be meaningless")
	}
	_ = status
}

func statusLabel(o core.CanvasObject) bool {
	l, ok := o.(*widget.Label)
	return ok && (strings.HasPrefix(l.Text, "Everything below") || strings.Contains(l.Text, "tapped at") ||
		strings.HasPrefix(l.Text, "Disabling in"))
}

func (p *probe) disabled() {
	p.selectTab("Disabled")
	p.variant(theme.VariantLight)
	p.shot("wg01-enabled-light", "the Disabled tab, enabled, light")

	p.click("the disable box", check("Disable everything below"))
	got := p.text("status", statusLabel)
	p.expect(got == "Everything below is disabled.", "ticking the box disables the page", got)
	p.shot("wg01-disabled-light", "WG-01: every control in the disabled look (about 38% text colour), light")
	p.variant(theme.VariantDark)
	p.shot("wg01-disabled-dark", "WG-01: the disabled look, dark; a disabled Button's label as readable as the entry's text")
	p.variant(theme.VariantLight)

	for _, b := range []string{"Button", "Icon button"} {
		p.click(b, button(b))
	}
	p.click("Assist chip", chip("Assist chip"))
	p.click("Filter chip", chip("Filter chip"))
	got = p.text("status", statusLabel)
	p.expect(got == "Everything below is disabled.", "WG-01: clicks on disabled buttons and chips report nothing", got)

	// WG-02 / #1378: the link inside the disabled tabs.
	link := func(o core.CanvasObject) bool { _, ok := o.(*widget.Hyperlink); return ok }
	f := p.reveal("the link in the first tab", link)
	x, y := centre(f)
	p.moveTo(x, y) // hover
	p.shot("wg02-link-hover-disabled", "WG-02: the pointer is over the link in the disabled first tab; no hover underline")
	p.clickAt(x, y)
	got = p.text("status", statusLabel)
	p.expect(got == "Everything below is disabled.", "WG-02 (#1378): a click on a link inside a disabled container reaches nothing", got)

	// WG-01 / #1404: the disabled search field's clear button.
	searchText := func() (t string) { bellui.DoAndWait(func() { t = disabledSearch.Text }); return t }
	var clear core.CanvasObject // read once: find runs its matcher on the main goroutine
	bellui.DoAndWait(func() { clear = disabledSearch.ActionItem })
	clearButton := func(o core.CanvasObject) bool { return clear != nil && o == clear }
	p.reveal("the form", func(o core.CanvasObject) bool { _, ok := o.(*widget.Form); return ok })
	p.shot("wg01-form-disabled", "WG-01 (#1403): the form's labels, hint, error, fields and buttons all disabled; the search field and date entry too")
	p.click("the disabled search field's clear button", clearButton)
	got = searchText()
	p.expect(got == "clear me with the ✕", "WG-01 (#1404): a click on a disabled search field's ✕ leaves its text", got)

	var before string
	tabs := ofType[*container.AppTabs]()
	inner := p.must("the inner tabs", func(o core.CanvasObject) bool {
		t, ok := o.(*container.AppTabs)
		return ok && t != p.tabs
	})
	bellui.DoAndWait(func() { before = inner.obj.(*container.AppTabs).Selected().Text })
	_ = tabs
	p.click("Second tab", tab("Second tab"))
	var after string
	bellui.DoAndWait(func() { after = inner.obj.(*container.AppTabs).Selected().Text })
	p.expect(after == before, "WG-02: a click on \"Second tab\" switches nothing while disabled", after)

	// WG-03 / #1288: Tab reaches only the two controls above the page.
	p.clickAt(4, 4)
	seen := p.tabWalk(60)
	allowed := true
	for _, s := range seen {
		if s != "Check:Disable everything below" && s != "Button:Disable everything in 3 seconds" {
			allowed = false
		}
	}
	p.expect(allowed && len(seen) > 0, "WG-03 (#1288): Tab stops only on the box and the 3-second button", strings.Join(seen, ", "))

	// Enabled again: the link takes the click and Tab, the control for both.
	p.click("the disable box", check("Disable everything below"))
	f = p.reveal("the link in the first tab", link)
	x, y = centre(f)
	p.clickAt(x, y)
	got = p.text("status", statusLabel)
	p.expect(strings.HasPrefix(got, "Link tapped"), "control: enabled, the same click reaches the link", got)
	seen = p.tabWalk(120)
	reached := false
	for _, s := range seen {
		if strings.HasPrefix(s, "Hyperlink") {
			reached = true
		}
	}
	p.expect(reached, "control: enabled, Tab reaches the link", fmt.Sprintf("%d stops", len(seen)))

	p.click("the search field's clear button", clearButton)
	got = searchText()
	p.expect(got == "", "control: enabled, the same click on the ✕ clears the search field", got)
	bellui.DoAndWait(func() { disabledSearch.SetText("clear me with the ✕") })
}

// tabWalk presses Tab n times and returns each newly focused object,
// described by type and text.
func (p *probe) tabWalk(n int) []string {
	var seen []string
	for i := 0; i < n; i++ {
		p.in.key(keyTab)
		p.settle(40 * time.Millisecond)
		var d string
		bellui.DoAndWait(func() {
			switch o := p.w.Canvas().Focused().(type) {
			case nil:
			case *widget.Check:
				d = "Check:" + o.Text
			case *widget.Button:
				d = "Button:" + o.Text
			case *widget.Hyperlink:
				d = "Hyperlink:" + o.Text
			default:
				d = fmt.Sprintf("%T", o)
			}
		})
		if d != "" && !slices.Contains(seen, d) {
			seen = append(seen, d)
		}
	}
	return seen
}

func (p *probe) chips() {
	p.selectTab("Chips & layout")
	p.variant(theme.VariantLight)
	status := labelPrefix("Selected:")
	p.shot("wg07-off-light", "WG-07: all chips off")
	p.click("Rock", chip("Rock"))
	got := p.text("status", status)
	p.expect(strings.Contains(got, "Rock is now selected"), "WG-07: the first tap selects Rock", got)
	p.shot("wg07-rock-selected-light", "WG-07: Rock selected: filled, a check mark in front")
	p.click("Rock", chip("Rock"))
	got = p.text("status", status)
	p.expect(strings.Contains(got, "Rock is now excluded") && strings.Contains(got, "Excluded: Rock"), "WG-07: the second tap excludes Rock", got)
	p.shot("wg07-rock-excluded-light", "WG-07: Rock excluded: a cross in front, a red outline, label struck through")
	var label string
	f := p.must("Rock", chip("Rock"))
	bellui.DoAndWait(func() { label = f.obj.(*widget.Chip).AccessibleLabel() })
	p.expect(label == "Rock, excluded", "A11Y-08: the excluded chip's accessible label", label)
	p.click("Rock", chip("Rock"))
	got = p.text("status", status)
	p.expect(strings.Contains(got, "Rock is now off") && strings.Contains(got, "Selected: none"), "WG-07: the third tap turns Rock off", got)

	for _, s := range []string{"Jazz", "Pop", "Folk"} {
		p.click(s, chip(s))
		p.click(s, chip(s))
	}
	p.click("Rock", chip("Rock"))
	p.click("Rock", chip("Rock"))
	got = p.text("status", status)
	p.expect(strings.Contains(got, "cannot be excluded") && strings.Contains(got, "Selected: Rock"), "WG-07: the last style cannot be excluded", got)
	p.shot("wg07-mixed-light", "WG-07: Rock selected, Jazz, Pop and Folk excluded, light")
	p.variant(theme.VariantDark)
	p.shot("wg07-mixed-dark", "WG-07: the same, dark: the three states still easy to tell apart")
	p.click("the disable box", check("Disable the group"))
	p.shot("wg07-mixed-disabled-dark", "WG-07: disabled: check mark, crosses and strike-through kept in the disabled colours")
	p.click("Jazz", chip("Jazz"))
	got = p.text("status", status)
	p.expect(strings.Contains(got, "Excluded: Jazz, Pop, Folk"), "WG-07: a disabled group takes no tap", got)
	p.click("the disable box", check("Disable the group"))
	p.variant(theme.VariantLight)

	p.click("Medium", chip("Medium"))
	p.click("Medium", chip("Medium"))
	var sel []string
	g := p.must("the single group", func(o core.CanvasObject) bool {
		c, ok := o.(*widget.ChipGroup)
		return ok && len(c.Options) > 0 && c.Options[0] == "Small"
	})
	bellui.DoAndWait(func() { sel = g.obj.(*widget.ChipGroup).Selected })
	p.expect(len(sel) == 0, "WG-07: in the one-at-a-time group a second tap deselects, never excludes", fmt.Sprint(sel))
}

func (p *probe) layouts() {
	p.selectTab("Chips & layout")
	group := func(o core.CanvasObject) bool {
		c, ok := o.(*widget.ChipGroup)
		return ok && len(c.Options) > 0 && c.Options[0] == "Ocean"
	}
	radio := p.must("the alignment choice", func(o core.CanvasObject) bool {
		r, ok := o.(*widget.RadioGroup)
		return ok && len(r.Options) == 3 && r.Options[0] == "Start"
	})
	for _, rtl := range []bool{false, true} {
		if rtl {
			p.click("the direction switch", button("Toggle the app's direction (RTL / auto)"))
		}
		dir := "ltr"
		if rtl {
			dir = "rtl"
		}
		for _, a := range []string{"Start", "Center", "End"} {
			bellui.DoAndWait(func() { radio.obj.(*widget.RadioGroup).SetSelected(a) })
			p.settle(250 * time.Millisecond)
			p.reveal("the chip rows", group)
			p.shot("rtl07-chips-"+strings.ToLower(a)+"-"+dir, fmt.Sprintf("RTL-07: chip rows at %s, %s; the radio row above them mirrored too (#1392)", a, dir))
		}
		// The alignment choice is a horizontal RadioGroup: its options run
		// right-to-left too (#1392).
		text := func(t string) func(core.CanvasObject) bool {
			return func(o core.CanvasObject) bool { c, ok := o.(*canvas.Text); return ok && c.Text == t }
		}
		st := p.must("the Start option", text("Start"))
		en := p.must("the End option", text("End"))
		if rtl {
			p.expect(st.pos.X > en.pos.X, "RTL-07 (#1392): right-to-left, the radio options run from the right", fmt.Sprintf("Start x=%.0f End x=%.0f", st.pos.X, en.pos.X))
		} else {
			p.expect(st.pos.X < en.pos.X, "RTL-07: left-to-right, the radio options run from the left", fmt.Sprintf("Start x=%.0f End x=%.0f", st.pos.X, en.pos.X))
		}
		one := p.reveal("flow button 1", button("1"))
		two := p.must("flow button 2", button("2"))
		first := p.must("flex 1", button("1 first"))
		last := p.must("flex 3", button("3 last"))
		if rtl {
			p.expect(one.pos.X > two.pos.X, "RTL-07: right-to-left, the flow's 1 is right of 2", fmt.Sprintf("x1=%.0f x2=%.0f", one.pos.X, two.pos.X))
			p.expect(first.pos.X > last.pos.X, "RTL-07: right-to-left, the flex row starts at the right", fmt.Sprintf("first=%.0f last=%.0f", first.pos.X, last.pos.X))
		} else {
			p.expect(one.pos.X < two.pos.X, "RTL-07: left-to-right, the flow's 1 is left of 2", fmt.Sprintf("x1=%.0f x2=%.0f", one.pos.X, two.pos.X))
			p.expect(first.pos.X < last.pos.X, "RTL-07: left-to-right, the flex row starts at the left", fmt.Sprintf("first=%.0f last=%.0f", first.pos.X, last.pos.X))
		}
		p.shot("rtl07-layouts-"+dir, "RTL-07: the row-wrap flow, flex row and CrossStart flex column, "+dir)
	}
	p.click("the direction switch", button("Toggle the app's direction (RTL / auto)"))
	bellui.DoAndWait(func() { radio.obj.(*widget.RadioGroup).SetSelected("Start") })
}

// strokes measures the ring's orange stroke across its middle row at both
// sizes, in captured pixels.
func (p *probe) strokes() {
	p.selectTab("Chips & layout")
	f := p.reveal("the stroke card", card("SVG strokes scale"))
	var img image.Image
	bellui.DoAndWait(func() { img = p.w.Canvas().Capture() })
	var canvasW float32
	bellui.DoAndWait(func() { canvasW = p.w.Canvas().Size().Width })
	px := float32(img.Bounds().Dx()) / canvasW // pixels per dp
	p.logf("- capture %d px wide for %.0f dp: %.2f px per dp\n", img.Bounds().Dx(), canvasW, px)

	var widths []float32
	for _, size := range []float32{40, 240} {
		sz := size
		img2, ok := p.find(func(o core.CanvasObject) bool {
			i, isImg := o.(*canvas.Image)
			return isImg && i.Resource != nil && i.Resource.Name() == "stroke.svg" && i.Size().Width == sz
		})
		if !ok {
			p.expect(false, fmt.Sprintf("SM-08: the %v dp ring is on screen", size), "not found")
			return
		}
		cy := int((img2.pos.Y + sz/2) * px)
		x0, x1 := int(img2.pos.X*px), int((img2.pos.X+sz)*px)
		run, best := 0, 0
		for x := x0; x < x1; x++ {
			r, g, b, _ := img.At(x, cy).RGBA()
			if r>>8 > 170 && g>>8 > 40 && g>>8 < 140 && b>>8 < 90 {
				run++
				if run > best {
					best = run
				}
			} else if best > 0 && run > 0 {
				break // the first stroke, the ring's left side, is enough
			} else {
				run = 0
			}
		}
		want := 7.0 / 120 * sz * px
		widths = append(widths, float32(best))
		p.expect(float32(best) >= want*0.7 && float32(best) <= want*1.3+1,
			fmt.Sprintf("SM-08 (#1292): the %v dp ring's stroke is about %.1f px (7/120 of its size)", size, want), fmt.Sprintf("%d px", best))
	}
	if len(widths) == 2 && widths[0] > 0 {
		p.logf("- stroke ratio large/small: %.2f (the sizes' ratio is 6)\n", widths[1]/widths[0])
	}
	_ = f
	p.shot("sm08-strokes", "SM-08: the small ring is the large one shrunk, its stroke not filling it in")
}

func (p *probe) swipes() {
	p.selectTab("Swipes")
	cardMatch := func(o core.CanvasObject) bool {
		s, ok := o.(*widget.SwipeArea)
		if !ok {
			return false
		}
		c, isCard := s.Content.(*widget.Card)
		return ok && isCard && c.Title == "Swipe card"
	}
	f := p.reveal("the swipe card", cardMatch)
	x, y := centre(f)
	p.drag(x, y, -260, 0)
	got := p.text("card status", labelPrefix("Card:"))
	p.expect(got == "Card: swiped left", "TK-13: a long drag left throws the card", got)

	modes := func(o core.CanvasObject) bool { s, ok := o.(*widget.SwipeArea); return ok && s.Directions != nil }
	f = p.reveal("the directions card", modes)
	x, y = centre(f)
	var before core.CanvasObject
	bellui.DoAndWait(func() { before = f.obj.(*widget.SwipeArea).Content })
	p.drag(x, y, 260, 0)
	got = p.text("directions status", labelPrefix("Directions card:"))
	var after core.CanvasObject
	bellui.DoAndWait(func() { after = f.obj.(*widget.SwipeArea).Content })
	p.expect(got == "Directions card: swiped right (1 so far)" && after == before,
		"TK-13 (#1103): right springs the card back, reported once, no new card", got)
	p.drag(x, y, 0, -120)
	p.drag(x, y, 0, 120)
	got = p.text("directions status", labelPrefix("Directions card:"))
	p.expect(got == "Directions card: swiped right (1 so far)", "TK-13 (#1103): up and down report nothing", got)
	p.drag(x, y, -260, 0)
	got = p.text("directions status", labelPrefix("Directions card:"))
	bellui.DoAndWait(func() { after = f.obj.(*widget.SwipeArea).Content })
	p.expect(got == "Directions card: swiped left (2 so far)" && after != before, "TK-13 (#1103): left throws it and a new card comes", got)
}

func (p *probe) keys() {
	p.selectTab("Touch & keys")
	target := card("Key target")
	p.click("the key target", target)
	label := labelPrefix("Key target:")
	got := p.text("key target", label)
	p.expect(got == "Key target: focused, press a key", "TK-06: a click focuses the key target", got)
	p.press(keyReturn)
	got = p.text("key target", label)
	p.expect(got == "Key target: key Return", "TK-06: Return reaches it", got)
	p.press(keyLeft)
	got = p.text("key target", label)
	p.expect(got == "Key target: key Left", "TK-06: the left arrow reaches it", got)
	p.press(keyA)
	got = p.text("key target", label)
	p.expect(got == "Key target: rune 'a'", "TK-06: a letter arrives as a rune", got)
}

func (p *probe) locales() {
	p.selectTab("Locale")
	info := labelPrefix("Locale:")
	for _, c := range []struct {
		loc  string
		want []string
		id   string
	}{
		{"en-US", []string{"Cancel", "03/09/2026", "1,234,567.89", "red, green, and blue"}, "I18N-01"},
		{"de-DE", []string{"Abbrechen", "09.03.2026", "1.234.567,89", "red, green und blue"}, "I18N-01"},
		{"ja-JP", []string{"キャンセル", "2026/03/09", "red、green、blue"}, "I18N-01"},
		{"fa-IR", []string{"Right-to-left: true", "Hello Ada"}, "I18N-03"},
		{"ar-EG", []string{"Right-to-left: true", "مرحبا Ada"}, "I18N-03"},
		{"sr-RS", []string{"\"Cancel\": Cancel", "09.03.2026"}, "I18N-04"},
		{"he-IL", []string{"Right-to-left: true", "\"Cancel\": Cancel"}, "I18N-04"},
		{"pt-BR", []string{"Cancelar", "red, green e blue"}, "I18N-04"},
	} {
		p.click(c.loc, button(c.loc))
		got := p.text("locale info", info)
		missing := []string{}
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				missing = append(missing, w)
			}
		}
		p.expect(len(missing) == 0, fmt.Sprintf("%s: %s shows %s", c.id, c.loc, strings.Join(c.want, " · ")),
			strings.ReplaceAll(got, "\n", " | "))
		if c.loc == "fa-IR" {
			p.shot("i18n03-fa-ir", "I18N-03: the whole window right-to-left, English greeting")
		}
	}
	p.click("System", button("System"))
}

func (p *probe) drawer() {
	p.selectTab("Drawer")
	state := labelPrefix("Drawer:")
	p.click("Show", button("Show"))
	p.settle(400 * time.Millisecond)
	got := p.text("drawer state", state)
	p.expect(got == "Drawer: open. OnOpenChanged has fired 1 times.", "WG-05: Show opens it, reported once", got)
	p.shot("wg05-start-open", "WG-05: the drawer at the start (left) edge, the rest dimmed")
	var w, h float32
	bellui.DoAndWait(func() { s := p.w.Canvas().Size(); w, h = s.Width, s.Height })
	p.clickAt(w-30, h/2)
	p.settle(400 * time.Millisecond)
	got = p.text("drawer state", state)
	p.expect(got == "Drawer: closed. OnOpenChanged has fired 2 times.", "WG-05: a click on the dimmed area closes it", got)
	p.click("Show", button("Show"))
	p.settle(400 * time.Millisecond)
	p.press(keyEscape)
	p.settle(400 * time.Millisecond)
	got = p.text("drawer state", state)
	p.expect(got == "Drawer: closed. OnOpenChanged has fired 4 times.", "WG-05: Escape closes it", got)

	side := p.must("the edge choice", func(o core.CanvasObject) bool {
		r, ok := o.(*widget.RadioGroup)
		return ok && len(r.Options) == 2 && r.Options[0] == "Start edge"
	})
	bellui.DoAndWait(func() { side.obj.(*widget.RadioGroup).SetSelected("End edge") })
	p.click("Show", button("Show"))
	p.settle(400 * time.Millisecond)
	p.shot("wg05-end-open", "WG-05: the drawer at the end (right) edge")
	p.press(keyEscape)
	p.settle(400 * time.Millisecond)
	bellui.DoAndWait(func() { side.obj.(*widget.RadioGroup).SetSelected("Start edge") })
}

func (p *probe) override() {
	p.selectTab("Theme override")
	for _, v := range []bellui.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		p.variant(v)
		name := map[bellui.ThemeVariant]string{theme.VariantLight: "light", theme.VariantDark: "dark"}[v]
		p.shot("sm06-list-"+name, "SM-06: list rows yellow, separators and the tab accent orange, "+name+" app theme")
	}
	p.variant(theme.VariantLight)
}

// windowSize checks the window keeps its size when the tabs with wrapping
// content open, and can be made smaller again.
func (p *probe) windowSize() {
	var start, after geo.Size
	bellui.DoAndWait(func() { start = p.w.Canvas().Size() })
	for _, t := range []string{"Chips & layout", "Text"} {
		p.selectTab(t)
	}
	p.settle(300 * time.Millisecond)
	bellui.DoAndWait(func() { after = p.w.Canvas().Size() })
	p.expect(after.Height <= start.Height+1, "SM-07 (#1349): opening the wrapping tabs does not make the window taller",
		fmt.Sprintf("%.0fx%.0f then %.0fx%.0f", start.Width, start.Height, after.Width, after.Height))
	// The smallest the window goes before and after widening: #1346 made
	// widening raise the floor to the widest row.
	smallest := func() geo.Size {
		bellui.DoAndWait(func() { p.w.Resize(geo.NewSize(300, 300)) })
		p.settle(500 * time.Millisecond)
		var s geo.Size
		bellui.DoAndWait(func() { s = p.w.Canvas().Size() })
		return s
	}
	floor := smallest()
	bellui.DoAndWait(func() { p.w.Resize(geo.NewSize(start.Width+300, start.Height)) })
	p.settle(500 * time.Millisecond)
	var wide geo.Size
	bellui.DoAndWait(func() { wide = p.w.Canvas().Size() })
	small := smallest()
	p.expect(wide.Width > start.Width && small.Width <= floor.Width+1 && small.Height <= floor.Height+1,
		"SM-07 (#1346): after widening, the window goes back down to the same smallest size",
		fmt.Sprintf("smallest %.0fx%.0f, widened to %.0f, then smallest %.0fx%.0f", floor.Width, floor.Height, wide.Width, small.Width, small.Height))
	p.shot("sm07-small-text", "SM-07: the Text tab at its smallest; the flow re-wrapped to fit")
	bellui.DoAndWait(func() { p.w.Resize(start) })
	p.settle(400 * time.Millisecond)
}

func (p *probe) fast() {
	p.selectTab("Locale")
	locs := []string{"en-US", "de-DE", "ar-EG", "ja-JP", "fa-IR", "pt-BR"}
	pos := map[string][2]float32{}
	for _, l := range locs {
		x, y := centre(p.reveal(l, button(l)))
		pos[l] = [2]float32{x, y}
	}
	startT := time.Now()
	for i := 0; i < 60; i++ {
		c := pos[locs[i%len(locs)]]
		p.in.mouse(mouseMove, c[0]*p.scale, c[1]*p.scale)
		p.in.mouse(mouseDown, c[0]*p.scale, c[1]*p.scale)
		p.in.mouse(mouseUp, c[0]*p.scale, c[1]*p.scale)
		time.Sleep(15 * time.Millisecond)
	}
	done := make(chan struct{})
	go func() { bellui.DoAndWait(func() {}); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		p.expect(false, "ED-05: the app keeps up with 60 clicks in about a second", "the main loop did not answer in 10 s")
		return
	}
	p.settle(800 * time.Millisecond)
	last := locs[(60-1)%len(locs)]
	got := p.text("locale info", labelPrefix("Locale:"))
	p.expect(strings.HasPrefix(got, "Locale: "+last[:2]), fmt.Sprintf("ED-05: after 60 fast clicks it ends on the last one, %s", last),
		fmt.Sprintf("%s in %v", strings.SplitN(got, "\n", 2)[0], time.Since(startT).Round(time.Millisecond)))
	p.click("System", button("System"))
	_ = lang.CurrentLocale
}
