package cdk

// The first check of the code generation of v0.2.0 U3 (security design §4,
// NFR6.17): on Windows, cdk is cdk.cmd, which cmd.exe runs. A --app with a
// path that holds a space is given in double quotes; this test says whether
// the quotes reach cdk.cmd as they were given, with the Go that builds the
// tool. When they do not, windowsQuotedApp is false and iac never quotes on
// Windows.

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/procexec"
)

// quotedCmd is a cdk.cmd that writes the arguments it was given, as cmd.exe
// hands them to it, to args.txt next to it.
const quotedCmd = "@echo off\r\n>\"%~dp0args.txt\" echo(%*\r\n"

// With windowsQuotedApp false, the test says the quotes still do not reach
// cdk.cmd as they were given; when they do, it fails so that windowsQuotedApp is
// looked at again. With it true, the quotes must reach cdk.cmd.
func TestQuotedArgReachesCmd(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("cdk.cmd is the cdk of Windows only")
	}
	dir := t.TempDir()
	cmd := filepath.Join(dir, "cdk.cmd")
	if err := os.WriteFile(cmd, []byte(quotedCmd), 0o600); err != nil {
		t.Fatal(err)
	}
	app := `"` + filepath.Join(dir, "a b", ".venv", "Scripts", "python.exe") + `" app.py`
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	res, err := procexec.System{}.Run(ctx, procexec.Command{Name: cmd, Args: []string{"synth", "--app", app}, Dir: dir,
		Timeout: 30 * time.Second})

	if err != nil || res.ExitCode != 0 {
		t.Fatalf("cdk.cmd did not run: %v, exit %d, %s", err, res.ExitCode, res.Stderr)
	}
	got, err := os.ReadFile(filepath.Join(dir, "args.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got2 := strings.TrimRight(string(got), "\r\n")
	reaches := got2 == "synth --app "+app
	switch {
	case windowsQuotedApp && !reaches:
		t.Fatalf("windowsQuotedApp is true but cdk.cmd got %s, want synth --app %s", got2, app)
	case !windowsQuotedApp && reaches:
		t.Fatal("the quoted argument now reaches cdk.cmd as it was given: look at windowsQuotedApp (security design section 4)")
	}
}
