// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mobile

import (
	"bytes"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/belluikit/bellui/cmd/bellui/internal/mobile/binres"
	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
	"github.com/belluikit/bellui/cmd/bellui/internal/util"
	"golang.org/x/tools/go/packages"
)

type manifestTmplData struct {
	JavaPkgPath  string
	Name         string
	Debug        bool
	LibName      string
	Version      string
	Build        int
	AdaptiveIcon bool
	URLSchemes   []string
	MimeTypes    []string
	Credit       string // the default icon's attribution, as a meta-data entry
}

// androidManifestData returns the AndroidManifest.xml bytes for the build: the
// project's own manifest if present (returning the library name parsed from it),
// otherwise one generated from the template. Split out of goAndroidBuild to keep
// that function under the cyclomatic-complexity cap.
func androidManifestData(dir, bundleID, appName, libName, version string, build int,
	iconFG, iconBG, iconMono string,
) (manifestData []byte, outLibName string, err error) {
	manifestPath := filepath.Join(dir, "AndroidManifest.xml")
	manifestData, err = os.ReadFile(filepath.Clean(manifestPath))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, libName, err
		}

		foreground, _, _ := detectAdaptiveIcons(dir, iconFG, iconBG, iconMono)
		adaptive := foreground != "" && util.Exists(foreground)

		buf := new(bytes.Buffer)
		buf.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
		err = templates.ManifestAndroid.Execute(buf, manifestTmplData{
			JavaPkgPath:  bundleID,
			Name:         strings.Title(appName), //lint:ignore SA1019 It is fine for our uses.
			Debug:        !buildRelease,
			LibName:      libName,
			Version:      version,
			Build:        build,
			AdaptiveIcon: adaptive,
			URLSchemes:   buildURLSchemes,
			MimeTypes:    buildFileMimeTypes,
			Credit:       buildIconCredit,
		})
		if err != nil {
			return nil, libName, err
		}
		manifestData = buf.Bytes()
		if buildV {
			fmt.Fprintf(os.Stderr, "generated AndroidManifest.xml:\n%s\n", manifestData)
		}
		return manifestData, libName, nil
	}
	outLibName, err = manifestLibName(manifestData)
	if err != nil {
		return nil, libName, fmt.Errorf("error parsing %s: %v", manifestPath, err)
	}
	return manifestData, outLibName, nil
}

// androidPageSizeLDFLAG links native libraries with 16 KB page alignment.
// Devices with 16 KB pages cannot load a library aligned for 4 KB, and Google
// Play requires the alignment for apps targeting Android 15 and up. NDKs before
// r28 link for 4 KB pages by default; the flag is harmless on later ones
// (#626).
const androidPageSizeLDFLAG = "-Wl,-z,max-page-size=16384"

// androidLinkEnv returns the CGO_LDFLAGS an Android build runs with: the
// user's own, then the page alignment. Every build gets it, not only release:
// an APK installed while testing runs on the same devices.
func androidLinkEnv(userLDFLAGS string) []string {
	return []string{"CGO_LDFLAGS=" + strings.TrimSpace(userLDFLAGS+" "+androidPageSizeLDFLAG)}
}

func goAndroidBuild(pkg *packages.Package, bundleID string, androidArchs []string,
	iconPath, appName, version string, build, target int, release, distribution bool, iconFG, iconBG, iconMono string,
) (map[string]bool, error) {
	env := androidLinkEnv(os.Getenv("CGO_LDFLAGS"))

	ndkRoot, err := ndkRoot()
	if err != nil {
		return nil, err
	}
	libName := androidPkgName(appName)

	// TODO(hajimehoshi): This works only with Go tools that assume all source files are in one directory.
	// Fix this to work with other Go tools.
	dir := filepath.Dir(pkg.GoFiles[0])

	var manifestData []byte
	manifestData, libName, err = androidManifestData(dir, bundleID, appName, libName, version, build, iconFG, iconBG, iconMono)
	if err != nil {
		return nil, err
	}

	// Library dependencies from android/libs (#672): merged into the
	// manifest, linked with aapt2 and dexed below.
	libs, err := prepareAndroidLibs(dir, manifestData, androidArchs, libName, iconPath, iconFG, iconBG, iconMono,
		target, build, version)
	if err != nil {
		return nil, err
	}

	libFiles := []string{}
	nmpkgs := make(map[string]map[string]bool) // map: arch -> extractPkgs' output

	for _, arch := range androidArchs {
		toolchain := ndk.Toolchain(arch)
		libPath := "lib/" + toolchain.abi + "/lib" + libName + ".so"
		libAbsPath := filepath.Join(tmpdir, libPath)
		if err := mkdir(filepath.Dir(libAbsPath)); err != nil {
			return nil, err
		}
		// If building release and no ldflags are set then remove the useless debug and DWARF build options
		if release && buildLdflags == "" {
			buildLdflags = "-w" // gomobile requires symbol check, so "-s" cannot be used yet - TODO resolve this
		}
		err = goBuild(
			pkg.PkgPath,
			append(env, androidEnv[arch]...),
			"-buildmode=c-shared",
			"-o", libAbsPath,
		)
		if err != nil {
			return nil, err
		}
		nmpkgs[arch], err = extractPkgs(toolchain.Path(ndkRoot, "nm"), libAbsPath)
		if err != nil {
			return nil, err
		}
		libFiles = append(libFiles, libPath)
	}
	libFiles = append(libFiles, libs.nativeFiles...)

	// Only a distribution build ("bellui release") produces the .aab store
	// bundle; a --release package stays an installable .apk (#144).
	ext := ".apk"
	if distribution {
		ext = ".aab"
	}
	if buildO == "" {
		buildO = androidPkgName(appName) + ext
	}
	if !strings.HasSuffix(buildO, ext) {
		return nil, fmt.Errorf("output file name %q does not end in '%s", buildO, ext)
	}

	var out io.Writer
	if !buildN {
		f, err := os.Create(buildO[:len(buildO)-3] + "apk")
		if err != nil {
			return nil, err
		}
		defer func() {
			if cerr := f.Close(); err == nil {
				err = cerr
			}
		}()
		out = f
	}

	appDex, err := mergeAppJavaDex(dir, libs.libs.jars, libs.rJava())
	if err != nil {
		return nil, err
	}

	apkw, err := buildAPK(out, nmpkgs, libFiles, androidArchs, appDex)
	if err != nil {
		return nil, err
	}
	err = addAssets(apkw, manifestData, dir, iconPath, target, build, version, iconFG, iconBG, iconMono, libs)
	if err != nil {
		return nil, err
	}

	// TODO: add gdbserver to apk?

	if !buildN {
		if err := apkw.Close(); err != nil {
			return nil, err
		}
	}
	if !distribution && !buildN {
		// The built-in Writer only applies an APK Signature Scheme v1 (JAR)
		// signature, which Android 11+ (API >= 30) rejects at install. Re-sign
		// the debug APK with apksigner (v1+v2) so `bellui package` output installs
		// directly on modern devices — required for the plain debug build
		// (targetSdk 34) and just as much for a --release one (targetSdk 35,
		// non-debuggable, still debug-key-signed so it sideloads, #144). Only
		// a distribution build skips this: its signing belongs to the caller's
		// keystore in `bellui release`.
		apkPath := buildO[:len(buildO)-3] + "apk"
		if err := resignDebugAPK(apkPath); err != nil {
			return nil, err
		}
	}
	if distribution {
		_, err := exec.LookPath("bundletool")
		if err != nil {
			_, _ = fmt.Fprint(os.Stderr, "Required command 'bundletool' not found when building Android for release.\n")
			_, _ = fmt.Fprint(os.Stderr, "For more information see https://developer.android.com/tools/bundletool.\n")
			return nil, fmt.Errorf("bundletool: command not found")
		}
		err = convertAPKToAAB(buildO)
		if err != nil {
			return nil, err
		}
	}

	// TODO: return nmpkgs
	return nmpkgs[androidArchs[0]], nil
}

// detectAdaptiveIcons checks for adaptive icon layers based on metadata or convention
// Returns: foreground path, background path, monochrome path (empty strings if not found)
func detectAdaptiveIcons(dir, foreground, background, monochrome string) (string, string, string) {
	if !util.Exists(foreground) {
		foreground = filepath.Join(dir, "Icon-foreground.png")
	}
	if !util.Exists(background) {
		background = filepath.Join(dir, "Icon-background.png")
	}
	if !util.Exists(monochrome) {
		monochrome = filepath.Join(dir, "Icon-monochrome.png")
	}

	if !util.Exists(foreground) {
		return "", "", ""
	}

	fg := foreground
	bg := ""
	if util.Exists(background) {
		bg = background
	}
	mono := ""
	if util.Exists(monochrome) {
		mono = monochrome
	}
	return fg, bg, mono
}

func addAssets(apkw *Writer, manifestData []byte, dir, iconPath string, target int, versionCode int,
	versionName, iconFG, iconBG, iconMono string, libs *preparedLibs,
) error {
	written := map[string]bool{}
	// Add any assets.
	var arsc struct {
		iconPath string
	}
	arsc.iconPath = iconPath
	assetsDir := filepath.Join(dir, "assets")
	assetsDirExists := true
	fi, err := os.Stat(assetsDir)
	if err != nil {
		if os.IsNotExist(err) {
			assetsDirExists = false
		} else {
			return err
		}
	} else {
		assetsDirExists = fi.IsDir()
	}
	if assetsDirExists {
		// if assets is a symlink, follow the symlink.
		assetsDir, err = filepath.EvalSymlinks(assetsDir)
		if err != nil {
			return err
		}
		err = filepath.WalkDir(assetsDir, func(path string, info fs.DirEntry, err error) error {
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

			if rel, err := filepath.Rel(assetsDir, path); rel == "icon.png" && err == nil {
				arsc.iconPath = path
				// TODO returning here does not write the assets/icon.png to the final assets output,
				// making it unavailable via the assets API. Should the file be duplicated into assets
				// or should assets API be able to retrieve files from the generated resource table?
				return nil
			}

			name := "assets/" + filepath.ToSlash(path[len(assetsDir)+1:])
			written[name] = true
			return apkwWriteFile(name, path, apkw)
		})
		if err != nil {
			return fmt.Errorf("asset %v", err)
		}
	}

	if err := libs.libs.writeLibraryAssets(apkw, written); err != nil {
		return err
	}
	if libs.linked != nil {
		// Library resources were linked with the app's before dexing, so
		// their R classes could be compiled in (#672).
		return writeLinkedResources(apkw, libs.linked)
	}

	iconForeground, iconBackground, iconMonochrome := detectAdaptiveIcons(dir, iconFG, iconBG, iconMono)

	// No adaptive icons, use legacy build
	if iconForeground == "" {
		return legacyAddAssets(apkw, manifestData, arsc.iconPath, target)
	}

	// Use iconForeground as background if no separate background provided
	if iconBackground == "" {
		iconBackground = iconForeground
	}

	// Compile adaptive icon resources with aapt2
	arscPath, resDir, compiledManifestPath, err := compileAndroidResources(
		tmpdir,
		manifestData,
		iconForeground,
		iconBackground,
		iconMonochrome,
		target,
		versionCode,
		versionName,
	)
	if err != nil {
		return fmt.Errorf("failed to compile adaptive icon resources: %w", err)
	}

	w, err := apkwCreate("resources.arsc", apkw)
	if err != nil {
		return err
	}
	arscData, err := os.ReadFile(arscPath)
	if err != nil {
		return err
	}
	if _, err := w.Write(arscData); err != nil {
		return err
	}

	err = filepath.Walk(resDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(resDir, path)
		if err != nil {
			return err
		}
		return apkwWriteFile("res/"+relPath, path, apkw)
	})
	if err != nil {
		return fmt.Errorf("failed to write res directory: %w", err)
	}

	return apkwWriteFile("AndroidManifest.xml", compiledManifestPath, apkw)
}

func legacyAddAssets(apkw *Writer, manifestData []byte, iconPath string, target int) error {
	// Legacy single icon mode using binres
	bxml, err := binres.UnmarshalXML(bytes.NewReader(manifestData), iconPath != "", target)
	if err != nil {
		return err
	}

	// generate resources.arsc identifying single xxxhdpi icon resource.
	if iconPath != "" {
		pkgname, err := bxml.RawValueByName("manifest", xml.Name{Local: "package"})
		if err != nil {
			return err
		}
		tbl, name := binres.NewMipmapTable(pkgname)
		if err := apkwWriteFile(name, iconPath, apkw); err != nil {
			return err
		}
		w, err := apkwCreate("resources.arsc", apkw)
		if err != nil {
			return err
		}
		bin, err := tbl.MarshalBinary()
		if err != nil {
			return err
		}
		if _, err := w.Write(bin); err != nil {
			return err
		}
	}

	w, err := apkwCreate("AndroidManifest.xml", apkw)
	if err != nil {
		return err
	}
	bin, err := bxml.MarshalBinary()
	if err != nil {
		return err
	}
	if _, err := w.Write(bin); err != nil {
		return err
	}
	return nil
}

func buildAPK(out io.Writer, nmpkgs map[string]map[string]bool, libFiles []string, androidArchs []string, appDex []string) (*Writer, error) {
	block, _ := pem.Decode([]byte(debugCert))
	if block == nil {
		return nil, errors.New("no debug cert")
	}
	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	var apkw *Writer
	if !buildN {
		apkw = NewWriter(out, privKey)
	}

	if len(appDex) > 0 {
		// The app shipped android/*.java or android/libs; use the merged dex
		// (built-in + app + libraries), which d8 may split over several files.
		for _, dex := range appDex {
			if err := apkwWriteFile(filepath.Base(dex), dex, apkw); err != nil {
				return nil, err
			}
		}
	} else {
		w, err := apkwCreate("classes.dex", apkw)
		if err != nil {
			return nil, err
		}
		dexData, err := base64.StdEncoding.DecodeString(dexStr)
		if err != nil {
			log.Fatalf("internal error bad dexStr: %v", err)
		}
		if _, err := w.Write(dexData); err != nil {
			return nil, err
		}
	}

	for _, libFile := range libFiles {
		if err := apkwWriteFile(libFile, filepath.Join(tmpdir, libFile), apkw); err != nil {
			return nil, err
		}
	}

	for _, arch := range androidArchs {
		toolchain := ndk.Toolchain(arch)
		if nmpkgs[arch]["github.com/belluikit/bellui/internal/driver/mobile/exp/audio/al"] {
			dst := "lib/" + toolchain.abi + "/libopenal.so"
			src := filepath.Join(gomobilepath, dst)
			if _, err := os.Stat(src); err != nil {
				return nil, errors.New("the Android requires the github.com/belluikit/bellui/internal/driver/mobile/exp/audio/al, but the OpenAL libraries was not found. Please run gomobile init with the -openal Flag pointing to an OpenAL source directory")
			}
			if err := apkwWriteFile(dst, src, apkw); err != nil {
				return nil, err
			}
		}
	}
	return apkw, nil
}

// resignDebugAPK adds an APK Signature Scheme v2 (+v1) signature to the debug
// APK with apksigner, using the built-in debug key. The internal Writer only
// produces a v1 (JAR) signature, which Android 11+ (API >= 30) refuses to
// install ("No signature found in package of version 2 or newer"). Re-signing
// makes `bellui package` output install directly on modern devices.
//
// It is a no-op that only warns (does not fail the build) when the Android
// build-tools are unavailable, so builds on machines without an SDK still work.
func resignDebugAPK(apkPath string) error {
	buildTools := util.AndroidBuildToolsPath()
	apksigner := filepath.Join(buildTools, "apksigner")
	if !util.Exists(apksigner) {
		if p, err := exec.LookPath("apksigner"); err == nil {
			apksigner = p
		} else {
			fmt.Fprintln(os.Stderr, "bellui: warning: apksigner not found in the Android build-tools; "+
				"the debug APK carries only a v1 signature and will not install on Android 11+ (API >= 30). "+
				"Install the Android build-tools, or re-sign the APK with apksigner.")
			return nil
		}
	}

	keyPath, certPath, cleanup, err := writeDebugSigningFiles()
	if err != nil {
		return err
	}
	defer cleanup()

	// apksigner preserves the alignment of its input, so zipalign (uncompressed
	// entries to 4 bytes) must run before signing. The Writer already 4-aligns,
	// but align defensively when the tool is present.
	if zipalign := filepath.Join(buildTools, "zipalign"); util.Exists(zipalign) {
		aligned := apkPath + ".aligned"
		cmd := exec.Command(zipalign, "-f", "4", apkPath, aligned)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("zipalign debug apk: %w", err)
		}
		if err := os.Rename(aligned, apkPath); err != nil {
			return err
		}
	}

	cmd := exec.Command(apksigner, "sign",
		"--key", keyPath, "--cert", certPath,
		"--v1-signing-enabled", "true", "--v2-signing-enabled", "true",
		apkPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apksigner sign debug apk: %w", err)
	}
	_ = os.Remove(apkPath + ".idsig") // apksigner sidecar, not needed to install
	return nil
}

// writeDebugSigningFiles writes the built-in debug key (PKCS#8 DER) and a
// matching self-signed certificate (DER) to temp files for apksigner's
// --key/--cert. The certificate is deterministic (fixed serial/subject/validity,
// identical to the internal JAR signer's) so every build produces the same
// signing identity and installed debug apps stay upgradable in place.
func writeDebugSigningFiles() (keyPath, certPath string, cleanup func(), err error) {
	block, _ := pem.Decode([]byte(debugCert))
	if block == nil {
		return "", "", nil, errors.New("no debug cert")
	}
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", "", nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", "", nil, err
	}

	// Same identity the internal signPKCS7 uses, so the signing cert is stable.
	template := &x509.Certificate{
		SerialNumber:       big.NewInt(0x5462c4dd),
		SignatureAlgorithm: x509.SHA1WithRSA,
		Subject:            pkix.Name{CommonName: "gomobile"},
		NotBefore:          time.Date(2013, time.January, 1, 12, 0, 0, 0, time.UTC),
		NotAfter:           time.Date(2120, time.January, 1, 12, 0, 0, 0, time.UTC),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, priv.Public(), priv)
	if err != nil {
		return "", "", nil, err
	}

	keyPath = filepath.Join(tmpdir, "bellui-debug.pk8")
	if err := os.WriteFile(keyPath, keyDER, 0o600); err != nil {
		return "", "", nil, err
	}
	certPath = filepath.Join(tmpdir, "bellui-debug.x509.der")
	if err := os.WriteFile(certPath, certDER, 0o600); err != nil {
		_ = os.Remove(keyPath)
		return "", "", nil, err
	}
	cleanup = func() {
		_ = os.Remove(keyPath)
		_ = os.Remove(certPath)
	}
	return keyPath, certPath, cleanup, nil
}

// mergeAppJavaDex compiles any android/*.java the app ships in its main package
// directory, plus extraJava (the R classes generated for library resources),
// against android.jar and the library jars, then merges the classes and the
// jars with bellui's built-in dex. It returns the dex files d8 wrote
// (classes.dex, then classes2.dex… when the methods overflow one file), or nil
// when the app ships neither android/*.java nor android/libs (the common
// case), in which case the built-in dex is used unchanged.
//
// This lets apps provide small JNI shims (e.g. a LocationListener for GPS)
// and library dependencies (#672) without a gradle project: drop a .java in
// <appdir>/android/, or a .jar/.aar in <appdir>/android/libs/, and `bellui
// package` compiles and bundles it. Requires ANDROID_HOME (javac + android.jar)
// and the build-tools (d8) on the system.
func mergeAppJavaDex(dir string, jars, extraJava []string) ([]string, error) {
	javaFiles, err := filepath.Glob(filepath.Join(dir, "android", "*.java"))
	if err != nil {
		return nil, err
	}
	javaFiles = append(javaFiles, extraJava...)
	if len(javaFiles) == 0 && len(jars) == 0 {
		return nil, nil
	}

	androidHome := os.Getenv("ANDROID_HOME")
	if androidHome == "" {
		return nil, errors.New("found android/*.java or android/libs but ANDROID_HOME is not set (needed to compile app Java)")
	}
	platform, err := findLastPlatform(androidHome)
	if err != nil {
		return nil, err
	}
	androidJar := filepath.Join(platform, "android.jar")

	classFiles, err := compileAppJava(javaFiles, jars, androidJar)
	if err != nil {
		return nil, err
	}

	// Write bellui's built-in dex so d8 can merge into it.
	baseDex := filepath.Join(tmpdir, "bellui-classes.dex")
	builtin, err := base64.StdEncoding.DecodeString(dexStr)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(baseDex, builtin, 0o644); err != nil {
		return nil, err
	}

	// d8 merges the built-in dex, the app classes and the library jars.
	// android.jar is the library d8 desugars against (default and static
	// interface methods in library bytecode need it).
	outDir := filepath.Join(tmpdir, "appdex")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	d8 := filepath.Join(util.AndroidBuildToolsPath(), "d8")
	d8Args := []string{"--min-api", "21", "--lib", androidJar, "--output", outDir, baseDex}
	d8Args = append(d8Args, classFiles...)
	d8Args = append(d8Args, jars...)
	if out, err := exec.Command(d8, d8Args...).CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		return nil, fmt.Errorf("d8 merge app dex: %w", err)
	}
	dexes, err := filepath.Glob(filepath.Join(outDir, "classes*.dex"))
	if err != nil {
		return nil, err
	}
	if len(dexes) == 0 {
		return nil, fmt.Errorf("d8 wrote no dex to %s", outDir)
	}
	sort.Strings(dexes) // classes.dex sorts before classes2.dex
	return dexes, nil
}

// compileAppJava runs javac over javaFiles with the jars on the classpath and
// returns the .class files it wrote. No sources means no classes.
func compileAppJava(javaFiles, jars []string, androidJar string) ([]string, error) {
	if len(javaFiles) == 0 {
		return nil, nil
	}
	work := filepath.Join(tmpdir, "appjava")
	if err := os.MkdirAll(work, 0o755); err != nil {
		return nil, err
	}
	javacArgs := []string{"-source", "1.8", "-target", "1.8", "-bootclasspath", androidJar, "-d", work}
	if len(jars) > 0 {
		javacArgs = append(javacArgs, "-classpath", strings.Join(jars, string(os.PathListSeparator)))
	}
	javacArgs = append(javacArgs, javaFiles...)
	if out, err := exec.Command("javac", javacArgs...).CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		return nil, fmt.Errorf("javac app android sources: %w", err)
	}

	var classFiles []string
	err := filepath.WalkDir(work, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".class") {
			classFiles = append(classFiles, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(classFiles) == 0 {
		return nil, fmt.Errorf("javac produced no classes from %v", javaFiles)
	}
	return classFiles, nil
}

// findLastPlatform returns the highest-sorted platforms/<android-NN> directory
// under the Android SDK, mirroring gendex's platform selection.
func findLastPlatform(androidHome string) (string, error) {
	dir := filepath.Join(androidHome, "platforms")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	last := ""
	for _, e := range entries {
		if e.IsDir() {
			last = e.Name() // os.ReadDir returns entries in sorted order
		}
	}
	if last == "" {
		return "", fmt.Errorf("no android platform found in %s", dir)
	}
	return filepath.Join(dir, last), nil
}

func apkwCreate(name string, apkw *Writer) (io.Writer, error) {
	if buildV {
		fmt.Fprintf(os.Stderr, "apk: %s\n", name)
	}
	if buildN {
		return io.Discard, nil
	}
	return apkw.Create(name)
}

func apkwWriteFile(dst, src string, apkw *Writer) error {
	w, err := apkwCreate(dst, apkw)
	if err != nil {
		return err
	}
	if !buildN {
		f, err := os.Open(filepath.Clean(src))
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(w, f); err != nil {
			return err
		}
	}
	return nil
}

// androidPkgName sanitizes the go package name to be acceptable as a android
// package name part. The android package name convention is similar to the
// java package name convention described in
// https://docs.oracle.com/javase/specs/jls/se8/html/jls-6.html#jls-6.5.3.1
// but not exactly same.
func androidPkgName(name string) string {
	var res []rune
	for _, r := range name {
		switch {
		case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9':
			res = append(res, r)
		default:
			res = append(res, '_')
		}
	}
	if len(res) == 0 || res[0] == '_' || ('0' <= res[0] && res[0] <= '9') {
		// Android does not seem to allow the package part starting with _.
		res = append([]rune{'g', 'o'}, res...)
	}
	s := string(res)
	// Look for Java keywords that are not Go keywords, and avoid using
	// them as a package name.
	//
	// This is not a problem for normal Go identifiers as we only expose
	// exported symbols. The upper case first letter saves everything
	// from accidentally matching except for the package name.
	//
	// Note that basic type names (like int) are not keywords in Go.
	switch s {
	case "abstract", "assert", "boolean", "byte", "catch", "char", "class",
		"do", "double", "enum", "extends", "final", "finally", "float",
		"implements", "instanceof", "int", "long", "native", "private",
		"protected", "public", "short", "static", "strictfp", "super",
		"synchronized", "this", "throw", "throws", "transient", "try",
		"void", "volatile", "while":
		s += "_"
	}
	return s
}

func convertAPKToAAB(aabPath string) error {
	apkPath := buildO[:len(aabPath)-3] + "apk"
	apkProtoPath := buildO[:len(aabPath)-3] + "apk-proto"
	tmpPath := filepath.Join(filepath.Dir(aabPath), "tmpbundle")
	err := os.MkdirAll(tmpPath, 0o755)
	if err != nil {
		return err
	}
	defer removeAll(tmpPath)

	aapt2, err := util.Aapt2Path()
	if err != nil {
		return err
	}
	cmd := exec.Command(aapt2, "convert", "--output-format", "proto", "-o", apkProtoPath, apkPath)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return err
	}
	_ = os.Remove(apkPath)

	cmd = exec.Command("unzip", apkProtoPath, "-x", "META-INF/*", "-d", tmpPath)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return err
	}
	_ = os.Remove(apkProtoPath)

	_ = os.MkdirAll(filepath.Join(tmpPath, "dex"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpPath, "manifest"), 0o755)
	_ = os.Rename(filepath.Join(tmpPath, "AndroidManifest.xml"), filepath.Join(tmpPath, "manifest", "AndroidManifest.xml"))
	_ = os.Rename(filepath.Join(tmpPath, "classes.dex"), filepath.Join(tmpPath, "dex", "classes.dex"))

	cmd = exec.Command("zip", "../base.zip", "-r", ".")
	cmd.Dir = tmpPath
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return err
	}
	defer os.Remove(filepath.Join(filepath.Dir(aabPath), "base.zip"))

	cmd = exec.Command("bundletool", "build-bundle", "--output", aabPath, "--modules", "base.zip")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

// A random uninteresting private key.
// Must be consistent across builds so newer app versions can be installed.
const debugCert = `
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAy6ItnWZJ8DpX9R5FdWbS9Kr1U8Z7mKgqNByGU7No99JUnmyu
NQ6Uy6Nj0Gz3o3c0BXESECblOC13WdzjsH1Pi7/L9QV8jXOXX8cvkG5SJAyj6hcO
LOapjDiN89NXjXtyv206JWYvRtpexyVrmHJgRAw3fiFI+m4g4Qop1CxcIF/EgYh7
rYrqh4wbCM1OGaCleQWaOCXxZGm+J5YNKQcWpjZRrDrb35IZmlT0bK46CXUKvCqK
x7YXHgfhC8ZsXCtsScKJVHs7gEsNxz7A0XoibFw6DoxtjKzUCktnT0w3wxdY7OTj
9AR8mobFlM9W3yirX8TtwekWhDNTYEu8dwwykwIDAQABAoIBAA2hjpIhvcNR9H9Z
BmdEecydAQ0ZlT5zy1dvrWI++UDVmIp+Ve8BSd6T0mOqV61elmHi3sWsBN4M1Rdz
3N38lW2SajG9q0fAvBpSOBHgAKmfGv3Ziz5gNmtHgeEXfZ3f7J95zVGhlHqWtY95
JsmuplkHxFMyITN6WcMWrhQg4A3enKLhJLlaGLJf9PeBrvVxHR1/txrfENd2iJBH
FmxVGILL09fIIktJvoScbzVOneeWXj5vJGzWVhB17DHBbANGvVPdD5f+k/s5aooh
hWAy/yLKocr294C4J+gkO5h2zjjjSGcmVHfrhlXQoEPX+iW1TGoF8BMtl4Llc+jw
lKWKfpECgYEA9C428Z6CvAn+KJ2yhbAtuRo41kkOVoiQPtlPeRYs91Pq4+NBlfKO
2nWLkyavVrLx4YQeCeaEU2Xoieo9msfLZGTVxgRlztylOUR+zz2FzDBYGicuUD3s
EqC0Wv7tiX6dumpWyOcVVLmR9aKlOUzA9xemzIsWUwL3PpyONhKSq7kCgYEA1X2F
f2jKjoOVzglhtuX4/SP9GxS4gRf9rOQ1Q8DzZhyH2LZ6Dnb1uEQvGhiqJTU8CXxb
7odI0fgyNXq425Nlxc1Tu0G38TtJhwrx7HWHuFcbI/QpRtDYLWil8Zr7Q3BT9rdh
moo4m937hLMvqOG9pyIbyjOEPK2WBCtKW5yabqsCgYEAu9DkUBr1Qf+Jr+IEU9I8
iRkDSMeusJ6gHMd32pJVCfRRQvIlG1oTyTMKpafmzBAd/rFpjYHynFdRcutqcShm
aJUq3QG68U9EAvWNeIhA5tr0mUEz3WKTt4xGzYsyWES8u4tZr3QXMzD9dOuinJ1N
+4EEumXtSPKKDG3M8Qh+KnkCgYBUEVSTYmF5EynXc2xOCGsuy5AsrNEmzJqxDUBI
SN/P0uZPmTOhJIkIIZlmrlW5xye4GIde+1jajeC/nG7U0EsgRAV31J4pWQ5QJigz
0+g419wxIUFryGuIHhBSfpP472+w1G+T2mAGSLh1fdYDq7jx6oWE7xpghn5vb9id
EKLjdwKBgBtz9mzbzutIfAW0Y8F23T60nKvQ0gibE92rnUbjPnw8HjL3AZLU05N+
cSL5bhq0N5XHK77sscxW9vXjG0LJMXmFZPp9F6aV6ejkMIXyJ/Yz/EqeaJFwilTq
Mc6xR47qkdzu0dQ1aPm4XD7AWDtIvPo/GG2DKOucLBbQc2cOWtKS
-----END RSA PRIVATE KEY-----
`
