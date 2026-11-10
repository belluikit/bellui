package commands

import (
	"bytes"
	"debug/buildinfo"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"errors"
	"fmt"
	"log"
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
// 2.1 §6b; player ADR-0037 D3; #1101).
const goffmpegLibsPrefix = "github.com/belluikit/goffmpeg-libs/"

// sharedLibDirs returns the directories holding the shared libraries the
// executable's modules ship: goffmpeg-libs' shared/ and shared-audio/, in the
// module cache, found through the executable's build info. None when it links
// no goffmpeg-libs module.
func sharedLibDirs(exe string) ([]string, error) {
	info, err := buildinfo.ReadFile(exe)
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
		root, err := moduleDir(dep.Path, dep.Version)
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

// moduleDir is where the module cache holds path@version, or path itself for
// a replacement by a local directory (no version).
func moduleDir(path, version string) (string, error) {
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

// bundleSharedLibs copies into dest every shared library exe loads that one
// of dirs holds, and the ones those load in turn, keeping their names. It
// returns the names copied, in the order found.
func bundleSharedLibs(exe string, dirs []string, dest string) ([]string, error) {
	var copied []string
	queue := []string{exe}
	for len(queue) > 0 {
		bin := queue[0]
		queue = queue[1:]
		libs, err := importedLibs(bin)
		if err != nil {
			return nil, fmt.Errorf("reading the libraries %s loads: %w", filepath.Base(bin), err)
		}
		for _, name := range libs {
			if slices.Contains(copied, name) {
				continue
			}
			src := findLib(dirs, name)
			if src == "" {
				continue // a system library, or not one of ours
			}
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return nil, err
			}
			if err := util.CopyExeFile(src, filepath.Join(dest, name)); err != nil {
				return nil, fmt.Errorf("copying %s: %w", name, err)
			}
			copied = append(copied, name)
			queue = append(queue, src)
		}
	}
	return copied, nil
}

// findLib returns the file called name in one of dirs, or "". Windows
// matches DLL names without regard to case.
func findLib(dirs []string, name string) string {
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.Name() == name || (strings.HasSuffix(strings.ToLower(name), ".dll") && strings.EqualFold(e.Name(), name)) {
				return filepath.Join(d, e.Name())
			}
		}
	}
	return ""
}

// importedLibs lists the shared libraries a Mach-O, ELF or PE file loads, by
// file name. On Mach-O only the ones found through the rpath (@rpath/…) count:
// the rest name the system's own.
func importedLibs(path string) ([]string, error) {
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

// underAny reports whether path is one of dirs or inside one.
func underAny(path string, dirs []string) bool {
	for _, d := range dirs {
		if path == d || strings.HasPrefix(path, d+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// moduleRoots returns the module directories dirs sit in: the rpaths goffmpeg
// adds name the set directory, and either set's must go.
func moduleRoots(dirs []string) []string {
	var roots []string
	for _, d := range dirs {
		if r := filepath.Dir(d); !slices.Contains(roots, r) {
			roots = append(roots, r)
		}
	}
	return roots
}

// relinkDarwin points a macOS executable at the libraries bundled beside it:
// it drops the rpaths into the module cache, which name the build machine,
// adds rpath, and signs the executable again (ad hoc), since changing a load
// command breaks its signature and arm64 runs no unsigned code.
func relinkDarwin(bin string, dirs []string, rpath string) error {
	f, err := macho.Open(bin)
	if err != nil {
		return err
	}
	var args []string
	has := false
	for _, l := range f.Loads {
		r, ok := l.(*macho.Rpath)
		if !ok {
			continue
		}
		if r.Path == rpath {
			has = true
		} else if underAny(r.Path, moduleRoots(dirs)) {
			args = append(args, "-delete_rpath", r.Path)
		}
	}
	f.Close()
	if !has {
		args = append(args, "-add_rpath", rpath)
	}
	if len(args) > 0 {
		if out, err := exec.Command("install_name_tool", append(args, bin)...).CombinedOutput(); err != nil {
			return fmt.Errorf("install_name_tool: %w\n%s", err, out)
		}
	}
	if out, err := exec.Command("codesign", "--force", "--sign", "-", bin).CombinedOutput(); err != nil {
		return fmt.Errorf("codesign: %w\n%s", err, out)
	}
	return nil
}

// relinkELF points an ELF executable at the libraries bundled beside it: its
// DT_RUNPATH (or DT_RPATH) keeps every entry outside the module cache's
// goffmpeg-libs and gains rpath in their place. The string is rewritten where
// it lies, so the new one may not be longer; the module cache paths it
// replaces are much longer than an $ORIGIN path.
func relinkELF(bin string, dirs []string, rpath string) error {
	f, err := elf.Open(bin)
	if err != nil {
		return err
	}
	defer f.Close()
	dynstr := f.Section(".dynstr")
	dynamic := f.Section(".dynamic")
	if dynstr == nil || dynamic == nil {
		return nil // statically linked: nothing loads a library
	}
	dyn, err := dynamic.Data()
	if err != nil {
		return err
	}
	strs, err := dynstr.Data()
	if err != nil {
		return err
	}
	entry := 16
	if f.Class == elf.ELFCLASS32 {
		entry = 8
	}
	for off := 0; off+entry <= len(dyn); off += entry {
		var tag, val uint64
		if entry == 16 {
			tag, val = f.ByteOrder.Uint64(dyn[off:]), f.ByteOrder.Uint64(dyn[off+8:])
		} else {
			tag, val = uint64(f.ByteOrder.Uint32(dyn[off:])), uint64(f.ByteOrder.Uint32(dyn[off+4:]))
		}
		if elf.DynTag(tag) != elf.DT_RUNPATH && elf.DynTag(tag) != elf.DT_RPATH {
			continue
		}
		if val >= uint64(len(strs)) {
			return fmt.Errorf("%s: an rpath outside .dynstr", bin)
		}
		oldLen := bytes.IndexByte(strs[val:], 0)
		if oldLen < 0 {
			return fmt.Errorf("%s: an unterminated rpath", bin)
		}
		kept := []string{}
		replaced := false
		for _, p := range strings.Split(string(strs[val:val+uint64(oldLen)]), ":") {
			if underAny(p, moduleRoots(dirs)) {
				replaced = true
				continue
			}
			if p != rpath {
				kept = append(kept, p)
			}
		}
		if !replaced {
			continue
		}
		next := strings.Join(append(kept, rpath), ":")
		if len(next) > oldLen {
			return fmt.Errorf("%s: the rpath %q does not fit where %q was", bin, next, strs[val:val+uint64(oldLen)])
		}
		buf := make([]byte, oldLen+1)
		copy(buf, next)
		return writeAt(bin, int64(dynstr.Offset+val), buf)
	}
	return nil
}

func writeAt(path string, off int64, b []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if _, err := file.WriteAt(b, off); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// bundleDarwinLibs puts the shared libraries exe loads in the app's
// Contents/Frameworks and points bin, the bundle's copy of exe, at them.
func bundleDarwinLibs(exe, bin, contentsDir string) error {
	dirs, err := sharedLibDirs(exe)
	if err != nil || len(dirs) == 0 {
		return err
	}
	libs, err := bundleSharedLibs(exe, dirs, filepath.Join(contentsDir, "Frameworks"))
	if err != nil || len(libs) == 0 {
		return err
	}
	log.Printf("bundled %s in Contents/Frameworks", strings.Join(libs, ", "))
	return relinkDarwin(bin, dirs, "@executable_path/../Frameworks")
}

// bundleUnixLibs puts the shared libraries exe loads in libDir and points
// bin, the package's copy of exe, at them through rpath, relative to bin
// ($ORIGIN). It reports whether it bundled any.
func bundleUnixLibs(exe, bin, libDir, rpath string) (bool, error) {
	dirs, err := sharedLibDirs(exe)
	if err != nil || len(dirs) == 0 {
		return false, err
	}
	libs, err := bundleSharedLibs(exe, dirs, libDir)
	if err != nil || len(libs) == 0 {
		return false, err
	}
	log.Printf("bundled %s in %s", strings.Join(libs, ", "), libDir)
	return true, relinkELF(bin, dirs, rpath)
}

// bundleWindowsLibs puts the DLLs exe loads beside it, where Windows looks
// first, and returns their names.
func bundleWindowsLibs(exe string) ([]string, error) {
	dirs, err := sharedLibDirs(exe)
	if err != nil || len(dirs) == 0 {
		return nil, err
	}
	libs, err := bundleSharedLibs(exe, dirs, filepath.Dir(exe))
	if err == nil && len(libs) > 0 {
		log.Printf("bundled %s beside %s", strings.Join(libs, ", "), filepath.Base(exe))
	}
	return libs, err
}
