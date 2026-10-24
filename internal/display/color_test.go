package display

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/planner"
)

// Unit tests of the colors (BR3.7, NFR11.7).

// standIn makes the writers in terms terminals, with the environment env
// and the virtual terminal answer vt, for the length of the test. It counts
// the calls of the restore function.
func standIn(t *testing.T, terms []io.Writer, env map[string]string, vt bool) *int {
	t.Helper()
	oldIs, oldVT, oldEnv := isTerminal, enableVirtualTerminal, getenv
	t.Cleanup(func() { isTerminal, enableVirtualTerminal, getenv = oldIs, oldVT, oldEnv })
	isTerminal = func(w io.Writer) bool {
		for _, term := range terms {
			if w == term {
				return true
			}
		}
		return false
	}
	restored := 0
	enableVirtualTerminal = func(io.Writer) (bool, func()) {
		if !vt {
			return false, nil
		}
		return true, func() { restored++ }
	}
	getenv = func(name string) string { return env[name] }
	return &restored
}

// NFR11.7: colors only when the stream is a terminal, NO_COLOR is not set or
// empty, TERM is not dumb and (on Windows) virtual terminal processing could
// be turned on; each stream on its own.
func TestColorsAreDecidedForEachStream(t *testing.T) {
	cases := map[string]struct {
		outTerm, errTerm bool
		env              map[string]string
		vt               bool
		wantOut, wantErr bool
	}{
		"both terminals":           {true, true, nil, true, true, true},
		"NO_COLOR set":             {true, true, map[string]string{"NO_COLOR": "1"}, true, false, false},
		"NO_COLOR empty":           {true, true, map[string]string{"NO_COLOR": ""}, true, true, true},
		"TERM dumb":                {true, true, map[string]string{"TERM": "dumb"}, true, false, false},
		"no virtual terminal":      {true, true, nil, false, false, false},
		"standard output only":     {true, false, nil, true, true, false},
		"standard error only":      {false, true, nil, true, false, true},
		"neither is a terminal":    {false, false, nil, true, false, false},
		"NO_COLOR with a terminal": {false, true, map[string]string{"NO_COLOR": "yes", "TERM": "xterm"}, true, false, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			var terms []io.Writer
			if tc.outTerm {
				terms = append(terms, &out)
			}
			if tc.errTerm {
				terms = append(terms, &errOut)
			}
			standIn(t, terms, tc.env, tc.vt)

			p := New(&out, &errOut)
			p.Warn("careful")
			p.PlanResult(samplePlan(), Output{Format: FormatTable})

			if p.colorOut != tc.wantOut || p.colorErr != tc.wantErr {
				t.Fatalf("colors: stdout %v, stderr %v; want %v, %v", p.colorOut, p.colorErr, tc.wantOut, tc.wantErr)
			}
			if got := strings.Contains(out.String(), "\x1b["); got != tc.wantOut {
				t.Errorf("color sequences on standard output = %v:\n%q", got, out.String())
			}
			if got := strings.Contains(errOut.String(), "\x1b[33m"); got != tc.wantErr {
				t.Errorf("color sequences on standard error = %v:\n%q", got, errOut.String())
			}
		})
	}
}

// NFR11.7: taking the color sequences out of the colored table and diff
// gives the output without colors, byte for byte: the meaning never rests on
// colors alone.
func TestColorsCanBeTakenOutWithoutLosingAnything(t *testing.T) {
	plan := goldenPlan(t)
	for _, f := range []Format{FormatTable, FormatDiff} {
		for _, lang := range []Lang{English, Japanese} {
			plain := render(t, lang, func(p *Printer) { p.PlanResult(plan, Output{Format: f}) })

			var out, errOut bytes.Buffer
			standIn(t, []io.Writer{&out}, nil, true)
			p := New(&out, &errOut)
			p.SetLang(lang)
			p.PlanResult(plan, Output{Format: f})
			colored := out.String()

			if !strings.Contains(colored, string(styleAdd)) || !strings.Contains(colored, string(styleImpact)) {
				t.Fatalf("%s %s: no colors on a terminal:\n%q", f, lang, colored)
			}
			if StripColors(colored) != plain {
				t.Errorf("%s %s: colored output without its colors differs:\n%s\nwant:\n%s", f, lang, StripColors(colored), plain)
			}
		}
	}
}

// JSON and Markdown never have colors, even on a terminal (BR3.7).
func TestJSONAndMarkdownHaveNoColors(t *testing.T) {
	for _, f := range []Format{FormatJSON, FormatMarkdown} {
		var out, errOut bytes.Buffer
		standIn(t, []io.Writer{&out, &errOut}, nil, true)
		p := New(&out, &errOut)
		p.PlanResult(goldenPlan(t), Output{Format: f})
		p.ScanResult(goldenScan(), Output{Format: f})
		if strings.Contains(out.String(), "\x1b") {
			t.Fatalf("%s has a color sequence:\n%q", f, out.String())
		}
	}
}

// NFR11.7: the console setting turned on for colors is put back by Close,
// once for each stream that was changed.
func TestCloseRestoresTheConsoleOfBothStreams(t *testing.T) {
	var out, errOut bytes.Buffer
	restored := standIn(t, []io.Writer{&out, &errOut}, nil, true)

	p := New(&out, &errOut)
	p.Close()
	p.Close()

	if *restored != 2 {
		t.Fatalf("restored %d times, want 2 (standard output and standard error)", *restored)
	}
	if p.colorOut || p.colorErr {
		t.Fatal("no colors after Close")
	}
	if paint(true, styleNone, "x") != "x" || paint(true, styleAdd, "") != "" || paint(false, styleAdd, "x") != "x" {
		t.Fatal("paint wraps only text with a style when colors are on")
	}
	if ok, restore := canColor(&bytes.Buffer{}); ok || restore != nil {
		t.Fatal("a buffer is never a terminal")
	}
}

// The diff marks lines with ! and ? and keeps them readable without colors.
func TestDiffMarksImpactedLines(t *testing.T) {
	plan := goldenPlan(t)
	// c-000003 adds CostCenter to the instance; only an undecided finding
	// names it, so its line gets "?". The key Environment is touched by an
	// affected rename and an undecided value change: "!" wins.
	plan.AttachImpact(planner.ImpactResult{Complete: true, Findings: append(plan.ImpactFindings,
		planner.ImpactFinding{Verdict: planner.VerdictUndecided, ChangeIDs: []string{"c-000003"}})})
	got := render(t, English, func(p *Printer) { p.PlanResult(plan, Output{Format: FormatDiff}) })
	for _, want := range []string{"\n! - env         = \"Prod\"\n", "\n! + Environment = \"prod\"\n", "\n? + CostCenter  = \"cc-1\"\n",
		"\n  + Owner       = \"compute-team\"\n", "\n! ~ Env -> Environment = \"dev\"\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("the diff has no %q:\n%s", want, got)
		}
	}
	plan.ImpactFindings = []planner.ImpactFinding{}
	plan.AttachImpact(planner.ImpactResult{Complete: true})
	plain := render(t, English, func(p *Printer) { p.PlanResult(plan, Output{Format: FormatDiff}) })
	if strings.Contains(plain, "\n! -") || strings.Contains(plain, "\n? ") {
		t.Fatalf("without findings no line is marked:\n%s", plain)
	}
}
