//go:build integration

package cli_test

// Sandbox account test for the U2 checkpoint. Like the U1 test it calls a
// real AWS account, so it only runs locally with the build tag "integration"
// and never in CI. scripts/verify.sh prepares and runs it:
//
//	TAG_REFACTOR_IT_PROFILE=tag-refactor-readonly \
//	TAG_REFACTOR_IT_UNTAGGED_ARN=<ARN of the queue without tags> \
//	TAG_REFACTOR_IT_INDEXED_REGIONS=ap-northeast-1 \
//	go test -mod=readonly -tags integration -run '^TestIntegrationU2' ./internal/cli/...
//
// TAG_REFACTOR_IT_INDEXED_REGIONS lists the regions that have a Resource
// Explorer index (verify.sh reads them with list-indexes); it decides whether
// the scan of every region is complete or partial. The expected results are
// those of infrastructure-specification.md. Other resources in the account
// add to the counts, so counts are checked as "at least".

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/awsaccess"
)

const (
	u2Region          = "ap-northeast-1"
	smallExplorerPage = 5
)

type u2Env struct {
	profile, untaggedARN string
	indexed              []string
}

func u2Environment(t *testing.T) u2Env {
	t.Helper()
	e := u2Env{profile: os.Getenv("TAG_REFACTOR_IT_PROFILE"), untaggedARN: os.Getenv("TAG_REFACTOR_IT_UNTAGGED_ARN")}
	if e.profile == "" || e.untaggedARN == "" {
		t.Skip("set TAG_REFACTOR_IT_PROFILE and TAG_REFACTOR_IT_UNTAGGED_ARN to run this test (scripts/verify.sh does)")
	}
	for _, r := range strings.Split(os.Getenv("TAG_REFACTOR_IT_INDEXED_REGIONS"), ",") {
		if r = strings.TrimSpace(r); r != "" {
			e.indexed = append(e.indexed, r)
		}
	}
	if !slices.Contains(e.indexed, u2Region) {
		e.indexed = append(e.indexed, u2Region)
	}
	return e
}

func u2Session(t *testing.T, e u2Env, pageSize int) *awsaccess.Session {
	t.Helper()
	s, err := awsaccess.NewSession(context.Background(), awsaccess.Options{Profile: e.profile, Region: u2Region, ExplorerPageSize: pageSize})
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	return s
}

// untaggedARNs reads the never-tagged resources of ap-northeast-1 through
// the default view with the given page size.
func untaggedARNs(t *testing.T, e u2Env, pageSize int) []string {
	t.Helper()
	s := u2Session(t, e, pageSize)
	ctx := context.Background()
	view, err := s.DefaultView(ctx, u2Region)
	if err != nil {
		t.Fatalf("default view: %v", err)
	}
	found, err := s.UntaggedResources(ctx, u2Region, view.ARN)
	if err != nil {
		t.Fatalf("ListResources with page size %d: %v", pageSize, err)
	}
	arns := make([]string, 0, len(found))
	for _, r := range found {
		arns = append(arns, r.ARN)
	}
	sort.Strings(arns)
	return arns
}

func countAfter(t *testing.T, stdout, pattern string) int {
	t.Helper()
	m := regexp.MustCompile(pattern).FindStringSubmatch(stdout)
	if m == nil {
		t.Fatalf("stdout does not match %q:\n%s", pattern, stdout)
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func TestIntegrationU2ScanOfTheIndexedRegion(t *testing.T) {
	e := u2Environment(t)
	rules := filepath.Join("..", "..", "test", "sandbox", "u2-rules.yaml")

	r := runAgainstAWS(t, "scan", "--profile", e.profile, "--region", u2Region, "--rules", rules)

	r.requireExit(t, 2)
	requireNotContains(t, "stderr", r.stderr, "Error:")
	requireNotContains(t, "stdout", r.stdout, "Results are incomplete")
	requireMatch(t, "stdout", r.stdout, `(?m)^Untagged-resource discovery: complete$`)
	requireContains(t, "stderr", r.stderr, "never-tagged resources via Resource Explorer in 1 region")
	if n := countAfter(t, r.stdout, `(?m)^Resources with no user tags: (\d+)$`); n < 1 {
		t.Fatalf("no never-tagged resource was found (the queue %s should be)", e.untaggedARN)
	}
	// QA, QB, QC, B1 and the queue without tags have no Owner.
	if n := countAfter(t, r.stdout, `(?m)^  Owner\s+(\d+) resources?`); n < 5 {
		t.Fatalf("Owner is missing on %d resources, want at least 5", n)
	}
	requireMatch(t, "stdout", r.stdout, `(?m)^  environment\s+\d+ resources$`)
}

func TestIntegrationU2ScanAndPlanOfEveryRegion(t *testing.T) {
	e := u2Environment(t)
	rules := filepath.Join("..", "..", "test", "sandbox", "u2-rules.yaml")

	scan := runAgainstAWS(t, "scan", "--profile", e.profile, "--rules", rules)
	plan := runAgainstAWS(t, "plan", "--profile", e.profile, "--rules", rules)

	scan.requireExit(t, 2)
	requireNotContains(t, "scan stderr", scan.stderr, "Error:")
	requireNotContains(t, "scan stdout", scan.stdout, "Results are incomplete")
	enabled := countAfter(t, scan.stdout, `(?m)^Summary: \d+ resources? scanned in \d+ of (\d+) regions`)
	if len(e.indexed) >= enabled {
		requireMatch(t, "scan stdout", scan.stdout, `(?m)^Untagged-resource discovery: complete$`)
	} else {
		requireMatch(t, "scan stdout", scan.stdout, `(?m)^Untagged-resource discovery: partial \(not checked in .*(AWS Config not recording).*\)$`)
		requireNotContains(t, "scan stdout", scan.stdout, "not checked in "+u2Region+",")
	}
	requireMatch(t, "scan stdout", scan.stdout, fmt.Sprintf(`(?m)^Summary: \d+ resources? scanned in %d of %d regions`, enabled, enabled))

	plan.requireExit(t, 2)
	requireNotContains(t, "plan stderr", plan.stderr, "Error:")
	requireNotContains(t, "plan stdout", plan.stdout, "incomplete")
	requireMatch(t, "plan stdout", plan.stdout, `(?m)^  ~ rename key\s+env -> Environment\s+`)
	requireMatch(t, "plan stdout", plan.stdout, `(?m)^\s+Statement "AllowDevQueues"\s+Condition StringEquals aws:ResourceTag/env = "dev"$`)
}

// NFR1.10: ListResources reads several small pages to the end; pages of 5
// and of 500 give the same ARNs, among them the queue without tags.
func TestIntegrationU2ExplorerPagesGiveTheSameResources(t *testing.T) {
	e := u2Environment(t)

	small := untaggedARNs(t, e, smallExplorerPage)
	large := untaggedARNs(t, e, 500)

	if strings.Join(small, ",") != strings.Join(large, ",") {
		t.Fatalf("pages of %d gave %d ARNs, pages of 500 gave %d", smallExplorerPage, len(small), len(large))
	}
	if !slices.Contains(large, e.untaggedARN) {
		t.Fatalf("the queue without tags %s is not in tag:none", e.untaggedARN)
	}
	if len(small) <= smallExplorerPage {
		t.Logf("only %d never-tagged resources: a single page of %d; NFR1.10 multi-page reading is then shown by the unit tests only", len(small), smallExplorerPage)
	}
}

// Security design §2: the three AWS Config operations work with the read-only
// role in an account without recording: no AccessDenied, and the query of
// the tool is accepted (no InvalidExpressionException).
func TestIntegrationU2ConfigOperationsAreAllowed(t *testing.T) {
	e := u2Environment(t)
	s := u2Session(t, e, 0)
	ctx := context.Background()

	recorders, err := s.ConfigRecorders(ctx, u2Region)
	if err != nil {
		t.Fatalf("DescribeConfigurationRecorders and Status: %v", err)
	}
	t.Logf("recorders in %s: %d", u2Region, len(recorders))
	resources, err := s.ConfigResources(ctx, u2Region)
	if err != nil {
		t.Fatalf("SelectResourceConfig with the tool's query: %v", err)
	}
	t.Logf("SelectResourceConfig returned %d results", len(resources))
}

var debugLine = regexp.MustCompile(`(?m)^debug: (\S+) (\S+) (\d+)ms`)

// NFR1.8: the 95th percentile of each operation's time, from the --debug
// lines of a scan and a plan of every region, and the estimate of
// performance design §4 for 10,000 resources.
func TestIntegrationU2LatencyEstimate(t *testing.T) {
	e := u2Environment(t)
	rules := filepath.Join("..", "..", "test", "sandbox", "u2-rules.yaml")
	start := time.Now()
	scan := runAgainstAWS(t, "scan", "--debug", "--profile", e.profile, "--rules", rules)
	plan := runAgainstAWS(t, "plan", "--debug", "--profile", e.profile, "--rules", rules)
	t.Logf("scan and plan of every region took %v", time.Since(start).Round(time.Millisecond))

	byOp := map[string][]int{}
	for _, m := range debugLine.FindAllStringSubmatch(scan.stderr+plan.stderr, -1) {
		ms, _ := strconv.Atoi(m[3])
		byOp[m[1]] = append(byOp[m[1]], ms)
	}
	ops := make([]string, 0, len(byOp))
	for op := range byOp {
		ops = append(ops, op)
	}
	sort.Strings(ops)
	worst, iam := 0.0, 0.0
	for _, op := range ops {
		l95 := percentile95(byOp[op])
		t.Logf("L95 %-45s %6.0fms (n=%d)", op, l95, len(byOp[op]))
		if strings.HasPrefix(op, "iam:") {
			iam = math.Max(iam, l95)
		} else {
			worst = math.Max(worst, l95)
		}
	}
	if len(ops) == 0 {
		t.Fatal("no debug lines were found")
	}
	// Performance design §4, the most uneven case: 214 calls in one region,
	// 101 IAM reads, 30 seconds of the tool's own work, safety factor 1.5.
	estimate := 214*worst/1000*1.5 + 101*iam/1000 + 30
	t.Logf("NFR1.8 estimate for 10,000 resources: %.0fs (limit 600s); the sandbox has few resources, so small pages may make L95 short", estimate)
	if estimate > 600 {
		t.Fatalf("the estimate %.0fs exceeds 10 minutes", estimate)
	}
}

func percentile95(ms []int) float64 {
	sorted := slices.Clone(ms)
	slices.Sort(sorted)
	i := int(math.Ceil(0.95*float64(len(sorted)))) - 1
	return float64(sorted[max(i, 0)])
}
