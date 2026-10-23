package mobile

import (
	"bytes"
	"debug/macho"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// An iOS app can carry native requirements the manifest has no field for, as
// files in an ios/ directory next to its main package (#672), the same way
// android/*.java adds Java to an Android build:
//
//	ios/Info.plist         a plist dict merged into the generated Info.plist
//	ios/Entitlements.plist a plist dict merged into the signing entitlements
//	ios/Frameworks/        *.xcframework and *.framework to link and embed
const (
	iosExtrasDir          = "ios"
	iosInfoPlistFragment  = "Info.plist"
	iosEntitlementsFile   = "Entitlements.plist"
	iosFrameworksDir      = "Frameworks"
	associatedDomainsKey  = "com.apple.developer.associated-domains"
	iosEmbeddedFrameworks = "@executable_path/Frameworks"
)

// readIOSFragment parses ios/<name> under the app directory. A missing file
// is no fragment (nil, nil).
func readIOSFragment(appDir, name string) (*plistValue, error) {
	p := filepath.Join(appDir, iosExtrasDir, name)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	v, err := parsePlist(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return v, nil
}

// manifestOwnedInfoKeys are the Info.plist keys the packager derives from the
// manifest or the command line. A fragment may not set them: two sources for
// one value would let the fragment silently disagree with the manifest.
var manifestOwnedInfoKeys = map[string]string{
	"CFBundleIdentifier":         "set the app ID (Details.ID in manifest.pxf, or --app-id)",
	"CFBundleExecutable":         "the packager names the executable",
	"CFBundleName":               "set Details.Name in manifest.pxf",
	"CFBundleShortVersionString": "set Details.Version in manifest.pxf",
	"CFBundleVersion":            "set Details.Build in manifest.pxf",
}

// mergeInfoPlist merges the app's ios/Info.plist fragment into the generated
// Info.plist. The rules:
//   - keys in manifestOwnedInfoKeys are an error;
//   - a key the manifest also produces — a usage description from
//     IOSUsageDescriptions, CFBundleURLTypes from URLSchemes,
//     CFBundleDocumentTypes from file associations, the default icon's
//     NSHumanReadableCopyright credit — is an error;
//   - any other key the generator writes (orientations, UIRequiresFullScreen,
//     …) is replaced by the fragment's value, in place;
//   - new keys are appended in the fragment's order.
//
// Replacement is whole-value: a fragment array replaces the generated array.
func mergeInfoPlist(generated []byte, fragment *plistValue, data infoplistTmplData) ([]byte, error) {
	if fragment == nil {
		return generated, nil
	}
	base, err := parsePlist(generated)
	if err != nil {
		return nil, fmt.Errorf("generated Info.plist: %w", err)
	}
	fromManifest := map[string]string{}
	for _, e := range data.UsageDescriptions {
		fromManifest[e.Key] = "it is in Details.IOSUsageDescriptions in manifest.pxf"
	}
	if len(data.URLSchemes) > 0 {
		fromManifest["CFBundleURLTypes"] = "it is generated from Details.URLSchemes in manifest.pxf"
	}
	if len(data.MimeTypes) > 0 {
		fromManifest["CFBundleDocumentTypes"] = "it is generated from the file associations in manifest.pxf"
	}
	if data.Credit != "" {
		fromManifest["NSHumanReadableCopyright"] = "it carries the default icon's required credit; ship your own Icon to drop it"
	}
	for i, k := range fragment.Keys {
		if why, ok := manifestOwnedInfoKeys[k]; ok {
			return nil, fmt.Errorf("ios/%s: key %s is managed by the packager: %s", iosInfoPlistFragment, k, why)
		}
		if why, ok := fromManifest[k]; ok {
			return nil, fmt.Errorf("ios/%s: key %s is set twice: %s", iosInfoPlistFragment, k, why)
		}
		base.set(k, fragment.Vals[i])
	}
	return encodePlist(base), nil
}

// iosEntitlements builds the signing entitlements: the Associated Domains the
// manifest declares, then the app's ios/Entitlements.plist. It returns nil
// when there is nothing to sign in.
func iosEntitlements(appDir string, associatedDomains []string) (*plistValue, error) {
	frag, err := readIOSFragment(appDir, iosEntitlementsFile)
	if err != nil {
		return nil, err
	}
	ent := &plistValue{Kind: "dict"}
	if len(associatedDomains) > 0 {
		links := make([]string, len(associatedDomains))
		for i, d := range associatedDomains {
			links[i] = "applinks:" + d
		}
		ent.set(associatedDomainsKey, plistStringArray(links))
	}
	if frag != nil {
		for i, k := range frag.Keys {
			if k == "application-identifier" {
				// Xcode and `bellui release` both derive it from the team and
				// the app ID; a fragment's copy would only fail at release.
				return nil, fmt.Errorf("ios/%s: application-identifier is set by the signing step", iosEntitlementsFile)
			}
			if k == associatedDomainsKey && len(associatedDomains) > 0 {
				return nil, fmt.Errorf("ios/%s: %s is set twice: it is generated from Details.AssociatedDomains in manifest.pxf",
					iosEntitlementsFile, associatedDomainsKey)
			}
			ent.set(k, frag.Vals[i])
		}
	}
	if len(ent.Keys) == 0 {
		return nil, nil
	}
	return ent, nil
}

// IOSReleaseEntitlements is the entitlements file `bellui release -os ios`
// re-signs the app with: the application identifier, then everything
// `bellui package` signed in from the manifest and ios/Entitlements.plist.
// Signing with the identifier alone dropped them from the store build.
//
// codesign does not expand build settings the way Xcode does at package
// time, so the ones an entitlements file uses ($(AppIdentifierPrefix), …)
// are expanded here; any other $(…) is an error rather than a literal in the
// store build's signature.
func IOSReleaseEntitlements(appDir, teamID, appID string, associatedDomains []string) ([]byte, error) {
	ent, err := iosEntitlements(appDir, associatedDomains)
	if err != nil {
		return nil, err
	}
	out := &plistValue{Kind: "dict"}
	out.set("application-identifier", plistString(teamID+"."+appID))
	if ent != nil {
		vars := map[string]string{
			"AppIdentifierPrefix":       teamID + ".",
			"TeamIdentifierPrefix":      teamID + ".",
			"DEVELOPMENT_TEAM":          teamID,
			"PRODUCT_BUNDLE_IDENTIFIER": appID,
		}
		for i, k := range ent.Keys {
			if err := expandBuildSettings(ent.Vals[i], vars); err != nil {
				return nil, fmt.Errorf("ios/%s: %s: %w", iosEntitlementsFile, k, err)
			}
			out.set(k, ent.Vals[i])
		}
	}
	return encodePlist(out), nil
}

// buildSettingRef matches an Xcode build-setting reference, $(NAME) or
// ${NAME}, with an optional :modifier.
var buildSettingRef = regexp.MustCompile(`\$[({]([A-Za-z0-9_]+)(:[^)}]*)?[)}]`)

// expandBuildSettings replaces the build-setting references in v's strings,
// recursively, with their values from vars.
func expandBuildSettings(v *plistValue, vars map[string]string) error {
	var unknown string
	if v.Kind == "string" {
		v.Text = buildSettingRef.ReplaceAllStringFunc(v.Text, func(ref string) string {
			m := buildSettingRef.FindStringSubmatch(ref)
			if val, ok := vars[m[1]]; ok && m[2] == "" {
				return val
			}
			if unknown == "" {
				unknown = ref
			}
			return ref
		})
	}
	if unknown != "" {
		return fmt.Errorf("%s is not a build setting `bellui release` can expand (it knows %s)", unknown, knownSettings(vars))
	}
	for _, c := range append(v.Vals, v.Items...) {
		if err := expandBuildSettings(c, vars); err != nil {
			return err
		}
	}
	return nil
}

func knownSettings(vars map[string]string) string {
	names := make([]string, 0, len(vars))
	for k := range vars {
		names = append(names, "$("+k+")")
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// iosFramework is one framework or library from ios/Frameworks, resolved to
// the slice that matches one build architecture.
type iosFramework struct {
	// Name is the framework's name ("Foo" for Foo.framework), or the library
	// file for a static library slice.
	Name string
	// Path is the slice: the .framework directory, or the .a file.
	Path string
	// Headers is a static library slice's header directory, or "".
	Headers string
	// Dynamic is true for a dynamic framework, which must be embedded in the
	// app and signed. Static code is linked into the binary only.
	Dynamic bool
}

// iosFrameworkSources lists ios/Frameworks/*.xcframework and *.framework,
// sorted.
func iosFrameworkSources(appDir string) ([]string, error) {
	dir := filepath.Join(appDir, iosExtrasDir, iosFrameworksDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		switch filepath.Ext(e.Name()) {
		case ".xcframework", ".framework":
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}

// xcframeworkArch maps a GOARCH to the architecture name xcframeworks use.
func xcframeworkArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "x86_64"
	case "386":
		return "i386"
	case "arm":
		return "armv7"
	}
	return goarch
}

// resolveIOSFramework picks src's slice for one architecture. simulator is
// true for an iossimulator build. A plain .framework is used as it is.
func resolveIOSFramework(src, goarch string, simulator bool) (iosFramework, error) {
	if filepath.Ext(src) == ".framework" {
		return frameworkSlice(src)
	}
	info, err := readPlistFile(filepath.Join(src, "Info.plist"))
	if err != nil {
		return iosFramework{}, fmt.Errorf("%s: %w", src, err)
	}
	libs := info.get("AvailableLibraries")
	if libs == nil || libs.Kind != "array" {
		return iosFramework{}, fmt.Errorf("%s: Info.plist has no AvailableLibraries", src)
	}
	arch := xcframeworkArch(goarch)
	wantVariant := ""
	if simulator {
		wantVariant = "simulator"
	}
	var have []string
	for _, lib := range libs.Items {
		id, path := lib.get("LibraryIdentifier"), lib.get("LibraryPath")
		platform, variant := lib.get("SupportedPlatform"), lib.get("SupportedPlatformVariant")
		if id == nil || path == nil || platform == nil {
			continue
		}
		have = append(have, id.Text)
		gotVariant := ""
		if variant != nil {
			gotVariant = variant.Text
		}
		if platform.Text != "ios" || gotVariant != wantVariant || !plistArrayHas(lib.get("SupportedArchitectures"), arch) {
			continue
		}
		slice := filepath.Join(src, id.Text, path.Text)
		switch {
		case strings.HasSuffix(path.Text, ".framework"):
			return frameworkSlice(slice)
		case strings.HasSuffix(path.Text, ".a"):
			f := iosFramework{Name: path.Text, Path: slice}
			if h := lib.get("HeadersPath"); h != nil {
				f.Headers = filepath.Join(src, id.Text, h.Text)
			}
			return f, nil
		default:
			return iosFramework{}, fmt.Errorf("%s: slice %s is %s; only frameworks and static libraries (.a) are supported",
				src, id.Text, path.Text)
		}
	}
	want := "ios " + arch
	if simulator {
		want += " simulator"
	}
	return iosFramework{}, fmt.Errorf("%s has no slice for %s (has %s)", src, want, strings.Join(have, ", "))
}

func plistArrayHas(a *plistValue, s string) bool {
	if a == nil {
		return false
	}
	for _, it := range a.Items {
		if it.Text == s {
			return true
		}
	}
	return false
}

// frameworkSlice describes a .framework directory, reading its binary to
// tell a dynamic framework from a static one.
func frameworkSlice(dir string) (iosFramework, error) {
	name := strings.TrimSuffix(filepath.Base(dir), ".framework")
	bin := filepath.Join(dir, name)
	dynamic, err := machODynamic(bin)
	if err != nil {
		return iosFramework{}, fmt.Errorf("%s: %w", dir, err)
	}
	return iosFramework{Name: name, Path: dir, Dynamic: dynamic}, nil
}

// machODynamic reports whether bin is a dynamic library (thin or fat), as
// opposed to a static archive.
func machODynamic(bin string) (bool, error) {
	data, err := os.ReadFile(bin)
	if err != nil {
		return false, err
	}
	if bytes.HasPrefix(data, []byte("!<arch>\n")) {
		return false, nil
	}
	if ff, err := macho.NewFatFile(bytes.NewReader(data)); err == nil {
		if len(ff.Arches) == 0 {
			return false, errors.New("empty universal binary")
		}
		return ff.Arches[0].Type == macho.TypeDylib, nil
	}
	f, err := macho.NewFile(bytes.NewReader(data))
	if err != nil {
		return false, fmt.Errorf("not a Mach-O binary or static archive: %w", err)
	}
	return f.Type == macho.TypeDylib, nil
}

// readPlistFile parses a plist file in XML or binary form (xcodebuild writes
// xcframework Info.plists as XML, but a hand-made one may be binary).
func readPlistFile(p string) (*plistValue, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	if bytes.HasPrefix(data, []byte("bplist")) {
		out, err := exec.Command("plutil", "-convert", "xml1", "-o", "-", p).Output()
		if err != nil {
			return nil, fmt.Errorf("plutil -convert xml1: %w", err)
		}
		data = out
	}
	return parsePlist(data)
}

// iosLinkFlags are the extra C compiler and linker flags one architecture's
// go build needs to compile against and link the resolved frameworks.
func iosLinkFlags(fws []iosFramework) (cflags, ldflags []string) {
	dynamic := false
	for _, f := range fws {
		switch {
		case strings.HasSuffix(f.Path, ".a"):
			if f.Headers != "" {
				cflags = append(cflags, "-I"+f.Headers)
			}
			ldflags = append(ldflags, f.Path)
		default:
			dir := filepath.Dir(f.Path)
			cflags = append(cflags, "-F"+dir)
			ldflags = append(ldflags, "-F"+dir, "-framework", f.Name)
			dynamic = dynamic || f.Dynamic
		}
	}
	if dynamic {
		ldflags = append(ldflags, "-Wl,-rpath,"+iosEmbeddedFrameworks)
	}
	return cflags, ldflags
}

// withIOSFlags appends cflags and ldflags to env's CGO_CFLAGS and CGO_LDFLAGS,
// which darwinEnv always sets. go splits those variables on spaces, so a flag
// holding one (an app under "~/My Projects") is quoted.
func withIOSFlags(env, cflags, ldflags []string) ([]string, error) {
	out := make([]string, len(env))
	copy(out, env)
	for i, kv := range out {
		var add []string
		switch {
		case strings.HasPrefix(kv, "CGO_CFLAGS="):
			add = cflags
		case strings.HasPrefix(kv, "CGO_LDFLAGS="):
			add = ldflags
		}
		for _, f := range add {
			q, err := quoteCgoFlag(f)
			if err != nil {
				return nil, err
			}
			kv += " " + q
		}
		out[i] = kv
	}
	return out, nil
}

// quoteCgoFlag quotes f the way go reads CGO_CFLAGS and CGO_LDFLAGS: a field
// in single or double quotes is taken as it is, with no escapes, so a flag
// holding both quote characters cannot be passed.
func quoteCgoFlag(f string) (string, error) {
	if f != "" && !strings.ContainsAny(f, " \t\n\r\v\f'\"") {
		return f, nil
	}
	switch {
	case !strings.Contains(f, "'"):
		return "'" + f + "'", nil
	case !strings.Contains(f, `"`):
		return `"` + f + `"`, nil
	}
	return "", fmt.Errorf("cannot pass %q to cgo: it holds both quote characters", f)
}

// iosEmbed is a dynamic framework the generated Xcode project copies into
// the app's Frameworks directory and signs (CodeSignOnCopy).
type iosEmbed struct {
	Name      string // Foo.framework
	FileRef   string // pbxproj object IDs, fixed per index
	BuildFile string
}

// resolveIOSFrameworks resolves every ios/Frameworks entry for each
// architecture. It returns per-arch flags and the dynamic frameworks to
// embed. All architectures must agree on the dynamic slice: one app bundle
// holds one copy of each framework.
func resolveIOSFrameworks(appDir string, archs []string, simulator bool) (map[string][]iosFramework, []iosFramework, error) {
	srcs, err := iosFrameworkSources(appDir)
	if err != nil || len(srcs) == 0 {
		return nil, nil, err
	}
	perArch := map[string][]iosFramework{}
	var embed []iosFramework
	for _, src := range srcs {
		var dyn *iosFramework
		for _, arch := range archs {
			f, err := resolveIOSFramework(src, arch, simulator)
			if err != nil {
				return nil, nil, err
			}
			perArch[arch] = append(perArch[arch], f)
			if !f.Dynamic {
				continue
			}
			if dyn != nil && dyn.Path != f.Path {
				return nil, nil, fmt.Errorf("%s: architectures %v need different slices (%s, %s); build one slice that holds them all",
					src, archs, dyn.Path, f.Path)
			}
			dyn = &f
		}
		if dyn != nil {
			embed = append(embed, *dyn)
		}
	}
	return perArch, embed, nil
}

// pbxEmbeds names the embedded frameworks' pbxproj objects. The IDs only
// need to be unique within the project and stable between builds.
func pbxEmbeds(fws []iosFramework) []iosEmbed {
	out := make([]iosEmbed, len(fws))
	for i, f := range fws {
		out[i] = iosEmbed{
			Name:      filepath.Base(f.Path),
			FileRef:   fmt.Sprintf("B5E1F0A0%016X", i),
			BuildFile: fmt.Sprintf("B5E1F0B0%016X", i),
		}
	}
	return out
}
