package commands

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppID(t *testing.T) {
	for _, test := range []struct {
		modpath string
		appid   string
	}{
		{"github.com/example/bellui-example", "com.github.example.bellui_example"},
		{"example/bellui-example", "example.bellui_example"},
		{"bellui-example", "bellui_example"},
		{"bellui.example", "example.bellui"},
		{"bellui.example/app", "example.bellui.app"},
		{"sub.example.com", "com.example.sub"},
	} {
		assert.Equal(t, test.appid, getAppID(test.modpath))
	}
}

func TestAppName(t *testing.T) {
	for _, test := range []struct {
		modpath string
		appname string
	}{
		{"github.com/example/bellui-example", "bellui-example"},
		{"example/bellui-example", "bellui-example"},
		{"bellui-example", "bellui-example"},
		{"example.bellui", "example"},
		{"bellui.example/app", "app"},
		{"sub.domain.bellui.example", "sub"},
	} {
		assert.Equal(t, test.appname, getAppName(test.modpath))
	}
}

func TestCheckFileOrDoCreate(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "checkfile")

	done := 0
	assert.Nil(t, checkFileOrDo(file, func() error {
		done++
		return nil
	}))
	assert.Equal(t, 1, done)
}
