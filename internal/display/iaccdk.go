package display

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/cdk"
)

// The progress of the CDK apps of iac (v0.2.0 U3, BR5.1 to BR5.4, security
// design §9 to §11). On a terminal that is redrawn, the apps are a table with
// one line per app and the elapsed time of the running ones is rewritten every
// second. Otherwise every change of state adds the one line of U1, and a
// running app gets one more line each minute. The clock is the printer's:
// cdk.ProgressSink keeps its five methods (contract C6).

var _ cdk.ToolFailureSink = (*Printer)(nil)

// iacCDKCatalog holds the messages U3 adds for the CDK apps.
var iacCDKCatalog = map[string]message{
	"iac.cdkWhyToolTooOld":     {en: "the program is too old", ja: "プログラムの版が古すぎます"},
	"iac.cdkToolHintNode":      {en: "install Node.js 18 or later", ja: "Node.js 18 以上を入れてください"},
	"iac.cdkToolHintCDK":       {en: "run npm install in the app, or npm install -g aws-cdk", ja: "アプリのフォルダーで npm install を実行するか、npm install -g aws-cdk で入れてください"},
	"iac.cdkToolHintPython":    {en: "install Python 3", ja: "Python 3 を入れてください"},
	"iac.cdkToolHintPythonOld": {en: "install Python %s or later", ja: "Python %s 以上を入れてください"},
	"iac.cdkToolNotFound":      {en: "%s was not found", ja: "%s が見つかりません"},
	"iac.cdkToolMissingFix":    {en: "  Fix: %s was not found; %s", ja: "  直し方: %s が見つかりません。%s"},
	"iac.cdkToolFix":           {en: "  Fix: %s", ja: "  直し方: %s"},
	"iac.cdkRunning":           {en: "%s: running (%s)", ja: "%s: 実行中(%s)"},
	"iac.minutesSeconds":       {en: "%dm%ds", ja: "%d 分 %d 秒"},
	"iac.cdkTableHeader":       {en: "CDK apps · %s", ja: "CDK のアプリ · %s"},
	"iac.cdkTableFooter":       {en: "  %d/%d apps · elapsed %s", ja: "  %d/%d アプリ · 経過 %s"},
	"iac.cdkState.waiting":     {en: "[waiting]", ja: "[待ち]"},
	"iac.cdkState.running":     {en: "[running]", ja: "[実行中]"},
	"iac.cdkState.done":        {en: "[done]", ja: "[済み]"},
	"iac.cdkState.failed":      {en: "[failed]", ja: "[失敗]"},
	"iac.cdkState.skipped":     {en: "[skipped]", ja: "[外した]"},

	"iac.badSynthTimeout":    {en: "--synth-timeout %s is not a duration of 1s or more", ja: "--synth-timeout %s は 1 秒以上の時間ではありません"},
	"iac.badSynthTimeoutFix": {en: "give it like 90s, 10m or 1h30m", ja: "90s、10m、1h30m のように指定してください"},
	"iac.cdkAppOutside":      {en: "--cdk-app %s points outside --dir", ja: "--cdk-app %s は --dir の外を指しています"},
	"iac.cdkAppNotFound":     {en: "--cdk-app %s is not a CDK app found below --dir", ja: "--cdk-app %s は --dir の下で見つかった CDK のアプリではありません"},
	"iac.cdkAppFix": {en: "give the folder of a cdk.json relative to --dir; iac does not look in .git, node_modules, cdk.out, .venv, venv or links",
		ja: "--dir からの相対で cdk.json のあるフォルダーを指定してください。.git、node_modules、cdk.out、.venv、venv、リンクの下は探しません"},
}

func init() {
	for id, m := range iacCDKCatalog {
		catalog[id] = m
	}
}

// stateSkipped is the state of an app that is not synthesized.
const stateSkipped = "skipped"

// appRow is the line of one app in the progress table.
type appRow struct {
	state     string
	language  string
	startedAt time.Time
	// elapsed is the time of a done or failed app.
	elapsed time.Duration
	// reason is the kind of reason of a failed or skipped app, tool the
	// program it failed with (cdk.ToolFailureSink).
	reason, tool string
	// lastLine is when the last line of a running app was added, when the
	// table is not redrawn.
	lastLine time.Time
}

// skippedApp is an app skipped before the table was made.
type skippedApp struct{ dir, reason string }

// Ticker is the clock of the progress; the tests stand in for it.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

type timeTicker struct{ t *time.Ticker }

func (t timeTicker) C() <-chan time.Time { return t.t.C }
func (t timeTicker) Stop()               { t.t.Stop() }

func newTimeTicker(d time.Duration) Ticker { return timeTicker{time.NewTicker(d)} }

// SetTicker sets how the clock of the progress is made (tests). The default
// wraps time.NewTicker.
func (p *Printer) SetTicker(newTicker func(time.Duration) Ticker) {
	p.locked(func() { p.newTicker = newTicker })
}

// appClock is one running clock.
type appClock struct{ stop chan struct{} }

// startClock starts the clock that ticks every second while an app runs. It
// is called with the lock held.
func (p *Printer) startClock() {
	if p.clock != nil {
		return
	}
	c := &appClock{stop: make(chan struct{})}
	p.clock = c
	newTicker := p.newTicker
	if newTicker == nil {
		newTicker = newTimeTicker
	}
	t := newTicker(time.Second)
	p.clockWG.Add(1)
	go func() {
		defer p.clockWG.Done()
		for {
			select {
			case <-t.C():
				p.mu.Lock()
				if p.clock == c {
					p.tick()
				}
				done := p.tickDone
				p.mu.Unlock()
				if done != nil {
					done()
				}
			case <-c.stop:
				t.Stop()
				return
			}
		}
	}()
}

// stopClock stops the clock without waiting for its goroutine, which may be
// waiting for the lock held here; Close waits for it.
func (p *Printer) stopClock() {
	if p.clock != nil {
		close(p.clock.stop)
		p.clock = nil
	}
}

// tick redraws the table, or adds a line for each app running for a minute
// since its last line.
func (p *Printer) tick() {
	pr := p.progress
	if pr == nil || pr.apps == nil {
		return
	}
	if p.redraw {
		p.drawTable(false)
		return
	}
	now := p.now()
	for _, name := range pr.apps {
		row := pr.byApp[name]
		if row.state == stateRunning && now.Sub(row.lastLine) >= time.Minute {
			p.say(styleNone, "iac.cdkRunning", name, p.cdkElapsed(now.Sub(row.startedAt)))
			row.lastLine = now
		}
	}
}

// cdkElapsed writes an elapsed time: in seconds as in U1 below a minute, with
// the minutes above.
func (p *Printer) cdkElapsed(d time.Duration) rawString {
	return rawString(cdkElapsed(p.lang, d))
}

func cdkElapsed(lang Lang, d time.Duration) string {
	s := int(d.Round(time.Second) / time.Second)
	if s < 60 {
		return T(lang, "iac.seconds", s)
	}
	return T(lang, "iac.minutesSeconds", s/60, s%60)
}

// appTable reports whether the progress is the table of the apps.
func (p *Printer) appTable() bool { return p.progress != nil && p.progress.apps != nil }

// appRowOf returns the row of an app in the table, or nil with a line for
// --debug when the table does not have it.
func (p *Printer) appRowOf(event, app string) *appRow {
	row := p.progress.byApp[app]
	if row == nil && p.debug {
		p.sayText(styleNone, "debug: display: progress for an app not in the table: "+event+" "+Sanitize(app))
	}
	return row
}

// AppsWaiting implements cdk.ProgressSink: it makes the table of the apps, with
// the apps skipped before it (BR1.4: iac tells about them first).
func (p *Printer) AppsWaiting(apps []string) {
	if len(apps) == 0 {
		return
	}
	p.locked(func() {
		pending := p.pendingSkipped
		p.pendingSkipped = nil
		p.endProgress()
		pr := &progress{start: p.now(), byApp: map[string]*appRow{}}
		for _, a := range apps {
			pr.byApp[a] = &appRow{state: stateWaiting}
		}
		for _, s := range pending {
			pr.byApp[s.dir] = &appRow{state: stateSkipped, reason: s.reason}
		}
		pr.apps = make([]string, 0, len(pr.byApp))
		for dir := range pr.byApp {
			pr.apps = append(pr.apps, dir)
		}
		slices.Sort(pr.apps)
		p.progress = pr
		if p.redraw {
			p.drawTable(true)
			return
		}
		p.say(styleNone, "iac.cdkWaiting", strings.Join(apps, ", "))
	})
}

// AppStarted implements cdk.ProgressSink. A done app that starts again (the
// synth with the trace) runs again from zero.
func (p *Printer) AppStarted(app, language string) {
	p.locked(func() {
		if !p.appTable() {
			p.say(styleNone, "iac.cdkStarted", app, languageName(language))
			return
		}
		row := p.appRowOf("AppStarted", app)
		if row == nil {
			return
		}
		if row.state != stateRunning {
			p.running++
			if p.running == 1 {
				p.startClock()
			}
		}
		now := p.now()
		*row = appRow{state: stateRunning, language: language, startedAt: now, lastLine: now}
		if p.redraw {
			p.drawTable(false)
			return
		}
		p.say(styleNone, "iac.cdkStarted", app, languageName(language))
	})
}

// appFinished puts an app of the table in a final state. It returns whether
// the one line of the state is to be written, and whether the app is known.
func (p *Printer) appFinished(event, app, state string, elapsed time.Duration, reason, tool string) (line, known bool) {
	if !p.appTable() {
		return true, true
	}
	row := p.appRowOf(event, app)
	if row == nil {
		return false, false
	}
	if row.state == stateRunning {
		p.running--
		if p.running == 0 {
			p.stopClock()
		}
	}
	row.state, row.elapsed, row.reason, row.tool = state, elapsed, reason, tool
	if p.redraw {
		p.drawTable(false)
		return false, true
	}
	return true, true
}

// AppDone implements cdk.ProgressSink.
func (p *Printer) AppDone(app string, elapsed time.Duration) {
	p.locked(func() {
		if line, _ := p.appFinished("AppDone", app, stateDone, elapsed, "", ""); line {
			p.say(styleNone, "iac.cdkDone", app, p.seconds(elapsed))
		}
	})
}

// AppFailed implements cdk.ProgressSink.
func (p *Printer) AppFailed(app string, elapsed time.Duration, reason string) {
	p.appFailed("AppFailed", app, elapsed, reason, "")
}

// AppToolFailed implements cdk.ToolFailureSink: the failure with the program
// it failed with, which gets how to install it (BR2.1, NFR10.5).
func (p *Printer) AppToolFailed(app string, elapsed time.Duration, reason, tool string) {
	p.appFailed("AppToolFailed", app, elapsed, reason, tool)
}

func (p *Printer) appFailed(event, app string, elapsed time.Duration, reason, tool string) {
	p.locked(func() {
		line, known := p.appFinished(event, app, stateFailed, elapsed, reason, tool)
		if !known {
			return
		}
		if line {
			p.say(styleRemove, "iac.cdkFailed", app, p.seconds(elapsed), p.why(reason))
		}
		p.failureFix(reason, tool)
	})
}

// failureFix adds the way out of a failure on the next line: keeping the
// values looked up for needsAWS (BR1.5), and how to install the program for
// toolMissing and toolTooOld. A program other than node, cdk and python is
// only named.
func (p *Printer) failureFix(reason, tool string) {
	switch cdk.FailureReason(reason) {
	case cdk.ReasonNeedsAWS:
		p.say(styleNone, "iac.cdkNeedsAWSFix")
	case cdk.ReasonToolMissing:
		if hint := toolHint(tool); hint != "" {
			p.say(styleNone, "iac.cdkToolMissingFix", tool, rawString(p.tr(hint)))
		} else if tool != "" {
			p.sayText(styleNone, "  "+p.tr("iac.cdkToolNotFound", Sanitize(tool)))
		}
	case cdk.ReasonToolTooOld:
		if tool == "python" {
			p.say(styleNone, "iac.cdkToolFix", rawString(p.tr("iac.cdkToolHintPythonOld", cdk.MinPythonVersion())))
		}
	}
}

// toolHint is the message that says how to install a program, or "".
func toolHint(tool string) string {
	switch tool {
	case "node":
		return "iac.cdkToolHintNode"
	case "cdk":
		return "iac.cdkToolHintCDK"
	case "python":
		return "iac.cdkToolHintPython"
	}
	return ""
}

// AppSkipped implements cdk.ProgressSink. Before the table is made, the app
// waits for it; the line of U1 is written at once unless the table is redrawn.
func (p *Printer) AppSkipped(app, reason string) {
	p.locked(func() {
		if !p.appTable() {
			p.pendingSkipped = append(p.pendingSkipped, skippedApp{dir: app, reason: reason})
			if !p.redraw {
				p.say(styleChange, "iac.cdkSkipped", app, p.why(reason))
			}
			return
		}
		line, _ := p.appFinished("AppSkipped", app, stateSkipped, 0, reason, "")
		if line {
			p.say(styleChange, "iac.cdkSkipped", app, p.why(reason))
		}
	})
}

// endAppProgress stops the clock and writes the skipped apps no table showed
// (security design §9). It is called by endProgress, with the lock held.
func (p *Printer) endAppProgress() {
	p.stopClock()
	p.running = 0
	pending := p.pendingSkipped
	p.pendingSkipped = nil
	if !p.redraw {
		return
	}
	for _, s := range pending {
		p.say(styleChange, "iac.cdkSkipped", s.dir, p.why(s.reason))
	}
}

// appTableLines renders the table of the apps.
func (p *Printer) appTableLines() []string {
	pr := p.progress
	now := p.now()
	lines := []string{p.tr("iac.cdkTableHeader", strconv.Itoa(len(pr.apps)))}
	finished := 0
	for _, name := range pr.apps {
		row := pr.byApp[name]
		var elapsed, why string
		switch row.state {
		case stateRunning:
			elapsed = cdkElapsed(p.lang, now.Sub(row.startedAt))
		case stateDone:
			finished++
			elapsed = cdkElapsed(p.lang, row.elapsed)
		case stateFailed:
			finished++
			elapsed = cdkElapsed(p.lang, row.elapsed)
			why = string(p.why(row.reason))
			if row.tool != "" {
				why += " (" + Sanitize(row.tool) + ")"
			}
		case stateSkipped:
			finished++
			why = string(p.why(row.reason))
		}
		line := "  " + padRight(p.tr("iac.cdkState."+row.state), 10) + " " + padRight(Sanitize(name), 24) + " " +
			padRight(elapsed, 8) + " " + why
		lines = append(lines, strings.TrimRight(line, " "))
	}
	return append(lines, p.tr("iac.cdkTableFooter", finished, len(pr.apps), cdkElapsed(p.lang, now.Sub(pr.start))))
}
