// Package cli implements the tag-refactor command line: the scan and plan
// subcommands, their options, the assembly of the read-only parts, and the
// mapping from error kinds to exit codes and two-line messages (contracts C5
// and C12).
package cli

import (
	"context"
	"io"
	"net/http"
	"time"
)

// Deps holds what the command line takes from its environment. The zero value
// is the production setting; tests replace individual fields.
type Deps struct {
	// Transport, when set, receives every HTTP request the AWS SDK sends,
	// including credential resolution. Tests point it at a fake AWS. Nil uses
	// the SDK's own HTTP transport.
	Transport http.RoundTripper
	// Now returns the current time. Nil means time.Now.
	Now func() time.Time
	// AttemptTimeout bounds one HTTP attempt of an AWS call. Zero means 30s.
	AttemptTimeout time.Duration
	// CallTimeout bounds one AWS API call including its retries. Zero means 120s.
	CallTimeout time.Duration
	// CredentialTimeout bounds resolving the AWS credentials. Zero means 30s.
	CredentialTimeout time.Duration
	// RetryDelay, when set, replaces the SDK's back-off delay between retry
	// attempts. Tests use it to avoid real waiting. Nil keeps the SDK default.
	RetryDelay func(attempt int) time.Duration
}

// Run executes the command line with args (without the program name) and
// returns the process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, deps Deps) int {
	// TODO(step 12): build the command tree. Until then the skeleton has no
	// commands and reports that as an error.
	_ = ctx
	_ = args
	_ = stdout
	_ = deps
	_, _ = io.WriteString(stderr, "Error: tag-refactor has no commands yet\n")
	return 1
}
