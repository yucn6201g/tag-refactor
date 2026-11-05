package iac

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/fileout"
)

// Unit tests of the writing and of the whole flow (BR5.3, BR6.1, BR6.2, NFR3.16,
// NFR5.26, NFR6.5, NFR6.6, NFR8.27), written after the implementation.

// twoFiles is a folder with two committed templates and a plan for both.
func twoFiles(t *testing.T) (*fixture, string) {
	t.Helper()
	f := newFixture(t)
	f.file("a.yaml", strings.ReplaceAll(gitTemplate, "Q1", "A"))
	f.file("b.yaml", strings.ReplaceAll(gitTemplate, "Q1", "B"))
	plan := f.plan(stackResource(arnQ1, "A", setValue("a", "b", "c")), stackResource(arnQ2, "B", setValue("a", "b", "c")))
	f.commit()
	return f, plan
}

func TestWriteChangesBothFilesAndLeavesNoTemporaryFile(t *testing.T) {
	f, plan := twoFiles(t)
	res := f.runWrite(plan)
	if res.WriteErr != nil || !res.Files[0].Written || !res.Files[1].Written || res.ExitCode() != 0 {
		t.Fatalf("result = %+v", res)
	}
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("left over: %s", e.Name())
		}
	}
}

func TestWithoutWriteNothingChanges(t *testing.T) {
	f, plan := twoFiles(t)
	before := f.read("a.yaml")
	res := f.run(plan)
	if len(res.Files) != 2 || res.Files[0].Written || f.read("a.yaml") != before {
		t.Fatalf("result = %+v", res)
	}
}

// NFR6.6: a file changed after it was read stops every file.
func TestFileChangedBeforeWritingStopsAll(t *testing.T) {
	f, plan := twoFiles(t)
	saved := gitCheck
	t.Cleanup(func() { gitCheck = saved })
	gitCheck = func(ctx context.Context, root string, files []gitFile, debug *[]string) error {
		err := saved(ctx, root, files, debug)
		// git has said the files are as committed: the moment to change one.
		f.file("b.yaml", strings.ReplaceAll(gitTemplate, "Q1", "B")+"# edit\n")
		return err
	}

	res := f.runWrite(plan)

	requireDetail(t, res.WriteErr, "iac.changedSinceRead")
	if strings.Contains(f.read("a.yaml"), "Value: c") {
		t.Fatal("the first file was written")
	}
	if res.ExitCode() != 1 {
		t.Fatalf("exit code = %d", res.ExitCode())
	}
}

func TestFileChangedWhileItIsReadGoesOnTheList(t *testing.T) {
	f := newFixture(t)
	f.file("t.yaml", gitTemplate)
	saved := openFile
	t.Cleanup(func() { openFile = saved })
	count := 0
	var mu sync.Mutex
	openFile = func(p string) (io.ReadCloser, error) {
		mu.Lock()
		defer mu.Unlock()
		count++
		if count == 2 {
			f.file("t.yaml", gitTemplate+"# changed\n")
		}
		return os.Open(p)
	}

	res := f.run(gitPlan(f))

	requireReasons(t, res, "a unreadableFile")
	if len(res.Files) != 0 || len(res.Debug) != 1 {
		t.Fatalf("result = %+v", res)
	}
}

func TestFileThatDisappearsOnTheSecondReadingGoesOnTheList(t *testing.T) {
	f := newFixture(t)
	f.file("t.yaml", gitTemplate)
	saved := openFile
	t.Cleanup(func() { openFile = saved })
	count := 0
	openFile = func(p string) (io.ReadCloser, error) {
		count++
		if count == 2 {
			return nil, os.ErrNotExist
		}
		return os.Open(p)
	}
	res := f.run(gitPlan(f))
	requireReasons(t, res, "a unreadableFile")
}

func TestUnreadableFileIsLeftOut(t *testing.T) {
	f := newFixture(t)
	f.file("t.yaml", gitTemplate)
	saved := openFile
	t.Cleanup(func() { openFile = saved })
	openFile = func(string) (io.ReadCloser, error) { return nil, os.ErrPermission }
	res := f.run(gitPlan(f))
	if len(res.SkippedFiles) != 1 || res.SkippedFiles[0].Reason != SkipUnreadable {
		t.Fatalf("result = %+v", res)
	}
	requireReasons(t, res, "a templateNotFound")
}

func TestFileThatGrewPastTheLimitIsLeftOut(t *testing.T) {
	f := newFixture(t)
	f.file("t.yaml", gitTemplate)
	saved := openFile
	t.Cleanup(func() { openFile = saved })
	openFile = func(string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(strings.Repeat("x", int(maxFileBytes)+5))), nil
	}
	res := f.run(gitPlan(f))
	if len(res.SkippedFiles) != 1 || res.SkippedFiles[0].Reason != SkipTooLarge {
		t.Fatalf("result = %+v", res)
	}
}

func TestReadOnlyFileStopsAll(t *testing.T) {
	f, plan := twoFiles(t)
	path := filepath.Join(f.dir, "b.yaml")
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	res := f.runWrite(plan)

	requireDetail(t, res.WriteErr, "iac.readOnly")
	if strings.Contains(f.read("a.yaml"), "Value: c") {
		t.Fatal("the first file was written")
	}
}

// NFR6.6: the second file fails; the first stays written and the result says so.
func TestSecondFileFailingLeavesTheFirstWritten(t *testing.T) {
	f, plan := twoFiles(t)
	saved := replaceFile
	t.Cleanup(func() { replaceFile = saved })
	replaceFile = func(p string, content []byte, opts fileout.ReplaceOptions) error {
		if filepath.Base(p) == "b.yaml" {
			return errors.New("disk full SECRET")
		}
		return saved(p, content, opts)
	}

	res := f.runWrite(plan)

	if !res.Files[0].Written || res.Files[1].Written {
		t.Fatalf("files = %+v", res.Files)
	}
	requireDetail(t, res.WriteErr, "iac.writeFailed")
	if strings.Contains(res.WriteErr.Error(), "SECRET") {
		t.Fatal(res.WriteErr)
	}
	if !strings.Contains(f.read("a.yaml"), "Value: c") || strings.Contains(f.read("b.yaml"), "Value: c") {
		t.Fatal("the files do not match the result")
	}
	if res.ExitCode() != 1 {
		t.Fatalf("exit code = %d", res.ExitCode())
	}
}

// Security design §7: Ctrl+C during the writing finishes the file being written
// and writes no more.
func TestCancelBetweenFilesStopsTheWriting(t *testing.T) {
	f, plan := twoFiles(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	saved := replaceFile
	t.Cleanup(func() { replaceFile = saved })
	replaceFile = func(p string, content []byte, opts fileout.ReplaceOptions) error {
		err := saved(p, content, opts)
		cancel()
		return err
	}

	res, err := New().Run(ctx, Request{PlanPath: plan, Dir: f.dir, Write: true})

	if err != nil {
		t.Fatal(err)
	}
	if !res.Interrupted || !res.Files[0].Written || res.Files[1].Written || res.ExitCode() != 1 {
		t.Fatalf("result = %+v", res)
	}
}

func TestCancelBeforeWritingWritesNothing(t *testing.T) {
	f, plan := twoFiles(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New().Run(ctx, Request{PlanPath: plan, Dir: f.dir, Write: true}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(f.read("a.yaml"), "Value: c") {
		t.Fatal("written")
	}
}

func TestPanicOfAWorkerStopsTheRun(t *testing.T) {
	f := newFixture(t)
	f.file("t.yaml", gitTemplate)
	saved := openFile
	t.Cleanup(func() { openFile = saved })
	openFile = func(string) (io.ReadCloser, error) { panic("boom") }
	_, err := New().Run(context.Background(), Request{PlanPath: gitPlan(f), Dir: f.dir})
	if errkind.KindOf(err) != errkind.Internal {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "boom") {
		t.Fatal(err)
	}
}

// The order of the result does not depend on which worker came first.
func TestResultsComeInFileOrder(t *testing.T) {
	f := newFixture(t)
	var resources []resource
	var names []string
	for _, n := range []string{"z", "b", "m", "a", "k", "c", "x", "d", "e", "y"} {
		id := "Q" + n
		f.file(n+"/t.yaml", strings.ReplaceAll(gitTemplate, "Q1", id))
		names = append(names, n+"/t.yaml")
		resources = append(resources, stackResource("arn:aws:sqs:ap-northeast-1:111122223333:"+n, id, setValue("a", "b", "c"), setValue("x", "1", "2")))
	}
	slices.Sort(names)
	plan := f.plan(resources...)
	var first string
	for round := range 5 {
		res := f.run(plan)
		var got []string
		for _, file := range res.Files {
			got = append(got, file.File)
		}
		if !slices.Equal(got, names) {
			t.Fatalf("files = %v", got)
		}
		text := strings.Join(reasons(res), "|")
		if round == 0 {
			first = text
		} else if text != first {
			t.Fatalf("the order changed: %s", text)
		}
		for i := 1; i < len(res.Manual); i++ {
			if res.Manual[i-1].ARN > res.Manual[i].ARN {
				t.Fatalf("not by ARN: %+v", res.Manual)
			}
		}
	}
}

func TestParseBudgetBoundsTheFilesAtOnce(t *testing.T) {
	saved := parseBudgetBytes
	parseBudgetBytes = 100
	t.Cleanup(func() { parseBudgetBytes = saved })
	b := newBudget()
	release, err := b.acquire(context.Background(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b.acquire(ctx, 1); err == nil {
		t.Fatal("a file bigger than the budget takes all of it")
	}
	release()
	if release, err := b.acquire(context.Background(), 100); err != nil {
		t.Fatal(err)
	} else {
		release()
	}
}

func TestMemoryLimitIsPutBack(t *testing.T) {
	before := setMemoryLimit()
	before()
	f := newFixture(t)
	f.file("t.yaml", gitTemplate)
	if res := f.run(gitPlan(f)); len(res.Files) != 1 {
		t.Fatalf("result = %+v", res)
	}
}

func TestExitCodes(t *testing.T) {
	for name, tc := range map[string]struct {
		res  Result
		want int
	}{
		"nothing":     {Result{}, 0},
		"reflected":   {Result{Reflected: 1, AlreadyReflected: 1, Unmanaged: 3, AssumedRemovals: []ManualItem{{}}}, 0},
		"manual":      {Result{Manual: []ManualItem{{}}}, 2},
		"failed":      {Result{WriteErr: errors.New("x"), Manual: []ManualItem{{}}}, 1},
		"interrupted": {Result{Interrupted: true}, 1},
	} {
		if got := tc.res.ExitCode(); got != tc.want {
			t.Errorf("%s: exit code %d, want %d", name, got, tc.want)
		}
	}
}

// NFR3.16: iac takes in neither the AWS access nor the SDK directly.
func TestImportsNoAWSDirectly(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not on the path")
	}
	out, err := exec.Command("go", "list", "-f", `{{join .Imports "\n"}}`, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasSuffix(line, "/internal/awsaccess") || strings.HasPrefix(line, "github.com/aws/aws-sdk-go-v2") {
			t.Errorf("iac imports %s", line)
		}
	}
	if !strings.Contains(string(out), "/internal/restype") {
		t.Fatalf("the list does not look right:\n%s", out)
	}
}

// NFR8.27: nothing outside the difference reaches the result.
func TestSecretsStayOutOfTheResult(t *testing.T) {
	const secret = "SECRET-VALUE-123"
	f := newFixture(t)
	body := "Parameters:\n  P:\n    Default: " + secret + "\n" + strings.Repeat("# filler\n", 8)
	f.file("t.yaml", body+gitTemplate)
	f.file("bad.yaml", "Resources:\n  X: ["+secret+"\n")
	f.file("bad.json", `{"Resources": `+secret+`}`)
	f.file("bad.tf", `resource "a" { password = "`+secret)
	state := stateFile(t, `{"format_version":"1.0","values":{"root_module":{"resources":[
	  {"address":"aws_sqs_queue.t","mode":"managed","values":{"arn":"`+arnQ2+`","password":"`+secret+`"}}]}}}`)
	f.file("main.tf", "resource \"aws_sqs_queue\" \"t\" {\n  tags = {\n    a = \"b\"\n  }\n}\n")
	plan := f.plan(stackResource(arnQ1, "Q1", setValue("a", "b", "c")), tfResource(arnQ2, setValue("a", "b", "c")))

	res := f.runState(plan, state)

	if len(res.Files) != 2 {
		t.Fatalf("files = %+v", res.Files)
	}
	var all strings.Builder
	for _, s := range res.SkippedFiles {
		all.WriteString(s.File + s.Kind)
	}
	for _, m := range res.Manual {
		all.WriteString(m.Detail)
	}
	for _, d := range res.Debug {
		all.WriteString(d)
	}
	for _, file := range res.Files {
		all.WriteString(file.Diff)
	}
	if strings.Contains(all.String(), secret) {
		t.Fatalf("the secret is in the result:\n%s", all.String())
	}
}

// NFR8.27: three lines around a change are in the difference, as specified.
func TestLinesNextToAChangeAreInTheDifference(t *testing.T) {
	f := newFixture(t)
	f.file("t.yaml", "# NEAR-VALUE\n"+gitTemplate)
	res := f.run(gitPlan(f))
	if strings.Contains(res.Files[0].Diff, "NEAR-VALUE") {
		t.Fatalf("seven lines above is not context:\n%s", res.Files[0].Diff)
	}
	g := newFixture(t)
	g.file("t.yaml", gitTemplate+"# NEAR-VALUE\n")
	res = g.run(gitPlan(g))
	if !strings.Contains(res.Files[0].Diff, "NEAR-VALUE") {
		t.Fatalf("the next line is context:\n%s", res.Files[0].Diff)
	}
}

func TestPermissionsStayTheSame(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no such bits")
	}
	f := newFixture(t)
	path := f.file("t.yaml", gitTemplate)
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	plan := gitPlan(f)
	f.commit()
	f.runWrite(plan)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("permissions = %o", info.Mode().Perm())
	}
}

func TestNoTargetsNeedNoFiles(t *testing.T) {
	f := newFixture(t)
	f.file("bad.yaml", "\xff\xff")
	res := f.run(f.plan(tfResource(arnQ1, remove("a", "1"))))
	if res.Unmanaged != 1 || len(res.SkippedFiles) != 0 || res.Resources != 1 {
		t.Fatalf("result = %+v", res)
	}
}

func TestResourceWithoutChangesIsNoTarget(t *testing.T) {
	f := newFixture(t)
	res := f.run(f.plan())
	if res.Resources != 0 || res.Unmanaged != 0 {
		t.Fatalf("result = %+v", res)
	}
}

func TestSamePlanGivesTheSameDifferenceOnEveryOS(t *testing.T) {
	// The expected text is fixed here, so all three operating systems of the CI
	// compare with the same bytes (NFR9.11).
	f := newFixture(t)
	f.file("stacks/orders.yaml", "Resources:\n  OrdersQueue:\n    Type: AWS::SQS::Queue\n    Properties:\n      Tags:\n        - Key: env          # 環境\n          Value: Prod\n        - Key: obsolete\n          Value: x\n")
	res := f.run(f.plan(stackResource(arnQ1, "OrdersQueue", rename("env", "Environment", "Prod"), setValue("Environment", "Prod", "prod"), remove("obsolete", "x"), addTag("CostCenter", "100"))))
	want := "--- a/stacks/orders.yaml\n+++ b/stacks/orders.yaml\n@@ -3,7 +3,7 @@\n     Type: AWS::SQS::Queue\n     Properties:\n       Tags:\n" +
		"-        - Key: env          # 環境\n-          Value: Prod\n-        - Key: obsolete\n-          Value: x\n" +
		"+        - Key: Environment          # 環境\n+          Value: prod\n+        - Key: CostCenter\n+          Value: \"100\""
	requireText(t, res.Files[0].Diff, want)
	if res.Reflected != 4 || res.Files[0].File != "stacks/orders.yaml" || res.Files[0].Tool != ToolCloudFormation {
		t.Fatalf("result = %+v", res.Files[0])
	}
}

func TestSandboxPlanFileIsValid(t *testing.T) {
	f := newFixture(t)
	f.file("u6-fixtures.yaml", "Resources:\n  VU6:\n    Type: AWS::EC2::VPC\n    Properties:\n      Tags:\n        - Key: project\n          Value: tag-refactor-fixture\n"+
		"  GU6:\n    Type: AWS::EC2::SecurityGroup\n    Properties:\n      Tags:\n        - Key: project\n          Value: tag-refactor-fixture\n  BU6R:\n    Type: AWS::S3::Bucket\n")
	res, err := New().Run(context.Background(), Request{PlanPath: filepath.Join("testdata", "sandbox-plan.json"), Dir: f.dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Reflected != 4 {
		t.Fatalf("reflected = %d: %v", res.Reflected, reasons(res))
	}
	requireReasons(t, res, "project keyNotInSource", "Project keyNotInSource")
}

// The real file of the sandbox, which the checkpoint of U8 rewrites too.
func TestRealSandboxTemplate(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "test", "sandbox", "u6-fixtures.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	f := newFixture(t)
	f.file("u6-fixtures.yaml", strings.ReplaceAll(string(raw), "\r\n", "\n"))
	res, err := New().Run(context.Background(), Request{PlanPath: filepath.Join("testdata", "sandbox-plan.json"), Dir: f.dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Reflected != 4 || len(res.Files) != 1 {
		t.Fatalf("reflected = %d: %v", res.Reflected, reasons(res))
	}
	if got := strings.Count(res.Files[0].Diff, "\n+"); got != 5 {
		// One +++ line and four lines with their keys and values.
		t.Fatalf("%d lines with +:\n%s", got, res.Files[0].Diff)
	}
}
