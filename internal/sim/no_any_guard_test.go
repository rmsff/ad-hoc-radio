package sim

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestProductionCodeHasNoAny enforces the project rule (CONTRIBUTING.md) that forbids the
// any type: no non-test Go file under cmd/, internal/ or firmware/ may use
// the identifier any or an empty interface. Comments and string literals
// are not part of the AST checked here, so they may mention either.
//
// The check is syntactic: it does not see types that are any without
// spelling it, such as the alias json.Token or the inferred type of
// x := recover().
func TestProductionCodeHasNoAny(t *testing.T) {
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	scanned := 0
	for _, dir := range []string{"cmd", "internal", "firmware"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			scanned++
			for _, use := range forbiddenAnyUses(fset, file) {
				t.Errorf("forbidden %s", use)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", dir, err)
		}
	}
	if scanned == 0 {
		t.Fatal("no production Go files scanned")
	}
}

// TestForbiddenAnyUses_DetectsOnlyCode proves the guard is not vacuous: it
// flags both spellings in code and ignores comments, strings and
// non-empty interfaces.
func TestForbiddenAnyUses_DetectsOnlyCode(t *testing.T) {
	cases := []struct {
		source string
		want   int
	}{
		{"var v any", 1},
		{"var v interface{}", 1},
		{"func f(m map[string]interface{}) any { return nil }", 2},
		{"func F[T any]() {}", 1},
		{"var v interface{ }", 1},
		{"type I interface{ M() }", 0},
		{"// any interface{}\nvar s = \"any interface{}\"", 0},
	}
	for _, tc := range cases {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "case.go", "package p\n"+tc.source+"\n", parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.source, err)
		}
		if got := forbiddenAnyUses(fset, file); len(got) != tc.want {
			t.Errorf("forbiddenAnyUses(%q) = %v, want %d uses", tc.source, got, tc.want)
		}
	}
}

// forbiddenAnyUses returns the positions of every any identifier and every
// empty interface type in file.
func forbiddenAnyUses(fset *token.FileSet, file *ast.File) []string {
	var uses []string
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.Ident:
			if n.Name == "any" {
				uses = append(uses, "any at "+fset.Position(n.Pos()).String())
			}
		case *ast.InterfaceType:
			if n.Methods == nil || len(n.Methods.List) == 0 {
				uses = append(uses, "empty interface at "+fset.Position(n.Pos()).String())
			}
		}
		return true
	})
	return uses
}
