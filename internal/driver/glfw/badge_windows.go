//go:build windows && !wasm

package glfw

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/driver/software"
	"github.com/belluikit/bellui/internal/async"
)

// Windows taskbar badge backend for bellui.BadgeApp (#706/#517). App.SetBadge on
// Windows draws a count overlay on the app's taskbar button(s) via
// ITaskbarList3::SetOverlayIcon — the platform's badge affordance. The count
// glyph is rendered through bellui's own software renderer (so it uses the theme
// font, no GDI text drawing) and converted to an HICON with cgo-free GDI.

const vtblTBSetOverlayIcon = 18 // ITaskbarList3::SetOverlayIcon(hwnd, hicon, desc)

var (
	modgdi32  = windows.NewLazySystemDLL("gdi32.dll")
	moduser32 = windows.NewLazySystemDLL("user32.dll")

	procCreateDIBSection   = modgdi32.NewProc("CreateDIBSection")
	procCreateBitmap       = modgdi32.NewProc("CreateBitmap")
	procDeleteObject       = modgdi32.NewProc("DeleteObject")
	procGetDC              = moduser32.NewProc("GetDC")
	procReleaseDC          = moduser32.NewProc("ReleaseDC")
	procCreateIconIndirect = moduser32.NewProc("CreateIconIndirect")
	procDestroyIcon        = moduser32.NewProc("DestroyIcon")
)

// bitmapInfoHeader mirrors the Win32 BITMAPINFOHEADER (40 bytes).
type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// iconInfo mirrors the Win32 ICONINFO.
type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  uintptr
	hbmColor uintptr
}

// SetBadge implements the badge half of bellui.BadgeApp on Windows (called via the
// badgeDriver seam in app_windows.go). A count <= 0 clears the overlay. Runs on
// the main goroutine, where the COM apartment and window HWNDs live.
func (d *gLDriver) SetBadge(count int) {
	async.EnsureMain(func() {
		tb := taskbar()
		if tb == nil {
			return
		}

		var hicon uintptr
		if count > 0 {
			hicon = makeBadgeIcon(renderBadgeImage(count))
		}

		descPtr, _ := windows.UTF16PtrFromString("badge")
		for _, w := range d.windowList() {
			ww, ok := w.(*window)
			if !ok {
				continue
			}
			view := ww.view()
			if view == nil {
				continue
			}
			hwnd := uintptr(unsafe.Pointer(view.GetWin32Window()))
			if hwnd == 0 {
				continue
			}
			// A NULL hicon clears the overlay; the taskbar copies the icon, so it
			// is safe to destroy after setting it on every window.
			hr := tbCall(tb, vtblTBSetOverlayIcon, hwnd, hicon, uintptr(unsafe.Pointer(descPtr)))
			taskbarLog("SetOverlayIcon", hr)
		}

		if hicon != 0 {
			procDestroyIcon.Call(hicon)
		}
	})
}

// renderBadgeImage draws a red count badge: a filled anti-aliased circle with the
// number (rendered through bellui's font stack) centered on it. Counts over 99
// collapse to "99+".
func renderBadgeImage(count int) *image.NRGBA {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	red := color.NRGBA{R: 0xD3, G: 0x2F, B: 0x2F, A: 0xFF}
	c, r := float64(size)/2, float64(size)/2-0.5
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			d := math.Hypot(float64(x)+0.5-c, float64(y)+0.5-c)
			var a float64
			switch {
			case d <= r-0.5:
				a = 1
			case d >= r+0.5:
				a = 0
			default:
				a = r + 0.5 - d
			}
			if a > 0 {
				px := red
				px.A = uint8(float64(red.A) * a)
				img.SetNRGBA(x, y, px)
			}
		}
	}

	label := strconv.Itoa(count)
	if count > 99 {
		label = "99+"
	}
	txt := canvas.NewText(label, color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF})
	txt.TextStyle = bellui.TextStyle{Bold: true}
	txt.TextSize = 18
	if len(label) >= 3 {
		txt.TextSize = 13
	}
	txt.Resize(txt.MinSize())
	textImg := software.Render(txt, bellui.CurrentApp().Settings().Theme())
	tb := textImg.Bounds()
	offset := image.Pt((size-tb.Dx())/2, (size-tb.Dy())/2)
	draw.Draw(img, tb.Add(offset.Sub(tb.Min)), textImg, tb.Min, draw.Over)

	return img
}

// makeBadgeIcon converts an NRGBA badge image into an HICON via a 32-bit top-down
// DIB (premultiplied BGRA) plus a monochrome mask, cgo-free. Returns 0 on failure.
func makeBadgeIcon(img *image.NRGBA) uintptr {
	w := int32(img.Bounds().Dx())
	h := int32(img.Bounds().Dy())

	bmi := bitmapInfoHeader{Size: 40, Width: w, Height: -h, Planes: 1, BitCount: 32}
	hdc, _, _ := procGetDC.Call(0)
	var bits unsafe.Pointer
	hbmColor, _, _ := procCreateDIBSection.Call(hdc, uintptr(unsafe.Pointer(&bmi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	procReleaseDC.Call(0, hdc)
	if hbmColor == 0 || bits == nil {
		return 0
	}

	// NRGBA (straight-alpha RGBA) -> premultiplied BGRA, which is what the taskbar
	// alpha-blends the overlay with.
	dst := unsafe.Slice((*byte)(bits), int(w)*int(h)*4)
	src := img.Pix
	for i := 0; i+3 < len(src) && i+3 < len(dst); i += 4 {
		a := uint16(src[i+3])
		dst[i] = byte(uint16(src[i+2]) * a / 255)   // B
		dst[i+1] = byte(uint16(src[i+1]) * a / 255) // G
		dst[i+2] = byte(uint16(src[i]) * a / 255)   // R
		dst[i+3] = src[i+3]                         // A
	}

	hbmMask, _, _ := procCreateBitmap.Call(uintptr(w), uintptr(h), 1, 1, 0)
	ii := iconInfo{fIcon: 1, hbmMask: hbmMask, hbmColor: hbmColor}
	hicon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))

	procDeleteObject.Call(hbmColor)
	procDeleteObject.Call(hbmMask)
	return hicon
}
