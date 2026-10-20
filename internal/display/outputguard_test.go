package display

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The packages that may write to the process output (security design §4).
var outputPackages = []string{"internal/display", "cmd/tag-refactor"}

// forbiddenOutput lists the identifiers that write to the process output
// directly. Everywhere else, output must go through a display.Printer so that
// it is sanitized and lands on the right stream.
var forbiddenOutput = map[string]map[string]bool{
	"os":  {"Stdout": true, "Stderr": true},
	"fmt": {"Print": true, "Printf": true, "Println": true},
	"log": {"Print": true, "Printf": true, "Println": true, "Fatal": true, "Fatalf": true, "Fatalln": true,
		"Panic": true, "Panicf": true, "Panicln": true, "Default": true, "Writer": true},
}

// TestOnlyDisplayAndCmdWriteToTheProcessOutput reads the Go sources of the
// module and fails if a package other than internal/display and
// cmd/tag-refactor uses os.Stdout, os.Stderr, the fmt.Print family, the log
// package's standard logger, or the print and println built-ins.
func TestOnlyDisplayAndCmdWriteToTheProcessOutput(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || d.Name() == "aidlc" || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || allowedToWrite(rel) {
			return nil
		}
		checked++
		for _, finding := range directOutput(t, path) {
			t.Errorf("%s: %s writes to the process output; use internal/display", rel, finding)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 10 {
		t.Fatalf("only %d files checked; the walk did not find the sources", checked)
	}
}

func allowedToWrite(rel string) bool {
	for _, p := range outputPackages {
		if strings.HasPrefix(rel, p+"/") {
			return true
		}
	}
	return false
}

func directOutput(t *testing.T, path string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	imports := map[string]string{} // local name -> import path
	for _, imp := range file.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		name := p[strings.LastIndex(p, "/")+1:]
		if imp.Name != nil {
			name = imp.Name.Name
		}
		imports[name] = p
	}
	var findings []string
	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := v.X.(*ast.Ident); ok {
				if names, ok := forbiddenOutput[imports[id.Name]]; ok && names[v.Sel.Name] {
					findings = append(findings, id.Name+"."+v.Sel.Name)
				}
			}
		case *ast.CallExpr:
			if id, ok := v.Fun.(*ast.Ident); ok && (id.Name == "print" || id.Name == "println") {
				findings = append(findings, id.Name)
			}
		}
		return true
	})
	return findings
}

func writeTestFile(path, src string) error {
	return os.WriteFile(path, []byte(src), 0o600)
}

func TestTheOutputGuardFindsDirectWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.go")
	src := "package bad\nimport (\n\t\"fmt\"\n\tstdos \"os\"\n)\nfunc f() { fmt.Println(1); stdos.Stderr.Write(nil); println(2); fmt.Sprintf(\"ok\") }\n"
	if err := writeTestFile(path, src); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(directOutput(t, path), ",")
	if got != "fmt.Println,stdos.Stderr,println" {
		t.Fatalf("findings = %s", got)
	}
}
