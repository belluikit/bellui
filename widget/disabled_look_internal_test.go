package widget

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// fallbackDimmed are the disableable widgets that draw no disabled look of
// their own, so the paint walk dims them (#1223).
var fallbackDimmed = map[string]bool{
	"DisableableWidget": true, // the base an app's widget embeds
	"SwipeArea":         true,
	"WheelPicker":       true,
	"ToolbarAction":     true, // not drawn: its button is, and Button draws its look
}

// Every disableable widget in the package has made the choice core.DisabledLook
// asks for: it draws its own disabled look (disabled_look.go), or it is on the
// fallback list above. A new widget with a Disable method fails here until it
// is added to one, so it is never dimmed twice, or not at all, by accident.
func TestDisabledLook_EveryDisableableWidgetChose(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	disableable := map[string]bool{} // has its own Disable method or embeds DisableableWidget
	ownLook := map[string]bool{}
	embeds := map[string][]string{}
	for _, f := range pkgs["widget"].Files {
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
	var chose func(string) bool
	chose = func(name string) bool {
		if ownLook[name] || fallbackDimmed[name] {
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
	if len(missing) > 0 {
		t.Errorf("disableable widgets that neither draw their own disabled look (DrawsDisabledLook in disabled_look.go) nor are on fallbackDimmed: %v", missing)
	}
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
