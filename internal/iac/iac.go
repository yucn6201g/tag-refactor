package iac

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/cdk"
	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/planner"
	"github.com/yucn6201g/tag-refactor/internal/procexec"
)

// Request is one run of iac (IacRequest).
type Request struct {
	// PlanPath is the plan plan --out saved (--plan).
	PlanPath string
	// Dir is the folder of the sources (--dir).
	Dir string
	// TFStatePath is the output of terraform show -json (--tf-state), or empty.
	TFStatePath string
	// Write changes the files (--write); without it only the difference is
	// shown (BR5.1).
	Write bool
	// Progress receives the states of the CDK apps (v0.2.0 contract C6). Nil
	// shows nothing.
	Progress cdk.ProgressSink
}

// ManualReason says why a change is left to be fixed by hand (rules.md).
type ManualReason string

// The reasons of the list to fix by hand.
const (
	ReasonTemplateNotFound    ManualReason = "templateNotFound"
	ReasonBlockNotFound       ManualReason = "blockNotFound"
	ReasonAmbiguous           ManualReason = "ambiguous"
	ReasonInModule            ManualReason = "inModule"
	ReasonMultipleInstances   ManualReason = "multipleInstances"
	ReasonTagsNotLiteral      ManualReason = "tagsNotLiteral"
	ReasonUnsupportedTagShape ManualReason = "unsupportedTagShape"
	ReasonNotLiteral          ManualReason = "notLiteral"
	ReasonKeyNotInSource      ManualReason = "keyNotInSource"
	ReasonValueDiffers        ManualReason = "valueDiffers"
	ReasonKeyAlreadyExists    ManualReason = "keyAlreadyExists"
	ReasonNoTagSite           ManualReason = "noTagSite"
	ReasonUnreadableFile      ManualReason = "unreadableFile"
)

// The reasons of CDK (v0.2.0 contract C7). Their texts are one line each in U1;
// U6 makes them complete.
const (
	ReasonCDKAppFailed           ManualReason = "cdkAppFailed"
	ReasonCDKUnsupportedLanguage ManualReason = "cdkUnsupportedLanguage"
	ReasonCDKV1                  ManualReason = "cdkV1"
	ReasonCDKNoSource            ManualReason = "cdkNoSource"
	ReasonCDKScopeTag            ManualReason = "cdkScopeTag"
	ReasonCDKSharedValue         ManualReason = "cdkSharedValue"
	ReasonCDKAmbiguous           ManualReason = "cdkAmbiguous"
	ReasonCDKExpression          ManualReason = "cdkExpression"
	ReasonCDKUnsupportedShape    ManualReason = "cdkUnsupportedShape"
)

// DetailKind says what ManualItem.Detail holds.
type DetailKind string

// The kinds of detail of a manual item.
const (
	DetailNone       DetailKind = ""
	DetailAddress    DetailKind = "address"    // the Terraform address
	DetailAddresses  DetailKind = "addresses"  // the addresses with the same ARN
	DetailCandidates DetailKind = "candidates" // the files that could be the place
	DetailOtherType  DetailKind = "otherType"  // the files with the logical ID and another Type
	DetailSkipped    DetailKind = "skipped"    // the candidate files left out
	DetailTool       DetailKind = "tool"       // the program a CDK app failed with
)

// ManualItem is a change left to be fixed by hand; in Result.AssumedRemovals,
// a removal counted as reflected, with an empty Reason. It holds no tag value.
type ManualItem struct {
	ARN, ChangeID, Key string
	// File is relative to --dir with /; Line starts at 1. Both can be empty.
	File string
	Line int
	// Reason is empty in AssumedRemovals.
	Reason ManualReason
	// Detail is what DetailKind says: file names or addresses.
	Detail     string
	DetailKind DetailKind
}

// FileResult is what happened to one file.
type FileResult struct {
	// File is relative to --dir with /.
	File string
	// Tool says whether the file is a template, a Terraform file or the source
	// of a CDK app.
	Tool Tool
	// Language is the language of a CDK source (typescript); empty otherwise.
	Language string
	// Reflected counts the changes the file takes.
	Reflected int
	// Diff is the unified difference of the file, in lines joined by \n. Its
	// lines hold the content of the file, not sanitized yet.
	Diff string
	// Written is true once --write has changed the file.
	Written bool
}

// Tool is the kind of IaC of a resource (IacTarget.tool).
type Tool int

// The kinds of IaC.
const (
	ToolNone Tool = iota
	ToolCloudFormation
	ToolTerraform
	ToolCDK
)

// CDKAppStatus is what became of a CDK app (v0.2.0 contract C7).
type CDKAppStatus string

// The states of a CDK app at the end of a run.
const (
	CDKAppDone    CDKAppStatus = "done"
	CDKAppFailed  CDKAppStatus = "failed"
	CDKAppSkipped CDKAppStatus = "skipped"
)

// ReasonNotInU1 is why an app after the first is skipped: U1 synthesizes one
// app (scalability design). It goes when U3 takes several apps; it puts
// nothing on the list to fix by hand and does not change the exit code.
const ReasonNotInU1 = "notInU1"

// CDKApp is one CDK app of the run.
type CDKApp struct {
	// Dir is relative to --dir, with /.
	Dir      string
	Language string
	Status   CDKAppStatus
	Elapsed  time.Duration
	// FailureReason is a reason of cdk.Failure, "helper" for a failure of the
	// helper, or ReasonNotInU1.
	FailureReason string
	// Tool is the program the app failed with: cdk, node, typescript or helper.
	Tool string
}

// SkipReason says why a candidate file was left out (BR3.1, NFR3.19).
type SkipReason string

// The reasons of the files left out.
const (
	SkipTooLarge   SkipReason = "tooLarge"
	SkipNotUTF8    SkipReason = "notUTF8"
	SkipTooDeep    SkipReason = "tooDeep"
	SkipParseError SkipReason = "parseError"
	SkipUnreadable SkipReason = "unreadable"
)

// SkippedFile is a candidate file left out. Line, Column and Kind are the only
// things kept of a parse error: the text of a parser can quote the source
// (NFR8.27).
type SkippedFile struct {
	File         string
	Reason       SkipReason
	Line, Column int
	Kind         string
}

// Result is the result of one run (IacResult).
type Result struct {
	// Files are the files that take at least one change, by name.
	Files []FileResult
	// Manual is the list to fix by hand, by ARN and change ID.
	Manual []ManualItem
	// Reflected counts the changes taken by the files.
	Reflected int
	// AlreadyReflected counts the changes the sources show already.
	AlreadyReflected int
	// AssumedRemovals are the removals counted as reflected because the key is
	// not in the source (BR4.3): the key may come from a stack tag or
	// default_tags.
	AssumedRemovals []ManualItem
	// Unmanaged counts the resources found in no IaC (BR2.3).
	Unmanaged int
	// Resources counts the resources with changes.
	Resources int
	// SkippedFiles and SkippedFolders are the files and folders left out, by
	// name (BR3.1).
	SkippedFiles   []SkippedFile
	SkippedFolders []string
	// HasTFState is true when --tf-state was given.
	HasTFState bool
	// WriteRequested is true when --write was given.
	WriteRequested bool
	// WriteErr is why --write stopped: the git check, the check before
	// writing, or a file that could not be written (BR5.2, BR5.3).
	WriteErr error
	// Interrupted is true when Ctrl+C stopped the writing between two files.
	Interrupted bool
	// Debug holds the lines for --debug: what git said, and why a file failed
	// the check after its edits. They hold no content of the sources.
	Debug []string
	// CDKApps are the CDK apps found below --dir, by folder (v0.2.0 C7).
	CDKApps []CDKApp
	// WrittenFiles are the files --write wrote, relative to --dir with /, in
	// the order of Files: what a pull request commits (v0.2.0 C1).
	WrittenFiles []string
}

// ExitCode is the exit code of the result (BR6.2): 1 when --write stopped, 2
// with anything to fix by hand or a CDK app that failed or was skipped
// (v0.2.0 contract C7), otherwise 0.
func (r *Result) ExitCode() int {
	switch {
	case r.WriteErr != nil || r.Interrupted:
		return 1
	case len(r.Manual) > 0:
		return 2
	}
	for _, a := range r.CDKApps {
		if a.Status != CDKAppDone && a.FailureReason != ReasonNotInU1 {
			return 2
		}
	}
	return 0
}

// Writer runs iac. It takes no AWS connection and no writer of AWS: there is
// nothing of the kind to pass it (U8 NFR3.16). The programs it runs for CDK
// apps get no AWS credentials either (v0.2.0 BR1.3).
type Writer struct {
	synth    cdkSynthesizer
	analyzer cdkAnalyzer
}

// cdkSynthesizer and cdkAnalyzer are the parts of package cdk the writer uses;
// the tests stand in for them.
type (
	cdkSynthesizer interface {
		Discover(ctx context.Context, root string, only []string) ([]cdk.App, error)
		Synthesize(ctx context.Context, app cdk.App, opts cdk.Options, sink cdk.ProgressSink) (cdk.Synthesis, error)
	}
	cdkAnalyzer interface {
		Analyze(ctx context.Context, app cdk.App, req cdk.AnalyzeRequest) (cdk.Analysis, error)
		Recheck(ctx context.Context, app cdk.App, edited map[string][]byte) (cdk.Analysis, error)
	}
)

// Options are what the writer takes from its surroundings for CDK apps.
type Options struct {
	// Runner runs cdk and node; nil runs the programs of the computer.
	Runner procexec.Runner
	// Environ is the environment the programs start from; nil is the one of
	// the process.
	Environ func() []string
	// Now is the clock of the elapsed times; nil is time.Now.
	Now func() time.Time
}

// New returns a writer that runs the programs of the computer.
func New() *Writer { return NewWithOptions(Options{}) }

// NewWithOptions returns a writer with the given surroundings.
func NewWithOptions(o Options) *Writer {
	cfg := cdk.Config{Runner: o.Runner, Environ: o.Environ, Now: o.Now}
	return &Writer{synth: cdk.NewSynthesizer(cfg), analyzer: cdk.NewAnalyzer(cfg)}
}

// usageError is the error of a wrong option or input (BR1.1).
func usageError(subject, detailID, hintID string, args ...any) error {
	return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Subject: subject,
		DetailID: detailID, DetailArgs: args, HintID: hintID}
}

// resolveRoot checks --dir and turns it, once, into the real place: nothing
// after this follows a link (BR1.1, NFR3.18).
func resolveRoot(dir string) (string, error) {
	notFolder := usageError("--dir", "iac.notFolder", "iac.notFolderFix", dir)
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", notFolder
	}
	root, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", notFolder
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", notFolder
	}
	return root, nil
}

// maxPlanBytes is the largest plan iac reads (NFR3.19 (g)), below the limit of
// the loader of U4.
var maxPlanBytes int64 = 200 << 20

// loadPlan reads the plan with the loader of U4 (BR1.2), after the limit of
// iac.
func loadPlan(path string) (*planner.Plan, error) {
	if info, err := os.Stat(path); err == nil && info.Size() > maxPlanBytes {
		return nil, usageError("--plan", "iac.planTooLarge", "iac.planTooLargeFix", path)
	}
	return planner.LoadPlan(path)
}
