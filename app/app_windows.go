//go:build !ci && !android && !ios && !wasm && !tinygo

package app

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/belluikit/bellui"
	internalapp "github.com/belluikit/bellui/internal/app"
)

// notificationTemplate reads its values from environment variables rather than
// interpolating them into the script body. PowerShell does not re-evaluate the
// contents of an environment variable, so attacker-influenced notification text
// (e.g. "$(Start-Process calc.exe)") is treated as literal data instead of an
// expression — closing the command-injection hole (pre-rebrand tracker #287).
const notificationTemplate = `$title = $env:BELLUI_NOTIFY_TITLE
$content = $env:BELLUI_NOTIFY_CONTENT
$iconPath = "file:///" + $env:BELLUI_NOTIFY_ICON
$appID = $env:BELLUI_NOTIFY_APPID
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastImageAndText02)
$toastXml = [xml] $template.GetXml()
$toastXml.GetElementsByTagName("text")[0].AppendChild($toastXml.CreateTextNode($title)) > $null
$toastXml.GetElementsByTagName("text")[1].AppendChild($toastXml.CreateTextNode($content)) > $null
$toastXml.GetElementsByTagName("image")[0].SetAttribute("src", $iconPath) > $null
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($toastXml.OuterXml)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($appID).Show($toast);`

func (a *belluiApp) OpenURL(url *url.URL) error {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url.String())
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

var scriptNum = 0

func (a *belluiApp) SendNotification(n *bellui.Notification) {
	iconFilePath := a.cachedIconPath()
	appID := a.UniqueID()
	if appID == "" || strings.Index(appID, "missing-id") == 0 {
		appID = a.Metadata().Name
	}

	// Values are passed to the script via the environment so that PowerShell
	// treats them as inert data, never as code (pre-rebrand tracker #287).
	env := []string{
		"BELLUI_NOTIFY_TITLE=" + n.Title,
		"BELLUI_NOTIFY_CONTENT=" + n.Content,
		"BELLUI_NOTIFY_ICON=" + iconFilePath,
		"BELLUI_NOTIFY_APPID=" + appID,
	}
	go runScript("notify", notificationTemplate, env)
}

// badgeDriver is implemented by the glfw driver on Windows to draw a taskbar
// overlay-icon count badge (ITaskbarList3::SetOverlayIcon). Asserted so the app
// package stays free of the COM/GDI backend and non-glfw drivers no-op.
type badgeDriver interface {
	SetBadge(count int)
}

// SetBadge draws a count badge on the app's taskbar button(s) via the Windows
// overlay-icon affordance (pre-rebrand tracker #706), part of the [bellui.BadgeApp] optional interface.
// A count <= 0 clears it. Best-effort: unsupported drivers no-op.
func (a *belluiApp) SetBadge(count int) {
	if d, ok := a.Driver().(badgeDriver); ok {
		d.SetBadge(count)
	}
}

func runScript(name, script string, env []string) {
	scriptNum++
	appID := bellui.CurrentApp().UniqueID()
	fileName := fmt.Sprintf("bellui-%s-%s-%d.ps1", appID, name, scriptNum)

	tmpFilePath := filepath.Join(os.TempDir(), fileName)
	err := os.WriteFile(tmpFilePath, []byte(script), 0o600)
	if err != nil {
		bellui.LogError("Could not write script to show notification", err)
		return
	}
	defer os.Remove(tmpFilePath)

	launch := "(Get-Content -Encoding UTF8 -Path " + tmpFilePath + " -Raw) | Invoke-Expression"
	cmd := exec.Command("PowerShell", "-ExecutionPolicy", "Bypass", launch)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	err = cmd.Run()
	if err != nil {
		bellui.LogError("Failed to launch windows notify script", err)
	}
}

func watchTheme(s *settings) {
	go internalapp.WatchTheme(func() {
		bellui.Do(s.setupTheme)
	})
}

func (a *belluiApp) registerRepositories() {
	// no-op
}
