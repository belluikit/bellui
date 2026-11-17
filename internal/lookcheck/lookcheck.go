// Package lookcheck finds the disableable widgets of a package that have not
// made the choice core.DisabledLook asks for (#1223). A package's guard test
// calls it, so that a new widget with a Disable method fails that test until
// it either draws its own disabled look or is listed as dimmed by the paint
// walk.
package lookcheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
)

// Unchosen parses the non-test Go files of package pkg in dir and returns,
// sorted, the types that are disableable (they have a Disable method or embed
// DisableableWidget) but neither have a DrawsDisabledLook method, nor embed a
// type that has one, nor are in fallback.
func Unchosen(dir, pkg string, fallback map[string]bool) ([]string, error) {
	fset := token.NewFileSet()
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	var files []*ast.File
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			return nil, err
		}
		if f.Name.Name == pkg {
			files = append(files, f)
		}
	}
	disableable := map[string]bool{} // has its own Disable method or embeds DisableableWidget
	ownLook := map[string]bool{}
	embeds := map[string][]string{}
	for _, f := range files {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil || len(d.Recv.List) == 0 {
					continue
				}
				recv := receiverName(d.Recv.List[0].Type)
				switch d.Name.Name {
				case "Disable":
					disableable[recv] = true
				case "DrawsDisabledLook":
					ownLook[recv] = true
				}
			case *ast.GenDecl:
				for _, s := range d.Specs {
					ts, ok := s.(*ast.TypeSpec)
					if !ok {
						continue
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok {
						continue
					}
					for _, field := range st.Fields.List {
						if len(field.Names) == 0 {
							embeds[ts.Name.Name] = append(embeds[ts.Name.Name], receiverName(field.Type))
						}
					}
				}
			}
		}
	}
	chose := func(name string) bool {
		if ownLook[name] || fallback[name] {
			return true
		}
		for _, e := range embeds[name] {
			if ownLook[e] {
				return true // inherited, as DateEntry from Entry
			}
		}
		return false
	}
	for name, es := range embeds {
		for _, e := range es {
			if e == "DisableableWidget" {
				disableable[name] = true
			}
		}
	}
	var missing []string
	for name := range disableable {
		if !chose(name) {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing, nil
}

func receiverName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.StarExpr:
		return receiverName(e.X)
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.IndexExpr:
		return receiverName(e.X)
	}
	return ""
}
