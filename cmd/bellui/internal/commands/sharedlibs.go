package commands

import (
	"bytes"
	"debug/elf"
	"debug/macho"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	realUtil "github.com/belluikit/bellui/cmd/bellui/internal/util"
)

// bundleSharedLibs copies into dest every shared library exe loads from
// dirs (realUtil.SharedLibs), keeping their names. It returns the names
// copied, in the order found.
func bundleSharedLibs(exe string, dirs []string, dest string) ([]string, error) {
	libs, err := realUtil.SharedLibs(exe, dirs)
	if err != nil || len(libs) == 0 {
		return nil, err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, err
	}
	var copied []string
	for _, l := range libs {
		if err := util.CopyExeFile(l.Path, filepath.Join(dest, l.Name)); err != nil {
			return nil, fmt.Errorf("copying %s: %w", l.Name, err)
		}
		copied = append(copied, l.Name)
	}
	return copied, nil
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
	dirs, err := realUtil.SharedLibDirs(exe)
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
	dirs, err := realUtil.SharedLibDirs(exe)
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
	dirs, err := realUtil.SharedLibDirs(exe)
	if err != nil || len(dirs) == 0 {
		return nil, err
	}
	libs, err := bundleSharedLibs(exe, dirs, filepath.Dir(exe))
	if err == nil && len(libs) > 0 {
		log.Printf("bundled %s beside %s", strings.Join(libs, ", "), filepath.Base(exe))
	}
	return libs, err
}
