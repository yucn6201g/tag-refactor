package testutil

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The test CDK apps of internal/cdk/testdata/apps (v0.2.0 U1, BR5.1). The
// tests never touch them: they copy an app to a temporary folder and link its
// node_modules there, so that npm ci runs once per app and not per test.

// CDKTestApp returns the absolute folder of the test CDK app name.
func CDKTestApp(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "cdk", "testdata", "apps", name)
}

// ToolTB is the part of testing.TB the tool check uses, so that a test can see
// what it does without failing itself.
type ToolTB interface {
	Helper()
	Fatalf(format string, args ...any)
	Skipf(format string, args ...any)
}

// ToolLookup is how the tool check looks at the computer.
type ToolLookup struct {
	LookPath func(string) (string, error)
	Getenv   func(string) string
	Stat     func(string) (fs.FileInfo, error)
}

// SystemLookup looks at the real computer.
var SystemLookup = ToolLookup{LookPath: exec.LookPath, Getenv: os.Getenv, Stat: os.Stat}

// RequireCDKTools checks that Node.js, the node_modules of the app and the CDK
// CLI of the app are there (NFR8.1, BR5.1). When one is missing, the test
// fails in CI (CI is not empty) and is skipped with the reason elsewhere. It
// returns true when the tools are there.
func RequireCDKTools(tb ToolTB, appDir string) bool {
	tb.Helper()
	return RequireCDKToolsWith(tb, appDir, SystemLookup)
}

// RequireCDKToolsWith is RequireCDKTools with the lookups given.
func RequireCDKToolsWith(tb ToolTB, appDir string, look ToolLookup) bool {
	tb.Helper()
	missing := missingCDKTools(appDir, look)
	if len(missing) == 0 {
		return true
	}
	why := "missing for the synth tests: " + strings.Join(missing, ", ") +
		" (install Node.js 18 or later and run npm ci --ignore-scripts in " + appDir + ")"
	if look.Getenv("CI") != "" {
		tb.Fatalf("%s", why)
	} else {
		tb.Skipf("%s", why)
	}
	return false
}

func missingCDKTools(appDir string, look ToolLookup) []string {
	var missing []string
	if _, err := look.LookPath("node"); err != nil {
		missing = append(missing, "Node.js (node)")
	}
	if info, err := look.Stat(filepath.Join(appDir, "node_modules")); err != nil || !info.IsDir() {
		missing = append(missing, "the node_modules of the app")
		return missing
	}
	cdk := filepath.Join(appDir, "node_modules", ".bin", "cdk")
	if runtime.GOOS == "windows" {
		cdk += ".cmd"
	}
	if _, err := look.Stat(cdk); err != nil {
		missing = append(missing, "the CDK CLI of the app (node_modules/.bin/cdk)")
	}
	return missing
}

// CopyCDKApp copies the app src to the new folder dst, all but node_modules and
// cdk.out, and links dst/node_modules to src/node_modules: a symbolic link on
// Unix, a junction on Windows, which needs no administrator. The cleanup takes
// the link away first, and only the link, before the folder goes.
func CopyCDKApp(t testing.TB, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == "cdk.out") {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		t.Fatalf("copy the CDK app: %v", err)
	}
	LinkNodeModules(t, src, dst)
}

// LinkNodeModules links dst/node_modules to the node_modules of the app src,
// when it has one, and takes the link away at the end of the test.
func LinkNodeModules(t testing.TB, src, dst string) {
	t.Helper()
	modules := filepath.Join(src, "node_modules")
	if _, err := os.Stat(modules); err != nil {
		return
	}
	link := filepath.Join(dst, "node_modules")
	if err := linkFolder(modules, link); err != nil {
		t.Fatalf("link node_modules: %v", err)
	}
	t.Cleanup(func() { removeLink(t, link) })
}

func linkFolder(target, link string) error {
	if runtime.GOOS != "windows" {
		return os.Symlink(target, link)
	}
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		return &linkError{err: err, out: strings.TrimSpace(string(out))}
	}
	return nil
}

type linkError struct {
	err error
	out string
}

func (e *linkError) Error() string { return e.err.Error() + ": " + e.out }

// removeLink removes the link alone, after checking it is one: removing what it
// points at would delete the node_modules of the test app.
func removeLink(t testing.TB, link string) {
	info, err := os.Lstat(link)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Errorf("look at the link %s: %v", link, err)
		return
	}
	if info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) == 0 && info.IsDir() {
		t.Errorf("%s is a folder, not a link; it is left as it is", link)
		return
	}
	if err := os.Remove(link); err != nil {
		t.Errorf("remove the link %s: %v", link, err)
	}
}
