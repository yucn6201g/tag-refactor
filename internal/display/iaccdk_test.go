package display

// Unit tests of the progress of the CDK apps of iac (v0.2.0 U3, security
// design §9 to §11), written after the implementation: the table, the clock,
// the skipped apps, the apps not in the table, the ways to install a program
// and the elapsed times.

import (
	"bytes"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeTicker is a clock the test ticks.
type fakeTicker struct {
	c       chan time.Time
	mu      sync.Mutex
	made    int
	stopped int
}

func (f *fakeTicker) C() <-chan time.Time { return f.c }

func (f *fakeTicker) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped++
}

func (f *fakeTicker) counts() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.made, f.stopped
}

// clockedPrinter is a printer with a stepped clock and a fake ticker; tick
// sends one tick and waits until the printer handled it.
type clockedPrinter struct {
	*Printer
	errOut *bytes.Buffer
	clock  *steppedClock
	ticker *fakeTicker
	ticked chan struct{}
}

func newClockedPrinter(t *testing.T, redraw bool) *clockedPrinter {
	t.Helper()
	var out, errOut bytes.Buffer
	if redraw {
		standInTerminal(t, &errOut, "xterm", true)
	}
	c := &clockedPrinter{errOut: &errOut, clock: &steppedClock{now: time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)},
		ticker: &fakeTicker{c: make(chan time.Time, 1)}, ticked: make(chan struct{}, 1)}
	c.Printer = New(&out, &errOut)
	c.SetClock(c.clock.Now)
	c.SetTicker(func(d time.Duration) Ticker {
		if d != time.Second {
			t.Errorf("the clock ticks every %v", d)
		}
		c.ticker.mu.Lock()
		c.ticker.made++
		c.ticker.mu.Unlock()
		return c.ticker
	})
	c.tickDone = func() { c.ticked <- struct{}{} }
	t.Cleanup(c.Close)
	return c
}

func (c *clockedPrinter) after(d time.Duration) {
	c.locked(func() { c.clock.now = c.clock.now.Add(d) })
}

func (c *clockedPrinter) tick(t *testing.T) {
	t.Helper()
	c.ticker.c <- time.Time{}
	select {
	case <-c.ticked:
	case <-time.After(5 * time.Second):
		t.Fatal("the tick was not handled")
	}
}

func (c *clockedPrinter) text() string {
	var s string
	c.locked(func() { s = StripColors(c.errOut.String()) })
	return s
}

// (a), (d): on a terminal the elapsed time of a running app is rewritten at
// each tick; a done app that starts again runs from zero.
func TestCDKTableRewritesTheElapsedTime(t *testing.T) {
	p := newClockedPrinter(t, true)

	p.AppsWaiting([]string{"infra/b", "infra/a"})
	p.AppStarted("infra/a", "typescript")
	p.after(5 * time.Second)
	p.tick(t)

	got := p.text()
	if !strings.Contains(got, "CDK apps · 2\n") || !regexp.MustCompile(`\[running\]\s+infra/a\s+5s\n\s+\[waiting\]\s+infra/b\n  0/2 apps · elapsed 5s\n`).MatchString(got) {
		t.Fatalf("table:\n%s", got)
	}
	p.AppDone("infra/a", 5*time.Second)
	p.after(70 * time.Second)
	p.AppStarted("infra/a", "typescript")
	p.after(2 * time.Second)
	p.tick(t)
	if !regexp.MustCompile(`\[running\]\s+infra/a\s+2s\n`).MatchString(p.text()[len(got):]) {
		t.Fatalf("table:\n%s", p.text())
	}
	p.AppToolFailed("infra/a", 2*time.Second, "toolMissing", "python")
	p.after(time.Second)
	p.AppSkipped("infra/b", "cdkV1")
	end := p.text()
	for _, want := range []string{"  Fix: python was not found; install Python 3\n", "a program is missing (python)", "[skipped]  infra/b", "2/2 apps · elapsed 1m18s"} {
		if !strings.Contains(end, want) {
			t.Errorf("no %q in:\n%s", want, end)
		}
	}
}

// (b), (c), (e): without a terminal a running app gets one more line each
// minute; the clock stops with the last running app, and Close leaves no
// goroutine.
func TestCDKRunningLineEachMinute(t *testing.T) {
	p := newClockedPrinter(t, false)

	p.AppsWaiting([]string{"infra"})
	p.AppStarted("infra", "python")
	p.after(59 * time.Second)
	p.tick(t)
	if strings.Contains(p.text(), "running") {
		t.Fatalf("a line before a minute:\n%s", p.text())
	}
	p.after(time.Second)
	p.tick(t)
	p.after(59 * time.Second)
	p.tick(t)
	p.after(time.Second)
	p.tick(t)
	if got := p.text(); strings.Count(got, "infra: running (") != 2 || !strings.Contains(got, "infra: running (1m0s)\n") ||
		!strings.Contains(got, "infra: running (2m0s)\n") {
		t.Fatalf("lines:\n%s", got)
	}
	p.AppDone("infra", 2*time.Minute)
	p.Close()
	if made, stopped := p.ticker.counts(); made != 1 || stopped != 1 {
		t.Fatalf("clock made %d, stopped %d", made, stopped)
	}
	if !strings.HasSuffix(p.text(), "infra: done (2m0s)\n") && !strings.HasSuffix(p.text(), "infra: done (120s)\n") {
		t.Fatalf("lines:\n%s", p.text())
	}
}

// (f): the apps skipped before the table are in it, in name order, and count as
// finished; without a terminal their line of U1 comes at once.
func TestCDKSkippedAppsBeforeTheTable(t *testing.T) {
	t.Run("redrawn", func(t *testing.T) {
		p := newClockedPrinter(t, true)
		p.AppSkipped("z-v1", "cdkV1")
		p.AppSkipped("java", "unsupportedLanguage")
		if strings.Contains(p.text(), "skipped") {
			t.Fatalf("drawn before the table:\n%s", p.text())
		}

		p.AppsWaiting([]string{"m"})

		if !regexp.MustCompile(`\[skipped\]\s+java\s+neither TypeScript nor Python\n\s+\[waiting\]\s+m\n\s+\[skipped\]\s+z-v1\s+CDK v1\n  2/3 apps`).MatchString(p.text()) {
			t.Fatalf("table:\n%s", p.text())
		}
	})
	t.Run("one line each", func(t *testing.T) {
		p := newClockedPrinter(t, false)
		p.AppSkipped("java", "unsupportedLanguage")
		p.AppsWaiting([]string{"m"})
		if p.text() != "java: skipped: neither TypeScript nor Python\nCDK apps to synthesize: m (waiting)\n" {
			t.Fatalf("lines:\n%s", p.text())
		}
	})
	t.Run("(h) no table came", func(t *testing.T) {
		p := newClockedPrinter(t, true)
		p.AppSkipped("java", "unsupportedLanguage")
		p.Close()
		if !strings.Contains(p.text(), "java: skipped: neither TypeScript nor Python\n") {
			t.Fatalf("lines:\n%s", p.text())
		}
	})
}

// (g): a notice of an app not in the table changes nothing; --debug tells it.
func TestCDKAppNotInTheTable(t *testing.T) {
	for _, debug := range []bool{false, true} {
		p := newClockedPrinter(t, false)
		p.SetDebug(debug)
		p.AppsWaiting([]string{"infra"})
		before := p.text()

		p.AppStarted("zz", "typescript")
		p.AppDone("zz", time.Second)
		p.AppToolFailed("zz", time.Second, "toolMissing", "node")
		p.AppSkipped("zz", "cdkV1")

		added := p.text()[len(before):]
		if made, _ := p.ticker.counts(); made != 0 {
			t.Fatalf("the clock started")
		}
		if !debug && added != "" || debug && strings.Count(added, "debug: display: progress for an app not in the table: ") != 4 {
			t.Fatalf("debug %v:\n%s", debug, added)
		}
		if debug && !strings.Contains(added, "AppStarted zz\n") {
			t.Fatalf("lines:\n%s", added)
		}
	}
}

// BR2.1, NFR10.5: the way to install node, cdk and python, in both languages;
// another program is only named.
func TestCDKToolFixes(t *testing.T) {
	for _, tc := range []struct {
		lang         Lang
		reason, tool string
		want         string
	}{
		{English, "toolMissing", "python", "  Fix: python was not found; install Python 3\n"},
		{English, "toolMissing", "node", "  Fix: node was not found; install Node.js 18 or later\n"},
		{English, "toolMissing", "cdk", "  Fix: cdk was not found; run npm install in the app, or npm install -g aws-cdk\n"},
		{English, "toolMissing", "pipenv", "  pipenv was not found\n"},
		{English, "toolMissing", "pip\x1benv", "  pip\\u{1B}env was not found\n"},
		{English, "toolTooOld", "python", "  Fix: install Python 3.10 or later\n"},
		{Japanese, "toolMissing", "python", "  直し方: python が見つかりません。Python 3 を入れてください\n"},
		{Japanese, "toolTooOld", "python", "  直し方: Python 3.10 以上を入れてください\n"},
		{English, "needsAWS", "cdk", "  Fix: run cdk synth once where AWS credentials are set"},
	} {
		p, _, errOut := newPrinter()
		p.SetLang(tc.lang)
		p.AppToolFailed("infra", time.Second, tc.reason, tc.tool)
		if !strings.Contains(errOut.String(), tc.want) {
			t.Errorf("%s %s: no %q in:\n%s", tc.reason, tc.tool, tc.want, errOut.String())
		}
	}
	p, _, errOut := newPrinter()
	p.AppToolFailed("infra", time.Second, "toolTooOld", "pipenv")
	p.AppToolFailed("infra", time.Second, "synthError", "cdk")
	if strings.Contains(errOut.String(), "Fix") || !strings.Contains(errOut.String(), "infra: failed (1s): the program is too old\n") {
		t.Fatalf("lines:\n%s", errOut.String())
	}
}

func TestCDKElapsed(t *testing.T) {
	for _, tc := range []struct {
		lang Lang
		d    time.Duration
		want string
	}{
		{English, 0, "0s"},
		{English, 42*time.Second + 300*time.Millisecond, "42s"},
		{English, 59*time.Second + 600*time.Millisecond, "1m0s"},
		{English, 72 * time.Second, "1m12s"},
		{English, 2*time.Hour + 5*time.Second, "120m5s"},
		{Japanese, 42 * time.Second, "42 秒"},
		{Japanese, 72 * time.Second, "1 分 12 秒"},
	} {
		if got := cdkElapsed(tc.lang, tc.d); got != tc.want {
			t.Errorf("cdkElapsed(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

// NFR10.5: every message of U3 is in both languages.
func TestIacCDKCatalogHasBothLanguages(t *testing.T) {
	for id, m := range iacCDKCatalog {
		if m.en == "" || m.ja == "" || strings.Count(m.en, "%") != strings.Count(m.ja, "%") {
			t.Errorf("%s: %q / %q", id, m.en, m.ja)
		}
	}
	for _, state := range []string{"waiting", "running", "done", "failed", "skipped"} {
		if _, ok := catalog["iac.cdkState."+state]; !ok {
			t.Errorf("no word for %s", state)
		}
	}
}

// Under -race (the race job): the clock, the notices, errors and debug lines
// run at once on a redrawn table, and Close ends them.
func TestCDKClockRacesNothing(t *testing.T) {
	var out, errOut bytes.Buffer
	standInTerminal(t, &errOut, "xterm", true)
	p := New(&out, &errOut)
	p.SetDebug(true)
	ticker := &fakeTicker{c: make(chan time.Time)}
	p.SetTicker(func(time.Duration) Ticker { return ticker })
	p.AppsWaiting([]string{"a", "b"})
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for {
			select {
			case ticker.c <- time.Now():
			case <-stop:
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := range 50 {
			app := []string{"a", "b"}[i%2]
			p.AppStarted(app, "typescript")
			p.AppDone(app, time.Second)
		}
	}()
	go func() {
		defer wg.Done()
		for range 50 {
			p.DebugLine("line")
			p.Error("what", "fix")
		}
	}()
	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
	p.Close()
}
