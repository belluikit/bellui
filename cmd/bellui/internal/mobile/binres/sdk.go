package binres

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
)

// MinSDK is the targeted sdk version for support by package binres.
const MinSDK = 21

// SDKPlatform is the SDK platform directory name the embedded resource table
// (arsc.go) is generated from. Regeneration (go generate, which runs
// genarsc.go) resolves exactly this platform — never simply the newest
// installed one — so the generated output is reproducible. Bump it
// deliberately when the table needs attribute or flag values introduced by a
// newer API level, then regenerate.
const SDKPlatform = "android-35"

// Requires environment variable ANDROID_HOME to be set.
func apiResources() ([]byte, error) {
	apiResPath, err := sdkAPIResourcesPath()
	if err != nil {
		return nil, err
	}
	zr, err := zip.OpenReader(apiResPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(`%v; consider installing with "sdkmanager 'platforms;%s'"`, err, SDKPlatform)
		}
		return nil, err
	}
	defer zr.Close()

	buf := new(bytes.Buffer)
	for _, f := range zr.File {
		if f.Name == "resources.arsc" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			_, err = io.Copy(buf, rc)
			if err != nil {
				return nil, err
			}
			err = rc.Close()
			if err != nil {
				return nil, err
			}
			break
		}
	}
	if buf.Len() == 0 {
		return nil, fmt.Errorf("failed to read resources.arsc")
	}
	return buf.Bytes(), nil
}

// sdkAPIResourcesPath returns the path of the android.jar belonging to the
// pinned SDKPlatform. It is the jar the embedded resource table is generated
// from; use LatestAPIResourcesPath for build steps that just need any recent
// platform jar.
func sdkAPIResourcesPath() (string, error) {
	sdkdir := os.Getenv("ANDROID_HOME")
	if sdkdir == "" {
		return "", fmt.Errorf("ANDROID_HOME env var not set")
	}
	return path.Join(sdkdir, "platforms", SDKPlatform, "android.jar"), nil
}

// LatestAPIResourcesPath returns the android.jar of the newest installed SDK
// platform.
func LatestAPIResourcesPath() (string, error) {
	sdkdir := os.Getenv("ANDROID_HOME")
	if sdkdir == "" {
		return "", fmt.Errorf("ANDROID_HOME env var not set")
	}
	parent := path.Join(sdkdir, "platforms")
	dir, err := os.ReadDir(parent)
	if err != nil {
		return "", err
	}

	maxSDK := ""
	for _, d := range dir {
		if d.Name() > maxSDK {
			maxSDK = d.Name()
		}
	}
	return path.Join(sdkdir, "platforms", maxSDK, "android.jar"), nil
}

// checkStripAssumptions reports whether pkg is shaped the way PackResources'
// stripping hard-codes: the four leading type specs being, in order,
// attr/id/style/string. A platform that breaks that must fail to pack rather
// than embed a garbage table. Both halves are load-bearing — the type pool
// names the types, but it is spec order that decides which specs PackResources'
// pkg.specs[3] and pkg.specs[:4] actually reach.
func checkStripAssumptions(pkg *Package) error {
	want := []string{"attr", "id", "style", "string"}
	if len(pkg.typePool.strings) < len(want) || !slices.Equal(pkg.typePool.strings[:len(want)], want) {
		return fmt.Errorf("%s type pool no longer starts with %v: %v", SDKPlatform, want, pkg.typePool.strings[:min(len(pkg.typePool.strings), 6)])
	}
	if len(pkg.specs) < len(want) {
		return fmt.Errorf("%s package declares %v type specs, want at-least %v", SDKPlatform, len(pkg.specs), len(want))
	}
	for i := range want {
		if int(pkg.specs[i].id) != i+1 {
			return fmt.Errorf("%s type spec %v has type id %v, want %v (%q)", SDKPlatform, i, pkg.specs[i].id, i+1, want[i])
		}
	}
	return nil
}

// PackResources produces a stripped down gzip version of the resources.arsc from api jar.
func PackResources() ([]byte, error) {
	tbl, err := OpenSDKTable()
	if err != nil {
		return nil, err
	}

	tbl.pool.strings = []string{} // should not be needed
	pkg := tbl.pkgs[0]

	if err := checkStripAssumptions(pkg); err != nil {
		return nil, err
	}

	// drop language string entries
	for _, typ := range pkg.specs[3].types {
		if typ.config.locale.language != 0 {
			for j, nt := range typ.entries {
				if nt == nil { // NoEntry
					continue
				}
				pkg.keyPool.strings[nt.key] = ""
				typ.indices[j] = NoEntry
				typ.entries[j] = nil
			}
		}
	}

	// drop strings from pool for specs to be dropped
	for _, spec := range pkg.specs[4:] {
		for _, typ := range spec.types {
			for _, nt := range typ.entries {
				if nt == nil { // NoEntry
					continue
				}
				// don't drop if there's a collision
				var collision bool
				for _, xspec := range pkg.specs[:4] {
					for _, xtyp := range xspec.types {
						for _, xnt := range xtyp.entries {
							if xnt == nil {
								continue
							}
							if collision = nt.key == xnt.key; collision {
								break
							}
						}
					}
				}
				if !collision {
					pkg.keyPool.strings[nt.key] = ""
				}
			}
		}
	}

	// entries are densely packed but probably safe to drop nil entries off the end
	for _, spec := range pkg.specs[:4] {
		for _, typ := range spec.types {
			var last int
			for i, nt := range typ.entries {
				if nt != nil {
					last = i
				}
			}
			typ.entries = typ.entries[:last+1]
			typ.indices = typ.indices[:last+1]
		}
	}

	// keeping 0:attr, 1:id, 2:style, 3:string
	pkg.typePool.strings = pkg.typePool.strings[:4]
	pkg.specs = pkg.specs[:4]

	bin, err := tbl.MarshalBinary()
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)

	zw := gzip.NewWriter(buf)
	if _, err := zw.Write(bin); err != nil {
		return nil, err
	}
	if err := zw.Flush(); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
