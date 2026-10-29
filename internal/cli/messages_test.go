package cli

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yucn6201g/tag-refactor/internal/display"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/planner"
)

func TestEveryErrorKindHasAWhatAndAFix(t *testing.T) {
	secret := errors.New("AKIAFAKEFAKEFAKE0000 secret text from the SDK")
	cases := []struct {
		err     *errkind.Error
		what    string
		fixPart string
	}{
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Detail: "bad flag"}, "bad flag", "--help"},
		{&errkind.Error{Kind: errkind.InvalidInput, Detail: "plain detail", Hint: "do this"}, "plain detail", "do this"},
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonOptionNotAvailable, Subject: "apply"}, "apply is not available in this version", "scan"},
		// U4: the files tag-refactor writes name the option to change.
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonOutputExists, Subject: "plan.json", Key: "--out"}, "plan.json already exists", "another file name to --out"},
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonOutputFolderMissing, Subject: "x/plan.json", Key: "--output"}, "the folder of x/plan.json does not exist", "--output"},
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonOutputTooLarge, Subject: "plan.json"}, "larger than 256 MiB", "--region"},
		{&errkind.Error{Kind: errkind.StorageFailed, Reason: errkind.ReasonOutputFailed, Subject: "plan.json", Cause: secret}, "plan.json could not be written", "free space"},
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonPlanFile, Subject: "p.json", Detail: "version is missing", Hint: "save it again"}, "p.json: version is missing", "save it again"},
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonProfileNotFound, Subject: "dev"}, `the AWS profile "dev" was not found`, "aws configure --profile"},
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonRegionNotSet}, "no AWS region is set", "AWS_REGION"},
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonRegionInvalid, Region: "x!"}, `"x!" is not an AWS region name`, "--region"},
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonRegionUnavailable, Region: "me-south-1"}, "the region me-south-1 is not available", "enabled region"},
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonRuleFile, Subject: "r.yaml", Line: 3, Detail: "unknown key", Hint: "fix it"}, "r.yaml line 3: unknown key", "fix it"},
		{&errkind.Error{Kind: errkind.UnsupportedVersion, Subject: "r.yaml", Line: 1, Detail: "version 2"}, "r.yaml line 1: version 2", "check the rule file"},
		{&errkind.Error{Kind: errkind.NotFound, Subject: "r.yaml", Detail: "missing", Hint: "check --rules"}, "r.yaml: missing", "check --rules"},
		{&errkind.Error{Kind: errkind.StorageFailed, Subject: "r.yaml", Detail: "unreadable", Cause: secret}, "r.yaml: unreadable", "check the rule file"},
		{&errkind.Error{Kind: errkind.CredentialsMissing, Cause: secret}, "no AWS credentials were found", "aws configure"},
		{&errkind.Error{Kind: errkind.CredentialsMissing, Reason: errkind.ReasonSSOSessionExpired, Subject: "default"}, "SSO session has expired", `"aws sso login" and`},
		{&errkind.Error{Kind: errkind.CredentialsMissing, Reason: errkind.ReasonCredentialsExpired, Op: "sts:GetCallerIdentity", Subject: "dev"}, "credentials have expired", "aws sso login --profile dev"},
		{&errkind.Error{Kind: errkind.CredentialsMissing, Reason: errkind.ReasonCredentialsRejected, Op: "sts:GetCallerIdentity"}, "AWS rejected the credentials", "access key"},
		{&errkind.Error{Kind: errkind.CredentialsMissing, Reason: errkind.ReasonCredentialsTimeout}, "could not be obtained in time", "network"},
		{&errkind.Error{Kind: errkind.AccessDenied, Op: "iam:GetAccountAuthorizationDetails", Cause: secret}, "access denied for iam:GetAccountAuthorizationDetails", "docs/iam/readonly-policy.json"},
		// ListResources needs the IAM action resource-explorer-2:Search.
		{&errkind.Error{Kind: errkind.AccessDenied, Op: "resource-explorer-2:ListResources", Region: "eu-west-1"},
			"access denied for resource-explorer-2:Search (region eu-west-1)", "allow resource-explorer-2:Search"},
		{&errkind.Error{Kind: errkind.Transient, Reason: errkind.ReasonTimeout, Op: "tag:GetResources", Region: "us-east-1"}, "did not answer tag:GetResources in time (region us-east-1)", "--debug"},
		{&errkind.Error{Kind: errkind.Transient, Op: "tag:GetResources"}, "kept failing on tag:GetResources after retries", "--debug"},
		{&errkind.Error{Kind: errkind.Internal, Cause: secret}, "an unexpected error occurred in tag-refactor", "--debug"},
		{&errkind.Error{Kind: errkind.Internal, Op: "tag:GetResources", Detail: "repeated token"}, "in tag:GetResources: repeated token", "--debug"},
		// U6: Conflict is the lock of an account held by another apply or
		// rollback (BR3.1); AccountMismatch a plan of another account (BR1.2).
		{&errkind.Error{Kind: errkind.Conflict, Subject: "123456789012"}, "another apply or rollback is running for account 123456789012", "--force-unlock"},
		{&errkind.Error{Kind: errkind.AccountMismatch, DetailID: "err.accountMismatch", DetailArgs: []any{"111111111111", "123456789012"},
			HintID: "err.accountMismatchFix", HintArgs: []any{"111111111111"}}, "the plan is for account 111111111111, but the credentials are for account 123456789012", "--profile"},
		{&errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Subject: "--record-to", DetailID: "journal.badLocation",
			DetailArgs: []any{"s3://"}, HintID: "journal.badLocationFix"}, `--record-to "s3://" is not a folder`, "bucket name"},
	}
	for _, tc := range cases {
		what, fix := describe(tc.err, display.English)
		if !strings.Contains(what, tc.what) || !strings.Contains(fix, tc.fixPart) {
			t.Errorf("describe(%s/%s) = %q / %q, want %q / %q", tc.err.Kind, tc.err.Reason, what, fix, tc.what, tc.fixPart)
		}
		if strings.Contains(what+fix, "AKIA") || strings.Contains(what+fix, "secret text") {
			t.Errorf("describe leaks the cause: %q / %q", what, fix)
		}
		// U4 BR3.9: the same error in Japanese is another text with the same
		// values, and never leaks the cause either.
		jaWhat, jaFix := describe(tc.err, display.Japanese)
		if jaWhat == "" || jaFix == "" || strings.Contains(jaWhat+jaFix, "AKIA") || strings.Contains(jaWhat+jaFix, "secret text") {
			t.Errorf("describe in Japanese = %q / %q", jaWhat, jaFix)
		}
		if tc.err.Subject != "" && tc.err.Kind != errkind.CredentialsMissing && !strings.Contains(jaWhat+jaFix, tc.err.Subject) {
			t.Errorf("the Japanese message lost the subject %q: %q / %q", tc.err.Subject, jaWhat, jaFix)
		}
	}
}

// The errors of a saved plan are shown in the user's language (for U6, which
// reads plans): the English lines are the planner's text, the Japanese ones
// come from the catalog with the same values.
func TestPlanFileErrorsAreShownInEitherLanguage(t *testing.T) {
	cases := []struct {
		data             string
		en, enFix        string
		ja, jaFix        string
		subjectInMessage bool
	}{
		{`{"version":2,"toolVersion":"v9.0.0"}`,
			`p.json: plan file version 2 (written by tag-refactor "v9.0.0") is not supported`, "reads plan file version 1",
			`p.json: 変更案のファイルの版 2(tag-refactor "v9.0.0" で作成)には対応していません`, "変更案のファイルの版 1 です", true},
		{`{"version":1}`, "p.json: toolVersion is missing", "save the plan again with plan --out",
			"p.json: toolVersion がありません", "plan --out で変更案を保存し直してください", true},
		{"\xff\xfe{", "p.json: the plan file is UTF-16", "save it again as UTF-8",
			"p.json: 変更案のファイルが UTF-16 です", "UTF-8 で保存し直してください", true},
		{`{"version":1,"inventoryComplete":"yes"}`, "p.json: inventoryComplete must be true or false, not a JSON string", "plan --out",
			"p.json: inventoryComplete は true か false でなければなりません(JSON の string です)", "plan --out", true},
	}
	for _, tc := range cases {
		_, err := planner.ParsePlan("p.json", []byte(tc.data))
		e, ok := errkind.As(err)
		if !ok {
			t.Fatalf("%s: err = %v", tc.data, err)
		}
		what, fix := describe(e, display.English)
		if what != tc.en || !strings.Contains(fix, tc.enFix) || what != "p.json: "+e.Detail || fix != e.Hint {
			t.Errorf("English: %q / %q", what, fix)
		}
		what, fix = describe(e, display.Japanese)
		if what != tc.ja || !strings.Contains(fix, tc.jaFix) {
			t.Errorf("Japanese: %q / %q, want %q / %q", what, fix, tc.ja, tc.jaFix)
		}
	}
	_, err := planner.LoadPlan(filepath.Join(t.TempDir(), "missing.json"))
	e, _ := errkind.As(err)
	if what, fix := describe(e, display.Japanese); !strings.Contains(what, "変更案のファイルがありません") || !strings.Contains(fix, "パスを確かめてください") {
		t.Fatalf("not found in Japanese: %q / %q", what, fix)
	}
}

// BR6.3: an error or an incomplete result is 1, anything to review is 2,
// otherwise 0; incomplete wins over review. plan takes its code from
// Plan.ExitCode (U4 BR2.6), the one place of its rule.
func TestExitCodeFollowsBR6_3(t *testing.T) {
	change := []planner.PlannedResource{{ARN: "a", Changes: []planner.Change{{ChangeID: "c-000001", Kind: planner.KindRemoveKey}}}}
	plan := func(inventory, impact bool, resources []planner.PlannedResource, candidates int) *planner.Plan {
		return &planner.Plan{InventoryComplete: inventory, ImpactAnalysisComplete: impact, Resources: resources,
			RemoveCandidates: make([]planner.RemoveCandidate, candidates)}
	}
	cases := []struct {
		o    outcome
		code int
		mean string
	}{
		{outcome{command: "plan", plan: plan(true, false, change, 0)}, 1, "impact analysis incomplete"},
		// U2 BR5.3, BR5.6: a region the tagging API could not read is 1.
		{outcome{command: "scan", regionsMissing: true, needsReview: true}, 1, "results incomplete"},
		{outcome{command: "plan", plan: plan(false, false, nil, 0)}, 1, "results incomplete"},
		{outcome{command: "scan", needsReview: true}, 2, "issues found"},
		{outcome{command: "plan", plan: plan(true, true, change, 0)}, 2, "review needed"},
		{outcome{command: "scan"}, 0, "no issues"},
		{outcome{command: "plan", plan: plan(true, true, nil, 3)}, 0, "no changes needed"},
	}
	for _, tc := range cases {
		code, id := tc.o.exitCode()
		if meaning := display.T(display.English, id); code != tc.code || meaning != tc.mean {
			t.Errorf("%+v -> %d %q, want %d %q", tc.o, code, meaning, tc.code, tc.mean)
		}
	}
	if ensureKind(nil) != nil || errkind.KindOf(ensureKind(errors.New("x"))) != errkind.Internal {
		t.Fatal("ensureKind must keep nil and classify plain errors as Internal")
	}
}
