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
	"strconv"
	"strings"
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

// SDKPlatformVersion and SDKPlatformCodename are what aapt records for
// SDKPlatform when packaging a manifest against it: the API level goes into
// android:compileSdkVersion and platformBuildVersionCode, the Android release
// name into android:compileSdkVersionCodename and platformBuildVersionName.
// UnmarshalXML synthesizes the same four attributes. Bump together with
// SDKPlatform.
const (
	SDKPlatformVersion  = 35
	SDKPlatformCodename = "15"
)

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

// platformAPILevel returns the numeric API level of a platforms/ directory
// named exactly "android-<level>", or false for anything else — preview
// platforms ("android-VanillaIceCream"), extension platforms
// ("android-36-ext4") and stray files.
func platformAPILevel(name string) (int, bool) {
	suffix, ok := strings.CutPrefix(name, "android-")
	if !ok {
		return 0, false
	}
	level, err := strconv.Atoi(suffix)
	if err != nil || level <= 0 {
		return 0, false
	}
	return level, true
}

// LatestAPIResourcesPath returns the android.jar of the newest installed SDK
// platform, by numeric API level — a lexical sort would rank android-9 above
// android-36 (#296). Platform directories without a numeric level (previews,
// extension SDKs) are deliberately ordered behind every numbered release and
// compared lexically among themselves, so one is only picked when nothing
// numbered is installed.
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

	maxSDK, maxLevel := "", 0
	for _, d := range dir {
		// A platform is a directory; a stray file named like one is not a
		// platform. ReadDir does not follow symlinks, so a symlinked platform
		// reports as ModeSymlink rather than a directory — still a candidate.
		if !d.IsDir() && d.Type()&os.ModeSymlink == 0 {
			continue
		}
		name := d.Name()
		if !strings.HasPrefix(name, "android-") {
			continue
		}
		level, numeric := platformAPILevel(name)
		switch {
		case numeric && level > maxLevel:
			maxSDK, maxLevel = name, level
		case !numeric && maxLevel == 0 && name > maxSDK:
			maxSDK = name
		}
	}
	if maxSDK == "" {
		return "", fmt.Errorf("no SDK platforms installed in %s", parent)
	}
	return path.Join(parent, maxSDK, "android.jar"), nil
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

// stripKeyPool blanks every key-pool string no surviving entry of the kept
// specs (pkg.specs[:4], the attr/id/style/string that checkStripAssumptions
// pins) references. A key index is shared whenever the same name recurs across
// specs — id/screenLayout and the dropped ^attr-private/screenLayout share one
// slot — so dropped entries cannot blank their keys independently: the old
// per-entry collision scan overwrote its own verdict on the next kept spec (its
// break left only the innermost loop) and stripped screenLayout out of the
// packed table, making the flag undeclarable (#299). Call it after the entries
// to be dropped have been nil'd, since a nil entry keeps nothing alive.
//
// Keeping every name whose entry survives costs table size: a string resource
// name is shared with each of its translations, so the old rule blanked ~2400
// default-locale string names (and 85 attr/id names, screenLayout among them)
// that belonged to entries still in the table. Restoring them grows the
// embedded android-35 table by about 13%. That is the deliberate trade —
// a name blanked out from under a live entry is #299's whole failure mode.
func stripKeyPool(pkg *Package) {
	kept := make(map[PoolRef]bool)
	for _, spec := range pkg.specs[:4] {
		for _, typ := range spec.types {
			for _, nt := range typ.entries {
				if nt != nil {
					kept[nt.key] = true
				}
			}
		}
	}
	for i := range pkg.keyPool.strings {
		if !kept[PoolRef(i)] {
			pkg.keyPool.strings[i] = ""
		}
	}
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
				typ.indices[j] = NoEntry
				typ.entries[j] = nil
			}
		}
	}

	stripKeyPool(pkg)

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
