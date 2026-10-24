//go:build integration

package cli_test

// Sandbox account test for the U4 checkpoint. Like the U1 and U2 tests it
// calls a real AWS account, so it only runs locally with the build tag
// "integration" and never in CI. scripts/verify.sh runs it after the U3
// check, with the read-only profile:
//
//	TAG_REFACTOR_IT_PROFILE=tag-refactor-readonly \
//	TAG_REFACTOR_SANDBOX_ACCOUNT_ID=<sandbox account ID> \
//	go test -mod=readonly -tags integration -run '^TestIntegrationU4' ./internal/cli/...
//
// It uses the env and Env tags of the U1 fixtures stack in ap-northeast-1
// and test/sandbox/u2-rules.yaml (Environment with the aliases env and Env,
// and the required Owner with the default platform). It has no subtests, so
// verify.sh can count its one PASS line.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/planner"
)

const u4Region = "ap-northeast-1"

// u4Run is one plan against the sandbox account, in English whatever the
// language of the computer.
func u4Run(t *testing.T, profile, rules string, args ...string) result {
	t.Helper()
	base := []string{"plan", "--profile", profile, "--region", u4Region, "--rules", rules, "--lang", "en"}
	return runAgainstAWS(t, append(base, args...)...)
}

// savedCounts reads the counts of a saved plan the way the formats show
// them.
func savedCounts(p *planner.Plan) planCounts {
	s := p.Summary()
	return planCounts{total: s.ResourcesToChange, rename: s.ChangesByKind[planner.KindRenameKey], value: s.ChangesByKind[planner.KindChangeValue],
		add: s.ChangesByKind[planner.KindAddRequired], rem: s.ChangesByKind[planner.KindRemoveKey], conflicts: s.ConflictResources,
		unset: s.UnsetRequiredResources, removeKeys: s.RemoveCandidateKeys, iac: s.IaCManagedResources,
		affected: s.AffectedReferences, undecided: s.UndecidedReferences}
}

func TestIntegrationU4PlanFormatsAndSavedPlan(t *testing.T) {
	profile, account := os.Getenv("TAG_REFACTOR_IT_PROFILE"), os.Getenv("TAG_REFACTOR_SANDBOX_ACCOUNT_ID")
	if profile == "" || account == "" {
		t.Skip("set TAG_REFACTOR_IT_PROFILE and TAG_REFACTOR_SANDBOX_ACCOUNT_ID to run this test (scripts/verify.sh does)")
	}
	t.Setenv("LC_ALL", "C")
	t.Setenv("LANG", "C")
	rules := filepath.Join("..", "..", "test", "sandbox", "u2-rules.yaml")
	dir := t.TempDir()

	var js, table, diff, md result
	var out string
	var counts planCounts
	for attempt := 1; ; attempt++ {
		// JSON and the saved plan come from one collection, so they must
		// agree exactly. The other three formats are separate collections;
		// when the account changed in between, the whole set runs once more,
		// saving to a new name (--out never overwrites).
		out = filepath.Join(dir, "plan-"+string(rune('0'+attempt))+".json")
		js = u4Run(t, profile, rules, "--format", "json", "--out", out)
		table = u4Run(t, profile, rules)
		diff = u4Run(t, profile, rules, "--format", "diff")
		md = u4Run(t, profile, rules, "--format", "markdown")
		for name, r := range map[string]result{"json": js, "table": table, "diff": diff, "markdown": md} {
			if r.code != 2 {
				t.Fatalf("%s: exit code %d, want 2 (changes to review)\nstderr:\n%s", name, r.code, r.stderr)
			}
			for _, line := range []string{"Exit code", "Save this plan", "Saved plan", "Account " + account + " · region", "Done in "} {
				requireNotContains(t, name+" stdout", r.stdout, line)
			}
			requireNotContains(t, name+" stderr", r.stderr, "Error:")
		}
		requireContains(t, "json stderr", js.stderr, "Saved plan to ")
		counts = countsFromJSON(t, js.stdout)
		agree := countsFromTable(t, table.stdout) == counts && countsFromDiff(t, diff.stdout) == counts &&
			countsFromMarkdown(t, md.stdout) == counts
		if agree {
			break
		}
		if attempt == 2 {
			t.Fatalf("the formats disagree twice:\njson %+v\ntable %+v\ndiff %+v\nmarkdown %+v", counts,
				countsFromTable(t, table.stdout), countsFromDiff(t, diff.stdout), countsFromMarkdown(t, md.stdout))
		}
		t.Logf("the formats disagree (the account may have changed between the runs); running them once more")
	}

	saved, err := planner.LoadPlan(out)
	if err != nil {
		t.Fatalf("the saved plan does not read back: %v", err)
	}
	if saved.AccountID != account {
		t.Fatalf("the saved plan is for account %s, not the sandbox account", saved.AccountID)
	}
	if got := savedCounts(saved); got != counts {
		t.Fatalf("the saved plan gives %+v, the JSON output %+v", got, counts)
	}

	// The U1 fixtures: env and Env become Environment, and resources without
	// Owner get the default "platform".
	renames := map[string]bool{}
	ownerAdded := false
	for _, c := range saved.Changes() {
		switch {
		case c.Kind == planner.KindRenameKey && c.ToKey == "Environment":
			renames[c.FromKey] = true
		case c.Kind == planner.KindAddRequired && c.ToKey == "Owner" && c.ToValue == "platform":
			ownerAdded = true
		}
	}
	if !renames["env"] || !renames["Env"] || !ownerAdded {
		t.Fatalf("renames from %v, Owner added %v; want env and Env renamed and Owner=platform added", renames, ownerAdded)
	}
	requireMatch(t, "table", table.stdout, `(?m)^  ~ rename key\s+env -> Environment\s+`)
	requireMatch(t, "table", table.stdout, `(?m)^  \+ add\s+Owner = platform\s+`)
	if !strings.HasPrefix(md.stdout, "## Target\n") {
		t.Fatalf("Markdown must start with the target section:\n%s", md.stdout)
	}
	t.Logf("U4 counts: %+v (saved plan %s)", counts, filepath.Base(out))
}
