package mobile

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/belluikit/bellui/cmd/bellui/internal/mobile/binres"
	"github.com/belluikit/bellui/cmd/bellui/internal/util"
)

// Android library dependencies (#672).
//
// An app declares them by dropping files into <appdir>/android/libs/:
//
//   - *.jar: compiled classes. They are on the javac classpath for the app's
//     android/*.java and are merged into the APK's dex.
//   - *.aar: Android libraries. Their classes.jar (and any libs/*.jar inside)
//     are treated like a jar; jni/<abi>/*.so go into the APK's lib/<abi>/;
//     assets/ join the APK assets; res/ is compiled and linked with aapt2,
//     with an R class generated for every library package; and their
//     manifest's permissions, features, queries and <application> children
//     are merged into the app's manifest.
//
// There is no Maven resolution: the directory must hold the library's whole
// dependency closure.

const (
	androidNS = "http://schemas.android.com/apk/res/android"
	toolsNS   = "http://schemas.android.com/tools"
)

// androidLibs is what <appdir>/android/libs holds, with every AAR extracted.
type androidLibs struct {
	jars []string // plain jars plus the jars extracted from AARs
	aars []*aarLib
}

// aarLib is one extracted AAR.
type aarLib struct {
	name     string // file name, for messages
	dir      string // extraction directory
	pkg      string // the manifest's package attribute
	manifest []byte
}

func (l *androidLibs) empty() bool { return l == nil || (len(l.jars) == 0 && len(l.aars) == 0) }

func (l *androidLibs) hasAARs() bool { return l != nil && len(l.aars) > 0 }

// collectAndroidLibs reads <dir>/android/libs, extracting each AAR under
// work. A missing directory yields an empty set.
func collectAndroidLibs(dir, work string) (*androidLibs, error) {
	libDir := filepath.Join(dir, "android", "libs")
	entries, err := os.ReadDir(libDir)
	if os.IsNotExist(err) {
		return &androidLibs{}, nil
	}
	if err != nil {
		return nil, err
	}
	libs := &androidLibs{}
	for _, e := range entries { // os.ReadDir sorts by name
		if e.IsDir() {
			continue
		}
		p := filepath.Join(libDir, e.Name())
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".jar":
			libs.jars = append(libs.jars, p)
		case ".aar":
			aar, jars, err := extractAAR(p, filepath.Join(work, strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))))
			if err != nil {
				return nil, fmt.Errorf("android/libs/%s: %w", e.Name(), err)
			}
			libs.aars = append(libs.aars, aar)
			libs.jars = append(libs.jars, jars...)
		}
	}
	return libs, nil
}

// extractAAR unpacks an AAR and returns it with the jars it carries.
func extractAAR(path, dest string) (*aarLib, []string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(filepath.Separator)) {
			return nil, nil, fmt.Errorf("entry %q escapes the archive", f.Name)
		}
		if err := extractZipEntry(f, target); err != nil {
			return nil, nil, err
		}
	}

	aar := &aarLib{name: filepath.Base(path), dir: dest}
	aar.manifest, err = os.ReadFile(filepath.Join(dest, "AndroidManifest.xml"))
	if err != nil {
		return nil, nil, fmt.Errorf("no AndroidManifest.xml: %w", err)
	}
	aar.pkg, err = manifestPackage(aar.manifest)
	if err != nil {
		return nil, nil, err
	}

	var jars []string
	if p := filepath.Join(dest, "classes.jar"); util.Exists(p) {
		jars = append(jars, p)
	}
	inner, err := filepath.Glob(filepath.Join(dest, "libs", "*.jar"))
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(inner)
	return aar, append(jars, inner...), nil
}

func extractZipEntry(f *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// manifestPackage returns the package attribute of a manifest's root element.
func manifestPackage(manifest []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(manifest))
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", fmt.Errorf("parsing AndroidManifest.xml: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			for _, a := range se.Attr {
				if a.Name.Space == "" && a.Name.Local == "package" {
					return a.Value, nil
				}
			}
			return "", nil
		}
	}
}

// resDir returns the AAR's res directory when it holds any file, else "".
func (a *aarLib) resDir() string {
	return nonEmptyDir(filepath.Join(a.dir, "res"))
}

// hasRTxt reports whether the AAR lists any resource in R.txt.
func (a *aarLib) hasRTxt() bool {
	fi, err := os.Stat(filepath.Join(a.dir, "R.txt"))
	return err == nil && fi.Size() > 0
}

func nonEmptyDir(dir string) string {
	found := false
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	if found {
		return dir
	}
	return ""
}

// nativeLibs returns the AARs' shared libraries for the built ABIs, as APK
// path → source file. An AAR that ships native code but not for a built ABI is
// an error: the APK would install and then fail to load the library on exactly
// those devices. A clash with another library, or with the app's own Go
// library, is an error too.
func (l *androidLibs) nativeLibs(abis []string, goLib string) (map[string]string, error) {
	out := map[string]string{}
	if l == nil {
		return out, nil
	}
	owner := map[string]string{}
	for _, a := range l.aars {
		jni := filepath.Join(a.dir, "jni")
		shipped, err := os.ReadDir(jni)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var have []string
		for _, e := range shipped {
			if e.IsDir() {
				have = append(have, e.Name())
			}
		}
		if len(have) == 0 {
			continue
		}
		for _, abi := range abis {
			sos, err := filepath.Glob(filepath.Join(jni, abi, "*.so"))
			if err != nil {
				return nil, err
			}
			if len(sos) == 0 {
				return nil, fmt.Errorf("android/libs/%s has native libraries for %v but not for %s; "+
					"build only the targets it supports (e.g. -target android/arm64) or supply a build for %s",
					a.name, have, abi, abi)
			}
			for _, so := range sos {
				dst := "lib/" + abi + "/" + filepath.Base(so)
				if filepath.Base(so) == goLib {
					return nil, fmt.Errorf("android/libs/%s ships %s, which is the app's own library name", a.name, dst)
				}
				if prev, dup := owner[dst]; dup {
					return nil, fmt.Errorf("%s is shipped by both android/libs/%s and android/libs/%s", dst, prev, a.name)
				}
				owner[dst] = a.name
				out[dst] = so
			}
		}
	}
	return out, nil
}

// writeLibraryAssets adds every AAR's assets/ to the APK. A path the app's own
// assets (or an earlier library) already wrote is skipped with a warning: the
// app wins.
func (l *androidLibs) writeLibraryAssets(apkw *Writer, written map[string]bool) error {
	if l == nil {
		return nil
	}
	for _, a := range l.aars {
		root := filepath.Join(a.dir, "assets")
		if !util.Exists(root) {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			name := "assets/" + filepath.ToSlash(rel)
			if written[name] {
				fmt.Fprintf(os.Stderr, "bellui: warning: android/libs/%s: %s is already provided, keeping the existing file\n", a.name, name)
				return nil
			}
			written[name] = true
			return apkwWriteFile(name, path, apkw)
		})
		if err != nil {
			return fmt.Errorf("android/libs/%s assets: %w", a.name, err)
		}
	}
	return nil
}

// manifestElem is one element lifted out of a manifest, keeping only the
// attributes that survive the merge (prefixed android: or unprefixed), with
// ${applicationId} and relative class names resolved. An element with an
// empty name is a run of text.
type manifestElem struct {
	name     string
	attrs    [][2]string // prefixed name, value
	text     string
	children []*manifestElem
}

// key identifies an element for deduplication: its name and android:name, or
// "" when it has none.
func (e *manifestElem) key() string {
	for _, a := range e.attrs {
		if a[0] == "android:name" && a[1] != "" {
			return e.name + " " + a[1]
		}
	}
	return ""
}

func (e *manifestElem) write(b *strings.Builder) {
	if e.name == "" {
		_ = xml.EscapeText(b, []byte(e.text))
		return
	}
	b.WriteString("<" + e.name)
	for _, a := range e.attrs {
		b.WriteString(" " + a[0] + `="`)
		_ = xml.EscapeText(b, []byte(a[1]))
		b.WriteString(`"`)
	}
	b.WriteString(">")
	for _, c := range e.children {
		c.write(b)
	}
	b.WriteString("</" + e.name + ">")
}

func (e *manifestElem) String() string {
	var b strings.Builder
	e.write(&b)
	return b.String()
}

// mergeChildren adds other's children to e, the way the Gradle manifest
// merger joins two libraries' declarations of one component: a named child e
// already has is merged in turn, an unnamed one is added unless e holds an
// identical one, and e's attributes stay.
func (e *manifestElem) mergeChildren(other *manifestElem) {
	for _, c := range other.children {
		if c.name == "" {
			continue
		}
		var match *manifestElem
		k, text := c.key(), c.String()
		for _, have := range e.children {
			if (k != "" && have.key() == k) || (k == "" && have.String() == text) {
				match = have
				break
			}
		}
		switch {
		case match == nil:
			e.children = append(e.children, c)
		case k != "":
			match.mergeChildren(c)
		}
	}
}

// mergeLibraryManifests copies each AAR manifest's uses-permission,
// uses-permission-sdk-23, uses-feature and queries elements, and the children
// of its <application>, into the app manifest. ${applicationId} becomes the
// app's package, and a relative component class name (".Foo") is qualified
// with the library's package. An element whose android:name the app already
// declares is skipped, so the app's own entry wins; one that two libraries
// declare is kept once, with the children of both (androidx.startup's
// InitializationProvider carries one meta-data per initializer, from each
// library that has one). tools:* attributes are dropped: there are no merge
// directives, and <application> attributes from libraries (e.g.
// appComponentFactory) are not merged.
func mergeLibraryManifests(appManifest []byte, libs *androidLibs) ([]byte, error) {
	if !libs.hasAARs() {
		return appManifest, nil
	}
	appPkg, err := manifestPackage(appManifest)
	if err != nil {
		return nil, err
	}
	topSeen, appSeen, err := liftManifestElems(appManifest, appPkg, appPkg)
	if err != nil {
		return nil, fmt.Errorf("app AndroidManifest.xml: %w", err)
	}
	fromApp := map[string]bool{}
	for _, e := range append(topSeen, appSeen...) {
		if k := e.key(); k != "" {
			fromApp[k] = true
		}
	}

	fromLibs := map[string]*manifestElem{}
	var top, app []*manifestElem
	for _, a := range libs.aars {
		t, ap, err := liftManifestElems(a.manifest, appPkg, a.pkg)
		if err != nil {
			return nil, fmt.Errorf("android/libs/%s AndroidManifest.xml: %w", a.name, err)
		}
		for _, list := range []struct {
			elems []*manifestElem
			into  *[]*manifestElem
		}{{t, &top}, {ap, &app}} {
			for _, e := range list.elems {
				k := e.key()
				switch {
				case k == "":
				case fromApp[k]:
					continue
				case fromLibs[k] != nil:
					fromLibs[k].mergeChildren(e)
					continue
				default:
					fromLibs[k] = e
				}
				*list.into = append(*list.into, e)
			}
		}
	}

	join := func(elems []*manifestElem) string {
		var b strings.Builder
		for _, e := range elems {
			b.WriteString("\n\t")
			e.write(&b)
		}
		return b.String()
	}
	out := string(appManifest)
	if len(app) > 0 {
		i := strings.LastIndex(out, "</application>")
		if i < 0 {
			return nil, fmt.Errorf("library manifests add <application> entries, but the app manifest has no </application> to put them in")
		}
		out = out[:i] + join(app) + "\n\t" + out[i:]
	}
	if len(top) > 0 {
		i := strings.LastIndex(out, "</manifest>")
		if i < 0 {
			return nil, fmt.Errorf("app manifest has no </manifest>")
		}
		out = out[:i] + join(top) + "\n" + out[i:]
	}
	return []byte(out), nil
}

var liftedTopLevel = map[string]bool{
	"uses-permission":        true,
	"uses-permission-sdk-23": true,
	"uses-feature":           true,
	"queries":                true,
}

// liftManifestElems returns the mergeable top-level elements and the
// <application> children of a manifest. appPkg replaces ${applicationId} in
// attribute values; relative class names resolve against pkg, the package of
// the manifest being read.
func liftManifestElems(manifest []byte, appPkg, pkg string) (top, app []*manifestElem, err error) {
	dec := xml.NewDecoder(bytes.NewReader(manifest))
	var path []string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return top, app, nil
		}
		if err != nil {
			return nil, nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth := len(path)
			inApp := depth == 2 && path[1] == "application"
			if (depth == 1 && liftedTopLevel[t.Name.Local]) || inApp {
				e, err := readManifestElem(dec, t, appPkg, pkg)
				if err != nil {
					return nil, nil, err
				}
				if inApp {
					app = append(app, e)
				} else {
					top = append(top, e)
				}
				continue
			}
			path = append(path, t.Name.Local)
		case xml.EndElement:
			path = path[:len(path)-1]
		}
	}
}

// readManifestElem reads start and everything up to its matching end.
func readManifestElem(dec *xml.Decoder, start xml.StartElement, appPkg, pkg string) (*manifestElem, error) {
	e := newManifestElem(start, appPkg, pkg)
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			c, err := readManifestElem(dec, t, appPkg, pkg)
			if err != nil {
				return nil, err
			}
			e.children = append(e.children, c)
		case xml.EndElement:
			return e, nil
		case xml.CharData:
			if s := strings.TrimSpace(string(t)); s != "" {
				e.children = append(e.children, &manifestElem{text: s})
			}
		}
	}
}

// classNameAttrs are the attributes, per element, that name a class and so
// may be relative to the manifest's package.
var classNameAttrs = map[string][]string{
	"activity":       {"name", "parentActivityName"},
	"activity-alias": {"name", "targetActivity"},
	"service":        {"name"},
	"receiver":       {"name"},
	"provider":       {"name"},
}

func newManifestElem(se xml.StartElement, appPkg, pkg string) *manifestElem {
	e := &manifestElem{name: se.Name.Local}
	for _, a := range se.Attr {
		var name string
		switch a.Name.Space {
		case "":
			if a.Name.Local == "xmlns" {
				continue
			}
			name = a.Name.Local
		case androidNS:
			name = "android:" + a.Name.Local
		case "xmlns", toolsNS:
			continue
		default:
			fmt.Fprintf(os.Stderr, "bellui: warning: dropping library manifest attribute %s:%s on <%s> (unsupported namespace)\n",
				a.Name.Space, a.Name.Local, se.Name.Local)
			continue
		}
		v := strings.ReplaceAll(a.Value, "${applicationId}", appPkg)
		if a.Name.Space == androidNS && slices.Contains(classNameAttrs[se.Name.Local], a.Name.Local) {
			v = qualifyClassName(v, pkg)
		}
		e.attrs = append(e.attrs, [2]string{name, v})
	}
	return e
}

// qualifyClassName resolves a manifest class name the way Android does: one
// that starts with a dot, or has none, is relative to the manifest's package.
// Once a library's entry sits in the app manifest, the app's package would
// otherwise be the one it resolves against.
func qualifyClassName(name, pkg string) string {
	switch {
	case pkg == "" || name == "":
		return name
	case strings.HasPrefix(name, "."):
		return pkg + name
	case !strings.Contains(name, "."):
		return pkg + "." + name
	}
	return name
}

// linkedResources is the output of linking the app's and the libraries'
// resources with aapt2.
type linkedResources struct {
	arsc, resDir, manifest string // resources.arsc, res/ tree, binary manifest
	rJava                  []string
}

// linkResourcesWithLibs compiles and links the app's resources (its icon)
// together with every AAR's res/, generating an R class for each library
// package. Library resources are overlays in file-name order and the app's
// come last, so the app overrides a library resource of the same name.
// legacyIcon, when set (and there is no adaptive icon), becomes
// @mipmap/icon, the same resource the binres path writes.
func linkResourcesWithLibs(work string, manifest []byte, libs *androidLibs, legacyIcon, iconFG, iconBG, iconMono string,
	target, versionCode int, versionName string,
) (*linkedResources, error) {
	aapt2, err := util.Aapt2Path()
	if err != nil {
		return nil, err
	}
	androidJar, err := binres.LatestAPIResourcesPath()
	if err != nil || !util.Exists(androidJar) {
		return nil, fmt.Errorf("android.jar not found for API %d at %s", binres.MinSDK, androidJar)
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		return nil, err
	}

	appRes := filepath.Join(work, "appres")
	manifest, err = writeAppIconRes(appRes, manifest, legacyIcon, iconFG, iconBG, iconMono)
	if err != nil {
		return nil, err
	}

	run := func(args ...string) error {
		cmd := exec.Command(aapt2, args...)
		if buildX {
			printcmd("%s %s", aapt2, strings.Join(args, " "))
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("aapt2 %s: %w\n%s", args[0], err, out)
		}
		return nil
	}

	appPkg, _ := manifestPackage(manifest)
	overlays, extraPkgs, err := compileLibraryRes(run, work, libs, appPkg)
	if err != nil {
		return nil, err
	}
	if nonEmptyDir(appRes) != "" {
		out := filepath.Join(work, "app.zip")
		if err := run("compile", "--dir", appRes, "-o", out); err != nil {
			return nil, err
		}
		overlays = append(overlays, out)
	}

	manifestPath := filepath.Join(work, "AndroidManifest.xml")
	if err := os.WriteFile(manifestPath, manifest, 0o644); err != nil {
		return nil, err
	}
	gen := filepath.Join(work, "gen")
	apk := filepath.Join(work, "resources.apk")
	args := []string{
		"link", "-o", apk,
		"--manifest", manifestPath,
		"--min-sdk-version", fmt.Sprint(binres.MinSDK),
		"--target-sdk-version", fmt.Sprint(target),
		"--version-code", fmt.Sprint(versionCode),
		"--version-name", versionName,
		"-I", androidJar,
		"--auto-add-overlay",
		"--java", gen,
	}
	if len(extraPkgs) > 0 {
		args = append(args, "--extra-packages", strings.Join(extraPkgs, ":"))
	}
	for _, o := range overlays {
		args = append(args, "-R", o)
	}
	if err := run(args...); err != nil {
		return nil, err
	}

	return collectLinkOutputs(work, apk, gen)
}

// collectLinkOutputs pulls the table, res/ tree and binary manifest out of
// aapt2's linked APK, and lists the R sources it generated.
func collectLinkOutputs(work, apk, gen string) (*linkedResources, error) {
	linked := &linkedResources{
		arsc:     filepath.Join(work, "resources.arsc"),
		resDir:   filepath.Join(work, "linked_res"),
		manifest: filepath.Join(work, "AndroidManifest.bin.xml"),
	}
	if err := extractFileFromZip(apk, "resources.arsc", linked.arsc); err != nil {
		return nil, err
	}
	if err := extractFileFromZip(apk, "AndroidManifest.xml", linked.manifest); err != nil {
		return nil, err
	}
	if err := extractDirFromZip(apk, "res/", linked.resDir); err != nil {
		return nil, err
	}
	err := filepath.WalkDir(gen, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".java") {
			linked.rJava = append(linked.rJava, p)
		}
		return err
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	sort.Strings(linked.rJava)
	return linked, nil
}

// writeAppIconRes writes the app's icon resources under appRes: the adaptive
// layers when there are any, else the legacy icon as @mipmap/icon, which it
// also names in the manifest.
func writeAppIconRes(appRes string, manifest []byte, legacyIcon, iconFG, iconBG, iconMono string) ([]byte, error) {
	if err := os.MkdirAll(appRes, 0o755); err != nil {
		return nil, err
	}
	switch {
	case iconFG != "":
		if iconBG == "" {
			iconBG = iconFG
		}
		return manifest, writeAdaptiveIconResources(appRes, iconFG, iconBG, iconMono)
	case legacyIcon != "":
		mip := filepath.Join(appRes, "mipmap-xxxhdpi")
		if err := os.MkdirAll(mip, 0o755); err != nil {
			return nil, err
		}
		if err := util.CopyFile(legacyIcon, filepath.Join(mip, "icon.png")); err != nil {
			return nil, err
		}
		return withApplicationIcon(manifest, "@mipmap/icon"), nil
	}
	return manifest, nil
}

// compileLibraryRes compiles each AAR's res/ into a zip of flat files and
// returns them in order, with the library packages that need an R class: those
// with resources, and those whose R.txt lists any (transitive R).
func compileLibraryRes(run func(...string) error, work string, libs *androidLibs, appPkg string) (overlays, extraPkgs []string, err error) {
	for i, a := range libs.aars {
		res := a.resDir()
		if res == "" {
			// No resources of its own, but an AAR built with transitive R
			// classes still reads its dependencies' resources through its own
			// R, which R.txt lists.
			if a.pkg != "" && a.pkg != appPkg && a.hasRTxt() && !slices.Contains(extraPkgs, a.pkg) {
				extraPkgs = append(extraPkgs, a.pkg)
			}
			continue
		}
		if a.pkg == "" {
			return nil, nil, fmt.Errorf("android/libs/%s has resources but its manifest names no package", a.name)
		}
		out := filepath.Join(work, fmt.Sprintf("lib%02d.zip", i))
		if err := run("compile", "--dir", res, "-o", out); err != nil {
			return nil, nil, fmt.Errorf("android/libs/%s: %w", a.name, err)
		}
		overlays = append(overlays, out)
		if a.pkg != appPkg && !slices.Contains(extraPkgs, a.pkg) {
			extraPkgs = append(extraPkgs, a.pkg)
		}
	}
	return overlays, extraPkgs, nil
}

// withApplicationIcon adds android:icon to <application> unless the manifest
// already sets one.
func withApplicationIcon(manifest []byte, icon string) []byte {
	s := string(manifest)
	i := strings.Index(s, "<application")
	if i < 0 {
		return manifest
	}
	end := strings.Index(s[i:], ">")
	if end < 0 || strings.Contains(s[i:i+end], "android:icon=") {
		return manifest
	}
	at := i + len("<application")
	return []byte(s[:at] + ` android:icon="` + icon + `"` + s[at:])
}

// writeLinkedResources puts aapt2's link output into the APK.
func writeLinkedResources(apkw *Writer, linked *linkedResources) error {
	if err := apkwWriteFile("resources.arsc", linked.arsc, apkw); err != nil {
		return err
	}
	err := filepath.WalkDir(linked.resDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(linked.resDir, path)
		if err != nil {
			return err
		}
		return apkwWriteFile("res/"+filepath.ToSlash(rel), path, apkw)
	})
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return apkwWriteFile("AndroidManifest.xml", linked.manifest, apkw)
}

// preparedLibs is the build's library state: the libraries themselves, the
// native libraries copied under tmpdir (as APK paths, for buildAPK), and the
// linked resources when any AAR is present.
type preparedLibs struct {
	libs        *androidLibs
	nativeFiles []string
	linked      *linkedResources
}

func (p *preparedLibs) rJava() []string {
	if p.linked == nil {
		return nil
	}
	return p.linked.rJava
}

// prepareAndroidLibs collects android/libs and does everything that must
// precede dexing: the manifest merge and the aapt2 link (which writes the R
// sources), and the native libraries' placement. Without an AAR the resource
// path is left untouched, so an app with no libraries (or only jars) packages
// exactly as before.
func prepareAndroidLibs(dir string, manifest []byte, archs []string, libName, iconPath, iconFG, iconBG, iconMono string,
	target, build int, version string,
) (*preparedLibs, error) {
	if buildN {
		// -n only prints commands. Every step here extracts files or runs an
		// SDK tool, and tmpdir is the literal "$WORK", not a directory.
		if util.Exists(filepath.Join(dir, "android", "libs")) {
			printcmd("# package android/libs: extract AARs, merge manifests, aapt2 link (not run under -n)")
		}
		return &preparedLibs{libs: &androidLibs{}}, nil
	}
	libs, err := collectAndroidLibs(dir, filepath.Join(tmpdir, "androidlibs"))
	if err != nil {
		return nil, err
	}
	p := &preparedLibs{libs: libs}
	if !libs.hasAARs() {
		return p, nil
	}

	abis := make([]string, len(archs))
	for i, arch := range archs {
		abis[i] = ndk.Toolchain(arch).abi
	}
	natives, err := libs.nativeLibs(abis, "lib"+libName+".so")
	if err != nil {
		return nil, err
	}
	for dst, src := range natives {
		if err := os.MkdirAll(filepath.Join(tmpdir, filepath.Dir(dst)), 0o755); err != nil {
			return nil, err
		}
		if err := util.CopyFile(src, filepath.Join(tmpdir, dst)); err != nil {
			return nil, err
		}
		p.nativeFiles = append(p.nativeFiles, dst)
	}
	sort.Strings(p.nativeFiles)

	merged, err := mergeLibraryManifests(manifest, libs)
	if err != nil {
		return nil, err
	}
	if buildV {
		fmt.Fprintf(os.Stderr, "merged AndroidManifest.xml:\n%s\n", merged)
	}
	fg, bg, mono := detectAdaptiveIcons(dir, iconFG, iconBG, iconMono)
	p.linked, err = linkResourcesWithLibs(filepath.Join(tmpdir, "libres"), merged, libs, appLegacyIcon(dir, iconPath),
		fg, bg, mono, target, build, version)
	return p, err
}

// appLegacyIcon is the single-icon path addAssets would use: assets/icon.png
// when the app ships one, else iconPath.
func appLegacyIcon(dir, iconPath string) string {
	assets, err := filepath.EvalSymlinks(filepath.Join(dir, "assets"))
	if err == nil {
		if p := filepath.Join(assets, "icon.png"); util.Exists(p) {
			return p
		}
	}
	return iconPath
}
