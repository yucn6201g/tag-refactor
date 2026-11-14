package pullrequest

import (
	"os"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// TestMain plays the fake that writes down its arguments when the test binary
// was started as git or gh by the shell tests of the resume commands
// (testutil.InstallFakeArgs), and runs the tests otherwise.
func TestMain(m *testing.M) {
	if dir, ok := testutil.FakeArgsRequested(); ok {
		os.Exit(testutil.PlayFakeArgs(dir, os.Args[1:]))
	}
	os.Exit(m.Run())
}
