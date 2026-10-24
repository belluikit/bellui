package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/cmd/bellui/internal/mobile"
	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
	realUtil "github.com/belluikit/bellui/cmd/bellui/internal/util"

	"github.com/urfave/cli/v2"
)

var macAppStoreCategories = []string{
	"business", "developer-tools", "education", "entertainment", "finance", "games", "action-games",
	"adventure-games", "arcade-games", "board-games", "card-games", "casino-games", "dice-games",
	"educational-games", "family-games", "kids-games", "music-games", "puzzle-games", "racing-games",
	"role-playing-games", "simulation-games", "sports-games", "strategy-games", "trivia-games", "word-games",
	"graphics-design", "healthcare-fitness", "lifestyle", "medical", "music", "news", "photography",
	"productivity", "reference", "social-networking", "sports", "travel", "utilities", "video", "weather",
}

// Release returns the cli command for bundling release builds of bellui applications
func Release() *cli.Command {
	r := NewReleaser()

	return &cli.Command{
		Name:    "release",
		Aliases: []string{"r"},
		Usage:   "Prepares an application for public distribution",
		Flags: []cli.Flag{
			stringFlags["target"](&r.os),
			stringFlags["keystore"](&r.keyStore),
			stringFlags["keystore-pass"](&r.keyStorePass),
			stringFlags["key-name"](&r.keyName),
			stringFlags["key-pass"](&r.keyPass),
			stringFlags["name"](&r.Name),
			stringFlags["tags"](&r.tags),
			stringFlags["app-version"](&r.AppVersion),
			intFlags["app-build"](&r.AppBuild),
			stringFlags["app-id"](&r.AppID),
			stringFlags["certificate"](&r.certificate),
			stringFlags["profile"](&r.profile),
			stringFlags["developer"](&r.developer),
			stringFlags["password"](&r.password),
			stringFlags["category"](&r.category),
			stringFlags["icon"](&r.icon),
			boolFlags["use-raw-icon"](&r.rawIcon),
			genericFlags["metadata"](&r.customMetadata),
		},
		Action: r.releaseAction,
	}
}

// Releaser adapts app packages form distribution.
type Releaser struct {
	Packager

	keyStore     string
	keyStorePass string
	keyName      string
	keyPass      string
	developer    string
	password     string
}

// NewReleaser returns a command that can handle the packaging a GUI apps for release from local Bellui source code.
func NewReleaser() *Releaser {
	r := &Releaser{}
	r.appData = &appData{}
	return r
}

func (r *Releaser) releaseAction(_ *cli.Context) error {
	r.Packager.distribution = true
	r.Packager.release = true

	if err := r.validate(); err != nil {
		return err
	}

	if err := r.beforePackage(); err != nil {
		return err
	}

	if err := r.Packager.packageWithoutValidate(); err != nil {
		return err
	}

	if err := r.afterPackage(); err != nil {
		return err
	}

	return nil
}

func (r *Releaser) afterPackage() error {
	if util.IsAndroid(r.os) {
		target := mobile.AppOutputName(r.os, r.Packager.Name, r.distribution)
		apk := filepath.Join(r.dir, target)
		if err := r.zipAlign(apk); err != nil {
			return err
		}
		return r.signAndroid(apk)
	}
	if r.os == "darwin" {
		return r.packageMacOSRelease()
	}
	if r.os == "ios" {
		return r.packageIOSRelease()
	}
	if r.os == "windows" {
		outName := r.Name + ".appx"
		if pos := strings.LastIndex(r.Name, ".exe"); pos > 0 {
			outName = r.Name[:pos] + ".appx"
		}

		outPath := filepath.Join(r.dir, outName)
		os.Remove(outPath) // MakeAppx will hang if the file exists... ignore result
		if err := r.packageWindowsRelease(outPath); err != nil {
			return err
		}
		return r.signWindows(outPath)
	}
	return nil
}

func (r *Releaser) beforePackage() error {
	if util.IsAndroid(r.os) {
		if err := util.RequireAndroidSDK(); err != nil {
			return err
		}
	}

	return nil
}

func (r *Releaser) nameFromCertInfo(info string) string {
	// format should be "CN=Company, O=Company, L=City, S=State, C=Country"
	parts := strings.Split(info, ",")
	cn := parts[0]
	pos := strings.Index(strings.ToUpper(cn), "CN=")
	if pos == -1 {
		return cn // not what we were expecting, but should be OK
	}

	return cn[pos+3:]
}

func (r *Releaser) packageIOSRelease() error {
	team, err := mobile.DetectIOSTeamID(r.certificate)
	if err != nil {
		return errors.New("failed to determine team ID")
	}

	payload := filepath.Join(r.dir, "Payload")
	_ = os.Mkdir(payload, 0o750)
	defer os.RemoveAll(payload)
	appName := mobile.AppOutputName(r.os, r.Name, r.distribution)
	payloadAppDir := filepath.Join(payload, appName)
	if err := os.Rename(filepath.Join(r.dir, appName), payloadAppDir); err != nil {
		return err
	}

	// Re-sign with everything `bellui package` signed in (Associated Domains,
	// ios/Entitlements.plist), not the application identifier alone (#672).
	entitlements, err := mobile.IOSReleaseEntitlements(r.srcDir, team, r.AppID, r.AssociatedDomains)
	if err != nil {
		return err
	}
	entitlementPath := filepath.Join(r.dir, "entitlements.plist")
	if err := os.WriteFile(entitlementPath, entitlements, 0o600); err != nil {
		return errors.New("failed to write entitlements plist")
	}
	defer os.Remove(entitlementPath)

	// Embedded frameworks (ios/Frameworks) carry the development signature
	// from packaging; the store checks every nested binary, so each is
	// re-signed with the release certificate before the app itself.
	frameworks, _ := filepath.Glob(filepath.Join(payloadAppDir, "Frameworks", "*.framework"))
	for _, fw := range frameworks {
		cmd := exec.Command("codesign", "-f", "-vv", "-s", r.certificate, fw)
		if out, err := cmd.CombinedOutput(); err != nil {
			bellui.LogError("Codesign failed: "+string(out), err)
			return errors.New("unable to codesign embedded framework " + filepath.Base(fw))
		}
	}

	cmd := exec.Command("codesign", "-f", "-vv", "-s", r.certificate, "--entitlements",
		"entitlements.plist", "Payload/"+appName+"/")
	if err := cmd.Run(); err != nil {
		bellui.LogError("Codesign failed", err)
		return errors.New("unable to codesign application bundle")
	}

	return exec.Command("zip", "-r", appName[:len(appName)-4]+".ipa", "Payload/").Run()
}

func (r *Releaser) packageMacOSRelease() error {
	// try to derive two certificates from one name (they will be consistent)
	appCert := strings.Replace(r.certificate, "Installer", "Application", 1)
	installCert := strings.Replace(r.certificate, "Application", "Installer", 1)
	unsignedPath := r.Name + "-unsigned.pkg"

	defer os.RemoveAll(r.Name + ".app") // this was the output of package and it can get in the way of future builds

	cleanup, err := r.writeEntitlements(templates.EntitlementsDarwin, nil)
	if err != nil {
		return errors.New("failed to write entitlements plist template")
	}
	defer cleanup()

	cmd := exec.Command("codesign", "-vfs", appCert, "--entitlement", "entitlements.plist", r.Name+".app")
	err = cmd.Run()
	if err != nil {
		bellui.LogError("Codesign failed", err)
		return errors.New("unable to codesign application bundle")
	}

	cmd = exec.Command("productbuild", "--component", r.Name+".app", "/Applications/",
		"--product", r.Name+".app/Contents/Info.plist", unsignedPath)
	err = cmd.Run()
	if err != nil {
		bellui.LogError("Product build failed", err)
		return errors.New("unable to build macOS app package")
	}
	defer os.Remove(unsignedPath)

	cmd = exec.Command("productsign", "--sign", installCert, unsignedPath, r.Name+".pkg")
	return cmd.Run()
}

func (r *Releaser) packageWindowsRelease(outFile string) error {
	payload := filepath.Join(r.dir, "Payload")
	_ = os.Mkdir(payload, 0o750)
	defer os.RemoveAll(payload)

	manifestPath := filepath.Join(payload, "appxmanifest.xml")
	manifest, err := os.Create(manifestPath)
	if err != nil {
		return err
	}
	manifestData := struct{ AppID, Developer, DeveloperName, Name, Version string }{
		AppID: r.AppID,
		// TODO read this info
		Developer:     encodeXMLString(r.developer),
		DeveloperName: encodeXMLString(r.nameFromCertInfo(r.developer)),
		Name:          encodeXMLString(r.Name),
		Version:       r.combinedVersion(),
	}
	err = templates.AppxManifestWindows.Execute(manifest, manifestData)
	manifest.Close()
	if err != nil {
		return errors.New("failed to write application manifest template")
	}

	util.CopyFile(r.icon, filepath.Join(payload, "Icon.png"))
	util.CopyFile(r.Name, filepath.Join(payload, r.Name))

	binDir, err := findWindowsSDKBin()
	if err != nil {
		return errors.New("cannot find makeappx.exe, make sure you have installed the Windows SDK")
	}

	cmd := exec.Command(filepath.Join(binDir, "makeappx.exe"), "pack", "/d", payload, "/p", outFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (r *Releaser) signAndroid(path string) error {
	cmd, err := r.androidSignCommand(path)
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// androidSignCommand returns the command that signs the package at path:
// jarsigner for a distribution .aab, apksigner otherwise. Neither gets a
// password on its command line, where any user could read it from the
// process list (#729): both read them from variables set on the child alone.
// That also keeps them away from cmd.exe, which runs apksigner.bat on Windows
// and would expand a % in an argument.
func (r *Releaser) androidSignCommand(path string) (*exec.Cmd, error) {
	signer := "jarsigner"
	var args []string
	if r.distribution {
		args = []string{"-keystore", r.keyStore}
	} else {
		signer = filepath.Join(util.AndroidBuildToolsPath(), realUtil.BuildToolFile("apksigner"))
		args = []string{"sign", "--ks", r.keyStore}
	}

	var env []string
	if r.keyStorePass != "" {
		if r.distribution {
			args = append(args, "-storepass:env", "BELLUI_KEYSTORE_PASS")
		} else {
			args = append(args, "--ks-pass", "env:BELLUI_KEYSTORE_PASS")
		}
		env = append(env, "BELLUI_KEYSTORE_PASS="+r.keyStorePass)
	}
	if r.keyPass != "" {
		if r.distribution {
			args = append(args, "-keypass:env", "BELLUI_KEY_PASS")
		} else {
			args = append(args, "--key-pass", "env:BELLUI_KEY_PASS")
		}
		env = append(env, "BELLUI_KEY_PASS="+r.keyPass)
	}
	args = append(args, path)
	if r.distribution {
		if r.keyName == "" { // Required to sign Google Play .aab
			return nil, errors.New("missing required --key-name (alias) parameter")
		}
		args = append(args, r.keyName)
	}

	cmd, err := realUtil.Command(signer, args...)
	if err != nil {
		return nil, err
	}
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	return cmd, nil
}

func (r *Releaser) signWindows(appx string) error {
	binDir, err := findWindowsSDKBin()
	if err != nil {
		return errors.New("cannot find signtool.exe, make sure you have installed the Windows SDK")
	}

	cmd := exec.Command(filepath.Join(binDir, "signtool.exe"),
		"sign", "/a", "/v", "/fd", "SHA256", "/f", r.certificate, "/p", r.password, appx)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (r *Releaser) validate() error {
	if r.os == "" {
		r.os = targetOS()
	}
	err := r.Packager.validate()
	if err != nil {
		return err
	}

	if util.IsMobile(r.os) || r.os == "windows" {
		if r.AppVersion == "" { // Here it is required, if provided then package validate will check format
			return errors.New("missing required --app-version parameter")
		}
		if r.AppBuild <= 0 {
			return errors.New("missing required --app-build parameter")
		}
	}
	if r.os == "windows" {
		if r.developer == "" {
			return errors.New("missing required --developer parameter for windows release,\n" +
				"use data from Partner Portal, format \"CN=Company, O=Company, L=City, S=State, C=Country\"")
		}
		if r.certificate == "" {
			return errors.New("missing required --certificate parameter for windows release")
		}
		if r.password == "" {
			return errors.New("missing required --password parameter for windows release")
		}
	}
	if util.IsAndroid(r.os) {
		if r.keyStore == "" {
			return errors.New("missing required --keystore parameter for android release")
		}
	} else if r.os == "darwin" {
		if r.certificate == "" {
			r.certificate = "3rd Party Mac Developer Application"
		}
		if r.profile == "" {
			return errors.New("missing required --profile parameter for macOS release")
		}
		if r.category == "" {
			return errors.New("missing required --category parameter for macOS release")
		} else if !isValidMacOSCategory(r.category) {
			return errors.New("category does not match one of the supported list: " +
				strings.Join(macAppStoreCategories, ", "))
		}
	} else if r.os == "ios" {
		if r.certificate == "" {
			r.certificate = "Apple Distribution"
		}
		if r.profile == "" {
			return errors.New("missing required --profile parameter for iOS release")
		}
	}
	return nil
}

func (r *Releaser) writeEntitlements(tmpl *template.Template, entitlementData any) (cleanup func(), err error) {
	entitlementPath := filepath.Join(r.dir, "entitlements.plist")
	entitlements, err := os.Create(entitlementPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if r := entitlements.Close(); r != nil && err == nil {
			err = r
		}
	}()

	if err := tmpl.Execute(entitlements, entitlementData); err != nil {
		return nil, err
	}
	return func() {
		_ = os.Remove(entitlementPath)
	}, nil
}

func (r *Releaser) zipAlign(path string) error {
	unaligned := filepath.Join(filepath.Dir(path), "unaligned.apk")
	if err := os.Rename(path, unaligned); err != nil {
		return fmt.Errorf("moving the package aside to align it: %w", err)
	}

	cmd := filepath.Join(util.AndroidBuildToolsPath(), realUtil.BuildToolFile("zipalign"))
	out, err := exec.Command(cmd, "16", unaligned, path).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("zipalign: %w: %s", err, strings.TrimSpace(string(out)))
		// Put the input back over whatever zipalign left at path: it is
		// the only good copy of the package (#728).
		if rerr := os.Rename(unaligned, path); rerr != nil {
			return errors.Join(err, fmt.Errorf("restoring %s: %w", path, rerr))
		}
		return err
	}
	return os.Remove(unaligned)
}

// lookPath is exec.LookPath, replaceable in tests.
var lookPath = exec.LookPath

// findWindowsSDKBin returns the directory holding makeappx.exe (and, beside
// it, signtool.exe): the one on PATH if there is one, else the best match
// under the standard Windows Kits install location (see pickWindowsSDKBin).
// Callers join the tool names onto it.
func findWindowsSDKBin() (string, error) {
	inPath, err := lookPath("makeappx.exe")
	if err == nil {
		return filepath.Dir(inPath), nil
	}

	matches, err := filepath.Glob("C:\\Program Files (x86)\\Windows Kits\\*\\bin\\*\\*\\makeappx.exe")
	if err != nil || len(matches) == 0 {
		return "", errors.New("failed to look up standard locations for makeappx.exe")
	}

	return pickWindowsSDKBin(matches, runtime.GOARCH), nil
}

// windowsSDKArchDirs lists, per GOARCH, the SDK architecture directories
// whose tools run on such a machine, best first.
var windowsSDKArchDirs = map[string][]string{
	"amd64": {"x64", "x86"},
	"386":   {"x86"},
	"arm64": {"arm64", "x64", "x86"},
}

// pickWindowsSDKBin chooses among .../bin/<version>/<arch>/makeappx.exe
// matches: the architecture that runs best on goarch, then the newest SDK
// version. filepath.Glob sorts lexically, so its first match is the oldest
// version's arm64 directory, which an x64 machine cannot run.
func pickWindowsSDKBin(matches []string, goarch string) string {
	rank := func(match string) int {
		arch := filepath.Base(filepath.Dir(match))
		for i, a := range windowsSDKArchDirs[goarch] {
			if strings.EqualFold(arch, a) {
				return i
			}
		}
		return len(windowsSDKArchDirs[goarch])
	}

	best := matches[0]
	for _, match := range matches[1:] {
		r, bestRank := rank(match), rank(best)
		if r < bestRank || (r == bestRank && compareSDKVersions(sdkVersionOf(match), sdkVersionOf(best)) > 0) {
			best = match
		}
	}
	return filepath.Dir(best)
}

func sdkVersionOf(match string) string {
	return filepath.Base(filepath.Dir(filepath.Dir(match)))
}

// compareSDKVersions compares dotted numeric versions such as 10.0.22621.0;
// a part that is not a number counts as 0.
func compareSDKVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var x, y int
		if i < len(as) {
			x, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			y, _ = strconv.Atoi(bs[i])
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func isValidMacOSCategory(in string) bool {
	found := false
	for _, cat := range macAppStoreCategories {
		if cat == strings.ToLower(in) {
			found = true
			break
		}
	}
	return found
}
