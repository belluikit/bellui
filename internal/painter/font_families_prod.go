//go:build !test

package painter

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/belluikit/typesetting/font"
	ot "github.com/belluikit/typesetting/font/opentype"
	"github.com/belluikit/typesetting/fontscan"
)

func scanSystemFontFamilies() []FontFamilyInfo {
	cacheDir := ""
	if runtime.GOOS == "android" {
		parent := os.Getenv("FILESDIR")
		cacheDir = filepath.Join(parent, "fontcache")
	}

	footprints, err := fontscan.SystemFonts(noopLogger{}, cacheDir)
	if err != nil {
		return nil
	}

	type scanEntry struct {
		file           string
		boldFile       string
		italicFile     string
		boldItalicFile string
		hasRegular     bool
		styles         map[string]bool
	}
	families := make(map[string]*scanEntry)

	for _, fp := range footprints {
		name := fp.Family
		if name == "" || strings.HasPrefix(name, ".") {
			continue
		}

		isItalic := fp.Aspect.Style == font.StyleItalic
		isBoldWeight := fp.Aspect.Weight >= font.WeightBold
		isNormalWeight := fp.Aspect.Weight == font.WeightNormal || fp.Aspect.Weight == 0

		fd, ok := families[name]
		if !ok {
			fd = &scanEntry{styles: make(map[string]bool)}
			families[name] = fd
		}

		switch {
		case !isItalic && isNormalWeight:
			fd.file = fp.Location.File
			fd.hasRegular = true
		case !isItalic && isBoldWeight && fd.boldFile == "":
			fd.boldFile = fp.Location.File
		case isItalic && isNormalWeight && fd.italicFile == "":
			fd.italicFile = fp.Location.File
		case isItalic && isBoldWeight && fd.boldItalicFile == "":
			fd.boldItalicFile = fp.Location.File
		}
		// Fallback: if no regular file yet, use whatever we have.
		if fd.file == "" {
			fd.file = fp.Location.File
		}
		fd.styles[aspectStyleName(fp.Aspect)] = true
	}

	// Build normalized→file map for name resolution.
	normToFile := make(map[string]string, len(families))
	for norm, fd := range families {
		normToFile[norm] = fd.file
	}
	friendlyNames := resolveFriendlyNames(normToFile)

	result := make([]FontFamilyInfo, 0, len(families))
	for norm, fd := range families {
		styles := make([]string, 0, len(fd.styles))
		for s := range fd.styles {
			styles = append(styles, s)
		}
		name := friendlyNames[norm]
		if name == "" {
			name = norm
		}
		result = append(result, FontFamilyInfo{
			Name:           name,
			File:           fd.file,
			BoldFile:       fd.boldFile,
			ItalicFile:     fd.italicFile,
			BoldItalicFile: fd.boldItalicFile,
			Styles:         styles,
		})
	}
	return result
}

// resolveFriendlyNames reads font file name tables to map normalized family
// names to their proper display names (e.g. "timesnewroman" → "Times New Roman").
func resolveFriendlyNames(normToFile map[string]string) map[string]string {
	result := make(map[string]string, len(normToFile))

	// Group by file to avoid opening the same file multiple times.
	fileToNorms := make(map[string][]string)
	for norm, file := range normToFile {
		fileToNorms[file] = append(fileToNorms[file], norm)
	}

	for file, norms := range fileToNorms {
		names := readFontFamilyNames(file)
		for _, norm := range norms {
			// Find the name whose normalized form matches.
			for _, name := range names {
				if normalize(name) == norm {
					result[norm] = name
					break
				}
			}
		}
	}
	return result
}

// readFontFamilyNames opens a font file (including collections) and returns
// all unique family names found in it.
func readFontFamilyNames(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	loaders, err := ot.NewLoaders(f)
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var names []string
	for _, ld := range loaders {
		desc, _ := font.Describe(ld, nil)
		if desc.Family != "" && !seen[desc.Family] {
			seen[desc.Family] = true
			names = append(names, desc.Family)
		}
	}
	return names
}

// normalize converts a display name to its fontscan-normalized form for comparison.
func normalize(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, "-", "")
	return name
}

func aspectStyleName(a font.Aspect) string {
	var parts []string
	switch a.Weight {
	case font.WeightThin:
		parts = append(parts, "Thin")
	case font.WeightExtraLight:
		parts = append(parts, "ExtraLight")
	case font.WeightLight:
		parts = append(parts, "Light")
	case font.WeightNormal:
		// omit for "Regular"
	case font.WeightMedium:
		parts = append(parts, "Medium")
	case font.WeightSemibold:
		parts = append(parts, "SemiBold")
	case font.WeightBold:
		parts = append(parts, "Bold")
	case font.WeightExtraBold:
		parts = append(parts, "ExtraBold")
	case font.WeightBlack:
		parts = append(parts, "Black")
	}
	if a.Style == font.StyleItalic {
		parts = append(parts, "Italic")
	}
	if len(parts) == 0 {
		return "Regular"
	}
	return strings.Join(parts, " ")
}
