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

// Infrastructure design: the branches of iac --pr older than a day are the ones
// left by an earlier run.
func TestStaleBranches(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.Local)
	for branch, want := range map[string]bool{
		"tag-refactor/iac-20261106-120000": true,
		"tag-refactor/iac-20261107-130000": false,
		"tag-refactor/iac-not-a-date":      false,
		"feature/x":                        false,
	} {
		if got := staleBranch(branch, now); got != want {
			t.Errorf("staleBranch(%s) = %v", branch, got)
		}
	}
}
