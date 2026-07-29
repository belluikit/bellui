//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// captureScreen reads the composited desktop with GDI, driven from PowerShell —
// the same shape as the UIA clients under hack/a11yprobe/windows, and for the
// same reason: it needs no build step and no extra dependency in a harness that
// is only ever run by hand.
//
// Graphics.CopyFromScreen is a BitBlt from the screen device context, so what
// it returns is what the desktop compositor put on screen, including the
// WebView2 child window that bellui's own Canvas.Capture cannot see. That
// difference is the measurement.
//
// Run it from an interactive session. A window in the session-0 window station
// an SSH command lands in is not on any desktop, so the capture is of nothing —
// the same trap #487's UIA verification hit (schtasks /IT is the way out).
func captureScreen(r screenshotRegion, path string) error {
	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
$bmp = New-Object System.Drawing.Bitmap %d, %d
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen(%d, %d, 0, 0, $bmp.Size)
$bmp.Save(%q, [System.Drawing.Imaging.ImageFormat]::Png)
$g.Dispose()
$bmp.Dispose()
`, r.w, r.h, r.x, r.y, path)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("screen capture: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}
