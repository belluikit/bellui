package commands

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

// The targets package names in its --target usage are the cases of
// doPackage's switch, read from the source, so the help can no longer leave
// out a target the command takes, as it did web and wasm (#1369).
func TestPackageTargets_MatchDoPackage(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "package.go", nil, 0)
	require.NoError(t, err)

	var cases []string
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "doPackage" {
			return true
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sw, ok := n.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			if sel, ok := sw.Tag.(*ast.SelectorExpr); !ok || sel.Sel.Name != "os" {
				return true
			}
			for _, stmt := range sw.Body.List {
				for _, e := range stmt.(*ast.CaseClause).List {
					lit, ok := e.(*ast.BasicLit)
					require.True(t, ok, "a case of doPackage's switch is not a string literal")
					v, err := strconv.Unquote(lit.Value)
					require.NoError(t, err)
					cases = append(cases, v)
				}
			}
			return false
		})
		return false
	})
	require.NotEmpty(t, cases, "doPackage's switch on p.os was not found")

	slices.Sort(cases)
	want := slices.Clone(packageTargets)
	slices.Sort(want)
	assert.Equal(t, cases, want)
	assert.Contains(t, cases, "web")
	assert.Contains(t, cases, "wasm")
}

// Each command's --target usage names exactly that command's targets.
func TestTargetFlag_Usage(t *testing.T) {
	for _, c := range []struct {
		cmd     *cli.Command
		targets []string
	}{
		{Package(), packageTargets},
		{Release(), packageTargets},
		{Build(), buildTargets},
		{Install(), installTargets},
	} {
		t.Run(c.cmd.Name, func(t *testing.T) {
			var usage string
			for _, f := range c.cmd.Flags {
				if sf, ok := f.(*cli.StringFlag); ok && sf.Name == "target" {
					usage = sf.Usage
				}
			}
			require.NotEmpty(t, usage, "no --target flag")
			inner := strings.TrimSuffix(strings.TrimPrefix(usage, "set operating system to target ("), ")")
			assert.Equal(t, c.targets, strings.Split(inner, ", "))
		})
	}
}
