package cli

import (
	"os"
	"testing"
)

// TestMain fixes the language of every test of this package, internal and
// external, to English (U4 BR3.8): without it, a test run on a computer whose
// language settings or Windows display language are Japanese would print
// Japanese and miss the English lines the tests look for. LANG=C is a value
// that is not empty, so the Windows display language is never asked either.
// Tests of the language itself set the variables again with t.Setenv.
func TestMain(m *testing.M) {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES"} {
		_ = os.Unsetenv(name)
	}
	_ = os.Setenv("LANG", "C")
	os.Exit(m.Run())
}
