package cdk

// Unit tests of the home folder in the lines of --debug (v0.2.0 U3 NFR1.12,
// security design §5), written after the implementation.

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestHomeShortHomeKeepsTheBoundaries(t *testing.T) {
	unix := []string{"/home/alice"}
	win := []string{`C:\Users\alice`}
	for _, tc := range []struct {
		name    string
		s       string
		homes   []string
		windows bool
		want    string
	}{
		{"a path below home", "at (/home/alice/work/a.ts:3:7)", unix, false, "at (~/work/a.ts:3:7)"},
		{"home alone", "/home/alice", unix, false, "~"},
		{"twice in a line", "/home/alice/a /home/alice/b", unix, false, "~/a ~/b"},
		{"a longer name", "/home/alice2/a", unix, false, "/home/alice2/a"},
		{"a name with a dot", "/home/alice.old/a", unix, false, "/home/alice.old/a"},
		{"a name with _ and -", "/home/alice_x /home/alice-y", unix, false, "/home/alice_x /home/alice-y"},
		{"Windows without case", `c:\users\ALICE\work\a.ts:3:7`, win, true, `~\work\a.ts:3:7`},
		{"Unix with case", "/HOME/alice/a", unix, false, "/HOME/alice/a"},
		{"no home", "/home/alice/a", nil, false, "/home/alice/a"},
		{"an empty home", "/a", []string{""}, false, "/a"},
		{"not ASCII after home", "/home/aliceé", unix, false, "/home/aliceé"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := shortHome(tc.s, tc.homes, tc.windows); got != tc.want {
				t.Fatalf("shortHome(%q) = %q, want %q", tc.s, got, tc.want)
			}
		})
	}
}

// The longest form is used first: the real home of a linked home is longer.
func TestHomeFormsAreLongestFirst(t *testing.T) {
	eval := func(string) (string, error) { return filepath.Join("/private", "home", "alice"), nil }
	forms := homeForms(filepath.Join("/home", "alice"), eval)
	if len(forms) != 2 || forms[0] != filepath.Join("/private", "home", "alice") {
		t.Fatalf("forms %q", forms)
	}
	if got := homeForms("/home/alice/", func(s string) (string, error) { return s, nil }); !slices.Equal(got, []string{filepath.Clean("/home/alice")}) {
		t.Fatalf("forms %q", got)
	}
	if got := homeForms("/none", func(string) (string, error) { return "", errors.New("missing") }); len(got) != 1 {
		t.Fatalf("forms %q", got)
	}
}

// A home that is a link is found in both forms (Unix: macOS has /var under
// /private).
func TestHomeOfARealLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need a privilege on Windows")
	}
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "home")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	homes := Config{Home: func() (string, error) { return link, nil }}.homes()
	resolved, _ := filepath.EvalSymlinks(real)
	for _, p := range []string{filepath.Join(link, "a.ts"), filepath.Join(resolved, "a.ts")} {
		if got := shortHome(p, homes, false); got != filepath.Join("~", "a.ts") {
			t.Errorf("shortHome(%q) = %q", p, got)
		}
	}
	if got := (Config{Home: func() (string, error) { return "", errors.New("no home") }}).homes(); got != nil {
		t.Fatalf("homes %q", got)
	}
}

// The kept lines of standard error write home as ~ before they are redacted.
func TestHomeInTheKeptLines(t *testing.T) {
	home := filepath.Join(string(filepath.Separator)+"home", "alice")
	if runtime.GOOS == "windows" {
		home = `C:\Users\alice`
	}
	stderr := []byte("Error: boom\n    at Object.<anonymous> (" + filepath.Join(home, "work", "a.ts") + ":3:7)\n")
	var debug []string

	keptLines(stderr, &debug, "cdk: x: ", []string{home})

	want := "cdk: x:     at Object.<anonymous> (" + filepath.Join("~", "work", "a.ts") + ":3:7)"
	if !slices.Contains(debug, want) {
		t.Fatalf("debug %q, want %q", debug, want)
	}
}
