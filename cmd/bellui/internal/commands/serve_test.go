package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_validatePort(t *testing.T) {
	valid := &Server{port: 80}
	assert.Nil(t, valid.validate())
	assert.Equal(t, 80, valid.port)

	def := &Server{}
	assert.Nil(t, def.validate())
	assert.Equal(t, 8080, def.port)

	invalidLow := &Server{port: -1}
	assert.NotNil(t, invalidLow.validate())

	invalidHigh := &Server{port: 65536}
	assert.NotNil(t, invalidHigh.validate())
}

// An app with no manifest and no --app-id has no ID on a web build, so serve
// names it by the name packaging gives it, the module's last path element
// (#703). The appData is the one serve shares with its Packager.
func Test_servingMessage_NoManifest(t *testing.T) {
	prevUtil := util
	util = defaultUtil{}
	t.Cleanup(func() { util = prevUtil })

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/apps/hello\n\ngo 1.25\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	t.Chdir(dir) // validate changes into the source directory; restore it afterwards

	s := &Server{appData: &appData{}, srcDir: dir}
	p := &Packager{os: "wasm", srcDir: s.srcDir, appData: s.appData, release: true}
	require.NoError(t, p.validate())
	t.Cleanup(func() { _ = os.RemoveAll(p.tempDir) })
	require.Empty(t, s.appData.AppID, "the case under test: no manifest, so no app ID")

	assert.Equal(t, "Serving hello at: http://localhost:8080\n", servingMessage(s.appData, 8080))

	s.appData.AppID = "com.example.hello"
	assert.Equal(t, "Serving com.example.hello at: http://localhost:8080\n", servingMessage(s.appData, 8080),
		"an app ID, when there is one, still names the app")
}
