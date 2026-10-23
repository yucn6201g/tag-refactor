package display

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
)

// standInTerminal makes term the only writer that counts as a terminal, with
// the given TERM and virtual terminal answer, for the length of the test.
func standInTerminal(t *testing.T, term io.Writer, termEnv string, vt bool) {
	t.Helper()
	oldIs, oldVT, oldEnv := isTerminal, enableVirtualTerminal, getenv
	t.Cleanup(func() { isTerminal, enableVirtualTerminal, getenv = oldIs, oldVT, oldEnv })
	isTerminal = func(w io.Writer) bool { return w == term }
	enableVirtualTerminal = func(io.Writer) (bool, func()) { return vt, nil }
	getenv = func(name string) string {
		if name == "TERM" {
			return termEnv
		}
		return ""
	}
}

// NFR11.5, NFR9.3: redraw only when standard error is a terminal, TERM is not
// dumb and virtual terminal processing could be turned on; standard output
// plays no part.
func TestRedrawIsChosenFromStandardErrorOnly(t *testing.T) {
	cases := map[string]struct {
		termIsErr bool
		termEnv   string
		vt        bool
		want      bool
	}{
		"terminal":                         {true, "xterm-256color", true, true},
		"TERM=dumb":                        {true, "dumb", true, false},
		"virtual terminal not available":   {true, "", false, false},
		"only standard output is terminal": {false, "xterm", true, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			term := io.Writer(&out)
			if tc.termIsErr {
				term = &errOut
			}
			standInTerminal(t, term, tc.termEnv, tc.vt)

			p := New(&out, &errOut)
			p.RegionsWaiting([]string{"us-east-1"})
			p.RegionDone("us-east-1", 1)
			p.Done(time.Second, 1) // draws the final state of the table

			if p.redraw != tc.want {
				t.Fatalf("redraw = %v, want %v", p.redraw, tc.want)
			}
			if got := strings.Contains(errOut.String(), "\x1b["); got != tc.want {
				t.Fatalf("cursor sequences written = %v:\n%q", got, errOut.String())
			}
			if out.Len() != 0 {
				t.Fatalf("progress reached standard output: %q", out.String())
			}
		})
	}
	// Without the stand-in, a buffer is never a terminal.
	if ok, restore := canRedraw(&bytes.Buffer{}); ok || restore != nil {
		t.Fatal("a buffer must not be taken for a terminal")
	}
}

// NFR9.3: the console setting New changed is put back by Close, once, and
// progress is no longer redrawn after it. Without a change, Close does
// nothing.
func TestCloseRestoresTheConsoleSetting(t *testing.T) {
	var out, errOut bytes.Buffer
	standInTerminal(t, &errOut, "xterm", true)
	restored := 0
	enableVirtualTerminal = func(io.Writer) (bool, func()) { return true, func() { restored++ } }

	p := New(&out, &errOut)
	p.RegionsWaiting([]string{"us-east-1"})
	p.Close()
	p.Close()

	if restored != 1 {
		t.Fatalf("console setting restored %d times, want 1", restored)
	}
	if p.redraw || p.progress != nil {
		t.Fatalf("after Close: redraw %v, progress %+v", p.redraw, p.progress)
	}

	enableVirtualTerminal = func(io.Writer) (bool, func()) { return true, nil }
	New(&out, &errOut).Close() // nothing to restore
	if !strings.Contains(errOut.String(), "us-east-1") || out.Len() != 0 {
		t.Fatalf("stdout %q, stderr %q", out.String(), errOut.String())
	}
}

type steppedClock struct{ now time.Time }

func (c *steppedClock) Now() time.Time { return c.now }

var cursorSequence = regexp.MustCompile(`\x1b\[(\d+A|J)`)

// NFR11.5: the redrawn table, captured through the terminal stand-in. Other
// lines on standard error are written in place of the table, which is drawn
// again below them; changes within 0.2 seconds wait for the next redraw.
func TestRedrawRewritesTheTableInPlace(t *testing.T) {
	var out, errOut bytes.Buffer
	standInTerminal(t, &errOut, "xterm", true)
	clock := &steppedClock{now: time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)}
	p := New(&out, &errOut)
	p.SetClock(clock.Now)
	p.SetDebug(true)

	p.Account(awsaccess.AccountContext{AccountID: "123456789012", Region: "us-east-1"})
	p.RegionsWaiting([]string{"eu-west-1", "us-east-1"})
	p.RegionStarted("us-east-1") // within 0.2s of the first draw: not drawn yet
	first := errOut.String()
	clock.now = clock.now.Add(time.Second)
	p.RegionDone("us-east-1", 3)
	p.Warn("x\x1b")
	clock.now = clock.now.Add(65 * time.Second)
	p.RegionFailed("eu-west-1", "AccessDenied (tag:GetResources)\u202e")
	p.DiscoveryStarted([]string{"eu-west-1", "us-east-1"})
	clock.now = clock.now.Add(time.Second)
	p.DiscoveryRegionDone("eu-west-1", "complete")
	p.Done(time.Minute, 3)

	got := errOut.String()
	if !strings.HasPrefix(first, "Account 123456789012 · region us-east-1\nAccount 123456789012 · 2 regions\n"+
		"  [waiting]  eu-west-1\n  [waiting]  us-east-1\n  0/2 regions · elapsed 00:00\n") || strings.Contains(first, "[running]") {
		t.Fatalf("first draw:\n%q", first)
	}
	for _, want := range []string{
		"\x1b[4A\r\x1b[J", // back over the four lines of the table
		"  [done]     us-east-1             3 resources\n",
		"  1/2 regions · elapsed 00:01\n",
		"warning: x\\u{1B}\n", // written in place of the table
		"  [failed]   eu-west-1        AccessDenied (tag:GetResources)\\u{202E}\n",
		"  2/2 regions · elapsed 01:06\n",
		"  discovering never-tagged resources: 0/2 regions · elapsed 01:06\n",
		"  discovering never-tagged resources: 1/2 regions · elapsed 01:07\n",
		"Done in 01:00 · 3 resources\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stderr does not contain %q:\n%q", want, got)
		}
	}
	if strings.Contains(cursorSequence.ReplaceAllString(got, ""), "\x1b") {
		t.Fatalf("an escape character other than the tool's own sequences was written:\n%q", got)
	}
	if strings.Index(got, "warning: x") > strings.LastIndex(got, "1/2 regions") {
		t.Fatal("the table must be drawn again below the warning")
	}
	if out.Len() != 0 {
		t.Fatalf("stdout: %q", out.String())
	}
}

// In append mode an error ends the progress without any cursor sequence, and
// a new collection starts a new list of waiting regions.
func TestAppendModeHasNoCursorSequences(t *testing.T) {
	p, _, errOut := newPrinter()
	p.RegionsWaiting([]string{"us-east-1"})
	p.RegionFailed("us-east-1", "Transient (tag:GetResources)")
	p.Error("stopped", "try again")
	p.RegionsWaiting([]string{"eu-west-1"})

	want := "scan: waiting: us-east-1\nscan: us-east-1 failed (Transient (tag:GetResources), elapsed 00:00)\n" +
		"Error: stopped\n  Fix: try again\nscan: waiting: eu-west-1\n"
	if errOut.String() != want {
		t.Fatalf("stderr:\n%q\nwant:\n%q", errOut.String(), want)
	}
}
