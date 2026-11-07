package cli_test

// Acceptance test for the version display of US6.2 (distribution). The
// release workflow and the README check the version of a published binary
// with --version, so it is part of AC6.2.1 and AC6.2.2. Written before the
// implementation (team testing posture: acceptance criteria first).

import (
	"regexp"
	"testing"
)

// AC6.2.1, AC6.2.2: Given the tool, When it runs with --version, Then it
// prints "tag-refactor <version>" as the only line on standard output, exits
// 0 and calls no AWS API. A test binary has no module version, so the
// version is "devel"; a released binary prints its tag (for example v0.1.0).
func TestAcceptanceUS62_VersionIsPrintedWithoutCallingAWS(t *testing.T) {
	w := newWorld(t)
	for _, args := range [][]string{{"--version"}, {"--lang", "ja", "--version"}} {
		r := w.run(args...)

		r.requireExit(t, 0)
		if !regexp.MustCompile(`^tag-refactor (devel|v[0-9][^\s]*)\n$`).MatchString(r.stdout) {
			t.Fatalf("%v: stdout = %q, want one line \"tag-refactor <version>\"", args, r.stdout)
		}
		requireNotContains(t, "stderr", r.stderr, "Error")
	}
	if calls := w.fake.Calls(); len(calls) != 0 {
		t.Fatalf("--version called AWS: %d calls", len(calls))
	}
}
