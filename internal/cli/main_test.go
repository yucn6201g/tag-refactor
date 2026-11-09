package cli

import (
	"os"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// TestMain fixes the language of every test of this package, internal and
// external, to English (U4 BR3.8): without it, a test run on a computer whose
// language settings or Windows display language are Japanese would print
// Japanese and miss the English lines the tests look for. LANG=C is a value
// that is not empty, so the Windows display language is never asked either.
// Tests of the language itself set the variables again with t.Setenv.
//
// When the test binary was started as the fake gh of the v0.2.0 tests, it
// plays gh and ends instead (testutil.InstallFakeGH).
func TestMain(m *testing.M) {
	if dir, ok := testutil.FakeGHRequested(); ok {
		os.Exit(testutil.PlayFakeGH(dir, os.Args[1:], os.Stdout, os.Stderr))
	}
	for _, name := range []string{"LC_ALL", "LC_MESSAGES"} {
		_ = os.Unsetenv(name)
	}
	_ = os.Setenv("LANG", "C")
	os.Exit(m.Run())
}
