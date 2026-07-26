package commands

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/belluikit/bellui/cmd/bellui/internal/metadata"
	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
	"github.com/fogleman/gg"
	"github.com/nfnt/resize"

	"github.com/jackmordaunt/icns/v2"
)

type darwinData struct {
	Name, ExeName string
	AppID         string
	Version       string
	Build         int
	Category      string
	Languages     []string

	FileAssociations  []darwinFileAssociation
	URLSchemes        []string
	UsageDescriptions []darwinUsageDescription
}

// darwinUsageDescription is a single Info.plist NS*UsageDescription entry. macOS
// only delivers location/camera/microphone access to a bundle whose Info.plist
// declares the matching usage string, mirroring the iOS path — the keys are the
// same NS* keys, so the darwin bundle reuses the app's IOSUsageDescriptions
// (appviewer#157).
type darwinUsageDescription struct {
	Key   string
	Value string
}

// darwinUsageDescriptions converts the portable usage-description map into
// Info.plist entries, sorted by key for deterministic output.
func darwinUsageDescriptions(m map[string]string) []darwinUsageDescription {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	r := make([]darwinUsageDescription, len(keys))
	for i, k := range keys {
		r[i] = darwinUsageDescription{Key: k, Value: m[k]}
	}
	return r
}

// darwinFileAssociation is a single CFBundleDocumentTypes entry.
type darwinFileAssociation struct {
	Name       string
	Role       string
	Extensions []string
	MimeTypes  []string
	Icon       string
}

// darwinFileAssociations converts the portable metadata into CFBundleDocumentTypes entries.
func darwinFileAssociations(assocs []metadata.FileAssociation) []darwinFileAssociation {
	if len(assocs) == 0 {
		return nil
	}

	r := make([]darwinFileAssociation, 0, len(assocs))
	for _, a := range assocs {
		name := a.Description
		if name == "" && len(a.Extensions) > 0 {
			name = a.Extensions[0]
		}
		role := a.Role
		if role == "" {
			role = "Editor"
		}
		var mimeTypes []string
		if a.MimeType != "" {
			mimeTypes = []string{a.MimeType}
		}
		r = append(r, darwinFileAssociation{
			Name:       name,
			Role:       role,
			Extensions: a.Extensions,
			MimeTypes:  mimeTypes,
			Icon:       a.Icon,
		})
	}
	return r
}

func darwinLangs(langs []string) []string {
	r := make([]string, len(langs))
	for n, lang := range langs {
		r[n] = strings.Replace(lang, "-", "_", 1)
	}
	return r
}

func (p *Packager) packageDarwin() (err error) {
	appDir := util.EnsureSubDir(p.dir, p.Name+".app")
	exeName := filepath.Base(p.exe)

	contentsDir := util.EnsureSubDir(appDir, "Contents")
	info := filepath.Join(contentsDir, "Info.plist")
	infoFile, err := os.Create(info)
	if err != nil {
		return fmt.Errorf("failed to create plist template: %w", err)
	}
	defer func() {
		if r := infoFile.Close(); r != nil && err == nil {
			err = r
		}
	}()

	tplData := darwinData{
		Name: p.Name, ExeName: exeName, AppID: p.AppID, Version: p.AppVersion, Build: p.AppBuild,
		Category: strings.ToLower(p.category), Languages: darwinLangs(p.langs),
		FileAssociations: darwinFileAssociations(p.FileAssociations), URLSchemes: p.URLSchemes,
		UsageDescriptions: darwinUsageDescriptions(p.IOSUsageDescriptions),
	}
	if err := templates.InfoPlistDarwin.Execute(infoFile, tplData); err != nil {
		return fmt.Errorf("failed to write plist template: %w", err)
	}

	macOSDir := util.EnsureSubDir(contentsDir, "MacOS")
	binName := filepath.Join(macOSDir, exeName)
	if err := util.CopyExeFile(p.exe, binName); err != nil {
		return fmt.Errorf("failed to copy executable: %w", err)
	}

	resDir := util.EnsureSubDir(contentsDir, "Resources")
	icnsPath := filepath.Join(resDir, "icon.icns")

	img, err := os.Open(p.icon)
	if err != nil {
		return fmt.Errorf("failed to open source image \"%s\": %w", p.icon, err)
	}
	defer img.Close()
	srcImg, _, err := image.Decode(img)
	if err != nil {
		return fmt.Errorf("failed to decode source image: %w", err)
	}
	dest, err := os.Create(icnsPath)
	if err != nil {
		return fmt.Errorf("failed to open destination file: %w", err)
	}
	defer func() {
		if r := dest.Close(); r != nil && err == nil {
			err = r
		}
	}()
	if !p.rawIcon {
		srcImg = processMacOSIcon(srcImg)
	}
	if err := icns.Encode(dest, srcImg); err != nil {
		return fmt.Errorf("failed to encode icns: %w", err)
	}

	return nil
}

func processMacOSIcon(in image.Image) image.Image {
	size := 1024
	border := 100
	radius := 185.4

	innerSize := int(float64(size) - float64(border*2)) // how many pixels inside border
	sized := resize.Resize(uint(innerSize), uint(innerSize), in, resize.Lanczos3)

	dc := gg.NewContext(size, size)
	dc.DrawRoundedRectangle(float64(border), float64(border), float64(innerSize), float64(innerSize), radius)
	dc.SetColor(color.Black)
	dc.Fill()
	mask := dc.AsMask()

	dc = gg.NewContext(size, size)
	_ = dc.SetMask(mask) // ignore error if size was not equal, as it is
	dc.DrawImage(sized, border, border)

	return dc.Image()
}
