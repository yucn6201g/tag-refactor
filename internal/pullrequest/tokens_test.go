package pullrequest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NFR1.1 (security design §8): the packages that run git, gh, cdk and node do
// not read any variable of the environment by its name, so no token is read:
// the names of the token variables are only compared with (internal/cdk).
func TestNoTokenIsRead(t *testing.T) {
	checked := 0
	for _, dir := range []string{".", filepath.Join("..", "cdk")} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			checked++
			for _, call := range envReads(t, file) {
				t.Errorf("%s reads the environment: %s", file, call)
			}
		}
	}
	if checked < 5 {
		t.Fatalf("only %d files checked", checked)
	}
}

// envReads lists the calls of os.Getenv and os.LookupEnv in a file.
func envReads(t *testing.T, path string) []string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "os" && (sel.Sel.Name == "Getenv" || sel.Sel.Name == "LookupEnv") {
			found = append(found, "os."+sel.Sel.Name)
		}
		return true
	})
	return found
}
