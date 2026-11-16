package cdk

import (
	"os"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// TestMain plays the fake cdk, node or python of the v0.2.0 U3 tests when the
// test binary was started as one of them (testutil.InstallFakeCDK), and runs
// the tests otherwise.
func TestMain(m *testing.M) {
	if dir, ok := testutil.FakeCDKRequested(); ok {
		os.Exit(testutil.PlayFakeCDK(dir, os.Args[1:], os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}
