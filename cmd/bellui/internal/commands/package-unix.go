package commands

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/belluikit/bellui/cmd/bellui/internal/metadata"
	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
)

type unixData struct {
	Name           string
	AppID          string
	Exec           string
	Icon           string
	Local          string
	GenericName    string
	Categories     string
	Comment        string
	Keywords       string
	ExecParams     string
	MimeTypes      string
	StartupWMClass string

	// BundleCEF marks a package carrying the CEF payload under bin/cef, so
	// the Makefile installs it and sets up the sandbox helper (#107).
	BundleCEF bool

	// Credits marks a package carrying share/doc/<AppID>/CREDITS: the
	// attribution the default gopher icon needs (#362).
	Credits bool

	SourceRepo, SourceDir string
}

func (p *Packager) packageUNIX() error {
	var prefixDir string
	local := "local/"
	dirName := filepath.Base(p.exe)

	outDir := p.dir
	if !p.install {
		outDir = util.EnsureSubDir(util.EnsureSubDir(p.dir, "tmp-pkg"), dirName)
	}

	if _, err := os.Stat(filepath.Join("/", "usr", "local")); os.IsNotExist(err) {
		prefixDir = util.EnsureSubDir(outDir, "usr")
		local = ""
	} else {
		prefixDir = util.EnsureSubDir(util.EnsureSubDir(outDir, "usr"), "local")
	}

	shareDir := util.EnsureSubDir(prefixDir, "share")

	binDir := util.EnsureSubDir(prefixDir, "bin")
	binName := filepath.Join(binDir, filepath.Base(p.exe))
	err := util.CopyExeFile(p.exe, binName)
	if err != nil {
		return fmt.Errorf("failed to copy application binary file: %w", err)
	}

	// A binary that links the CEF binding needs the engine beside it — on
	// Linux the application bundles CEF, unlike every other webview backend
	// (#107). Without the payload the shipped app's web views degrade to
	// inert hosts at first Attach.
	bundledCEF := false
	if p.os == "linux" {
		if ok, goarch := needsCEFPayload(p.exe); ok {
			version, err := bundleCEFPayload(binDir, goarch)
			if err != nil {
				return fmt.Errorf("failed to bundle the CEF payload the executable needs: %w", err)
			}
			bundledCEF = true
			log.Printf("bundled CEF %s under bin/cef — the sandbox helper needs setuid root, which `make install` sets up; without it the app runs sandbox-disabled with a notice", version)
		}
	}

	appIDOrName := p.AppID
	if appIDOrName == "" {
		appIDOrName = p.Name
	}

	iconDir := util.EnsureSubDir(shareDir, "pixmaps")
	iconName := appIDOrName + filepath.Ext(p.icon)
	iconPath := filepath.Join(iconDir, iconName)
	err = util.CopyFile(p.icon, iconPath)
	if err != nil {
		return fmt.Errorf("failed to copy icon: %w", err)
	}

	// Build the MimeType= list from the legacy CanOpen block, the file
	// associations and one x-scheme-handler entry per registered URL scheme.
	var mimeList []string
	if p.CanOpen != nil && p.CanOpen.MimeTypes != "" {
		mimeList = append(mimeList, splitDesktopFileList(p.CanOpen.MimeTypes)...)
	}
	mimeList = append(mimeList, p.fileMimeTypes()...)
	for _, scheme := range p.URLSchemes {
		mimeList = append(mimeList, "x-scheme-handler/"+scheme)
	}
	mimes := formatDesktopFileList(dedupeStrings(mimeList))

	openWith := ""
	if len(mimeList) > 0 {
		openWith = " %F"
	}
	if len(p.URLSchemes) > 0 {
		// %U passes URLs (including file:// for opened documents) so a single
		// exec line serves both file associations and scheme handlers.
		openWith = " %U"
	}

	appsDir := util.EnsureSubDir(shareDir, "applications")
	desktop := filepath.Join(appsDir, appIDOrName+".desktop")
	deskFile, err := os.Create(desktop)
	if err != nil {
		return fmt.Errorf("failed to create desktop file: %w", err)
	}

	defer deskFile.Close()

	linuxBSD := metadata.LinuxAndBSD{}
	if p.linuxAndBSDMetadata != nil {
		linuxBSD = *p.linuxAndBSDMetadata
	}
	tplData := unixData{
		Name:           p.Name,
		AppID:          p.AppID,
		Exec:           filepath.Base(p.exe) + openWith,
		Icon:           appIDOrName,
		Local:          local,
		GenericName:    linuxBSD.GenericName,
		Keywords:       formatDesktopFileList(linuxBSD.Keywords),
		Comment:        linuxBSD.Comment,
		Categories:     formatDesktopFileList(linuxBSD.Categories),
		ExecParams:     linuxBSD.ExecParams,
		MimeTypes:      mimes,
		StartupWMClass: p.Name,
		BundleCEF:      bundledCEF,
		Credits:        p.iconCredit() != "",
	}
	if err := writeUnixCredits(shareDir, appIDOrName, p.iconCredit()); err != nil {
		return err
	}

	if p.sourceMetadata != nil {
		tplData.SourceRepo = p.sourceMetadata.Repo
		tplData.SourceDir = p.sourceMetadata.Dir
	}

	err = templates.DesktopFileUNIX.Execute(deskFile, tplData)
	if err != nil {
		return fmt.Errorf("failed to write desktop entry string: %w", err)
	}

	if p.install {
		return nil
	}

	parent := filepath.Dir(outDir)
	defer os.RemoveAll(parent)

	// The Makefile is closed before tar reads it: it was left open, which
	// also kept Windows from removing the staging directory afterwards.
	makefile, err := os.Create(filepath.Join(outDir, "Makefile"))
	if err != nil {
		return fmt.Errorf("failed to create Makefile: %w", err)
	}
	err = templates.MakefileUNIX.Execute(makefile, makefileData(tplData, appIDOrName, filepath.Base(p.exe), iconName))
	if cerr := makefile.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("failed to write Makefile string: %w", err)
	}

	tarCmdArgs := []string{"-Jcf", filepath.Join(p.dir, p.Name+".tar.xz")}
	if p.os == "openbsd" {
		tarCmdArgs = []string{"-zcf", filepath.Join(p.dir, p.Name+".tar.gz")}
	}

	// Compatibility mode for old bellui-cross versions using images with new CLI
	// versions resulting in a tar archive prefix of "./appname/" instead of "./".
	// This is to allow producing a final set of compatible images before
	// switching to a new namespace and merging bellui-cross into bellui-tools as
	// one of the supported commands.
	if belluiCrossCompat {
		tarCmdArgs = append(tarCmdArgs, "-C", outDir, "Makefile", "usr")
	} else {
		tarCmdArgs = append(tarCmdArgs, "-C", parent, dirName)
	}

	var buf bytes.Buffer
	tarCmd := exec.Command("tar", tarCmdArgs...)
	tarCmd.Stderr = &buf
	if err = tarCmd.Run(); err != nil {
		return fmt.Errorf("failed to create archive with tar: %s - %w", buf.String(), err)
	}

	return nil
}

func formatDesktopFileList(items []string) string {
	if len(items) == 0 {
		return ""
	}

	return strings.Join(items, ";") + ";"
}

// splitDesktopFileList splits a semicolon-separated desktop-file list into its
// non-empty entries, tolerating a trailing separator.
func splitDesktopFileList(list string) []string {
	var items []string
	for _, item := range strings.Split(list, ";") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// dedupeStrings returns items with duplicates removed, preserving first-seen order.
func dedupeStrings(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := items[:0:0]
	for _, item := range items {
		if seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

// makefileData adapts the desktop-entry data for the Makefile template, whose
// AppID, Exec and Icon name the files it installs rather than desktop-entry
// values. The .desktop file is written as appIDOrName (an app without an ID is
// named after itself), the binary as exe, without the desktop entry's %F/%U
// field code, and the icon as iconName, which keeps the source icon's
// extension. Any other name makes an install step point at no file.
func makefileData(desktop unixData, appIDOrName, exe, iconName string) unixData {
	desktop.AppID = appIDOrName
	desktop.Exec = exe
	desktop.Icon = iconName
	return desktop
}

// writeUnixCredits writes share/doc/<appIDOrName>/CREDITS when credit is not
// empty: the default icon's attribution, shipped in the archive and installed
// by the Makefile (#362).
func writeUnixCredits(shareDir, appIDOrName, credit string) error {
	if credit == "" {
		return nil
	}
	docDir := util.EnsureSubDir(util.EnsureSubDir(shareDir, "doc"), appIDOrName)
	if err := os.WriteFile(filepath.Join(docDir, "CREDITS"), []byte(credit+"\n"), 0o644); err != nil {
		return fmt.Errorf("failed to write the icon credits: %w", err)
	}
	return nil
}
