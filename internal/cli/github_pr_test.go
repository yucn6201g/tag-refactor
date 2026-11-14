package cli_test

// The integration test of v0.2.0 U1 with the real GitHub (AC1.1.11, BR5.2,
// NFR9.1). It runs only when TAG_REFACTOR_TEST_REPO (owner/name of the
// test-only repository) and TAG_REFACTOR_TEST_REPO_TOKEN (a fine-grained
// token that can write only there) are both not empty; elsewhere, and in CI
// runs without the secret (pull requests from forks, Dependabot), it is
// skipped. It was written before the implementation.
//
// The token is handled as security-design.md §6 says: it is taken out of the
// environment of this process at once, and only the gh and git children and
// tag-refactor itself, built and run as another process, get it as GH_TOKEN.
// npm ci never gets it. It is never printed.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	testRepoEnv      = "TAG_REFACTOR_TEST_REPO"
	testRepoTokenEnv = "TAG_REFACTOR_TEST_REPO_TOKEN"
	// prBranchPrefix is the start of the branches iac --pr makes.
	prBranchPrefix = "tag-refactor/iac-"
)

// tokenNames are the variables that can carry a GitHub token.
var tokenNames = []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN", testRepoTokenEnv}

// withoutTokens returns env without the variables that carry a GitHub token,
// comparing the names as Windows does.
func withoutTokens(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		keep := true
		for _, t := range tokenNames {
			if strings.EqualFold(name, t) {
				keep = false
			}
		}
		if keep {
			out = append(out, kv)
		}
	}
	return out
}

// githubEnvs returns the environments of the integration test: gh and git
// with the token as GH_TOKEN, npm without any token, and tag-refactor with
// GH_TOKEN and without the variable of the secret.
func githubEnvs(base []string, token string) (ghEnv, npmEnv, toolEnv []string) {
	clean := withoutTokens(base)
	ghEnv = append(append([]string(nil), clean...), "GH_TOKEN="+token, "GH_PROMPT_DISABLED=1")
	npmEnv = append([]string(nil), clean...)
	toolEnv = append(append([]string(nil), clean...), "GH_TOKEN="+token)
	return ghEnv, npmEnv, toolEnv
}

// runIn runs a program and returns its standard output. A failure ends the test
// with what the program said; nothing of it holds the token.
func runIn(t *testing.T, env []string, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Env, cmd.Dir = env, dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, stderr.String())
	}
	return string(out)
}

// requireIntegrationTools ends the test when a program is missing: a failure
// in CI, a skip elsewhere (NFR8.1).
func requireIntegrationTools(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			if os.Getenv("CI") != "" {
				t.Fatalf("%s is missing for the GitHub integration test", name)
			}
			t.Skipf("%s is missing for the GitHub integration test", name)
		}
	}
}

// AC1.1.11 (US1.1), NFR9.2 (v0.2.0 U2): Given the test-only repository with
// the test app on main, When tag-refactor iac --pr runs in a clone of it, Then
// a pull request is opened whose difference is the one line of the key, and
// the test closes it and deletes its branch. The subtest default runs without
// the options of the pull request; the subtest options gives --pr-branch,
// --pr-title and --pr-body-file and reads them back from GitHub. Each subtest
// works in a clone of its own; the command is built once.
func TestGitHubPullRequest(t *testing.T) {
	repo, token := os.Getenv(testRepoEnv), os.Getenv(testRepoTokenEnv)
	if repo == "" || token == "" {
		t.Skipf("%s and %s are not both set", testRepoEnv, testRepoTokenEnv)
	}
	if err := os.Unsetenv(testRepoTokenEnv); err != nil {
		t.Fatal(err)
	}
	requireIntegrationTools(t, "gh", "git", "node", "npm", "go")
	ghEnv, npmEnv, toolEnv := githubEnvs(os.Environ(), token)
	cleanStaleBranches(t, ghEnv, repo, time.Now())
	envs := integrationEnvs{gh: ghEnv, npm: npmEnv, tool: toolEnv}
	tool := buildTagRefactor(t, t.TempDir())

	t.Run("default", func(t *testing.T) {
		clone, plan := integrationClone(t, envs, repo)

		url := openIntegrationPullRequest(t, envs, clone, tool, plan)

		requireOneLineDiff(t, envs, clone, url)
	})
	t.Run("options", func(t *testing.T) {
		clone, plan := integrationClone(t, envs, repo)
		branch := optionsBranch(time.Now(), os.Getenv("GITHUB_RUN_ID"))
		title := "tag-refactor integration test with options"
		bodyText := "This body comes from --pr-body-file.\n"
		body := filepath.Join(t.TempDir(), "body.md")
		if err := os.WriteFile(body, []byte(bodyText), 0o600); err != nil {
			t.Fatal(err)
		}

		url := openIntegrationPullRequest(t, envs, clone, tool, plan, "--pr-branch", branch, "--pr-title", title, "--pr-body-file", body)

		var view struct {
			HeadRefName string `json:"headRefName"`
			Title       string `json:"title"`
			Body        string `json:"body"`
		}
		if err := json.Unmarshal([]byte(runIn(t, envs.gh, clone, "gh", "pr", "view", url, "--json", "headRefName,title,body")), &view); err != nil {
			t.Fatal(err)
		}
		if view.HeadRefName != branch || view.Title != title || !strings.HasPrefix(strings.ReplaceAll(view.Body, "\r\n", "\n"), bodyText) {
			t.Fatalf("the pull request has the branch %q, the title %q and the body:\n%s", view.HeadRefName, view.Title, view.Body)
		}
		requireOneLineDiff(t, envs, clone, url)
	})
}

// integrationEnvs are the environments of the programs of the integration
// test (githubEnvs).
type integrationEnvs struct{ gh, npm, tool []string }

// integrationClone clones the test-only repository into a folder of the test,
// sets git up for gh, installs the test app and saves the plan.
func integrationClone(t *testing.T, envs integrationEnvs, repo string) (clone, plan string) {
	t.Helper()
	work := t.TempDir()
	clone = filepath.Join(work, "clone")
	runIn(t, envs.gh, work, "gh", "repo", "clone", repo, clone, "--", "-q")
	for _, kv := range [][2]string{
		{"user.name", "tag-refactor test"}, {"user.email", "tag-refactor-test@example.invalid"},
		{"commit.gpgsign", "false"}, {"credential.https://github.com.helper", ""},
	} {
		runIn(t, envs.gh, clone, "git", "config", "--local", kv[0], kv[1])
	}
	runIn(t, envs.gh, clone, "git", "config", "--local", "--add", "credential.https://github.com.helper", "!gh auth git-credential")
	runIn(t, envs.npm, clone, "npm", "ci", "--ignore-scripts", "--no-audit", "--no-fund")
	return clone, writeIntegrationPlan(t, work)
}

// openIntegrationPullRequest runs iac --pr in the clone and returns the URL it
// printed; the cleanup closes the pull request and deletes its branch.
func openIntegrationPullRequest(t *testing.T, envs integrationEnvs, clone, tool, plan string, args ...string) string {
	t.Helper()
	out := runIn(t, envs.tool, clone, tool, append([]string{"iac", "--lang", "en", "--plan", plan, "--dir", clone, "--pr"}, args...)...)
	url := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "https://github.com/") {
			url = strings.TrimSpace(line)
		}
	}
	if url == "" {
		t.Fatalf("no URL of a pull request on standard output:\n%s", out)
	}
	t.Cleanup(func() {
		cmd := exec.Command("gh", "pr", "close", url, "--delete-branch")
		cmd.Env, cmd.Dir = envs.gh, clone
		if msg, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("the pull request %s and its branch were left: %v\n%s", url, err, msg)
		}
	})
	return url
}

// requireOneLineDiff checks that the difference of the pull request is the one
// line of the key.
func requireOneLineDiff(t *testing.T, envs integrationEnvs, clone, url string) {
	t.Helper()
	diff := runIn(t, envs.gh, clone, "gh", "pr", "diff", url)
	var removed, added []string
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "---"), strings.HasPrefix(line, "+++"):
		case strings.HasPrefix(line, "-"):
			removed = append(removed, line)
		case strings.HasPrefix(line, "+"):
			added = append(added, line)
		}
	}
	if len(removed) != 1 || len(added) != 1 || !strings.Contains(removed[0], "'Env'") || !strings.Contains(added[0], "'Environment'") {
		t.Fatalf("the difference is not the one line of the key:\n%s", diff)
	}
}

// writeIntegrationPlan saves a plan that renames Env to Environment on the
// bucket of the test app; it is made here, without AWS.
func writeIntegrationPlan(t *testing.T, dir string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{"version": 1, "toolVersion": "test", "planId": "p", "accountId": "111122223333",
		"createdAt": "2026-11-08T00:00:00Z", "regions": []string{"ap-northeast-1"}, "inventoryComplete": true,
		"impactAnalysisComplete": true, "ruleFile": "r.yaml", "resources": []map[string]any{skeletonResource()}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// buildTagRefactor builds the command into dir: the test runs it as another
// process, so that only it and its git and gh get the token.
func buildTagRefactor(t *testing.T, dir string) string {
	t.Helper()
	out := filepath.Join(dir, "tag-refactor")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	env := append(withoutTokens(os.Environ()), "CGO_ENABLED=0")
	runIn(t, env, root, "go", "build", "-o", out, "./cmd/tag-refactor")
	return out
}

// cleanStaleBranches closes the pull requests and deletes the branches of
// earlier runs that are more than a day old, which a cancelled CI run can
// leave (infrastructure design). The age is read from the branch name.
func cleanStaleBranches(t *testing.T, env []string, repo string, now time.Time) {
	t.Helper()
	refs := runIn(t, env, "", "gh", "api", "repos/"+repo+"/git/matching-refs/heads/"+prBranchPrefix, "--jq", ".[].ref")
	for _, ref := range strings.Fields(refs) {
		branch := strings.TrimPrefix(ref, "refs/heads/")
		if !staleBranch(branch, now) {
			continue
		}
		numbers := runIn(t, env, "", "gh", "pr", "list", "--repo", repo, "--head", branch, "--state", "open", "--json", "number", "--jq", ".[].number")
		for _, n := range strings.Fields(numbers) {
			runIn(t, env, "", "gh", "pr", "close", n, "--repo", repo)
		}
		runIn(t, env, "", "gh", "api", "-X", "DELETE", "repos/"+repo+"/git/refs/heads/"+branch)
	}
}

// optionsBranch is the branch of the subtest options: the time in UTC, then
// the ID of the run of GitHub Actions when there is one, and a random part, so
// that two runs in the same second do not take the same name (review N6).
// staleBranch reads only the time at its start.
func optionsBranch(now time.Time, runID string) string {
	random := make([]byte, 4)
	if _, err := rand.Read(random); err != nil {
		panic(err)
	}
	name := prBranchPrefix + now.UTC().Format("20060102-150405") + "Z-options-"
	if runID != "" {
		name += runID + "-"
	}
	return name + hex.EncodeToString(random)
}

// staleBranch reports a branch of iac --pr older than a day, by the time in its
// name: the YYYYMMDD-HHMMSS right after the prefix, read in UTC (U2 BR1.2,
// NFR6.13). What follows it, the Z of the default name or the -options of the
// integration test, is not read.
func staleBranch(branch string, now time.Time) bool {
	const layout = "20060102-150405"
	stamp, ok := strings.CutPrefix(branch, prBranchPrefix)
	if !ok || len(stamp) < len(layout) {
		return false
	}
	at, err := time.ParseInLocation(layout, stamp[:len(layout)], time.UTC)
	return err == nil && now.Sub(at) > 24*time.Hour
}
