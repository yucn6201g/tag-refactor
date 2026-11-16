package cli_test

// Unit tests of the setup of the GitHub integration test (v0.2.0 U1, NFR2.4,
// security design §6), written after the implementation: where the token can
// be in the workflow, and which programs of the test get it.

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

// The token of the test-only repository is named in the env of one step of the
// job github-pr, the step that runs the integration test alone; nowhere else
// in the workflows.
func TestTheTokenReachesOneStepOnly(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", ".github", "workflows", "*.yml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no workflows: %v", err)
	}
	type step struct {
		Run string            `yaml:"run"`
		Env map[string]string `yaml:"env"`
	}
	type job struct {
		Env   map[string]string `yaml:"env"`
		Steps []step            `yaml:"steps"`
	}
	var places []string
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var wf struct {
			Env  map[string]string `yaml:"env"`
			Jobs map[string]job    `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(data, &wf); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		mentions := strings.Count(string(data), "TAG_REFACTOR_TEST_REPO_TOKEN")
		found := 0
		if _, ok := wf.Env["TAG_REFACTOR_TEST_REPO_TOKEN"]; ok {
			t.Errorf("%s gives the token to every job", file)
		}
		for name, j := range wf.Jobs {
			if _, ok := j.Env["TAG_REFACTOR_TEST_REPO_TOKEN"]; ok {
				t.Errorf("%s gives the token to every step of %s", file, name)
			}
			for _, s := range j.Steps {
				if v, ok := s.Env["TAG_REFACTOR_TEST_REPO_TOKEN"]; ok {
					found++
					places = append(places, name)
					if v != "${{ secrets.TAG_REFACTOR_TEST_REPO_TOKEN }}" {
						t.Errorf("the token of %s is %q, not the secret", name, v)
					}
					if !strings.Contains(s.Run, "-run '^TestGitHubPullRequest$' ./internal/cli/") || strings.Contains(s.Run, "\n") {
						t.Errorf("the step of %s with the token runs %q", name, s.Run)
					}
				}
			}
		}
		// A comment of the workflow may name the token; any other mention is a
		// place it could leak to.
		comments := 0
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				comments += strings.Count(line, "TAG_REFACTOR_TEST_REPO_TOKEN")
			}
		}
		if mentions != comments+2*found {
			t.Errorf("%s names the token %d times, %d of them in step env", file, mentions, found)
		}
	}
	if !slices.Equal(places, []string{"github-pr"}) {
		t.Fatalf("the token is in the steps of %v, want github-pr only", places)
	}
}

// NFR8.1, BR5.1: the synth tests fail in CI without Node.js and the test CDK
// app, so every job of every workflow that runs go test installs Node.js and
// runs npm ci first (ci.yml and release.yml alike).
func TestEveryJobThatRunsGoTestInstallsNode(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", ".github", "workflows", "*.yml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no workflows: %v", err)
	}
	type step struct {
		Uses string `yaml:"uses"`
		Run  string `yaml:"run"`
	}
	goTests := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var wf struct {
			Jobs map[string]struct {
				Steps []step `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(data, &wf); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for name, j := range wf.Jobs {
			node, npm := false, false
			for _, s := range j.Steps {
				node = node || strings.HasPrefix(s.Uses, "actions/setup-node@")
				npm = npm || strings.Contains(s.Run, "npm ci") && strings.Contains(s.Run, "--ignore-scripts")
				if strings.Contains(s.Run, "go test") {
					goTests++
					if !node || !npm {
						t.Errorf("%s: the job %s runs go test before Node.js (%v) and npm ci (%v)", filepath.Base(file), name, node, npm)
					}
				}
			}
		}
	}
	if goTests < 5 {
		t.Fatalf("only %d steps run go test; the workflows were not read", goTests)
	}
}

// workflowFile is what the tests of U3 read of a workflow.
type workflowFile struct {
	Env  map[string]string `yaml:"env"`
	Jobs map[string]struct {
		Env   map[string]string `yaml:"env"`
		Steps []struct {
			Uses             string            `yaml:"uses"`
			Run              string            `yaml:"run"`
			WorkingDirectory string            `yaml:"working-directory"`
			Env              map[string]string `yaml:"env"`
			With             map[string]string `yaml:"with"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func readWorkflows(t *testing.T) map[string]workflowFile {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", ".github", "workflows", "*.yml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no workflows: %v", err)
	}
	all := map[string]workflowFile{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var wf workflowFile
		if err := yaml.Unmarshal(data, &wf); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		all[filepath.Base(file)] = wf
	}
	return all
}

// v0.2.0 U3 NFR8.3, NFR6.19 (security design §10): every job that runs all the
// Go tests installs Python, of the version and the action of the lint job,
// and the .venv of py-minimal with wheels only and pip check, and npm ci of
// both test apps without scripts, before go test. A job that runs one test
// with -run (github-pr) needs none of it.
func TestEveryJobThatRunsAllGoTestsInstallsPython(t *testing.T) {
	workflows := readWorkflows(t)
	ci := workflows["ci.yml"]
	lintPython := ""
	for _, s := range ci.Jobs["lint"].Steps {
		if strings.HasPrefix(s.Uses, "actions/setup-python@") {
			lintPython = s.Uses
			if s.With["python-version"] != ci.Env["PYTHON_VERSION"] {
				t.Errorf("the lint job uses Python %q, the tests %q", s.With["python-version"], ci.Env["PYTHON_VERSION"])
			}
		}
	}
	if lintPython == "" {
		t.Fatal("the lint job sets up no Python")
	}
	jobs := 0
	for file, wf := range workflows {
		for name, j := range wf.Jobs {
			python, venv, lock := false, false, false
			for _, s := range j.Steps {
				if strings.HasPrefix(s.Uses, "actions/setup-python@") && s.With["python-version"] == "${{ env.PYTHON_VERSION }}" {
					python = true
					if s.Uses != lintPython {
						t.Errorf("%s: %s uses %s, the lint job %s", file, name, s.Uses, lintPython)
					}
				}
				if strings.HasPrefix(s.Uses, "actions/setup-node@") {
					lock = strings.Contains(s.With["cache-dependency-path"], "${{ env.CDK_TEST_APP }}/package-lock.json") &&
						strings.Contains(s.With["cache-dependency-path"], "${{ env.PY_TEST_APP }}/package-lock.json")
				}
				if s.WorkingDirectory == "${{ env.PY_TEST_APP }}" && strings.Contains(s.Run, "--only-binary=:all:") &&
					strings.Contains(s.Run, "pip check") && strings.Contains(s.Run, "npm ci --ignore-scripts") {
					venv = true
				}
				if strings.Contains(s.Run, "go test") && !strings.Contains(s.Run, "-run") {
					jobs++
					if !python || !venv || !lock {
						t.Errorf("%s: the job %s runs go test before Python (%v), the .venv of py-minimal (%v) or with one lock file (%v)",
							file, name, !python, !venv, !lock)
					}
				}
			}
		}
		if wf.Env["PY_TEST_APP"] != "" && (wf.Env["PY_TEST_APP"] != "internal/cdk/testdata/apps/py-minimal" || wf.Env["PYTHON_VERSION"] != ci.Env["PYTHON_VERSION"]) {
			t.Errorf("%s: PY_TEST_APP %q, PYTHON_VERSION %q", file, wf.Env["PY_TEST_APP"], wf.Env["PYTHON_VERSION"])
		}
	}
	if jobs < 4 {
		t.Fatalf("only %d jobs run all the Go tests; the workflows were not read", jobs)
	}
}

// v0.2.0 U3 NFR8.4: no workflow holds AWS credentials or an action that sets
// them, and the one secret is the token of the test-only repository.
func TestNoWorkflowHoldsAWSOrNewSecrets(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", ".github", "workflows", "*.yml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no workflows: %v", err)
	}
	isAWS := func(env map[string]string) string {
		for k := range env {
			if strings.HasPrefix(k, "AWS_") {
				return k
			}
		}
		return ""
	}
	secrets := 0
	for file, wf := range readWorkflows(t) {
		if k := isAWS(wf.Env); k != "" {
			t.Errorf("%s sets %s", file, k)
		}
		for name, j := range wf.Jobs {
			if k := isAWS(j.Env); k != "" {
				t.Errorf("%s: %s sets %s", file, name, k)
			}
			for _, s := range j.Steps {
				if k := isAWS(s.Env); k != "" {
					t.Errorf("%s: %s sets %s", file, name, k)
				}
				if strings.Contains(s.Uses, "configure-aws-credentials") {
					t.Errorf("%s: %s uses %s", file, name, s.Uses)
				}
			}
		}
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		secrets += strings.Count(string(data), "${{ secrets.")
	}
	if secrets != 1 {
		t.Fatalf("the workflows name %d secrets, want 1 (the token of github-pr)", secrets)
	}
}

// v0.2.0 U3 NFR6.18: Dependabot keeps the pip and npm packages of py-minimal
// up to date every week, the pip ones in one pull request.
func TestDependabotFollowsPyMinimal(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "dependabot.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Updates []struct {
			Ecosystem string `yaml:"package-ecosystem"`
			Directory string `yaml:"directory"`
			Schedule  struct {
				Interval string `yaml:"interval"`
			} `yaml:"schedule"`
			CommitMessage struct {
				Prefix string `yaml:"prefix"`
			} `yaml:"commit-message"`
			Groups map[string]any `yaml:"groups"`
		} `yaml:"updates"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, u := range cfg.Updates {
		if u.Directory != "/internal/cdk/testdata/apps/py-minimal" {
			continue
		}
		found[u.Ecosystem] = true
		if u.Schedule.Interval != "weekly" || u.CommitMessage.Prefix != "chore(deps)" {
			t.Errorf("%s: %+v", u.Ecosystem, u)
		}
		if _, ok := u.Groups["py-minimal-pip"]; u.Ecosystem == "pip" && !ok {
			t.Errorf("the pip updates are not in the group py-minimal-pip: %v", u.Groups)
		}
	}
	if !found["pip"] || !found["npm"] {
		t.Fatalf("py-minimal has updates of %v, want pip and npm", found)
	}
}

// Security design §6: npm ci gets no token; gh, git and tag-refactor get the
// token as GH_TOKEN; the variable of the secret reaches none of them.
func TestTheEnvironmentsOfTheIntegrationTest(t *testing.T) {
	base := []string{"PATH=/bin", "GH_TOKEN=ghp_user", "github_token=ghs_x", "TAG_REFACTOR_TEST_REPO_TOKEN=github_pat_secret",
		"TAG_REFACTOR_TEST_REPO=o/sandbox"}

	gh, npm, tool := githubEnvs(base, "github_pat_secret")

	has := func(env []string, kv string) bool { return slices.Contains(env, kv) }
	names := func(env []string) string {
		var out []string
		for _, kv := range env {
			name, _, _ := strings.Cut(kv, "=")
			out = append(out, strings.ToUpper(name))
		}
		return strings.Join(out, ",")
	}
	if strings.Contains(names(npm), "TOKEN") || !has(npm, "PATH=/bin") {
		t.Errorf("npm ci gets %s", names(npm))
	}
	for what, env := range map[string][]string{"gh": gh, "tag-refactor": tool} {
		if !has(env, "GH_TOKEN=github_pat_secret") || has(env, "GH_TOKEN=ghp_user") || strings.Contains(names(env), "TAG_REFACTOR_TEST_REPO_TOKEN") ||
			strings.Contains(names(env), "GITHUB_TOKEN") {
			t.Errorf("%s gets %s", what, names(env))
		}
	}
}

// Review N6: the branch of the subtest options takes the time in UTC, the ID
// of the run when there is one, and a random part.
func TestOptionsBranch(t *testing.T) {
	at := time.Date(2026, 11, 8, 18, 30, 5, 0, time.FixedZone("UTC+9", 9*60*60))

	withRun, again, alone := optionsBranch(at, "987654321"), optionsBranch(at, "987654321"), optionsBranch(at, "")

	requireMatch(t, "branch", withRun, `^tag-refactor/iac-20261108-093005Z-options-987654321-[0-9a-f]{8}$`)
	requireMatch(t, "branch", alone, `^tag-refactor/iac-20261108-093005Z-options-[0-9a-f]{8}$`)
	if withRun == again {
		t.Fatalf("two names in the same second are the same: %s", withRun)
	}
}

// Infrastructure design: the branches of iac --pr older than a day are the ones
// left by an earlier run.
func TestStaleBranches(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	for branch, want := range map[string]bool{
		"tag-refactor/iac-20261106-120000":          true,
		"tag-refactor/iac-20261107-130000":          false,
		"tag-refactor/iac-20261107-115959Z":         true,
		"tag-refactor/iac-20261107-130000Z-options": false,
		"tag-refactor/iac-20261106-120000Z-options": true,
		"tag-refactor/iac-not-a-date":               false,
		"feature/x":                                 false,
		optionsBranch(time.Date(2026, 11, 6, 21, 0, 0, 0, time.FixedZone("UTC+9", 9*60*60)), "12345"): true,
		optionsBranch(time.Date(2026, 11, 7, 13, 0, 0, 0, time.UTC), ""):                              false,
	} {
		if got := staleBranch(branch, now); got != want {
			t.Errorf("staleBranch(%s) = %v", branch, got)
		}
	}
}
