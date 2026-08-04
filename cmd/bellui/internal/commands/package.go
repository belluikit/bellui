package commands

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"image"
	_ "image/jpeg" // import image encodings
	"image/png"    // import image encodings
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/belluikit/bellui/internal/ico" // import image encodings
	"github.com/urfave/cli/v2"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"

	"github.com/belluikit/bellui"

	"github.com/belluikit/bellui/cmd/bellui/internal/metadata"
	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
)

const (
	defaultAppBuild   = 1
	defaultAppVersion = "0.0.1"
)

// Package returns the cli command for packaging bellui applications
func Package() *cli.Command {
	p := NewPackager()

	return &cli.Command{
		Name:        "package",
		Aliases:     []string{"p"},
		Usage:       "Packages an application for distribution",
		Description: "You may specify the --executable to package, otherwise --source-dir will be built.",
		Flags: []cli.Flag{
			stringFlags["target"](&p.os),
			stringFlags["executable"](&p.exe),
			stringFlags["name"](&p.Name),
			stringFlags["tags"](&p.tags),
			stringFlags["app-version"](&p.AppVersion),
			intFlags["app-build"](&p.AppBuild),
			stringFlags["src"](&p.srcDir),
			stringFlags["icon"](&p.icon),
			boolFlags["use-raw-icon"](&p.rawIcon),
			stringFlags["app-id"](&p.AppID),
			stringFlags["certificate"](&p.certificate),
			stringFlags["profile"](&p.profile),
			boolFlags["release"](&p.release),
			genericFlags["metadata"](&p.customMetadata),
		},
		Action: func(_ *cli.Context) error {
			if p.customMetadata.m == nil {
				p.customMetadata.m = map[string]string{}
			}

			return p.Package()
		},
	}
}

// Packager wraps executables into full GUI app packages.
type Packager struct {
	*appData
	srcDir, dir, exe, os           string
	install, release, distribution bool
	certificate, profile           string // optional flags for releasing
	tags, category                 string
	tempDir                        string
	langs                          []string

	customMetadata      keyValueFlag
	linuxAndBSDMetadata *metadata.LinuxAndBSD
	sourceMetadata      *metadata.AppSource
}

// NewPackager returns a command that can handle the packaging a GUI apps built using Bellui from local source code.
func NewPackager() *Packager {
	return &Packager{appData: &appData{}}
}

// Package starts the packaging process
func (p *Packager) Package() error {
	err := p.validate()
	if err != nil {
		return err
	}

	return p.packageWithoutValidate()
}

func (p *Packager) packageWithoutValidate() error {
	err := p.doPackage(nil)
	if err != nil {
		return err
	}

	// Bump the build number in the manifest, preserving the author's comments,
	// key ordering and formatting (a full re-serialize would strip them on every
	// package).
	return metadata.BumpBuildStandard(p.srcDir)
}

func (p *Packager) buildPackage(runner runner, tags []string) ([]string, error) {
	b := &Builder{
		os:      p.os,
		srcdir:  p.srcDir,
		target:  p.exe,
		release: p.release,
		tags:    tags,
		runner:  runner,

		appData: p.appData,
	}

	return []string{p.exe}, b.build()
}

func (p *Packager) combinedVersion() string {
	versions := strings.Split(p.AppVersion, ".")
	for len(versions) < 3 {
		versions = append(versions, "0")
	}
	appVersion := strings.Join(versions, ".")

	return fmt.Sprintf("%s.%d", appVersion, p.AppBuild)
}

func (p *Packager) doPackage(runner runner) error {
	// sensible defaults - validation deemed them optional
	if p.AppVersion == "" {
		p.AppVersion = defaultAppVersion
	}
	if p.AppBuild <= 0 {
		p.AppBuild = defaultAppBuild
	}
	defer os.RemoveAll(p.tempDir)

	var tags []string
	if p.tags != "" {
		tags = strings.Split(p.tags, ",")
	}

	if !util.Exists(p.exe) && !util.IsMobile(p.os) {
		files, err := p.buildPackage(runner, tags)
		if err != nil {
			return fmt.Errorf("error building application: %w", err)
		}
		for _, file := range files {
			if p.os != "web" && !util.Exists(file) {
				return fmt.Errorf("unable to build directory to expected executable, %s", file)
			}
		}
		if p.os != "windows" {
			defer p.removeBuild(files)
		}
	}
	if util.IsMobile(p.os) { // we don't use the normal build command for mobile so inject before gomobile...
		close, err := injectMetadataIfPossible(newCommand("go"), p.dir, p.appData, createMetadataInitFile)
		if err != nil {
			bellui.LogError("Failed to inject metadata init file, omitting metadata", err)
		} else if close != nil {
			defer close()
		}
	}

	switch p.os {
	case "darwin":
		return p.packageDarwin()
	case "linux", "openbsd", "freebsd", "netbsd":
		return p.packageUNIX()
	case "windows":
		return p.packageWindows(tags)
	case "android/arm", "android/arm64", "android/amd64", "android/386", "android":
		return p.packageAndroid(p.os, tags)
	case "ios", "iossimulator":
		return p.packageIOS(p.os, tags)
	case "web", "wasm":
		return p.packageWasm()
	default:
		return fmt.Errorf("unsupported target operating system \"%s\"", p.os)
	}
}

func (p *Packager) removeBuild(files []string) {
	for _, file := range files {
		err := os.Remove(file)
		if err != nil {
			log.Println("Unable to remove temporary build file", p.exe)
		}
	}
}

func (p *Packager) validate() (err error) {
	p.tempDir, err = os.MkdirTemp("", "bellui-package-*")
	defer func() {
		if err != nil {
			_ = os.RemoveAll(p.tempDir)
		}
	}()

	if p.os == "" {
		p.os = targetOS()
	}
	baseDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("unable to get the current directory, needed to find main executable: %w", err)
	}
	if p.dir == "" {
		p.dir = baseDir
	}
	if p.srcDir == "" {
		p.srcDir = baseDir
	} else {
		if p.os == "ios" || p.os == "android" {
			return errors.New("parameter --source-dir is currently not supported for mobile builds. " +
				"Change directory to the main package and try again")
		}
		p.srcDir = util.EnsureAbsPath(p.srcDir)
	}
	if err := os.Chdir(p.srcDir); err != nil {
		return err
	}
	if !hasGoCode(p.srcDir) {
		return fmt.Errorf("failed to find go code in source directory: %s", p.srcDir)
	}

	p.appData.CustomMetadata = p.customMetadata.m
	p.appData.Release = p.release

	data, err := metadata.LoadStandard(p.srcDir)
	if err == nil {
		// When icon path specified in metadata file, we should make it relative to metadata file
		if data.Details.Icon != "" {
			data.Details.Icon = util.MakePathRelativeTo(p.srcDir, data.Details.Icon)
		}

		p.appData.mergeMetadata(data)
		p.sourceMetadata = data.Source
		p.langs = data.Languages

		p.linuxAndBSDMetadata = data.LinuxAndBSD
	}

	exeName := calculateExeName(p.srcDir, p.os)

	if p.exe == "" {
		p.exe = filepath.Join(p.srcDir, exeName)

		if util.Exists(p.exe) { // the exe was not specified, assume stale
			p.removeBuild([]string{p.exe})
		}
	} else if p.os == "ios" || p.os == "android" {
		_, _ = fmt.Fprint(os.Stderr, "Parameter --executable is ignored for mobile builds.\n")
	}

	if err := p.validateMainPackage(); err != nil {
		return err
	}

	if p.Name == "" {
		p.Name = exeName
	}
	if p.icon == "" || p.icon == "Icon.png" {
		p.icon = filepath.Join(p.srcDir, "Icon.png")
		if !util.Exists(p.icon) {
			// No app-provided icon — fall back to the bundled toolkit logo
			// so packaging succeeds out of the box.
			tmp := filepath.Join(p.tempDir, "default-icon.png")
			if err := os.WriteFile(tmp, templates.DefaultIcon, 0o644); err != nil {
				return fmt.Errorf("failed to write default application icon: %w", err)
			}
			log.Println("No Icon.png found — using the default toolkit icon; provide an Icon.png or --icon for a custom one")
			p.icon = tmp
		}
	}
	if !util.Exists(p.icon) {
		return errors.New("Missing application icon at \"" + p.icon + "\"")
	}
	if strings.ToLower(filepath.Ext(p.icon)) != ".png" {
		tmp, err := p.normaliseIcon(p.icon)
		if err != nil {
			return err
		}
		p.icon = tmp
	}

	p.AppID, err = validateAppID(p.AppID, p.os, p.Name, p.release)
	if err != nil {
		return err
	}
	if p.AppVersion != "" && !isValidVersion(p.AppVersion) {
		return errors.New("invalid --app-version parameter, integer and '.' characters only up to x.y.z")
	}

	return nil
}

func calculateExeName(sourceDir, osys string) string {
	exeName := filepath.Base(sourceDir)
	/* #nosec */
	if data, err := os.ReadFile(filepath.Join(sourceDir, "go.mod")); err == nil {
		modulePath := modfile.ModulePath(data)
		moduleName, _, ok := module.SplitPathVersion(modulePath)
		if ok {
			paths := strings.Split(moduleName, "/")
			name := paths[len(paths)-1]
			if name != "" {
				exeName = name
			}
		}
	}

	if osys == "windows" {
		exeName = exeName + ".exe"
	}

	return exeName
}

func isValidVersion(ver string) bool {
	if semver.IsValid("v" + ver) {
		return true
	}
	parts := strings.Split(ver, ".")
	if len(parts) < 1 || len(parts) > 2 {
		return false
	}
	for _, p := range parts {
		_, err := strconv.Atoi(p)
		if err != nil {
			return false
		}
	}
	return true
}

// normaliseIcon takes a non-png image file and converts it to PNG for use in packaging.
// Successful conversion will return a path to the new file.
// Any errors that occur will be returned with an empty string for new path.
func (p *Packager) normaliseIcon(path string) (string, error) {
	// convert icon
	img, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open source image: %w", err)
	}
	defer img.Close()
	srcImg, _, err := image.Decode(img)
	if err != nil {
		return "", fmt.Errorf("failed to decode source image: %w", err)
	}

	out, err := os.CreateTemp(p.tempDir, "bellui-ico-*.png")
	if err != nil {
		return "", fmt.Errorf("failed to open image output file: %w", err)
	}
	tmpPath := out.Name()
	defer out.Close()

	err = png.Encode(out, srcImg)
	return tmpPath, err
}

func validateAppID(appID, os, name string, release bool) (string, error) {
	// old darwin compatibility
	if os == "darwin" {
		if appID == "" {
			return "com.example." + name, nil
		}
	} else if os == "ios" || util.IsAndroid(os) || (os == "windows" && release) {
		// all mobile, and for windows when releasing, needs a unique id - usually reverse DNS style
		if appID == "" {
			return "", errors.New("missing app-id parameter for package")
		} else if !strings.Contains(appID, ".") {
			return "", errors.New("app-id must be globally unique and contain at least 1 '.'")
		} else if util.IsAndroid(os) {
			if strings.Contains(appID, "-") {
				return "", errors.New("app-id can not contain '-'")
			}

			// appID package names can not start with '_' or a number
			packageNames := strings.Split(appID, ".")
			for _, name := range packageNames {
				if len(name) == 0 {
					continue
				}

				if name[0] == '_' {
					return "", fmt.Errorf("app-id package names can not start with '_' (%s)", name)
				} else if name[0] >= '0' && name[0] <= '9' {
					return "", fmt.Errorf("app-id package names can not start with a number (%s)", name)
				}
			}
		}
	}

	return appID, nil
}

// validateMainPackage fails fast when packaging would build p.srcDir but its
// own package is not main. Without this the failure surfaces much later and
// much more confusingly: the metadata init file (package main) is injected
// into the non-main package and the build dies with go's "found packages X
// and main" (#248) — typically after packaging was run from the module root
// while the application lives in a subdirectory. Wrapping a prebuilt
// --executable neither builds nor injects, so the source package is
// irrelevant there and the check is skipped; mobile targets always build
// through gomobile, so they are always checked.
func (p *Packager) validateMainPackage() error {
	if !util.IsMobile(p.os) && util.Exists(p.exe) {
		return nil
	}
	pkgName, ok := mainPackageIn(p.srcDir)
	if ok {
		return nil
	}
	if pkgName == "" {
		return fmt.Errorf("source directory %s has no buildable Go files of its own — run bellui package from the application's main package directory", p.srcDir)
	}
	return fmt.Errorf("source directory %s is package %q, not a main package — run bellui package from the application's main package directory", p.srcDir, pkgName)
}

// mainPackageIn reports whether dir's own package (the one `bellui package`
// builds — not subdirectories) declares package main in at least one Go file.
// Build constraints are deliberately ignored: a main package for another
// target OS still counts. Files the go tool never builds are skipped:
// _test.go files and names starting with "_" or ".". When no file declares
// main, the name of the package found there (if any) is returned for the
// error message.
func mainPackageIn(dir string) (pkgName string, ok bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") ||
			strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.PackageClauseOnly)
		if err != nil {
			continue
		}
		if f.Name.Name == "main" {
			return "main", true
		}
		pkgName = f.Name.Name
	}
	return pkgName, false
}

var errStopWalking = errors.New("stop walking")

func hasGoCode(dir string) bool {
	found := false
	_ = filepath.Walk(dir+string(filepath.Separator), func(path string, fi fs.FileInfo, err error) error {
		if fi.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}
		found = true
		return errStopWalking
	})
	return found
}
