package util

import (
	"debug/buildinfo"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/module"
)

// goffmpegLibsPrefix is the path of goffmpeg's FFmpeg builds, one module per
// target (github.com/belluikit/goffmpeg-libs/darwin_arm64, …). Each holds the
// shared libraries an app links: the media codec set in shared/, the audio
// set (goffmpeg_novideo) in shared-audio/ (goffmpeg#40, #41). A packaged app
// carries the ones its executable loads, so a user can replace them (LGPL
// 2.1 §6b; player ADR-0037 D3; #1101, #1122).
const goffmpegLibsPrefix = "github.com/belluikit/goffmpeg-libs/"

// SharedLibDirs returns the directories holding the shared libraries the
// binary's modules ship: goffmpeg-libs' shared/ and shared-audio/, in the
// module cache, found through the binary's build info. bin is an executable
// or, on Android, the app's c-shared library. None when it links no
// goffmpeg-libs module.
func SharedLibDirs(bin string) ([]string, error) {
	info, err := buildinfo.ReadFile(bin)
	if err != nil {
		return nil, nil // not a Go binary we can inspect; nothing to bundle
	}
	var dirs []string
	for _, dep := range info.Deps {
		if dep.Replace != nil {
			dep = dep.Replace
		}
		if !strings.HasPrefix(dep.Path, goffmpegLibsPrefix) {
			continue
		}
		root, err := ModuleDir(dep.Path, dep.Version)
		if err != nil {
			return nil, err
		}
		for _, set := range []string{"shared", "shared-audio"} {
			if d := filepath.Join(root, set); dirExists(d) {
				dirs = append(dirs, d)
			}
		}
	}
	return dirs, nil
}

// ModuleDir is where the module cache holds path@version, or path itself for
// a replacement by a local directory (no version).
func ModuleDir(path, version string) (string, error) {
	if version == "" {
		return path, nil
	}
	cache, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		return "", fmt.Errorf("go env GOMODCACHE: %w", err)
	}
	escPath, err := module.EscapePath(path)
	if err != nil {
		return "", err
	}
	escVersion, err := module.EscapeVersion(version)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(strings.TrimSpace(string(cache)), escPath+"@"+escVersion)
	if !dirExists(dir) {
		return "", fmt.Errorf("the executable links %s@%s, which is not in the module cache (%s): run go mod download", path, version, dir)
	}
	return dir, nil
}

func dirExists(dir string) bool {
	fi, err := os.Stat(dir)
	return err == nil && fi.IsDir()
}

// SharedLib is one shared library a binary loads from a SharedLibDirs
// directory.
type SharedLib struct {
	// Name is how the binary names it, relative to the directory: a file
	// (libavutil-goffmpeg.so), or the binary inside a framework
	// (libavutil-goffmpeg.framework/libavutil-goffmpeg).
	Name string
	// Path is the file in the directory.
	Path string
}

// Framework is the framework directory the library sits in, relative to its
// SharedLibDirs directory (libavutil-goffmpeg.framework), or "" for a plain
// file.
func (l SharedLib) Framework() string {
	if dir, _, ok := strings.Cut(filepath.ToSlash(l.Name), "/"); ok && strings.HasSuffix(dir, ".framework") {
		return dir
	}
	return ""
}

// SharedLibs returns every shared library bin loads that one of dirs holds,
// and the ones those load in turn, in the order found.
func SharedLibs(bin string, dirs []string) ([]SharedLib, error) {
	var found []SharedLib
	queue := []string{bin}
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		names, err := ImportedLibs(next)
		if err != nil {
			return nil, fmt.Errorf("reading the libraries %s loads: %w", filepath.Base(next), err)
		}
		for _, name := range names {
			if slices.ContainsFunc(found, func(l SharedLib) bool { return l.Name == name }) {
				continue
			}
			src := findLib(dirs, name)
			if src == "" {
				continue // a system library, or not one of ours
			}
			found = append(found, SharedLib{Name: name, Path: src})
			queue = append(queue, src)
		}
	}
	return found, nil
}

// findLib returns the file called name in one of dirs, or "". name may be a
// path inside a framework. Windows matches DLL names without regard to case.
func findLib(dirs []string, name string) string {
	for _, d := range dirs {
		if strings.Contains(name, "/") {
			if p := filepath.Join(d, filepath.FromSlash(name)); fileExists(p) {
				return p
			}
			continue
		}
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if e.Name() == name || (strings.HasSuffix(strings.ToLower(name), ".dll") && strings.EqualFold(e.Name(), name)) {
				return filepath.Join(d, e.Name())
			}
		}
	}
	return ""
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// ImportedLibs lists the shared libraries a Mach-O, ELF or PE file loads, by
// file name. On Mach-O only the ones found through the rpath (@rpath/…) count,
// named by what follows it: the rest name the system's own.
func ImportedLibs(path string) ([]string, error) {
	if f, err := macho.Open(path); err == nil {
		defer f.Close()
		libs, err := f.ImportedLibraries()
		var names []string
		for _, l := range libs {
			if rest, ok := strings.CutPrefix(l, "@rpath/"); ok {
				names = append(names, rest)
			}
		}
		return names, err
	}
	if f, err := elf.Open(path); err == nil {
		defer f.Close()
		return f.ImportedLibraries()
	}
	if f, err := pe.Open(path); err == nil {
		defer f.Close()
		// pe.File.ImportedLibraries is a stub that returns none; each
		// imported symbol names its DLL ("CreateFileW:kernel32.dll").
		syms, err := f.ImportedSymbols()
		var names []string
		for _, s := range syms {
			if _, dll, ok := strings.Cut(s, ":"); ok && !slices.Contains(names, dll) {
				names = append(names, dll)
			}
		}
		return names, err
	}
	return nil, errors.New("not a Mach-O, ELF or PE file")
}
