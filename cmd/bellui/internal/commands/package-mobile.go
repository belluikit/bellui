package commands

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"log"
	"os"
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
	p.warnOwnManifestExtractsLibs()
	if err := mobile.SetAndroidMinSDK(p.androidMinSDKOrDefault()); err != nil {
		return err
	}
	return mobile.RunNewBuild(arch, p.AppID, p.icon, p.Name, p.AppVersion, p.AppBuild, p.release, p.distribution,
		"", "", tags, iconFG, iconBG, iconMono, p.URLSchemes, p.fileMimeTypes(), nil, p.AssociatedDomains, p.iconCredit())
}

// androidMinSDKOrDefault is --android-min-sdk, or the default when the
// Packager was built without the flag (0).
func (p *Packager) androidMinSDKOrDefault() int {
	if p.androidMinSDK == 0 {
		return mobile.DefaultAndroidMinSDK
	}
	return p.androidMinSDK
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

// warnOwnManifestExtractsLibs says when an app's own manifest leaves out
// android:extractNativeLibs. The generated manifest sets it false, so Android
// runs the library from the APK; an app's own manifest is used unchanged, and
// without the attribute Android keeps a second, extracted copy of the
// library, about doubling the installed size (#889). A manifest that sets it
// either way has chosen, and gets no warning.
func (p *Packager) warnOwnManifestExtractsLibs() {
	data, err := os.ReadFile(filepath.Join(p.srcDir, "AndroidManifest.xml"))
	if err != nil || declaresExtractNativeLibs(data) {
		return
	}
	log.Println(`The app's own AndroidManifest.xml does not set android:extractNativeLibs, so Android ` +
		`keeps a second copy of the native library and the installed app takes about twice the space. ` +
		`Add android:extractNativeLibs="false" to <application>, unless a bundled library needs its .so ` +
		`as a file on disk.`)
}

// declaresExtractNativeLibs reports whether manifest's <application> sets
// android:extractNativeLibs. A manifest that does not parse is left to the
// build, which reports it.
func declaresExtractNativeLibs(manifest []byte) bool {
	d := xml.NewDecoder(bytes.NewReader(manifest))
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return false
		}
		if err != nil {
			return true
		}
		el, ok := tok.(xml.StartElement)
		if !ok || el.Name.Local != "application" {
			continue
		}
		for _, a := range el.Attr {
			if a.Name.Local == "extractNativeLibs" {
				return true
			}
		}
		return false
	}
}

func (p *Packager) packageIOS(target string, tags []string) error {
	// The app icon is compiled by Xcode inside this build, so it is signed
	// with the rest of the bundle (#677).
	return mobile.RunNewBuild(target, p.AppID, p.icon, p.Name, p.AppVersion, p.AppBuild, p.release, p.distribution,
		p.certificate, p.profile, tags, "", "", "", p.URLSchemes, p.fileMimeTypes(), p.IOSUsageDescriptions, p.AssociatedDomains, p.iconCredit())
}
