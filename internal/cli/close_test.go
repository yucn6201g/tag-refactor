package cli

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/display"
	"github.com/yucn6201g/tag-refactor/internal/testutil"
)

// NFR9.3: every way out of Run closes the display, so the console setting it
// changed is put back: help, an error and a command that ran.
func TestRunClosesTheDisplayOnEveryExit(t *testing.T) {
	env := testutil.IsolateAWSEnv(t)
	env.UseFakeCredentials("us-east-1")
	fake := testutil.NewFakeAWS(t)
	fake.HandleDefaults("us-east-1")
	fake.Handle("tag:GetResources", testutil.TagPages(nil))

	old := closeDisplay
	t.Cleanup(func() { closeDisplay = old })
	var closed []*display.Printer
	closeDisplay = func(p *display.Printer) {
		closed = append(closed, p)
		old(p)
	}

	for name, tc := range map[string]struct {
		args []string
		code int
	}{
		"help":       {[]string{"--help"}, ExitOK},
		"usage":      {[]string{"scan", "--bogus"}, ExitError},
		"scan ran":   {[]string{"scan", "--region", "us-east-1"}, ExitOK},
		"plan error": {[]string{"plan"}, ExitError},
	} {
		closed = nil
		var stdout, stderr bytes.Buffer
		deps := Deps{Transport: fake, RetryDelay: func(int) time.Duration { return 0 }}

		code := Run(context.Background(), tc.args, &stdout, &stderr, deps)

		if code != tc.code {
			t.Fatalf("%s: exit code %d, want %d\n%s", name, code, tc.code, stderr.String())
		}
		if len(closed) != 1 || closed[0] == nil {
			t.Fatalf("%s: the display was closed %d times, want once", name, len(closed))
		}
	}
}
