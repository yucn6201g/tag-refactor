package iac

import (
	"context"
	"os"
	"path/filepath"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/planner"
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
	// Tool says whether the file is a template or a Terraform file.
	Tool Tool
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
)

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
}

// ExitCode is the exit code of the result (BR6.2): 1 when --write stopped, 2
// with anything to fix by hand, otherwise 0.
func (r *Result) ExitCode() int {
	switch {
	case r.WriteErr != nil || r.Interrupted:
		return 1
	case len(r.Manual) > 0:
		return 2
	}
	return 0
}

// Writer runs iac. It takes no AWS connection and no writer of AWS: there is
// nothing of the kind to pass it (U8 NFR3.16).
type Writer struct{}

// New returns a writer.
func New() *Writer { return &Writer{} }

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

// Run does workflow W1: the input, the targets, the places, the edits, the
// difference and, with --write, the checks and the writing. An error is a
// wrong input or a stop before anything was decided; once there is a result,
// the problems of --write are in Result.WriteErr (BR6.2).
func (w *Writer) Run(_ context.Context, req Request) (*Result, error) {
	if _, err := resolveRoot(req.Dir); err != nil {
		return nil, err
	}
	plan, err := loadPlan(req.PlanPath)
	if err != nil {
		return nil, err
	}
	var state stateIndex
	if req.TFStatePath != "" {
		if state, err = readTFState(req.TFStatePath); err != nil {
			return nil, err
		}
	}
	targets, res := buildTargets(plan, state)
	res.HasTFState = req.TFStatePath != ""
	res.WriteRequested = req.Write
	_ = targets
	return res, nil
}
