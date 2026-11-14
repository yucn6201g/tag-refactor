package cli

import (
	"context"

	"github.com/yucn6201g/tag-refactor/internal/iac"
	"github.com/yucn6201g/tag-refactor/internal/pullrequest"
)

// SetIacForTest replaces the writer of the iac command in deps, for the
// acceptance tests of package cli_test that need a result iac cannot be made
// to give on every system (a write that fails half way, v0.2.0 U1 BR4.2).
func SetIacForTest(deps *Deps, run func(context.Context, iac.Request) (*iac.Result, error)) {
	deps.iac = run
}

// SetPullRequestTimeoutsForTest replaces the limits of the steps of --pr in
// deps, for the acceptance test of a gh that does not answer (v0.2.0 U2
// AC1.1.10).
func SetPullRequestTimeoutsForTest(deps *Deps, t pullrequest.Timeouts) {
	deps.prTimeouts = t
}
