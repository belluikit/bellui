package commands

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"

	"github.com/belluikit/bellui/cmd/bellui/internal/mobile"
)

func (p *Packager) packageAndroid(arch string, tags []string) error {
	iconFG, iconBG, iconMono := "", "", ""
	if p.appData.AdaptiveIcon != nil {
		iconFG = p.appData.AdaptiveIcon.Foreground
		iconBG = p.appData.AdaptiveIcon.Background
		iconMono = p.appData.AdaptiveIcon.Monochrome
	}

	p.warnOwnManifestDropsCredit()
	return mobile.RunNewBuild(arch, p.AppID, p.icon, p.Name, p.AppVersion, p.AppBuild, p.release, p.distribution,
		"", "", tags, iconFG, iconBG, iconMono, p.URLSchemes, p.fileMimeTypes(), nil, p.AssociatedDomains, p.iconCredit())
}

// warnOwnManifestDropsCredit says when the default icon's credit cannot reach
// the package: it goes into the AndroidManifest.xml bellui generates, and an
// app with its own manifest is built from that one unchanged (#362).
func (p *Packager) warnOwnManifestDropsCredit() {
	if p.iconCredit() == "" || !util.Exists(filepath.Join(p.srcDir, "AndroidManifest.xml")) {
		return
	}
	log.Println("The app's own AndroidManifest.xml is used, so the default icon's credit is not added to it. " +
		"Add an Icon.png, or credit the icon in the manifest: " + p.iconCredit())
}

func (p *Packager) packageIOS(target string, tags []string) error {
	// The app icon is compiled by Xcode inside this build, so it is signed
	// with the rest of the bundle (#677).
	return mobile.RunNewBuild(target, p.AppID, p.icon, p.Name, p.AppVersion, p.AppBuild, p.release, p.distribution,
		p.certificate, p.profile, tags, "", "", "", p.URLSchemes, p.fileMimeTypes(), p.IOSUsageDescriptions, p.AssociatedDomains, p.iconCredit())
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
