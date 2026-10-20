// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gl

import (
	"archive/tar"
	"compress/gzip"
	"debug/pe"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

var debug = log.New(io.Discard, "gl: ", log.LstdFlags)

func downloadDLLs() (path string, err error) {
	url := "https://dl.google.com/go/mobile/angle-bd3f8780b-" + runtime.GOARCH + ".tgz"
	debug.Printf("downloading %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("gl: %v", err)
	}
	defer func() {
		err2 := resp.Body.Close()
		if err == nil && err2 != nil {
			err = fmt.Errorf("gl: error reading body from %v: %v", url, err2)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("gl: error fetching %v, status: %v", url, resp.Status)
		return "", err
	}

	r, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("gl: error reading gzip from %v: %v", url, err)
	}
	tr := tar.NewReader(r)
	var bytesGLESv2, bytesEGL, bytesD3DCompiler []byte
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("gl: error reading tar from %v: %v", url, err)
		}
		switch header.Name {
		case "angle-" + runtime.GOARCH + "/libglesv2.dll":
			bytesGLESv2, err = io.ReadAll(tr)
		case "angle-" + runtime.GOARCH + "/libegl.dll":
			bytesEGL, err = io.ReadAll(tr)
		case "angle-" + runtime.GOARCH + "/d3dcompiler_47.dll":
			bytesD3DCompiler, err = io.ReadAll(tr)
		default: // skip
		}
		if err != nil {
			return "", fmt.Errorf("gl: error reading %v from %v: %v", header.Name, url, err)
		}
	}
	if len(bytesGLESv2) == 0 || len(bytesEGL) == 0 || len(bytesD3DCompiler) == 0 {
		return "", fmt.Errorf("gl: did not find all DLLs in %v", url)
	}

	writeDLLs := func(path string) error {
		if err := installDLL(path, "libglesv2.dll", bytesGLESv2); err != nil {
			return fmt.Errorf("gl: cannot install ANGLE: %v", err)
		}
		if err := installDLL(path, "libegl.dll", bytesEGL); err != nil {
			return fmt.Errorf("gl: cannot install ANGLE: %v", err)
		}
		if err := installDLL(path, "d3dcompiler_47.dll", bytesD3DCompiler); err != nil {
			return fmt.Errorf("gl: cannot install ANGLE: %v", err)
		}
		return nil
	}

	// First, we attempt to install these DLLs in LOCALAPPDATA/Shiny.
	//
	// Traditionally we would use the system32 directory, but it is
	// no longer writable by normal programs.
	os.MkdirAll(appdataPath(), 0o775)
	if err := writeDLLs(appdataPath()); err == nil {
		return appdataPath(), nil
	}
	debug.Printf("DLLs could not be written to %s", appdataPath())

	// Second, install in GOPATH/pkg if it exists.
	gopath := os.Getenv("GOPATH")
	gopathpkg := filepath.Join(gopath, "pkg")
	if _, err := os.Stat(gopathpkg); err == nil && gopath != "" {
		if err := writeDLLs(gopathpkg); err == nil {
			return gopathpkg, nil
		}
	}
	debug.Printf("DLLs could not be written to GOPATH")

	// Third, pick a temporary directory.
	tmp := os.TempDir()
	if err := writeDLLs(tmp); err != nil {
		return "", fmt.Errorf("gl: unable to install ANGLE DLLs: %v", err)
	}
	return tmp, nil
}

// installDLL writes data to dir/name so that no process ever reads a partial
// file. Processes started together on a fresh machine all miss ANGLE and all
// install it into the same directory (#438): writing in place let one read a
// file another had only half written, or fail to open one another had already
// loaded, which Windows locks. The data goes to a temporary file in the same
// directory and is renamed into place. When the rename fails because another
// process has already installed the file, and loaded it, a complete copy is
// there and is kept.
func installDLL(dir, name string, data []byte) error {
	f, err := os.CreateTemp(dir, name+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, werr := f.Write(data)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(tmp)
		return werr
	}
	if err := os.Rename(tmp, filepath.Join(dir, name)); err != nil {
		os.Remove(tmp)
		if compatibleDLL(dir, name) {
			return nil // installed by another process
		}
		return err
	}
	return nil
}

func appdataPath() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "GoGL", runtime.GOARCH)
}

func containsDLLs(dir string) bool {
	return compatibleDLL(dir, "libglesv2.dll") && compatibleDLL(dir, "libegl.dll") && compatibleDLL(dir, "d3dcompiler_47.dll")
}

// compatibleDLL reports whether dir/name is a complete DLL for this machine.
func compatibleDLL(dir, name string) bool {
	file, err := pe.Open(filepath.Join(dir, name))
	if err != nil {
		return false
	}
	defer file.Close()

	switch file.Machine {
	case pe.IMAGE_FILE_MACHINE_AMD64:
		return "amd64" == runtime.GOARCH
	case pe.IMAGE_FILE_MACHINE_ARM:
		return "arm" == runtime.GOARCH
	case pe.IMAGE_FILE_MACHINE_I386:
		return "386" == runtime.GOARCH
	}
	return false
}

func chromePath() string {
	// dlls are stored in:
	//   <BASE>/<VERSION>/libglesv2.dll

	installdirs := []string{
		// Chrome User
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application"),
		// Chrome System
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application"),
		// Chromium
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Chromium", "Application"),
		// Chrome Canary
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome SxS", "Application"),
	}

	for _, installdir := range installdirs {
		versiondirs, err := os.ReadDir(installdir)
		if err != nil {
			continue
		}

		for _, versiondir := range versiondirs {
			if !versiondir.IsDir() {
				continue
			}

			versionpath := filepath.Join(installdir, versiondir.Name())
			if containsDLLs(versionpath) {
				return versionpath
			}
		}
	}

	return ""
}

func findDLLs() (err error) {
	load := func(path string) (bool, error) {
		if path != "" {
			// don't try to start when one of the files is missing
			if !containsDLLs(path) {
				return false, nil
			}

			LibD3DCompiler.Name = filepath.Join(path, filepath.Base(LibD3DCompiler.Name))
			LibGLESv2.Name = filepath.Join(path, filepath.Base(LibGLESv2.Name))
			LibEGL.Name = filepath.Join(path, filepath.Base(LibEGL.Name))
		}

		if err := LibGLESv2.Load(); err == nil {
			if err := LibEGL.Load(); err != nil {
				return false, fmt.Errorf("gl: loaded libglesv2 but not libegl: %v", err)
			}
			if err := LibD3DCompiler.Load(); err != nil {
				return false, fmt.Errorf("gl: loaded libglesv2, libegl but not d3dcompiler: %v", err)
			}
			if path == "" {
				debug.Printf("DLLs found")
			} else {
				debug.Printf("DLLs found in: %q", path)
			}
			return true, nil
		}

		return false, nil
	}

	// Look in the system directory.
	if ok, err := load(""); ok || err != nil {
		return err
	}

	// Look in the AppData directory.
	if ok, err := load(appdataPath()); ok || err != nil {
		return err
	}

	// Look for a Chrome installation
	if dir := chromePath(); dir != "" {
		if ok, err := load(dir); ok || err != nil {
			return err
		}
	}

	// Look in GOPATH/pkg.
	if ok, err := load(filepath.Join(os.Getenv("GOPATH"), "pkg")); ok || err != nil {
		return err
	}

	// Look in temporary directory.
	if ok, err := load(os.TempDir()); ok || err != nil {
		return err
	}

	// Download the DLL binary.
	path, err := downloadDLLs()
	if err != nil {
		return err
	}
	debug.Printf("DLLs written to %s", path)
	ok, err := load(path)
	if !ok && err == nil {
		// Retry once: a failed load right after installing is the race #438
		// was, and installDLL should have closed it.
		ok, err = load(path)
	}
	if !ok || err != nil {
		return fmt.Errorf("gl: unable to load ANGLE after installation: %v", err)
	}
	return nil
}
