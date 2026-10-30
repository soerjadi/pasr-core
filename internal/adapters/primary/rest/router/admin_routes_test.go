package router

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAdminRoutes_OnlyRegisteredThroughAdminRoutes fails when a /v1/admin path appears anywhere in
// this package's code outside AdminRoutes: such a route would bypass the RequireOperator wrapper
// New applies to every AdminRoutes entry, and the operator-role test table would not see it.
func TestAdminRoutes_OnlyRegisteredThroughAdminRoutes(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	inside := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		var allowed *ast.FuncDecl
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "AdminRoutes" {
				allowed = fn
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING || !strings.Contains(lit.Value, "/v1/admin") {
				return true
			}
			if allowed != nil && lit.Pos() >= allowed.Pos() && lit.End() <= allowed.End() {
				inside++
				return true
			}
			t.Errorf("%s: %s is outside AdminRoutes; register operator routes only there", fset.Position(lit.Pos()), lit.Value)
			return true
		})
	}
	if want := len(AdminRoutes(nil)); inside != want {
		t.Errorf("found %d /v1/admin literals in AdminRoutes, want %d (one per route)", inside, want)
	}
}
