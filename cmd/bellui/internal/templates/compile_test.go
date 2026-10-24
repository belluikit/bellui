package templates

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"
	"golang.org/x/mod/modfile"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/cmd/bellui/internal/metadata"
)

const toolkitModule = "github.com/belluikit/bellui"

// The Go sources the CLI writes into an app must compile against the toolkit
// they ship with. This module builds against a released toolkit, so a string
// check of the template cannot tell when the toolkit's API moves under it: the
// main.go bellui init wrote called w.ShowAndRun() long after that method was
// gone (#337, #710). This renders the app sources into a temporary module,
// wires it to the checkout's toolkit with a go.work, and runs go vet on it.
//
// The sources are vetted together, as bellui build leaves them: main.go from
// bellui init, plus the metadata and pprof files build injects next to it.
func TestGeneratedAppSourcesCompileAgainstToolkit(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go vet on a module that imports the whole toolkit")
	}
	goTool, err := exec.LookPath("go")
	require.NoError(t, err)

	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	require.NoError(t, err)
	rootMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	require.NoError(t, err, "the toolkit checkout is expected three levels above cmd/bellui")
	mf, err := modfile.ParseLax("go.mod", rootMod, nil)
	require.NoError(t, err)
	require.Equal(t, toolkitModule, mf.Module.Mod.Path, "%s is not the toolkit checkout", root)
	require.NotNil(t, mf.Go, "the toolkit go.mod has no go directive")

	// What bellui init passes to HelloWorld.
	initData := &metadata.BelluiApp{}
	initData.Details.ID = "com.example.hello"
	initData.Details.Name = `Hello "World"`
	initData.Details.Version = "0.0.0"
	initData.Migrations = map[string]bool{"belluiDo": true}

	// The fields bellui build fills in for the metadata init file (appData in
	// package commands; its tests check those field names against the template).
	icon := bellui.NewStaticResource("Icon.png", []byte{0x89, 'P', 'N', 'G'})
	meta := struct {
		Name, AppID, AppVersion, ResGoString string
		AppBuild                             int
		Release                              bool
		CustomMetadata                       map[string]string
		Migrations                           map[string]bool
	}{
		Name: `My "Pro" App`, AppID: "com.example.hello", AppVersion: "1.2.3",
		ResGoString: icon.GoString(), AppBuild: 4, Release: true,
		CustomMetadata: map[string]string{`dir\name`: "beta"},
		Migrations:     map[string]bool{"belluiDo": true},
	}

	// The go command matches a use path against the working directory with
	// symlinks resolved, and macOS temp dirs sit under the /var symlink.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	app := filepath.Join(dir, "app")
	write := func(name string, content []byte) {
		t.Helper()
		p := filepath.Join(app, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, content, 0o644))
	}
	render := func(name string, tmpl *template.Template, data any) {
		t.Helper()
		var b bytes.Buffer
		require.NoError(t, tmpl.Execute(&b, data), name)
		write(name, b.Bytes())
	}

	render("main.go", HelloWorld, &initData)
	render("bellui_metadata_init.go", BelluiMetadataInit, meta)
	render("bellui_pprof.go", BelluiPprofInit, struct{ Port int }{6060})
	write("translations/en.pxf", []byte("\"Hello World!\" = \"Hello World!\"\n"))
	write("go.mod", []byte("module example.com/hello\n\ngo "+mf.Go.Version+"\n"))

	work := filepath.Join(dir, "go.work")
	require.NoError(t, os.WriteFile(work, []byte("go "+mf.Go.Version+"\n\nuse (\n\t"+
		quoteModPath(app)+"\n\t"+quoteModPath(root)+"\n)\n"), 0o644))

	// GOWORK picks the workspace above rather than whatever the test runs
	// under; GOFLAGS is cleared so a -mod or -tags meant for this module's
	// tests does not reach the app build. A desktop app needs cgo even to
	// type-check: systray and wgpu have no cgo-free build, so with cgo off vet
	// reports their native halves undefined rather than anything about the app.
	env := append(os.Environ(), "GOWORK="+work, "GOFLAGS=", "CGO_ENABLED=1")

	// The point is the checkout's API, not a released one: check where the
	// toolkit resolves before trusting a clean vet.
	list := exec.Command(goTool, "list", "-m", "-f", "{{.Dir}}", toolkitModule)
	list.Dir, list.Env = app, env
	out, err := list.CombinedOutput()
	require.NoError(t, err, "%s", out)
	require.Equal(t, root, string(bytes.TrimSpace(out)), "the app must resolve the toolkit to the checkout")

	vet := exec.Command(goTool, "vet", "./...")
	vet.Dir, vet.Env = app, env
	out, err = vet.CombinedOutput()
	if err != nil {
		t.Fatalf("go vet of the generated app failed: %v\n%s\nmain.go:\n%s", err, out, mustRead(t, filepath.Join(app, "main.go")))
	}
}

// quoteModPath quotes p for a go.work use directive; a Windows temp path or a
// checkout path with spaces needs it.
func quoteModPath(p string) string {
	return modfile.AutoQuote(filepath.ToSlash(p))
}

func mustRead(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	return string(b)
}
