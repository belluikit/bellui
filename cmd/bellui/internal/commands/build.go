package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/belluikit/bellui"

	"github.com/belluikit/bellui/cmd/bellui/internal/metadata"
	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
)

// Partly based on https://gitlab.com/freedesktop-sdk/freedesktop-sdk/-/blob/master/include/flags.yml?ref_type=heads.
const (
	baseCFLAGSRegular      = "-O2 -g -fexceptions -fasynchronous-unwind-tables -pipe"
	baseCFLAGSRelease      = "-O3 -pipe"
	hardeningCFLAGS        = "-D_FORTIFY_SOURCE=3 -fstack-protector-strong"
	hardeningLDFLAGSLinux  = "-Wl,-z,relro,-z,now -Wl,--as-needed"
	hardeningLDFLAGSDarwin = "-Wl,-dead_strip_dylibs"
)

// Builder generate the executables.
type Builder struct {
	*appData
	os, srcdir, target string
	goPackage          string
	release            bool
	pprof              bool
	pprofPort          int
	tags               []string
	tagsToParse        string

	customMetadata keyValueFlag

	runner runner
}

// NewBuilder returns a command that can handle the build of GUI apps built using Bellui.
func NewBuilder() *Builder {
	return &Builder{appData: &appData{}}
}

// Build returns the cli command for building bellui applications
func Build() *cli.Command {
	b := NewBuilder()

	return &cli.Command{
		Name:        "build",
		Aliases:     []string{"b"},
		Usage:       "Builds an application",
		Description: "You can specify --target to define the OS to build for. The executable file will default to an appropriate name but can be overridden using -o.",
		Flags: []cli.Flag{
			stringFlags["target"](&b.os),
			stringFlags["src"](&b.srcdir),
			stringFlags["tags"](&b.tagsToParse),
			boolFlags["release"](&b.release),
			stringFlags["output"](&b.target),
			boolFlags["pprof"](&b.pprof),
			intFlags["pprof-port"](&b.pprofPort),
			genericFlags["metadata"](&b.customMetadata),
		},
		Action: func(ctx *cli.Context) error {
			argCount := ctx.Args().Len()
			if argCount > 0 {
				if argCount != 1 {
					return fmt.Errorf("incorrect amount of path provided")
				}
				b.goPackage = ctx.Args().First()
			}

			return b.Build()
		},
	}
}

// Build parse the tags and start building
func (b *Builder) Build() error {
	if b.srcdir != "" {
		b.srcdir = util.EnsureAbsPath(b.srcdir)
		dirStat, err := os.Stat(b.srcdir)
		if err != nil {
			return err
		}
		if !dirStat.IsDir() {
			return fmt.Errorf("specified source directory is not a valid directory")
		}
	}
	if b.tagsToParse != "" {
		b.tags = strings.Split(b.tagsToParse, ",")
	}
	b.appData.Release = b.release
	b.appData.CustomMetadata = b.customMetadata.m

	return b.build()
}

func isWeb(goos string) bool {
	return goos == "js" || goos == "wasm" || goos == "web"
}

type goModEdit struct {
	Module struct {
		Path string
	}
	Require []struct {
		Path    string
		Version string
	}
}

// requireBellui reports an error unless the module the runner's directory
// belongs to is bellui itself or requires it: the metadata init file imports
// bellui, so it can only be written into an app that builds against it.
func requireBellui(runner runner) error {
	dependenciesOutput, err := runner.runOutput("mod", "edit", "-json")
	if err != nil {
		return err
	}

	var parsed goModEdit
	err = json.Unmarshal(dependenciesOutput, &parsed)
	if err != nil {
		return err
	}

	if parsed.Module.Path == "github.com/belluikit/bellui" {
		return nil
	}

	for _, dep := range parsed.Require {
		if dep.Path == "github.com/belluikit/bellui" {
			return nil
		}
	}

	return fmt.Errorf("the module does not require github.com/belluikit/bellui")
}

func (b *Builder) build() error {
	goos := b.os
	if goos == "" {
		goos = targetOS()
	}

	belluiGoModRunner := b.updateAndGetGoExecutable()

	srcdir, err := b.computeSrcDir()
	if err != nil {
		return err
	}

	b.updateToDefaultIconIfNotSet(srcdir)

	if b.pprof {
		close, err := injectPprofFile(srcdir, b.pprofPort)
		if err != nil {
			bellui.LogError("Failed to inject pprof file, omitting pprof", err)
		} else if close != nil {
			defer close()
		}
	}

	// go mod edit must read the app's go.mod, not the one (if any) of the
	// directory bellui was started in: --src may point anywhere.
	belluiGoModRunner.setDir(srcdir)
	close, err := injectMetadataIfPossible(belluiGoModRunner, srcdir, b.appData, createMetadataInitFile)
	if err != nil {
		bellui.LogError("Failed to inject metadata init file, omitting metadata", err)
	} else if close != nil {
		defer close()
	}

	args := []string{"build"}

	// Take the environment only after -ldflags has been moved out of GOFLAGS,
	// or the child go build still sees the original GOFLAGS (#371).
	ldFlags := extractLdflagsFromGoFlags()
	env := os.Environ()
	if goos == "windows" {
		ldFlags += " -H=windowsgui"
	}

	if b.release {
		ldFlags += " -s -w"
		args = append(args, "-trimpath")
	}

	if len(ldFlags) > 0 {
		args = append(args, "-ldflags", strings.TrimSpace(ldFlags))
	}

	if b.target != "" {
		args = append(args, "-o", b.target)
	}

	if !isWeb(goos) {
		env = append(env, "CGO_ENABLED=1") // in case someone is trying to cross-compile...
		b.applyCAndLDFlags(&env, goos)
	} else {
		env = append(env, "CGO_ENABLED=0") // CGO is not available in WebAssembly
	}

	// handle build tags
	tags := b.tags
	if b.release {
		tags = append(tags, "release")
	}
	if ok, set := b.appData.Migrations["belluiDo"]; ok && set {
		tags = append(tags, "migrated_belluido")
	}
	if len(tags) > 0 {
		args = append(args, "-tags", strings.Join(tags, ","))
	}

	if b.goPackage != "" {
		args = append(args, b.goPackage)
	}

	if goos != "ios" && goos != "android" && !isWeb(goos) {
		env = append(env, "GOOS="+goos)
	} else if goos == "web" || goos == "wasm" {
		env = append(env, "GOARCH=wasm")
		env = append(env, "GOOS=js")
	}

	b.runner.setDir(b.srcdir)
	b.runner.setEnv(env)
	out, err := b.runner.runOutput(args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", string(out))
	}
	return err
}

func (b *Builder) computeSrcDir() (string, error) {
	if b.goPackage == "" || b.goPackage == "." {
		return b.srcdir, nil
	}

	srcdir := b.srcdir
	if strings.HasPrefix(b.goPackage, "."+string(os.PathSeparator)) ||
		strings.HasPrefix(b.goPackage, ".."+string(os.PathSeparator)) {
		srcdir = filepath.Join(srcdir, b.goPackage)
	} else if strings.HasPrefix(b.goPackage, string(os.PathSeparator)) {
		srcdir = b.goPackage
	} else {
		return "", fmt.Errorf("unrecognized go package: %s", b.goPackage)
	}
	return srcdir, nil
}

func (b *Builder) updateToDefaultIconIfNotSet(srcdir string) {
	if b.icon == "" {
		defaultIcon := filepath.Join(srcdir, "Icon.png")
		if util.Exists(defaultIcon) {
			b.icon = defaultIcon
		}
	}
}

func injectPprofFile(srcdir string, port int) (func(), error) {
	pprofInitFilePath := filepath.Join(srcdir, "bellui_pprof.go")
	pprofInitFile, err := os.Create(pprofInitFilePath)
	if err != nil {
		return func() {}, err
	}
	defer pprofInitFile.Close()

	pprofInfo := struct {
		Port int
	}{
		Port: port,
	}

	err = templates.BelluiPprofInit.Execute(pprofInitFile, pprofInfo)
	if err != nil {
		os.Remove(pprofInitFilePath)
		return func() {}, err
	}

	return func() { os.Remove(pprofInitFilePath) }, nil
}

func (b *Builder) updateAndGetGoExecutable() runner {
	belluiGoModRunner := b.runner
	if b.runner == nil {
		belluiGoModRunner = newCommand("go")
		goBin := os.Getenv("GO")
		if goBin != "" {
			belluiGoModRunner = newCommand(goBin)
			b.runner = belluiGoModRunner
		} else {
			b.runner = newCommand("go")
		}
	}
	return belluiGoModRunner
}

func (b *Builder) applyCAndLDFlags(env *[]string, goos string) {
	cflags := []string{baseCFLAGSRegular, hardeningCFLAGS}
	if b.release {
		cflags[0] = baseCFLAGSRelease
	}

	ldflags := []string{}
	switch goos {
	case "linux":
		ldflags = append(ldflags, hardeningLDFLAGSLinux)
	case "darwin":
		ldflags = append(ldflags, hardeningLDFLAGSDarwin)

		cflags = append(cflags, "-mmacosx-version-min=10.13")
		ldflags = append(ldflags, "-mmacosx-version-min=10.13")
	}

	switch targetArch() {
	case "arm64":
		cflags = append(cflags, "-mbranch-protection=bti+pac-ret")
	}

	appendEnv(env, "CGO_CFLAGS", strings.Join(cflags, " "))
	appendEnv(env, "CGO_LDFLAGS", strings.Join(ldflags, " "))
}

func createMetadataInitFile(srcdir string, app *appData) (func(), error) {
	data, err := metadata.LoadStandard(srcdir)
	if err == nil {
		// When icon path specified in metadata file, we should make it relative to metadata file
		if data.Details.Icon != "" {
			data.Details.Icon = util.MakePathRelativeTo(srcdir, data.Details.Icon)
		}

		app.mergeMetadata(data)
	}

	// The init file replaces the runtime's metadata wholesale, so an app with
	// no version or build (no manifest, no flags) must get the same defaults
	// app.Metadata reports without one, as bellui package already does.
	if app.AppVersion == "" {
		app.AppVersion = defaultAppVersion
	}
	if app.AppBuild <= 0 {
		app.AppBuild = defaultAppBuild
	}

	app.ResGoString = "nil"
	if app.icon != "" {
		res, err := bellui.LoadResourceFromPath(app.icon)
		if err != nil {
			bellui.LogError("Unable to load metadata icon file "+app.icon, err)
			return func() {}, err
		}

		res = metadata.ScaleIcon(res, 512)

		// The return type of bellui.LoadResourceFromPath is always a *bellui.StaticResource.
		app.ResGoString = res.(*bellui.StaticResource).GoString()
	}

	metadataInitFilePath := filepath.Join(srcdir, "bellui_metadata_init.go")
	metadataInitFile, err := os.Create(metadataInitFilePath)
	if err != nil {
		return func() {}, err
	}

	err = templates.BelluiMetadataInit.Execute(metadataInitFile, app)
	if closeErr := metadataInitFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		// Callers drop the cleanup on error and carry on building, so a
		// half-written file left here would break this build and every later
		// one in the user's source tree.
		bellui.LogError("Error executing metadata template", err)
		os.Remove(metadataInitFilePath)
		return func() {}, err
	}

	return func() { os.Remove(metadataInitFilePath) }, nil
}

// injectMetadataIfPossible writes the metadata init file into srcdir when the
// app builds against bellui. Every bellui release supports the whole of
// AppMetadata (ID, name, version, build, icon, release, custom and migrations),
// so there is no version to check; an app whose go.mod does not mention bellui
// at all is reported as an error by requireBellui.
func injectMetadataIfPossible(runner runner, srcdir string, app *appData,
	createMetadataInitFile func(srcdir string, app *appData) (func(), error),
) (func(), error) {
	if err := requireBellui(runner); err != nil {
		return nil, err
	}

	return createMetadataInitFile(srcdir, app)
}

func targetArch() string {
	archEnv, ok := os.LookupEnv("GOARCH")
	if ok {
		return archEnv
	}

	return runtime.GOARCH
}

func targetOS() string {
	osEnv, ok := os.LookupEnv("GOOS")
	if ok {
		return osEnv
	}

	return runtime.GOOS
}

func appendEnv(env *[]string, varName, value string) {
	for i := range *env {
		keyValue := strings.SplitN((*env)[i], "=", 2)

		if keyValue[0] == varName {
			(*env)[i] += " " + value
			return
		}
	}

	*env = append(*env, varName+"="+value)
}

func extractLdflagsFromGoFlags() string {
	goFlags := os.Getenv("GOFLAGS")

	ldFlags, goFlags := extractLdFlags(goFlags)
	if goFlags != "" {
		os.Setenv("GOFLAGS", goFlags)
	} else {
		os.Unsetenv("GOFLAGS")
	}

	return ldFlags
}

func extractLdFlags(goFlags string) (string, string) {
	if goFlags == "" {
		return "", ""
	}

	flags := strings.Fields(goFlags)
	ldflags := ""
	newGoFlags := ""

	for _, flag := range flags {
		if strings.HasPrefix(flag, "-ldflags=") {
			ldflags += strings.TrimPrefix(flag, "-ldflags=") + " "
		} else {
			newGoFlags += flag + " "
		}
	}

	ldflags = strings.TrimSpace(ldflags)
	newGoFlags = strings.TrimSpace(newGoFlags)

	return ldflags, newGoFlags
}
