package commands

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/cmd/bellui/internal/mobile"
	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
)

func (p *Packager) packageAndroid(arch string, tags []string) error {
	iconFG, iconBG, iconMono := "", "", ""
	if p.appData.AdaptiveIcon != nil {
		iconFG = p.appData.AdaptiveIcon.Foreground
		iconBG = p.appData.AdaptiveIcon.Background
		iconMono = p.appData.AdaptiveIcon.Monochrome
	}

	return mobile.RunNewBuild(arch, p.AppID, p.icon, p.Name, p.AppVersion, p.AppBuild, p.release, p.distribution,
		"", "", tags, iconFG, iconBG, iconMono, p.URLSchemes, p.fileMimeTypes(), nil, p.AssociatedDomains)
}

func (p *Packager) packageIOS(target string, tags []string) error {
	err := mobile.RunNewBuild(target, p.AppID, p.icon, p.Name, p.AppVersion, p.AppBuild, p.release, p.distribution,
		p.certificate, p.profile, tags, "", "", "", p.URLSchemes, p.fileMimeTypes(), p.IOSUsageDescriptions, p.AssociatedDomains)
	if err != nil {
		return err
	}

	assetDir := util.EnsureSubDir(p.dir, "Images.xcassets")
	defer os.RemoveAll(assetDir)
	err = os.WriteFile(filepath.Join(assetDir, "Contents.json"), []byte(`{
  "info" : {
    "author" : "xcode",
    "version" : 1
  }
}`), 0o644)
	if err != nil {
		bellui.LogError("Content err", err)
	}

	iconDir := util.EnsureSubDir(assetDir, "AppIcon.appiconset")
	contentFile, _ := os.Create(filepath.Join(iconDir, "Contents.json"))

	err = templates.XCAssetsDarwin.Execute(contentFile, nil)
	if err != nil {
		return fmt.Errorf("failed to write xcassets content template: %w", err)
	}

	if err = copyResizeIcon(1024, iconDir, p.icon); err != nil {
		return err
	}
	if err = copyResizeIcon(180, iconDir, p.icon); err != nil {
		return err
	}
	if err = copyResizeIcon(120, iconDir, p.icon); err != nil {
		return err
	}
	if err = copyResizeIcon(76, iconDir, p.icon); err != nil {
		return err
	}
	if err = copyResizeIcon(152, iconDir, p.icon); err != nil {
		return err
	}

	appDir := filepath.Join(p.dir, mobile.AppOutputName(p.os, p.Name, p.distribution))
	return runCmdCaptureOutput("xcrun", iosActoolArgs(appDir)...)
}

// iosActoolArgs compiles the app icon asset catalog into appDir, for the same
// deployment target as the binary and the Xcode project.
func iosActoolArgs(appDir string) []string {
	return []string{"actool", "Images.xcassets", "--compile", appDir, "--platform",
		"iphoneos", "--target-device", "iphone", "--minimum-deployment-target", mobile.IOSDeploymentTarget(), "--app-icon", "AppIcon",
		"--output-format", "human-readable-text", "--output-partial-info-plist", "/dev/null"}
}

func copyResizeIcon(size int, dir, source string) error {
	strSize := strconv.Itoa(size)
	path := dir + "/Icon_" + strSize + ".png"
	return runCmdCaptureOutput("sips", "-o", path, "-Z", strSize, source)
}

// runCmdCaptureOutput is a exec.Command wrapper that offers better error messages from stdout and stderr.
func runCmdCaptureOutput(name string, args ...string) error {
	var (
		outbuf = &bytes.Buffer{}
		errbuf = &bytes.Buffer{}
	)
	cmd := exec.Command(name, args...)
	cmd.Stdout = outbuf
	cmd.Stderr = errbuf
	err := cmd.Run()
	if err != nil {
		outstr := outbuf.String()
		errstr := errbuf.String()
		if outstr != "" {
			err = fmt.Errorf(outbuf.String()+": %w", err)
		}
		if errstr != "" {
			err = fmt.Errorf(outbuf.String()+": %w", err)
		}
		return err
	}
	return nil
}
