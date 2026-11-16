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
	// Glob finds the installed packages of a .venv; nil is filepath.Glob.
	Glob func(string) ([]string, error)
}

// SystemLookup looks at the real computer.
var SystemLookup = ToolLookup{LookPath: exec.LookPath, Getenv: os.Getenv, Stat: os.Stat, Glob: filepath.Glob}

// pythonToolsHow is where the reason of a missing Python test app points.
const pythonToolsHow = "make the .venv and node_modules of the app with the three lines of the section Development of README.md, in "

// RequirePythonTools checks what the synth tests of the Python test app need
// (v0.2.0 U3 NFR8.3, security design §10): the Python of the .venv of the app,
// aws-cdk-lib installed in that .venv, Node.js and the CDK CLI of the app.
// When one is missing, the test fails in CI and is skipped with the reason
// elsewhere, as RequireCDKTools does. It returns true when they are there.
func RequirePythonTools(tb ToolTB, appDir string) bool {
	tb.Helper()
	return RequirePythonToolsWith(tb, appDir, SystemLookup)
}

// RequirePythonToolsWith is RequirePythonTools with the lookups given.
func RequirePythonToolsWith(tb ToolTB, appDir string, look ToolLookup) bool {
	tb.Helper()
	missing := missingPythonTools(appDir, look)
	if len(missing) == 0 {
		return true
	}
	why := "missing for the Python synth tests: " + strings.Join(missing, ", ") + " (" + pythonToolsHow + appDir + ")"
	if look.Getenv("CI") != "" {
		tb.Fatalf("%s", why)
	} else {
		tb.Skipf("%s", why)
	}
	return false
}

// VenvPython returns where the Python of the .venv of an app is.
func VenvPython(appDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(appDir, ".venv", "Scripts", "python.exe")
	}
	return filepath.Join(appDir, ".venv", "bin", "python")
}

func missingPythonTools(appDir string, look ToolLookup) []string {
	glob := look.Glob
	if glob == nil {
		glob = filepath.Glob
	}
	var missing []string
	if info, err := look.Stat(VenvPython(appDir)); err != nil || !info.Mode().IsRegular() {
		missing = append(missing, "the Python of the .venv of the app")
	} else {
		var found []string
		for _, pattern := range []string{
			filepath.Join(appDir, ".venv", "lib", "python*", "site-packages", "aws_cdk_lib-*.dist-info"),
			filepath.Join(appDir, ".venv", "Lib", "site-packages", "aws_cdk_lib-*.dist-info"),
		} {
			m, _ := glob(pattern)
			found = append(found, m...)
		}
		if len(found) == 0 {
			missing = append(missing, "aws-cdk-lib in the .venv of the app")
		}
	}
	return append(missing, missingCDKTools(appDir, look)...)
}

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

// copySkipped are the folders CopyCDKApp does not copy.
var copySkipped = map[string]bool{"node_modules": true, "cdk.out": true, ".venv": true, "venv": true, "__pycache__": true}

// CopyCDKApp copies the app src to the new folder dst, all but node_modules,
// cdk.out, .venv, venv and __pycache__, and links dst/node_modules and
// dst/.venv to those of src: a symbolic link on Unix, a junction on Windows,
// which needs no administrator. The cleanup takes the links away first, and
// only the links, before the folder goes.
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
		if d.IsDir() && copySkipped[d.Name()] {
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
	linkAppFolder(t, src, dst, ".venv")
}

// LinkNodeModules links dst/node_modules to the node_modules of the app src,
// when it has one, and takes the link away at the end of the test.
func LinkNodeModules(t testing.TB, src, dst string) {
	t.Helper()
	linkAppFolder(t, src, dst, "node_modules")
}

// linkAppFolder links dst/name to src/name, when src has it.
func linkAppFolder(t testing.TB, src, dst, name string) {
	t.Helper()
	target := filepath.Join(src, name)
	if _, err := os.Stat(target); err != nil {
		return
	}
	link := filepath.Join(dst, name)
	if err := linkFolder(target, link); err != nil {
		t.Fatalf("link %s: %v", name, err)
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
