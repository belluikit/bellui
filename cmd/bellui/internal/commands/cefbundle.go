package commands

import (
	"archive/tar"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/belluikit/gocef-libs/manifest"
	"github.com/klauspost/compress/zstd"
)

// CEF payload bundling for the Linux webview backend (#107).
//
// Every other webview backend rides an engine the OS supplies; on Linux the
// engine travels with the application (belluikit/gocef, dlopen'd libcef). So
// packaging a Linux app whose binary links the CEF binding must lay the
// payload out where gocef.Locate finds it — a `cef` directory beside the
// executable — or the shipped app degrades to an inert host at first Attach.
//
// The payload comes from the pinned belluikit/gocef-libs release: the
// manifest package is the pin, the release asset is the bytes, and the
// SHA-256 recorded in the manifest is what makes the download trustworthy.
// An asset whose manifest hash is empty is refused outright — an unverified
// Chromium is the one thing the mirror exists to prevent.

// needsCEFPayload reports whether the executable links the CEF binding, and
// the GOARCH it was built for. Reading the binary's build info answers both
// for cross-compiled artifacts too, and works the same whether the CLI built
// the binary itself or was handed one with --executable.
func needsCEFPayload(exe string) (bool, string) {
	info, err := buildinfo.ReadFile(exe)
	if err != nil {
		return false, "" // not a Go binary we can inspect; nothing to bundle
	}
	arch := ""
	for _, s := range info.Settings {
		if s.Key == "GOARCH" {
			arch = s.Value
		}
	}
	for _, dep := range info.Deps {
		path := dep.Path
		if dep.Replace != nil {
			path = dep.Replace.Path
		}
		if path == "github.com/belluikit/gocef" {
			return true, arch
		}
	}
	return false, arch
}

// bundleCEFPayload fetches, verifies and unpacks the pinned CEF payload into
// binDir/cef — the packaged layout gocef.Locate prefers. It returns the CEF
// version bundled, for the packager's one-line account of what it did.
func bundleCEFPayload(binDir, goarch string) (string, error) {
	platform, err := manifest.PlatformFor("linux", goarch)
	if err != nil {
		return "", err
	}
	asset, err := manifest.For(platform)
	if err != nil {
		return "", err
	}
	if asset.SHA256 == "" {
		return "", fmt.Errorf("the gocef-libs manifest has no checksum for %s: refusing an unverifiable Chromium", asset.Name)
	}

	archive, err := fetchCEFAsset(asset)
	if err != nil {
		return "", err
	}

	dest := filepath.Join(binDir, "cef")
	if err := extractTarZst(archive, dest); err != nil {
		os.RemoveAll(dest) // no half payloads: absent degrades cleanly, partial does not
		return "", fmt.Errorf("unpacking %s: %w", asset.Name, err)
	}
	return manifest.CEFVersion, nil
}

// fetchCEFAsset returns a local path to the verified archive, downloading it
// into the user cache unless a previous run already left a copy that still
// hashes correctly. The hash is checked on every use, not only on download —
// a cache is user-writable, and verification is seconds against a payload
// measured in minutes.
func fetchCEFAsset(asset manifest.Asset) (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(cacheRoot, "bellui", "cef")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	cached := filepath.Join(cacheDir, asset.Name)

	if _, err := os.Stat(cached); err == nil {
		ok, err := fileHashMatches(cached, asset.SHA256)
		if err != nil {
			return "", err
		}
		if ok {
			return cached, nil
		}
		log.Printf("cached %s does not match the manifest checksum — re-downloading", asset.Name)
		if err := os.Remove(cached); err != nil {
			return "", err
		}
	}

	log.Printf("downloading the CEF payload %s (a few hundred MB, cached for later runs) ...", asset.Name)
	tmp, err := downloadCEFAsset(asset, cacheDir)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp)

	ok, err := fileHashMatches(tmp, asset.SHA256)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("%s does not match the manifest's SHA-256 %s — refusing it", asset.Name, asset.SHA256)
	}
	if err := os.Rename(tmp, cached); err != nil {
		return "", err
	}
	return cached, nil
}

// downloadCEFAsset fetches the release asset into a temporary file in dir and
// returns its path. It tries the plain release URL first — right whenever the
// mirror is reachable unauthenticated — and falls back to `gh release
// download`, which owns the authentication story while belluikit/gocef-libs
// is private (GitHub answers 404, not 403, for a private release asset).
func downloadCEFAsset(asset manifest.Asset, dir string) (string, error) {
	url := asset.DownloadURL()
	resp, err := http.Get(url)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			tmp, err := os.CreateTemp(dir, asset.Name+".part-*")
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(tmp, resp.Body); err != nil {
				tmp.Close()
				os.Remove(tmp.Name())
				return "", fmt.Errorf("downloading %s: %w", url, err)
			}
			if err := tmp.Close(); err != nil {
				os.Remove(tmp.Name())
				return "", err
			}
			return tmp.Name(), nil
		}
	}

	gh, lookErr := exec.LookPath("gh")
	if lookErr != nil {
		return "", fmt.Errorf("downloading %s failed and the gh CLI is not installed for an authenticated fetch — install gh (and run gh auth login) or make the asset reachable", url)
	}
	tmpDir, err := os.MkdirTemp(dir, "gh-dl-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)
	cmd := exec.Command(gh, "release", "download", manifest.ReleaseTag(),
		"--repo", "belluikit/gocef-libs", "--pattern", asset.Name, "--dir", tmpDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh release download %s: %s: %w", asset.Name, strings.TrimSpace(string(out)), err)
	}
	// Move out of the auto-removed dir; CreateTemp gives the collision-safe name.
	tmp, err := os.CreateTemp(dir, asset.Name+".part-*")
	if err != nil {
		return "", err
	}
	tmp.Close()
	if err := os.Rename(filepath.Join(tmpDir, asset.Name), tmp.Name()); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

func fileHashMatches(path, wantHex string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return hex.EncodeToString(h.Sum(nil)) == strings.ToLower(wantHex), nil
}

// extractTarZst unpacks a zstd-compressed tarball into dest, refusing any
// entry that would land outside it.
func extractTarZst(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	zr, err := zstd.NewReader(f)
	if err != nil {
		return err
	}
	defer zr.Close()

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		if name == "." {
			continue
		}
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return fmt.Errorf("archive entry escapes the payload directory: %q", hdr.Name)
		}
		target := filepath.Join(dest, name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, hdr.FileInfo().Mode().Perm()); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode().Perm())
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		default:
			// The mirror packs plain files and directories only (tar --sort
			// =name over a directory the workflow populated with cp); links
			// or specials would mean a payload this code has never seen.
			return fmt.Errorf("archive entry %q has unsupported type %d", hdr.Name, hdr.Typeflag)
		}
	}
}
