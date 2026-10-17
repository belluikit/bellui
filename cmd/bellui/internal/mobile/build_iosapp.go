// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mobile

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"golang.org/x/tools/go/packages"
)

func goIOSBuild(pkg *packages.Package, bundleID string, archs []string,
	appName, version string, build int, release bool, cert, profile string,
	usageDescriptions map[string]string,
) (map[string]bool, error) {
	src := pkg.PkgPath
	buildO = rfc1034Label(appName) + ".app"
	// Detect the team ID
	teamID, err := DetectIOSTeamID(cert)
	if err != nil {
		return nil, fmt.Errorf("failed to look up certificate %s: %s", cert, err.Error())
	}

	projPbxproj := new(bytes.Buffer)
	projData := iosProjectData(bundleID, cert, profile, teamID, release)
	if err := projPbxprojTmpl.Execute(projPbxproj, projData); err != nil {
		return nil, err
	}

	infoplist := new(bytes.Buffer)
	if err := infoplistTmpl.Execute(infoplist, infoplistTmplData{
		BundleID:          bundleID,
		Name:              strings.Title(appName), //lint:ignore SA1019 It is fine for our uses.
		Version:           version,
		Build:             build,
		Legacy:            len(allArchs["ios"]) > 2,
		UsageDescriptions: sortedPlistEntries(usageDescriptions),
		URLSchemes:        buildURLSchemes,
		MimeTypes:         buildFileMimeTypes,
		ContentTypes:      mimeTypesToUTIs(buildFileMimeTypes),
	}); err != nil {
		return nil, err
	}

	files := []struct {
		name     string
		contents []byte
	}{
		{tmpdir + "/main.xcodeproj/project.pbxproj", projPbxproj.Bytes()},
		{tmpdir + "/main/Info.plist", infoplist.Bytes()},
		{tmpdir + "/main/Images.xcassets/AppIcon.appiconset/Contents.json", []byte(contentsJSON)},
	}

	// Emit an Associated Domains entitlement so Universal Links (tapped
	// https://<domain>/… links) route to the app (pre-rebrand tracker #622). Signed in via
	// CODE_SIGN_ENTITLEMENTS; xcodebuild -allowProvisioningUpdates registers the
	// capability with the App ID under automatic signing.
	if projData.Entitlements {
		entitlements := new(bytes.Buffer)
		if err := entitlementsTmpl.Execute(entitlements, buildAssociatedDomains); err != nil {
			return nil, err
		}
		files = append(files, struct {
			name     string
			contents []byte
		}{tmpdir + "/main/main.entitlements", entitlements.Bytes()})
	}

	for _, file := range files {
		if err := mkdir(filepath.Dir(file.name)); err != nil {
			return nil, err
		}
		if buildX {
			printcmd("echo \"%s\" > %s", file.contents, file.name)
		}
		if !buildN {
			if err := os.WriteFile(file.name, file.contents, 0o600); err != nil {
				return nil, err
			}
		}
	}

	// We are using lipo tool to build multiarchitecture binaries.
	cmd := exec.Command(
		"xcrun", "lipo",
		"-o", filepath.Join(tmpdir, "main/main"),
		"-create",
	)
	var nmpkgs map[string]bool
	for _, arch := range archs {
		path := filepath.Join(tmpdir, arch)
		// Disable DWARF; see golang.org/issues/25148.
		if err := goBuild(src, darwinEnv[arch], "-ldflags=-w", "-o="+path); err != nil {
			return nil, err
		}
		if nmpkgs == nil {
			var err error
			nmpkgs, err = extractPkgs(darwinArmNM, path)
			if err != nil {
				return nil, err
			}
		}
		cmd.Args = append(cmd.Args, path)
	}

	if err := runCmd(cmd); err != nil {
		return nil, err
	}

	// TODO(jbd): Set the launcher icon.
	if err := iosCopyAssets(pkg, tmpdir); err != nil {
		return nil, err
	}

	// Build and move the release build to the output directory.
	cmdStrings := []string{
		"xcodebuild",
		"-configuration", "Release",
		"-project", tmpdir + "/main.xcodeproj",
		"-allowProvisioningUpdates",
		"DEVELOPMENT_TEAM=" + teamID,
	}

	cmd = exec.Command("xcrun", cmdStrings...)
	if err := runCmd(cmd); err != nil {
		return nil, err
	}

	// TODO(jbd): Fallback to copying if renaming fails.
	if buildO == "" {
		n := pkg.PkgPath
		if n == "." {
			// use cwd name
			cwd, err := os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("cannot create .app; cannot get the current working dir: %v", err)
			}
			n = cwd
		}
		n = path.Base(n)
		buildO = n + ".app"
	}
	if buildX {
		printcmd("mv %s %s", tmpdir+"/build/Release-iphoneos/main.app", buildO)
	}
	if !buildN {
		// if output already exists, remove.
		if err := os.RemoveAll(buildO); err != nil {
			return nil, err
		}
		if err := os.Rename(tmpdir+"/build/Release-iphoneos/main.app", buildO); err != nil {
			return nil, err
		}
	}

	// Use codesign to remove the codesign certificate for the built application
	// so that it can run in iOS simulator.
	if buildTarget == "iossimulator" {
		if out, err := exec.Command("codesign", "--force", "--sign", "-", buildO).CombinedOutput(); err != nil {
			printcmd("codesign --force --sign --keychain %s\n%s", buildO, out)
			return nil, err
		}
	}

	return nmpkgs, nil
}

// DetectIOSTeamID finds the team ID used by the app certificates.
// The optional parameter specifies the certificate name to use.
// If none is specified the code will check standard names
func DetectIOSTeamID(optCert string) (string, error) {
	pemString, err := lookupCert(optCert)
	if err != nil {
		return "", err
	}

	block, _ := pem.Decode(pemString)
	if block == nil {
		err = fmt.Errorf("failed to decode the PEM to determine your team ID: %s", pemString)
		return "", err
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		err = fmt.Errorf("failed to parse your signing certificate to determine your team ID: %v", err)
		return "", err
	}

	if len(cert.Subject.OrganizationalUnit) == 0 {
		err = fmt.Errorf("the signing certificate has no organizational unit (team ID)")
		return "", err
	}

	return cert.Subject.OrganizationalUnit[0], nil
}

func iosCopyAssets(pkg *packages.Package, xcodeProjDir string) error {
	dstAssets := xcodeProjDir + "/main/assets"
	if err := mkdir(dstAssets); err != nil {
		return err
	}

	// TODO(hajimehoshi): This works only with Go tools that assume all source files are in one directory.
	// Fix this to work with other Go tools.
	srcAssets := filepath.Join(filepath.Dir(pkg.GoFiles[0]), "assets")
	fi, err := os.Stat(srcAssets)
	if err != nil {
		if os.IsNotExist(err) {
			// skip walking through the directory to deep copy.
			return nil
		}
		return err
	}
	if !fi.IsDir() {
		// skip walking through to deep copy.
		return nil
	}
	// if assets is a symlink, follow the symlink.
	srcAssets, err = filepath.EvalSymlinks(srcAssets)
	if err != nil {
		return err
	}
	return filepath.WalkDir(srcAssets, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name := filepath.Base(path); strings.HasPrefix(name, ".") {
			// Do not include the hidden files.
			return nil
		}
		if info.IsDir() {
			return nil
		}
		dst := dstAssets + "/" + path[len(srcAssets)+1:]
		return copyFile(dst, path)
	})
}

func lookupCert(optName string) ([]byte, error) {
	if optName != "" {
		return lookupCertNamed(optName)
	}
	// Distribution certs are for store artifacts ("bellui release") only; a
	// --release package keeps the Developer/Development cert so it still
	// installs on registered devices (#144).
	if buildDistribution {
		// Grabs the first certificate for "iPhone Distribution"; will not work if there
		// are multiple certificates and the first is not desired.
		pemString, err := lookupCertNamed("iPhone Distribution")
		if err != nil {
			// If no "iPhone Distribution" cert is found then try the new "Apple Development".
			return lookupCertNamed("Apple Distribution")
		}
		return pemString, nil
	}
	// Grabs the first certificate for "iPhone Developer"; will not work if there
	// are multiple certificates and the first is not desired.
	pemString, err := lookupCertNamed("iPhone Developer")
	if err != nil {
		// If no "iPhone Developer" cert is found then try the new "Apple Development".
		return lookupCertNamed("Apple Development")
	}
	return pemString, err
}

func lookupCertNamed(name string) ([]byte, error) {
	cmd := exec.Command(
		"security", "find-certificate",
		"-c", name, "-p",
	)
	return cmd.Output()
}

type infoplistTmplData struct {
	BundleID string
	Name     string
	Version  string
	Build    int
	Legacy   bool
	// UsageDescriptions are the iOS Info.plist usage-description entries
	// (NSCameraUsageDescription, …), pre-sorted by key for deterministic output.
	UsageDescriptions []plistEntry
	// URLSchemes declares custom URL schemes the app handles (pre-rebrand tracker #622), emitted
	// as CFBundleURLTypes so iOS routes myapp:// deep links to the app.
	URLSchemes []string
	// MimeTypes declares the document MIME types the app can open, emitted as a
	// CFBundleDocumentTypes entry so the app appears in "Open in…" for them.
	MimeTypes []string
	// ContentTypes are the UTIs (derived from MimeTypes) emitted as
	// LSItemContentTypes. iOS 14+ routes "Open in…" / share-sheet document
	// targets by UTI, not the legacy CFBundleTypeMIMETypes, so without these the
	// app never surfaces as a file handler (pre-rebrand tracker #622).
	ContentTypes []string
}

// mimeTypesToUTIs maps document MIME types to the iOS Uniform Type Identifiers
// used in LSItemContentTypes. Well-known types resolve to their system UTI;
// otherwise the top-level type picks a broad system UTI (text/image/audio/video
// → public.text/image/audio/movie), falling back to public.data. Deduplicated,
// order-preserving.
func mimeTypesToUTIs(mimes []string) []string {
	known := map[string]string{
		"text/plain":               "public.plain-text",
		"text/html":                "public.html",
		"text/csv":                 "public.comma-separated-values-text",
		"text/markdown":            "net.daringfireball.markdown",
		"text/xml":                 "public.xml",
		"application/xml":          "public.xml",
		"application/json":         "public.json",
		"application/pdf":          "com.adobe.pdf",
		"application/zip":          "public.zip-archive",
		"application/rtf":          "public.rtf",
		"application/octet-stream": "public.data",
		"image/png":                "public.png",
		"image/jpeg":               "public.jpeg",
		"image/gif":                "com.compuserve.gif",
		"image/tiff":               "public.tiff",
		"image/heic":               "public.heic",
		"image/svg+xml":            "public.svg-image",
		"audio/mpeg":               "public.mp3",
		"audio/wav":                "com.microsoft.waveform-audio",
		"video/mp4":                "public.mpeg-4",
		"video/quicktime":          "com.apple.quicktime-movie",
	}
	var out []string
	seen := map[string]bool{}
	add := func(uti string) {
		if uti != "" && !seen[uti] {
			seen[uti] = true
			out = append(out, uti)
		}
	}
	for _, m := range mimes {
		m = strings.ToLower(strings.TrimSpace(m))
		if uti, ok := known[m]; ok {
			add(uti)
			continue
		}
		switch {
		case strings.HasPrefix(m, "text/"):
			add("public.text")
		case strings.HasPrefix(m, "image/"):
			add("public.image")
		case strings.HasPrefix(m, "audio/"):
			add("public.audio")
		case strings.HasPrefix(m, "video/"):
			add("public.movie")
		default:
			add("public.data")
		}
	}
	return out
}

// plistEntry is one Info.plist key/value pair (a usage-description string).
type plistEntry struct {
	Key   string
	Value string
}

// sortedPlistEntries flattens the usage-description map into a slice sorted by
// key, so the generated Info.plist is deterministic (Go map iteration order is
// randomized) — reproducible builds and stable test golden output.
func sortedPlistEntries(m map[string]string) []plistEntry {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	entries := make([]plistEntry, len(keys))
	for i, k := range keys {
		entries[i] = plistEntry{Key: k, Value: m[k]}
	}
	return entries
}

// entitlementsTmpl renders the iOS code-signing entitlements from the list of
// associated domains (pre-rebrand tracker #622 Universal Links). The template data is the
// []string of domains (without the "applinks:" prefix).
var entitlementsTmpl = template.Must(template.New("entitlements").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>com.apple.developer.associated-domains</key>
  <array>
{{- range .}}
    <string>applinks:{{.}}</string>
{{- end}}
  </array>
</dict>
</plist>
`))

var infoplistTmpl = template.Must(template.New("infoplist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key>
  <string>en</string>
  <key>CFBundleExecutable</key>
  <string>main</string>
  <key>CFBundleIdentifier</key>
  <string>{{.BundleID}}</string>
  <key>CFBundleInfoDictionaryVersion</key>
  <string>6.0</string>
  <key>CFBundleName</key>
  <string>{{.Name}}</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>{{.Version}}</string>
  <key>CFBundleSignature</key>
  <string>????</string>
  <key>CFBundleVersion</key>
  <string>{{.Build}}</string>
  <key>CFBundleIcons</key>
  <dict>
    <key>CFBundlePrimaryIcon</key>
    <dict>
      <key>CFBundleIconFiles</key>
      <array>
        <string>AppIcon60x60</string>
      </array>
      <key>CFBundleIconName</key>
      <string>AppIcon</string>
    </dict>
  </dict>
  <key>CFBundleIcons~ipad</key>
  <dict>
    <key>CFBundlePrimaryIcon</key>
    <dict>
      <key>CFBundleIconFiles</key>
      <array>
        <string>AppIcon60x60</string>
        <string>AppIcon76x76</string>
      </array>
      <key>CFBundleIconName</key>
      <string>AppIcon</string>
    </dict>
  </dict>
  <key>CFBundleIconName</key>
  <string>AppIcon</string>
  <key>LSRequiresIPhoneOS</key>
  <true/>
  <key>UILaunchStoryboardName</key>
  <string>LaunchScreen</string>
  <key>UIRequiredDeviceCapabilities</key>
  <array>
{{ if .Legacy }}
    <string>armv7</string>
{{ else }}
    <string>arm64</string>
{{ end }}
  </array>
  <key>UIRequiresFullScreen</key>
  <true/>
  <key>UISupportedInterfaceOrientations</key>
  <array>
    <string>UIInterfaceOrientationPortrait</string>
    <string>UIInterfaceOrientationLandscapeLeft</string>
    <string>UIInterfaceOrientationLandscapeRight</string>
  </array>
  <key>UISupportedInterfaceOrientations~ipad</key>
  <array>
    <string>UIInterfaceOrientationPortrait</string>
    <string>UIInterfaceOrientationPortraitUpsideDown</string>
    <string>UIInterfaceOrientationLandscapeLeft</string>
    <string>UIInterfaceOrientationLandscapeRight</string>
  </array>
{{- range .UsageDescriptions}}
  <key>{{.Key}}</key>
  <string>{{.Value}}</string>
{{- end}}
{{- if .URLSchemes}}
  <key>CFBundleURLTypes</key>
  <array>
    <dict>
      <key>CFBundleURLName</key>
      <string>{{.BundleID}}</string>
      <key>CFBundleURLSchemes</key>
      <array>
{{- range .URLSchemes}}
        <string>{{.}}</string>
{{- end}}
      </array>
    </dict>
  </array>
{{- end}}
{{- if .MimeTypes}}
  <key>CFBundleDocumentTypes</key>
  <array>
    <dict>
      <key>CFBundleTypeName</key>
      <string>{{.Name}} Document</string>
      <key>CFBundleTypeRole</key>
      <string>Editor</string>
      <key>LSHandlerRank</key>
      <string>Alternate</string>
{{- if .ContentTypes}}
      <key>LSItemContentTypes</key>
      <array>
{{- range .ContentTypes}}
        <string>{{.}}</string>
{{- end}}
      </array>
{{- end}}
      <key>CFBundleTypeMIMETypes</key>
      <array>
{{- range .MimeTypes}}
        <string>{{.}}</string>
{{- end}}
      </array>
    </dict>
  </array>
{{- end}}
</dict>
</plist>
`))

// iosProjectData is the data for the generated Xcode project, from the
// arguments and the build settings (IOSDeploymentTarget among them).
func iosProjectData(bundleID, cert, profile, teamID string, release bool) projPbxprojTmplData {
	projData := projPbxprojTmplData{
		BitcodeEnabled:   bitcodeEnabled,
		BundleID:         bundleID,
		Certificate:      cert,
		Debug:            !release,
		Profile:          profile,
		TeamID:           teamID,
		Type:             "Manual",
		Entitlements:     len(buildAssociatedDomains) > 0,
		DeploymentTarget: IOSDeploymentTarget(),
	}
	if profile == "" {
		projData.Certificate = "iPhone Developer"
		projData.Type = "Auto"
	}
	return projData
}

type projPbxprojTmplData struct {
	BitcodeEnabled bool
	BundleID       string
	Certificate    string
	Profile        string
	TeamID, Type   string
	Debug          bool
	// Entitlements is true when a main/main.entitlements file is generated (e.g.
	// for Associated Domains / Universal Links) and must be code-signed in.
	Entitlements bool
	// DeploymentTarget is the lowest iOS version the app runs on, from
	// IOSDeploymentTarget. It used to be a fixed 14.0, which Xcode 27 rejects
	// (#410).
	DeploymentTarget string
}

var projPbxprojTmpl = template.Must(template.New("projPbxproj").Parse(`// !$*UTF8*$!
{
  archiveVersion = 1;
  classes = {
  };
  objectVersion = 46;
  objects = {

/* Begin PBXBuildFile section */
    254BB84F1B1FD08900C56DE9 /* Images.xcassets in Resources */ = {isa = PBXBuildFile; fileRef = 254BB84E1B1FD08900C56DE9 /* Images.xcassets */; };
    254BB8681B1FD16500C56DE9 /* main in Resources */ = {isa = PBXBuildFile; fileRef = 254BB8671B1FD16500C56DE9 /* main */; };
    25FB30331B30FDEE0005924C /* assets in Resources */ = {isa = PBXBuildFile; fileRef = 25FB30321B30FDEE0005924C /* assets */; };
/* End PBXBuildFile section */

/* Begin PBXFileReference section */
    254BB83E1B1FD08900C56DE9 /* main.app */ = {isa = PBXFileReference; explicitFileType = wrapper.application; includeInIndex = 0; path = main.app; sourceTree = BUILT_PRODUCTS_DIR; };
    254BB8421B1FD08900C56DE9 /* Info.plist */ = {isa = PBXFileReference; lastKnownFileType = text.plist.xml; path = Info.plist; sourceTree = "<group>"; };
    254BB84E1B1FD08900C56DE9 /* Images.xcassets */ = {isa = PBXFileReference; lastKnownFileType = folder.assetcatalog; path = Images.xcassets; sourceTree = "<group>"; };
    254BB8671B1FD16500C56DE9 /* main */ = {isa = PBXFileReference; lastKnownFileType = "compiled.mach-o.executable"; path = main; sourceTree = "<group>"; };
    25FB30321B30FDEE0005924C /* assets */ = {isa = PBXFileReference; lastKnownFileType = folder; name = assets; path = main/assets; sourceTree = "<group>"; };
/* End PBXFileReference section */

/* Begin PBXGroup section */
    254BB8351B1FD08900C56DE9 = {
      isa = PBXGroup;
      children = (
        25FB30321B30FDEE0005924C /* assets */,
        254BB8401B1FD08900C56DE9 /* main */,
        254BB83F1B1FD08900C56DE9 /* Products */,
      );
      sourceTree = "<group>";
      usesTabs = 0;
    };
    254BB83F1B1FD08900C56DE9 /* Products */ = {
      isa = PBXGroup;
      children = (
        254BB83E1B1FD08900C56DE9 /* main.app */,
      );
      name = Products;
      sourceTree = "<group>";
    };
    254BB8401B1FD08900C56DE9 /* main */ = {
      isa = PBXGroup;
      children = (
        254BB8671B1FD16500C56DE9 /* main */,
        254BB84E1B1FD08900C56DE9 /* Images.xcassets */,
        254BB8411B1FD08900C56DE9 /* Supporting Files */,
      );
      path = main;
      sourceTree = "<group>";
    };
    254BB8411B1FD08900C56DE9 /* Supporting Files */ = {
      isa = PBXGroup;
      children = (
        254BB8421B1FD08900C56DE9 /* Info.plist */,
      );
      name = "Supporting Files";
      sourceTree = "<group>";
    };
/* End PBXGroup section */

/* Begin PBXNativeTarget section */
    254BB83D1B1FD08900C56DE9 /* main */ = {
      isa = PBXNativeTarget;
      buildConfigurationList = 254BB8611B1FD08900C56DE9 /* Build configuration list for PBXNativeTarget "main" */;
      buildPhases = (
        254BB83C1B1FD08900C56DE9 /* Resources */,
      );
      buildRules = (
      );
      dependencies = (
      );
      name = main;
      productName = main;
      productReference = 254BB83E1B1FD08900C56DE9 /* main.app */;
      productType = "com.apple.product-type.application";
    };
/* End PBXNativeTarget section */

/* Begin PBXProject section */
    254BB8361B1FD08900C56DE9 /* Project object */ = {
      isa = PBXProject;
      attributes = {
        LastUpgradeCheck = 0630;
        ORGANIZATIONNAME = Developer;
        TargetAttributes = {
          254BB83D1B1FD08900C56DE9 = {
            CreatedOnToolsVersion = 6.3.1;
            DevelopmentTeam = {{.TeamID}};
            ProvisioningStyle = {{.Type}};
          };
        };
      };
      buildConfigurationList = 254BB8391B1FD08900C56DE9 /* Build configuration list for PBXProject "main" */;
      compatibilityVersion = "Xcode 3.2";
      developmentRegion = English;
      hasScannedForEncodings = 0;
      knownRegions = (
        en,
        Base,
      );
      mainGroup = 254BB8351B1FD08900C56DE9;
      productRefGroup = 254BB83F1B1FD08900C56DE9 /* Products */;
      projectDirPath = "";
      projectRoot = "";
      targets = (
        254BB83D1B1FD08900C56DE9 /* main */,
      );
    };
/* End PBXProject section */

/* Begin PBXResourcesBuildPhase section */
    254BB83C1B1FD08900C56DE9 /* Resources */ = {
      isa = PBXResourcesBuildPhase;
      buildActionMask = 2147483647;
      files = (
        25FB30331B30FDEE0005924C /* assets in Resources */,
        254BB8681B1FD16500C56DE9 /* main in Resources */,
        254BB84F1B1FD08900C56DE9 /* Images.xcassets in Resources */,
      );
      runOnlyForDeploymentPostprocessing = 0;
    };
/* End PBXResourcesBuildPhase section */

/* Begin XCBuildConfiguration section */
    254BB8601B1FD08900C56DE9 /* Release */ = {
      isa = XCBuildConfiguration;
      buildSettings = {
        ALWAYS_SEARCH_USER_PATHS = NO;
        CLANG_CXX_LANGUAGE_STANDARD = "gnu++0x";
        CLANG_CXX_LIBRARY = "libc++";
        CLANG_ENABLE_MODULES = YES;
        CLANG_ENABLE_OBJC_ARC = YES;
        CLANG_WARN_BOOL_CONVERSION = YES;
        CLANG_WARN_CONSTANT_CONVERSION = YES;
        CLANG_WARN_DIRECT_OBJC_ISA_USAGE = YES_ERROR;
        CLANG_WARN_EMPTY_BODY = YES;
        CLANG_WARN_ENUM_CONVERSION = YES;
        CLANG_WARN_INT_CONVERSION = YES;
        CLANG_WARN_OBJC_ROOT_CLASS = YES_ERROR;
        CLANG_WARN_UNREACHABLE_CODE = YES;
        CLANG_WARN__DUPLICATE_METHOD_MATCH = YES;
        "CODE_SIGN_IDENTITY[sdk=iphoneos*]" = "{{.Certificate}}";
        COPY_PHASE_STRIP = NO;
        DEBUG_INFORMATION_FORMAT = "dwarf-with-dsym";
        ENABLE_NS_ASSERTIONS = NO;
        ENABLE_STRICT_OBJC_MSGSEND = YES;
        GCC_C_LANGUAGE_STANDARD = gnu99;
        GCC_NO_COMMON_BLOCKS = YES;
        GCC_WARN_64_TO_32_BIT_CONVERSION = YES;
        GCC_WARN_ABOUT_RETURN_TYPE = YES_ERROR;
        GCC_WARN_UNDECLARED_SELECTOR = YES;
        GCC_WARN_UNINITIALIZED_AUTOS = YES_AGGRESSIVE;
        GCC_WARN_UNUSED_FUNCTION = YES;
        GCC_WARN_UNUSED_VARIABLE = YES;
        IPHONEOS_DEPLOYMENT_TARGET = {{.DeploymentTarget}};
        MTL_ENABLE_DEBUG_INFO = {{if .Debug}}YES{{else}}NO{{end}};
        PRODUCT_BUNDLE_IDENTIFIER = {{.BundleID}};
        PROVISIONING_PROFILE_SPECIFIER = "{{.Profile}}";
        SDKROOT = iphoneos;
        TARGETED_DEVICE_FAMILY = "1,2";
        VALIDATE_PRODUCT = YES;
        {{if not .BitcodeEnabled}}ENABLE_BITCODE = NO;{{end}}
      };
      name = Release;
    };
    254BB8631B1FD08900C56DE9 /* Release */ = {
      isa = XCBuildConfiguration;
      buildSettings = {
        ASSETCATALOG_COMPILER_APPICON_NAME = AppIcon;
        {{if .Entitlements}}CODE_SIGN_ENTITLEMENTS = main/main.entitlements;{{end}}
        INFOPLIST_FILE = main/Info.plist;
        LD_RUNPATH_SEARCH_PATHS = "$(inherited) @executable_path/Frameworks";
        PRODUCT_NAME = "$(TARGET_NAME)";
      };
      name = Release;
    };
/* End XCBuildConfiguration section */

/* Begin XCConfigurationList section */
    254BB8391B1FD08900C56DE9 /* Build configuration list for PBXProject "main" */ = {
      isa = XCConfigurationList;
      buildConfigurations = (
        254BB8601B1FD08900C56DE9 /* Release */,
      );
      defaultConfigurationIsVisible = 0;
      defaultConfigurationName = Release;
    };
    254BB8611B1FD08900C56DE9 /* Build configuration list for PBXNativeTarget "main" */ = {
      isa = XCConfigurationList;
      buildConfigurations = (
        254BB8631B1FD08900C56DE9 /* Release */,
      );
      defaultConfigurationIsVisible = 0;
      defaultConfigurationName = Release;
    };
/* End XCConfigurationList section */
  };
  rootObject = 254BB8361B1FD08900C56DE9 /* Project object */;
}
`))

const contentsJSON = `{
  "images" : [
    {
      "idiom" : "iphone",
      "size" : "29x29",
      "scale" : "2x"
    },
    {
      "idiom" : "iphone",
      "size" : "29x29",
      "scale" : "3x"
    },
    {
      "idiom" : "iphone",
      "size" : "40x40",
      "scale" : "2x"
    },
    {
      "idiom" : "iphone",
      "size" : "40x40",
      "scale" : "3x"
    },
    {
      "idiom" : "iphone",
      "size" : "60x60",
      "scale" : "2x"
    },
    {
      "idiom" : "iphone",
      "size" : "60x60",
      "scale" : "3x"
    },
    {
      "idiom" : "ipad",
      "size" : "29x29",
      "scale" : "1x"
    },
    {
      "idiom" : "ipad",
      "size" : "29x29",
      "scale" : "2x"
    },
    {
      "idiom" : "ipad",
      "size" : "40x40",
      "scale" : "1x"
    },
    {
      "idiom" : "ipad",
      "size" : "40x40",
      "scale" : "2x"
    },
    {
      "idiom" : "ipad",
      "size" : "76x76",
      "scale" : "1x"
    },
    {
      "idiom" : "ipad",
      "size" : "76x76",
      "scale" : "2x"
    }
  ],
  "info" : {
    "version" : 1,
    "author" : "xcode"
  }
}
`

// rfc1034Label sanitizes the name to be usable in a uniform type identifier.
// The sanitization is similar to xcode's rfc1034identifier macro that
// replaces illegal characters (not conforming the rfc1034 label rule) with '-'.
func rfc1034Label(name string) string {
	// * Uniform type identifier:
	//
	// According to
	// https://developer.apple.com/library/ios/documentation/FileManagement/Conceptual/understanding_utis/understand_utis_conc/understand_utis_conc.html
	//
	// A uniform type identifier is a Unicode string that usually contains characters
	// in the ASCII character set. However, only a subset of the ASCII characters are
	// permitted. You may use the Roman alphabet in upper and lower case (A–Z, a–z),
	// the digits 0 through 9, the dot (“.”), and the hyphen (“-”). This restriction
	// is based on DNS name restrictions, set forth in RFC 1035.
	//
	// Uniform type identifiers may also contain any of the Unicode characters greater
	// than U+007F.
	//
	// Note: the actual implementation of xcode does not allow some unicode characters
	// greater than U+007f. In this implementation, we just replace everything non
	// alphanumeric with "-" like the rfc1034identifier macro.
	//
	// * RFC1034 Label
	//
	// <label> ::= <letter> [ [ <ldh-str> ] <let-dig> ]
	// <ldh-str> ::= <let-dig-hyp> | <let-dig-hyp> <ldh-str>
	// <let-dig-hyp> ::= <let-dig> | "-"
	// <let-dig> ::= <letter> | <digit>
	const surrSelf = 0x10000
	begin := false

	var res []rune
	for i, r := range name {
		if r == '.' && !begin {
			continue
		}
		begin = true

		switch {
		case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z':
			res = append(res, r)
		case '0' <= r && r <= '9':
			if i == 0 {
				res = append(res, '-')
			} else {
				res = append(res, r)
			}
		default:
			if r < surrSelf {
				res = append(res, '-')
			} else {
				res = append(res, '-', '-')
			}
		}
	}
	return string(res)
}
