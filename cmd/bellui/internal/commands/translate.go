package commands

import (
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/natefinch/atomic"
	"github.com/urfave/cli/v2"
)

// Translate returns the cli command to scan for new translation strings.
//
// Since: 1.0
func Translate() *cli.Command {
	return &cli.Command{
		Name:      "translate",
		Aliases:   []string{"t"},
		Usage:     "Scans for new translation strings",
		ArgsUsage: "translationsFile [source ...]",
		Description: "Recursively scans the current or given directories/files for\n" +
			"translation strings, and creates or updates the translations file,\n" +
			"a PXF catalog such as translations/en.pxf.",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "imports",
				Aliases: []string{"i"},
				Usage:   "enable scanning of all imports (slow)",
			},
			&cli.BoolFlag{
				Name:    "update",
				Aliases: []string{"u"},
				Usage:   "update existing translations (use with care)",
			},
			boolFlags["verbose"](nil),
		},
		Action: func(ctx *cli.Context) error {
			opts := translateOpts{
				Imports: ctx.Bool("imports"),
				Update:  ctx.Bool("update"),
				Verbose: ctx.Bool("verbose"),
			}

			usage := func() {
				fmt.Fprintf(
					os.Stderr, "usage: %s %s [command options] %s\n",
					ctx.App.Name,
					ctx.Command.Name,
					ctx.Command.ArgsUsage,
				)
				os.Exit(1)
			}

			translationsFile := ctx.Args().First()
			if translationsFile == "" {
				fmt.Fprintln(os.Stderr, "Missing argument: translationsFile")
				usage()
			}

			switch ext := filepath.Ext(translationsFile); {
			case translationsFile == "-", ext == ".pxf":
			case ext == ".json":
				fmt.Fprintln(os.Stderr, "JSON translation catalogs are no longer supported: use a .pxf translationsFile")
				usage()
			default:
				fmt.Fprintln(os.Stderr, "Need .pxf extension for translationsFile")
				usage()
			}

			sources, err := findSources(ctx.Args().Tail(), ".go", ".")
			if err != nil {
				return err
			}

			return updateTranslationsFile(translationsFile, sources, &opts)
		},
	}
}

// Takes the files/directories, recursively scans them for files with the given extension,
// and returns them. Uses the current directory when used with an empty list.
func findSources(files []string, ext, cur string) ([]string, error) {
	if len(files) == 0 {
		return findFilesExt(cur, ext)
	}

	sources := []string{}
	for _, file := range files {
		if filepath.Ext(file) == ext {
			sources = append(sources, file)
			continue
		}
		files, err := findFilesExt(file, ext)
		if err != nil {
			return nil, err
		}
		sources = append(sources, files...)
	}

	return sources, nil
}

// Recursively walk the given directory and return all files with the matching extension
func findFilesExt(dir, ext string) ([]string, error) {
	files := []string{}
	err := filepath.Walk(dir, func(path string, fi fs.FileInfo, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}

		if filepath.Ext(path) != ext || fi.IsDir() || !fi.Mode().IsRegular() {
			return nil
		}

		files = append(files, path)

		return nil
	})
	return files, err
}

type translateOpts struct {
	Imports bool
	Update  bool
	Verbose bool
}

// Create or add to translations file by scanning the given files for translation calls.
// Works with and without existing translations file.
func updateTranslationsFile(file string, files []string, opts *translateOpts) error {
	translations := make(map[string]any)

	data, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(data) > 0 {
		if translations, err = readCatalog(data); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "scanning files: %v\n", files)
	}

	if err := updateTranslationsHash(translations, files, opts); err != nil {
		return err
	}

	if len(translations) == 0 {
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "no translations found")
		}
		return nil
	}

	b := formatCatalog(translations)

	if file == "-" {
		fmt.Print(string(b))
		return nil
	}

	return writeTranslationsFile(b, file)
}

// Write data to given file and rename atomically to prevent file corruption
func writeTranslationsFile(b []byte, file string) error {
	nf, err := os.CreateTemp(filepath.Dir(file), filepath.Base(file)+"-*")
	if err != nil {
		return err
	}

	if len(b) > 0 && b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}

	n, err := nf.Write(b)
	if err != nil {
		return err
	}

	if n < len(b) {
		return io.ErrShortWrite
	}

	if err := nf.Close(); err != nil {
		return err
	}

	return atomic.ReplaceFile(nf.Name(), file)
}

// Update translations hash by scanning the given files, then parsing and walking the AST
func updateTranslationsHash(m map[string]any, srcs []string, opts *translateOpts) error {
	fset := token.NewFileSet()
	specs := []*ast.ImportSpec{}

	for _, src := range srcs {
		af, err := parser.ParseFile(fset, src, nil, parser.AllErrors)
		if err != nil {
			return err
		}

		specs = append(specs, af.Imports...)
	}

	if opts.Imports {
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "loading imports ...\n")
		}

		imp := importer.ForCompiler(fset, "source", nil)
		for _, spec := range specs {
			if err := handleImport(imp, spec, opts); err != nil {
				return err
			}
		}
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "scanning code ...\n")
	}

	var r error
	seen := make(map[string]bool)
	fset.Iterate(func(f *token.File) bool {
		fname := f.Name()
		if seen[fname] {
			return false
		}
		seen[fname] = true

		af, err := parser.ParseFile(fset, fname, nil, parser.AllErrors)
		if err != nil {
			if filepath.Base(fname) == "_cgo_gotypes.go" {
				return true
			}
			r = err
			return false
		}

		ast.Walk(&visitor{opts: opts, m: m}, af)

		return true
	})

	return r
}

// Imports given paths to add to fset
func handleImport(imp types.Importer, spec *ast.ImportSpec, opts *translateOpts) error {
	path, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return err
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "importing: %s\n", path)
	}

	_, err = imp.Import(path)
	if err != nil {
		return err
	}

	return nil
}

// Visitor pattern with state machine to find translations fallback (and key)
type visitor struct {
	opts     *translateOpts
	state    stateFn
	name     string
	key      string
	fallback string
	m        map[string]any
}

// Method to walk AST using interface for ast.Walk
func (v *visitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	if v.state == nil {
		v.state = translateNew
		v.name = ""
		v.key = ""
		v.fallback = ""
	}

	v.state = v.state(v, node)

	return v
}

// State machine to pick out translation key and fallback from AST
type stateFn func(*visitor, ast.Node) stateFn

// All translation calls need to start with the literal "lang" or "binding"
// receiver. Matching the identifier (not the import path) mirrors upstream and
// keeps working against the fork's renamed module path; the distinctive method
// names below filter out any unrelated call on a variable that happens to be
// named "binding".
func translateNew(v *visitor, node ast.Node) stateFn {
	ident, ok := node.(*ast.Ident)
	if !ok {
		return nil
	}

	if ident.Name != "lang" && ident.Name != "binding" {
		return nil
	}

	return translateCall
}

// A known translation method needs to be used. The supported cases are:
//   - simple cases (L, N, lang.BindText, binding.NewLocalizedString/Plural): only
//     the first argument is relevant (the key doubles as the fallback)
//   - more complex cases (X, XN, binding.NewLocalized*WithKey): the first two
//     arguments matter (explicit key + fallback)
//   - grammatical-select cases (LSelect, LSelectPlural): the first argument is a
//     key prefix and the second a selector suffix (see translateSelect)
func translateCall(v *visitor, node ast.Node) stateFn {
	ident, ok := node.(*ast.Ident)
	if !ok {
		return nil
	}

	v.name = ident.Name

	switch ident.Name {
	case "L", "Localize":
		return translateLocalize
	case "N", "LocalizePlural":
		return translateLocalize
	case "X", "LocalizeKey":
		return translateKey
	case "XN", "LocalizePluralKey":
		return translateKey
	// Fork localization seams (pre-rebrand tracker #92/#90/#93). Key-only forms where the key
	// doubles as the fallback, mirroring lang.L/N:
	case "BindText", "NewLocalizedString", "NewLocalizedPlural":
		return translateLocalize
	// Explicit key+fallback forms, mirroring lang.X/XN:
	case "NewLocalizedStringWithKey", "NewLocalizedPluralWithKey":
		return translateKey
	// Grammatical-select seams (pre-rebrand tracker #505): the first arg is the key *prefix*, the
	// second is the selector (usually a runtime variable). We seed the mandatory
	// "<prefix>.other" variant, plus any literal selector variant.
	case "LSelect", "LSelectPlural":
		return translateSelect
	}

	return nil
}

// Parse the LSelect/LSelectPlural key prefix (first arg) and remember it.
func translateSelect(v *visitor, node ast.Node) stateFn {
	basiclit, ok := node.(*ast.BasicLit)
	if !ok {
		return nil
	}

	val, err := strconv.Unquote(basiclit.Value)
	if err != nil {
		return nil
	}

	v.key = val // the key prefix, e.g. "profile_updated"

	return translateSelectSelector
}

// Parse the LSelect/LSelectPlural selector (second arg). The selector is usually
// a runtime variable, so we can only guarantee the mandatory "<prefix>.other"
// fallback key; when it is a string literal we seed that explicit variant too.
// Nothing is ever emitted under the bare prefix, so an existing key is never
// clobbered.
func translateSelectSelector(v *visitor, node ast.Node) stateFn {
	plural := v.name == "LSelectPlural"
	prefix := v.key

	addTranslation(v, prefix+".other", prefix+".other", plural)

	if basiclit, ok := node.(*ast.BasicLit); ok {
		if val, err := strconv.Unquote(basiclit.Value); err == nil && val != "" && val != "other" {
			addTranslation(v, prefix+"."+val, prefix+"."+val, plural)
		}
	}

	return nil
}

// Parse first argument, use string as key and fallback, and finish
func translateLocalize(v *visitor, node ast.Node) stateFn {
	basiclit, ok := node.(*ast.BasicLit)
	if !ok {
		return nil
	}

	val, err := strconv.Unquote(basiclit.Value)
	if err != nil {
		return nil
	}

	v.key = val
	v.fallback = val

	return translateFinish(v)
}

// Parse first argument and use as key
func translateKey(v *visitor, node ast.Node) stateFn {
	basiclit, ok := node.(*ast.BasicLit)
	if !ok {
		return nil
	}

	val, err := strconv.Unquote(basiclit.Value)
	if err != nil {
		return nil
	}

	v.key = val

	return translateKeyFallback
}

// Parse second argument and use as fallback, and finish
func translateKeyFallback(v *visitor, node ast.Node) stateFn {
	var vals []string

	binExpr, binOk := node.(*ast.BinaryExpr)
	for binOk && binExpr.Op == token.ADD {
		if y, ok := binExpr.Y.(*ast.BasicLit); ok {
			vals = append(vals, y.Value)
		}

		switch x := binExpr.X.(type) {
		case *ast.BasicLit:
			vals = append(vals, x.Value)
			binOk = false
		case *ast.BinaryExpr:
			binExpr = x
		default:
			return nil
		}
	}

	if len(vals) > 0 {
		x := ""
		for n := range vals {
			val, err := strconv.Unquote(vals[len(vals)-1-n])
			if err != nil {
				return nil
			}
			x += val
		}
		v.fallback = x

		return translateFinish(v)
	}

	basiclit, ok := node.(*ast.BasicLit)
	if !ok {
		return nil
	}

	val, err := strconv.Unquote(basiclit.Value)
	if err != nil {
		return nil
	}

	v.fallback = val

	return translateFinish(v)
}

// Finish scan for translation and add to translation hash with the right type (singular or plural).
// Only adding new keys, ignoring changed or removed ones.
// Removing is potentially dangerous as there could be dynamic keys that get removed.
// By default ignore existing translations to prevent accidental overwriting.
func translateFinish(v *visitor) stateFn {
	plural := false
	switch v.name {
	case "LocalizePlural", "LocalizePluralKey", "N", "XN",
		"NewLocalizedPlural", "NewLocalizedPluralWithKey":
		plural = true
	}

	addTranslation(v, v.key, v.fallback, plural)

	return nil
}

// addTranslation records a discovered key with its fallback, honouring the
// found/Update/verbose bookkeeping. A plural key is stored as a {other = …}
// message so it matches the plural catalog shape. The fallback is written
// with named placeholders, {Name}, whichever form the source used; the key is
// kept as written, since it is the ID the code looks up. Shared by the
// singular/plural finishers and the select seams.
func addTranslation(v *visitor, key, fallback string, plural bool) {
	fallback = namedPlaceholders(fallback)
	_, found := v.m[key]
	if found {
		if !v.opts.Update {
			if v.opts.Verbose {
				fmt.Fprintf(os.Stderr, "ignoring: %s\n", key)
			}
			return
		}
		if v.opts.Verbose {
			fmt.Fprintf(os.Stderr, "updating: %s\n", key)
		}
	} else {
		if v.opts.Verbose {
			fmt.Fprintf(os.Stderr, "adding: %s\n", key)
		}
	}

	if plural {
		v.m[key] = map[string]string{"other": fallback}
	} else {
		v.m[key] = fallback
	}
}
