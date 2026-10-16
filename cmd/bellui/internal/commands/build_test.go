package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_BuildWasmVersion(t *testing.T) {
	expected := []mockRunner{
		{
			expectedValue: expectedValue{args: []string{"mod", "edit", "-json"}, dir: "myTest"},
			mockReturn: mockReturn{
				ret: []byte("{ \"Module\": { \"Path\": \"github.com/belluikit/bellui\"} }"),
			},
		},
		{
			expectedValue: expectedValue{
				args:  []string{"build"},
				env:   []string{"GOARCH=wasm", "GOOS=js", "CGO_ENABLED=0"},
				osEnv: true,
				dir:   "myTest",
			},
			mockReturn: mockReturn{ret: []byte("")},
		},
	}

	wasmBuildTest := &testCommandRuns{runs: expected, t: t}
	b := &Builder{appData: &appData{}, os: "wasm", srcdir: "myTest", runner: wasmBuildTest}
	err := b.build()
	assert.Nil(t, err)
	wasmBuildTest.verifyExpectation()
}

// -ldflags in GOFLAGS moves to the command line; the child go build must not
// also see it in its environment, where it would override the merged flags
// (#371).
func Test_BuildGoFlagsLdflagsLeaveTheEnvironment(t *testing.T) {
	t.Setenv("GOFLAGS", "-mod=mod -ldflags=-X=main.v=1")
	expected := []mockRunner{
		{
			expectedValue: expectedValue{args: []string{"mod", "edit", "-json"}, dir: "myTest"},
			mockReturn: mockReturn{
				ret: []byte("{ \"Module\": { \"Path\": \"github.com/belluikit/bellui\"} }"),
			},
		},
		{
			expectedValue: expectedValue{
				args:  []string{"build", "-ldflags", "-X=main.v=1"},
				env:   []string{"GOARCH=wasm", "GOOS=js", "CGO_ENABLED=0"},
				osEnv: true,
				dir:   "myTest",
			},
			mockReturn: mockReturn{ret: []byte("")},
		},
	}

	runs := &testCommandRuns{runs: expected, t: t}
	b := &Builder{appData: &appData{}, os: "wasm", srcdir: "myTest", runner: runs}
	assert.Nil(t, b.build())
	runs.verifyExpectation()
	assert.Equal(t, "-mod=mod", os.Getenv("GOFLAGS"))
}

func Test_BuildWasmReleaseVersion(t *testing.T) {
	expected := []mockRunner{
		{
			expectedValue: expectedValue{args: []string{"mod", "edit", "-json"}, dir: "myTest"},
			mockReturn: mockReturn{
				ret: []byte("{ \"Module\": { \"Path\": \"github.com/belluikit/bellui\"} }"),
			},
		},
		{
			expectedValue: expectedValue{
				args:  []string{"build", "-trimpath", "-ldflags", "-s -w", "-tags", "release"},
				env:   []string{"GOARCH=wasm", "GOOS=js", "CGO_ENABLED=0"},
				osEnv: true,
				dir:   "myTest",
			},
			mockReturn: mockReturn{
				ret: []byte(""),
			},
		},
	}

	wasmBuildTest := &testCommandRuns{runs: expected, t: t}
	b := &Builder{appData: &appData{}, os: "wasm", srcdir: "myTest", release: true, runner: wasmBuildTest}
	err := b.build()
	assert.Nil(t, err)
	wasmBuildTest.verifyExpectation()
}

func Test_BuildLinuxReleaseVersion(t *testing.T) {
	relativePath := "." + string(os.PathSeparator) + filepath.Join("cmd", "terminal")

	// The build merges any ambient CGO_CFLAGS/CGO_LDFLAGS in place (appendEnv),
	// yielding a single combined entry. The osEnv-based expectation below also
	// re-appends the raw ambient value, so a developer shell that exports these
	// (e.g. Homebrew sets both) would double-count them and fail the comparison
	// — while a clean CI env passes. Clear them for the test so the expected env
	// is deterministic regardless of the host environment.
	for _, key := range []string{"CGO_CFLAGS", "CGO_LDFLAGS"} {
		if orig, ok := os.LookupEnv(key); ok {
			os.Unsetenv(key)
			t.Cleanup(func() { os.Setenv(key, orig) })
		}
	}

	archcflags := ""
	switch targetArch() {
	case "arm64":
		archcflags = " -mbranch-protection=bti+pac-ret"
	}

	expected := []mockRunner{
		{
			expectedValue: expectedValue{args: []string{"mod", "edit", "-json"}, dir: filepath.Join("myTest", relativePath)},
			mockReturn: mockReturn{
				ret: []byte("{ \"Module\": { \"Path\": \"github.com/belluikit/bellui\"} }"),
			},
		},
		{
			expectedValue: expectedValue{
				args:  []string{"build", "-trimpath", "-ldflags", "-s -w", "-tags", "release", relativePath},
				env:   []string{"CGO_ENABLED=1", "GOOS=linux", fmt.Sprintf("CGO_CFLAGS=%s %s%s", baseCFLAGSRelease, hardeningCFLAGS, archcflags), "CGO_LDFLAGS=" + hardeningLDFLAGSLinux},
				osEnv: true,
				dir:   "myTest",
			},
			mockReturn: mockReturn{
				ret: []byte(""),
			},
		},
	}

	linuxBuildTest := &testCommandRuns{runs: expected, t: t}
	b := &Builder{appData: &appData{}, os: "linux", srcdir: "myTest", release: true, runner: linuxBuildTest, goPackage: relativePath}
	err := b.build()
	assert.Nil(t, err)
	linuxBuildTest.verifyExpectation()
}

type jsonTest struct {
	expected bool
	json     []byte
}

// Every bellui release supports embedded metadata, so the init file is written
// whenever the app builds against bellui, whatever its version. Before the
// rebrand this was gated on Fyne's version numbers (>=2.2), which no bellui
// release matches, so no packaged app got its metadata (#337).
func Test_BelluiGoMod(t *testing.T) {
	jsonTests := []jsonTest{
		{true, []byte(`{"Module": {"Path": "example.com/app"},"Go": "1.25", "Require": [ { "Path": "github.com/belluikit/bellui","Version": "v1.0.0"} ] }`)},
		{true, []byte(`{"Module": {"Path": "example.com/app"},"Go": "1.25", "Require": [ { "Path": "github.com/belluikit/bellui","Version": "v1.1.0"} ] }`)},
		{true, []byte(`{"Module": {"Path": "example.com/app"},"Go": "1.25", "Require": [ { "Path": "github.com/belluikit/bellui","Version": "v1.1.1-0.20261015101010-abcdef123456"} ] }`)},
		{true, []byte(`{ "Module": {"Path": "github.com/belluikit/bellui"},"Require": [{ "Path": "test","Version": "v2.1.4"} ] }`)},
		{false, []byte(`{"Module": {"Path": "example.com/app"},"Go": "1.25", "Require": [ { "Path": "example.com/other","Version": "v1.0.0"} ] }`)},
	}

	for _, j := range jsonTests {
		expected := []mockRunner{
			{
				expectedValue: expectedValue{args: []string{"mod", "edit", "-json"}},
				mockReturn:    mockReturn{ret: j.json},
			},
		}

		called := false

		belluiGoModTest := &testCommandRuns{runs: expected, t: t}
		_, err := injectMetadataIfPossible(belluiGoModTest, "myTest", &appData{},
			func(string, *appData) (func(), error) {
				called = true
				return func() {}, nil
			})

		assert.Equal(t, j.expected, called, string(j.json))
		assert.Equal(t, !j.expected, err != nil, string(j.json))
	}
}

func Test_AppendEnv(t *testing.T) {
	env := []string{
		"foo=bar",
		"bar=baz",
		"foo1=bar=baz",
	}

	appendEnv(&env, "foo2", "baz2")
	appendEnv(&env, "foo", "baz")
	appendEnv(&env, "foo1", "-bar")

	if assert.Len(t, env, 4) {
		assert.Equal(t, "foo=bar baz", env[0])
		assert.Equal(t, "bar=baz", env[1])
		assert.Equal(t, "foo1=bar=baz -bar", env[2])
		assert.Equal(t, "foo2=baz2", env[3])
	}
}

type extractTest struct {
	value       string
	wantLdFlags string
	wantGoFlags string
}

func Test_ExtractLdFlags(t *testing.T) {
	goFlagsTests := []extractTest{
		{"-ldflags=-w", "-w", ""},
		{"-ldflags=-s", "-s", ""},
		{"-ldflags=-w -ldflags=-s", "-w -s", ""},
		{"-mod=vendor", "", "-mod=vendor"},
	}

	for _, test := range goFlagsTests {
		ldFlags, goFlags := extractLdFlags(test.value)
		assert.Equal(t, test.wantLdFlags, ldFlags)
		assert.Equal(t, test.wantGoFlags, goFlags)
	}
}
