package iac

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Unit tests of the check with git (BR5.2, NFR3.20, security design §6), written
// after the implementation. The repositories are temporary ones, without the
// settings of the user; git itself is stood in for where a test needs a failure
// it cannot make.

const gitTemplate = "Resources:\n  Q1:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        - Key: a\n          Value: b\n"

func gitPlan(f *fixture) string { return f.plan(stackResource(arnQ1, "Q1", setValue("a", "b", "c"))) }

func requireWritten(t *testing.T, f *fixture, rel string, res *Result) {
	t.Helper()
	if res.WriteErr != nil {
		t.Fatalf("write stopped: %v", res.WriteErr)
	}
	if !strings.Contains(f.read(rel), "Value: c") {
		t.Fatalf("%s was not written:\n%s", rel, f.read(rel))
	}
}

func requireStopped(t *testing.T, f *fixture, rel, id, before string, res *Result) {
	t.Helper()
	if res.WriteErr == nil {
		t.Fatalf("the write was not stopped: %+v", res)
	}
	requireDetail(t, res.WriteErr, id)
	if res.ExitCode() != 1 {
		t.Fatalf("exit code = %d", res.ExitCode())
	}
	if got := f.read(rel); got != before {
		t.Fatalf("%s changed:\n%s", rel, got)
	}
	if len(res.Files) == 0 || res.Files[0].Diff == "" || res.Files[0].Written {
		t.Fatalf("the difference must still show: %+v", res.Files)
	}
}

func TestGitAllowsACommittedFile(t *testing.T) {
	f := newFixture(t)
	f.file("t.yaml", gitTemplate)
	f.commit()
	requireWritten(t, f, "t.yaml", f.runWrite(gitPlan(f)))
}

func TestGitAllowsASubfolderOfARepository(t *testing.T) {
	f := newFixture(t)
	f.file("infra/t.yaml", gitTemplate)
	f.commit()
	plan := gitPlan(f)
	res, err := New().Run(context.Background(), Request{PlanPath: plan, Dir: filepath.Join(f.dir, "infra"), Write: true})
	if err != nil || res.WriteErr != nil {
		t.Fatalf("%v %v", err, res)
	}
	if !strings.Contains(f.read("infra/t.yaml"), "Value: c") {
		t.Fatal("not written")
	}
}

func TestGitStopsOnEveryStateNotCommitted(t *testing.T) {
	t.Run("not a repository", func(t *testing.T) {
		f := newFixture(t)
		f.file("t.yaml", gitTemplate)
		requireStopped(t, f, "t.yaml", "iac.notWorkTree", gitTemplate, f.runWrite(gitPlan(f)))
	})
	t.Run("no commit", func(t *testing.T) {
		f := newFixture(t)
		f.file("t.yaml", gitTemplate)
		f.git("init", "-q")
		f.git("add", "-A")
		requireStopped(t, f, "t.yaml", "iac.noCommit", gitTemplate, f.runWrite(gitPlan(f)))
	})
	t.Run("untracked", func(t *testing.T) {
		f := newFixture(t)
		f.file("other.txt", "x")
		f.commit()
		f.file("t.yaml", gitTemplate)
		requireStopped(t, f, "t.yaml", "iac.notTracked", gitTemplate, f.runWrite(gitPlan(f)))
	})
	t.Run("ignored", func(t *testing.T) {
		f := newFixture(t)
		f.file(".gitignore", "t.yaml\n")
		f.commit()
		f.file("t.yaml", gitTemplate)
		requireStopped(t, f, "t.yaml", "iac.notTracked", gitTemplate, f.runWrite(gitPlan(f)))
	})
	t.Run("changed", func(t *testing.T) {
		f := newFixture(t)
		f.file("t.yaml", gitTemplate)
		f.commit()
		f.file("t.yaml", gitTemplate+"# edit\n")
		requireStopped(t, f, "t.yaml", "iac.notCommitted", gitTemplate+"# edit\n", f.runWrite(gitPlan(f)))
	})
	t.Run("added, not committed", func(t *testing.T) {
		f := newFixture(t)
		f.file("t.yaml", gitTemplate)
		f.commit()
		f.file("t.yaml", gitTemplate+"# edit\n")
		f.git("add", "t.yaml")
		requireStopped(t, f, "t.yaml", "iac.notCommitted", gitTemplate+"# edit\n", f.runWrite(gitPlan(f)))
	})
	t.Run("new, added, not committed", func(t *testing.T) {
		f := newFixture(t)
		f.file("x.txt", "x")
		f.commit()
		f.file("t.yaml", gitTemplate)
		f.git("add", "t.yaml")
		requireStopped(t, f, "t.yaml", "iac.notCommitted", gitTemplate, f.runWrite(gitPlan(f)))
	})
	t.Run("conflict", func(t *testing.T) {
		f := newFixture(t)
		f.file("t.yaml", gitTemplate)
		f.commit()
		f.git("branch", "-M", "main")
		f.git("checkout", "-q", "-b", "other")
		f.file("t.yaml", gitTemplate+"# other\n")
		f.git("commit", "-q", "-am", "other")
		f.git("checkout", "-q", "main")
		f.file("t.yaml", gitTemplate+"# main\n")
		f.git("commit", "-q", "-am", "main")
		cmd := exec.Command("git", "merge", "other")
		cmd.Dir = f.dir
		if err := cmd.Run(); err == nil {
			t.Skip("the merge did not conflict")
		}
		res := f.runWrite(gitPlan(f))
		if res.WriteErr == nil {
			t.Skip("the file with conflict markers does not parse, so it takes no changes")
		}
		requireDetail(t, res.WriteErr, "iac.conflict")
	})
}

// The conflict itself, for a file that parses in spite of it.
func TestCheckFileOnEachStateOfTheIndex(t *testing.T) {
	raw := []byte("x\r\ny\r\n")
	file := blobHashes("a", raw)
	lf := blobSHA1([]byte("x\ny\n"))
	at := func(mode, hash, stage string) map[string]indexEntry {
		return map[string]indexEntry{"a": {mode, hash, stage, "a"}}
	}
	cases := map[string]struct {
		format string
		index  map[string]indexEntry
		tree   map[string]string
		want   string
	}{
		"same":         {"sha1", at("100644", file.blobSHA1, "0"), map[string]string{"a": file.blobSHA1}, ""},
		"executable":   {"sha1", at("100755", file.blobSHA1, "0"), map[string]string{"a": file.blobSHA1}, ""},
		"CRLF as LF":   {"sha1", at("100644", lf, "0"), map[string]string{"a": lf}, ""},
		"sha256":       {"sha256", at("100644", file.blobSHA256, "0"), map[string]string{"a": file.blobSHA256}, ""},
		"another form": {"sha1", map[string]indexEntry{"a": {"100644", file.blobSHA1, "0", "b"}}, map[string]string{"b": file.blobSHA1}, ""},
		"not in index": {"sha1", map[string]indexEntry{}, map[string]string{}, "iac.notTracked"},
		"conflict":     {"sha1", at("100644", file.blobSHA1, "2"), map[string]string{"a": file.blobSHA1}, "iac.conflict"},
		"symlink":      {"sha1", at("120000", file.blobSHA1, "0"), map[string]string{"a": file.blobSHA1}, "iac.notRegular"},
		"submodule":    {"sha1", at("160000", file.blobSHA1, "0"), map[string]string{"a": file.blobSHA1}, "iac.notRegular"},
		"not in HEAD":  {"sha1", at("100644", file.blobSHA1, "0"), map[string]string{}, "iac.notCommitted"},
		"index ahead":  {"sha1", at("100644", file.blobSHA1, "0"), map[string]string{"a": "other"}, "iac.notCommitted"},
		"file changed": {"sha1", at("100644", "other", "0"), map[string]string{"a": "other"}, "iac.notCommitted"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := checkFile(&file, tc.format, tc.index, tc.tree)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			requireDetail(t, err, tc.want)
		})
	}
	noCRLF := blobHashes("a", []byte("x\n"))
	if noCRLF.lfSHA1 != "" {
		t.Fatal("a file without CRLF has one form")
	}
}

func TestParseIndexKeepsAConflictStage(t *testing.T) {
	index := map[string]indexEntry{}
	parseIndex([]byte("100644 aaa 1\tf\x00100644 bbb 2\tf\x00100644 ccc 3\tf\x00100644 ddd 0\tg\x00junk\x00"), index)
	if index["f"].stage == "0" || index["g"].hash != "ddd" || len(index) != 2 {
		t.Fatalf("index = %+v", index)
	}
}

func TestBlobHashesAreThoseOfGit(t *testing.T) {
	// git hash-object of "hello\n".
	if got := blobSHA1([]byte("hello\n")); got != "ce013625030ba8dba906f756967f9e9ca394464a" {
		t.Fatal(got)
	}
	if got := blobSHA256([]byte("hello\n")); got != "2cf8d83d9ee29543b34a87727421fdecb7e3f3a183d337639025de576db9ebb4" {
		t.Fatal(got)
	}
}

func TestGitCheckWithSHA256Repository(t *testing.T) {
	f := newFixture(t)
	requireGit(t)
	f.file("t.yaml", gitTemplate)
	cmd := exec.Command("git", "init", "-q", "--object-format=sha256")
	cmd.Dir = f.dir
	if err := cmd.Run(); err != nil {
		t.Skipf("this git has no SHA-256: %v", err)
	}
	f.git("config", "core.autocrlf", "false")
	f.git("add", "-A")
	f.git("commit", "-q", "-m", "x")
	requireWritten(t, f, "t.yaml", f.runWrite(gitPlan(f)))
}

// core.autocrlf: the working tree has CRLF and the repository LF, and git says
// the file is unchanged.
func TestGitAutoCRLFFileCountsAsUnchanged(t *testing.T) {
	f := newFixture(t)
	crlf := strings.ReplaceAll(gitTemplate, "\n", "\r\n")
	f.file("t.yaml", crlf)
	f.git("init", "-q")
	f.git("config", "core.autocrlf", "true")
	f.git("add", "-A")
	f.git("commit", "-q", "-m", "x")
	res := f.runWrite(gitPlan(f))
	requireWritten(t, f, "t.yaml", res)
	if !strings.Contains(f.read("t.yaml"), "Value: c\r\n") {
		t.Fatalf("the line breaks changed: %q", f.read("t.yaml"))
	}
}

// fsmonitor and clean filters are commands a repository can set: git status
// would run them. The markers show that nothing did.
func TestGitDoesNotRunWhatTheRepositorySets(t *testing.T) {
	f := newFixture(t)
	f.file("t.yaml", gitTemplate)
	marker := filepath.ToSlash(filepath.Join(t.TempDir(), "marker"))
	f.file(".gitattributes", "*.yaml filter=evil\n")
	f.git("init", "-q")
	f.git("config", "core.autocrlf", "false")
	f.git("add", "-A")
	f.git("commit", "-q", "-m", "x")
	f.git("config", "core.fsmonitor", "touch "+marker+"-fsmonitor; true")
	f.git("config", "filter.evil.clean", "touch "+marker+"-clean; cat")
	f.git("config", "filter.evil.smudge", "cat")

	res := f.runWrite(gitPlan(f))

	requireWritten(t, f, "t.yaml", res)
	for _, name := range []string{marker + "-fsmonitor", marker + "-clean"} {
		if _, err := os.Stat(filepath.FromSlash(name)); err == nil {
			t.Fatalf("%s ran", name)
		}
	}
}

// NFR3.20: GIT_DIR must not point the check at another repository.
func TestGitIgnoresTheVariablesOfTheEnvironment(t *testing.T) {
	f := newFixture(t)
	f.file("t.yaml", gitTemplate)
	f.commit()
	other := newFixture(t)
	other.file("x.txt", "x")
	other.commit()
	for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE"} {
		t.Setenv(name, filepath.Join(other.dir, ".git"))
	}
	if runtime.GOOS == "windows" {
		t.Setenv("Git_Dir", filepath.Join(other.dir, ".git"))
	}
	plan := gitPlan(f)
	res, err := New().Run(context.Background(), Request{PlanPath: plan, Dir: f.dir, Write: true})
	if err != nil {
		t.Fatal(err)
	}
	requireWritten(t, f, "t.yaml", res)
}

func TestGitEnvRemovesAndAdds(t *testing.T) {
	env := gitEnv([]string{"PATH=x", "GIT_DIR=a", "Git_Dir=b", "GITHUB=c"}, true)
	joined := strings.Join(env, " ")
	if strings.Contains(joined, "GIT_DIR") || strings.Contains(joined, "Git_Dir") || !strings.Contains(joined, "PATH=x") || !strings.Contains(joined, "GITHUB=c") {
		t.Fatal(joined)
	}
	for _, want := range []string{"GIT_OPTIONAL_LOCKS=0", "GIT_LITERAL_PATHSPECS=1", "GIT_TERMINAL_PROMPT=0"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("no %s in %s", want, joined)
		}
	}
	if env := strings.Join(gitEnv([]string{"Git_Dir=b"}, false), " "); !strings.Contains(env, "Git_Dir=b") {
		t.Fatalf("names have a case outside Windows: %s", env)
	}
}

func TestGitFilesWithSpecialNames(t *testing.T) {
	names := []string{"with space.yaml", "-dash.yaml", "日本語.yaml", "[x].yaml", "a'b.yaml"}
	if runtime.GOOS != "windows" {
		names = append(names, "star*.yaml", "colon:x.yaml", ":leading.yaml", "quo\"te.yaml")
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.file(name, gitTemplate)
			f.file("dummy.yaml", "x: 1\n")
			f.commit()
			requireWritten(t, f, name, f.runWrite(gitPlan(f)))
		})
	}
	t.Run("a pattern does not match another file", func(t *testing.T) {
		f := newFixture(t)
		f.file("[x].yaml", gitTemplate)
		f.file("x.yaml", "y: 1\n")
		f.commit()
		f.file("[x].yaml", gitTemplate+"# edit\n")
		res := f.runWrite(gitPlan(f))
		requireDetail(t, res.WriteErr, "iac.notCommitted")
	})
}

func TestGitChecksAboutAThousandFilesInBatches(t *testing.T) {
	if testing.Short() {
		t.Skip("a thousand files")
	}
	f := newFixture(t)
	var resources []resource
	const n = 1000
	long := strings.Repeat("d", 60)
	for i := range n {
		id := "Q" + strconv.Itoa(i)
		f.file(long+"/t"+strconv.Itoa(i)+".yaml", strings.ReplaceAll(gitTemplate, "Q1", id))
		arn := "arn:aws:sqs:ap-northeast-1:111122223333:" + id
		resources = append(resources, stackResource(arn, id, setValue("a", "b", "c")))
	}
	f.commit()
	calls := 0
	saved := runGit
	t.Cleanup(func() { runGit = saved })
	runGit = func(ctx context.Context, git string, env, args []string) ([]byte, []byte, error) {
		calls++
		total := 0
		for _, a := range args {
			total += len(a) + 1
		}
		if total > maxGitArgsLength+1000 {
			t.Errorf("a command line of %d characters", total)
		}
		return runGitCommand(ctx, git, env, args)
	}

	res := f.runWrite(f.plan(resources...))

	if res.WriteErr != nil || len(res.Files) != n || !res.Files[n-1].Written {
		t.Fatalf("result: %v, %d files", res.WriteErr, len(res.Files))
	}
	if calls < 4 {
		t.Fatalf("%d calls of git: the files were not split", calls)
	}
}

func TestBatchesKeepUnderTheLimit(t *testing.T) {
	c := &gitChecker{root: "r"}
	files := make([]gitFile, 40)
	for i := range files {
		files[i].rel = strings.Repeat("x", 1000)
	}
	batches := c.batches(files)
	if len(batches) != 2 || len(batches[0])+len(batches[1]) != 40 {
		t.Fatalf("%d batches", len(batches))
	}
	if got := c.batches(nil); len(got) != 0 {
		t.Fatalf("%v", got)
	}
}

// standGit replaces git with a function of the subcommand.
func standGit(t *testing.T, answer func(sub string, args []string) ([]byte, []byte, error)) {
	t.Helper()
	savedRun, savedLook := runGit, lookGit
	t.Cleanup(func() { runGit, lookGit = savedRun, savedLook })
	lookGit = func(string) (string, error) { return "git", nil }
	runGit = func(_ context.Context, _ string, _, args []string) ([]byte, []byte, error) {
		for i, a := range args {
			if a == "-C" {
				return answer(args[i+2], args[i+3:])
			}
		}
		t.Fatalf("no -C in %v", args)
		return nil, nil, nil
	}
}

func TestGitIsNotInstalled(t *testing.T) {
	savedLook := lookGit
	t.Cleanup(func() { lookGit = savedLook })
	lookGit = func(string) (string, error) { return "", exec.ErrNotFound }
	f := newFixture(t)
	f.file("t.yaml", gitTemplate)
	res := f.runWrite(gitPlan(f))
	requireStopped(t, f, "t.yaml", "iac.gitMissing", gitTemplate, res)
}

func TestGitFailuresStopTheWrite(t *testing.T) {
	file := blobHashes("t.yaml", []byte(gitTemplate))
	entry := "100644 " + file.blobSHA1 + " 0\tt.yaml\x00"
	tree := "100644 blob " + file.blobSHA1 + "\tt.yaml\x00"
	fail := errors.New("failed")
	asked := 0
	cases := map[string]struct {
		answer func(sub string, args []string) ([]byte, []byte, error)
		want   string
	}{
		"not true":        {func(string, []string) ([]byte, []byte, error) { return []byte("false\n"), nil, nil }, "iac.notWorkTree"},
		"rev-parse fails": {func(string, []string) ([]byte, []byte, error) { return nil, []byte("fatal: x"), fail }, "iac.notWorkTree"},
		"format fails": {func(sub string, args []string) ([]byte, []byte, error) {
			if args[0] == "--show-object-format" {
				return nil, nil, fail
			}
			return []byte("true\n"), nil, nil
		}, "iac.gitFailed"},
		"format unknown": {func(sub string, args []string) ([]byte, []byte, error) {
			if args[0] == "--show-object-format" {
				return []byte("md5\n"), nil, nil
			}
			return []byte("true\n"), nil, nil
		}, "iac.gitFailed"},
		"ls-files fails": {func(sub string, args []string) ([]byte, []byte, error) {
			switch {
			case sub == "ls-files":
				return nil, nil, fail
			case args[0] == "--show-object-format":
				return []byte("sha1\n"), nil, nil
			}
			return []byte("true\n"), nil, nil
		}, "iac.gitFailed"},
		"ls-tree stops": {func(sub string, args []string) ([]byte, []byte, error) {
			switch {
			case sub == "ls-files":
				return []byte(entry), nil, nil
			case sub == "ls-tree":
				return nil, nil, context.DeadlineExceeded
			case args[0] == "--show-object-format":
				return []byte("sha1\n"), nil, nil
			}
			return []byte("true\n"), nil, nil
		}, "iac.gitFailed"},
		"a path in another form": {func(sub string, args []string) ([]byte, []byte, error) {
			switch {
			case sub == "ls-files":
				// The list of all files leaves it out; the question about it alone
				// finds it.
				asked++
				if asked == 1 {
					return nil, nil, nil
				}
				return []byte(entry), nil, nil
			case sub == "ls-tree":
				return []byte(tree), nil, nil
			case args[0] == "--show-object-format":
				return []byte("sha1\n"), nil, nil
			}
			return []byte("true\n"), nil, nil
		}, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			standGit(t, tc.answer)
			debug := []string{}
			err := checkGit(context.Background(), "root", []gitFile{file}, &debug)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			requireDetail(t, err, tc.want)
			if errkind.KindOf(err) != errkind.InvalidInput {
				t.Fatal(err)
			}
		})
	}
}

func TestGitTimeLimit(t *testing.T) {
	requireGit(t)
	saved := gitTimeout
	gitTimeout = time.Nanosecond
	t.Cleanup(func() { gitTimeout = saved })
	f := newFixture(t)
	f.file("t.yaml", gitTemplate)
	f.commit()
	res := f.runWrite(gitPlan(f))
	if res.WriteErr == nil {
		t.Fatal("the time limit did not stop the check")
	}
	if len(res.Debug) == 0 || !strings.HasPrefix(res.Debug[0], "git ") {
		t.Fatalf("debug = %v", res.Debug)
	}
}

func TestWriteWithoutAnythingToWriteNeedsNoGit(t *testing.T) {
	savedLook := lookGit
	t.Cleanup(func() { lookGit = savedLook })
	lookGit = func(string) (string, error) {
		t.Error("git was looked for")
		return "", exec.ErrNotFound
	}
	f := newFixture(t)
	f.file("t.yaml", gitTemplate)
	res := f.runWrite(f.plan(stackResource(arnQ1, "Q1", setValue("a", "x", "y"))))
	if res.WriteErr != nil || len(res.Manual) != 1 {
		t.Fatalf("result = %+v", res)
	}
}
