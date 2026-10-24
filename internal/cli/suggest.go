package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/spf13/cobra"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/rules"
)

// suggestCommandName is the command of U3 workflow W2.
const suggestCommandName = "rules suggest"

// draftTooLargeWarning is printed when the draft is over 64 KiB even without
// value candidates (U3 BR6.7).
const draftTooLargeWarning = "the draft is larger than 64 KiB and cannot be read as it is; remove keys you do not need"

func (a *app) rulesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Work with rule files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(a.suggestCommand())
	return cmd
}

func (a *app) suggestCommand() *cobra.Command {
	var profile, output string
	var regions []string
	cmd := &cobra.Command{
		Use:   "suggest",
		Short: "Draft a rule file from the tag keys and values in use (every region)",
		Long: "rules suggest reads the tagged resources of every enabled region (or the --region values), groups\n" +
			"the tag keys and values written in different ways, and prints a draft rule file to standard output:\n" +
			"the most used spelling of each key as its canonical key, the others as aliases, and value candidates.\n" +
			"On Windows PowerShell, save it with --output so it stays UTF-8.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := rejectUnavailableFlags(cmd); err != nil {
				return err
			}
			return ensureKind(a.runSuggest(cmd.Context(), profile, output, regions))
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "AWS profile to use (default: the standard AWS settings)")
	cmd.Flags().StringArrayVar(&regions, "region", nil, regionFlagHelp)
	cmd.Flags().StringVar(&output, "output", "",
		"write the draft to this file (UTF-8) instead of standard output; an existing file is never overwritten")
	addUnavailableFlags(cmd)
	return cmd
}

// runSuggest is U3 workflow W2. It connects with the account and tagging
// readers only (NFR3.6), collects like scan without looking for never-tagged
// resources, and writes the draft to standard output or to --output.
func (a *app) runSuggest(ctx context.Context, profile, output string, regions []string) error {
	a.started = a.deps.Now()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if output != "" {
		// Before AWS is called, so a wrong path does not cost a collection.
		if err := checkOutputPath(output); err != nil {
			return err
		}
	}
	_, snap, err := collect(ctx, a, profile, regions, tagReadersOf)
	if err != nil {
		return err
	}
	draft := rules.Suggest(suggestInput(snap, a.deps.Now()))
	if output != "" {
		if err := writeDraftFile(output, draft.Text); err != nil {
			return err
		}
		a.p.DraftWritten(output, len(draft.Keys))
	} else {
		a.p.WriteDraft(draft.Text)
	}
	a.p.DraftIncomplete(snap.FailedRegions)
	if draft.TooLarge {
		a.p.Warn(draftTooLargeWarning)
	}
	a.p.SuggestScope(snap)
	a.result = &outcome{
		command:        suggestCommandName,
		regionsMissing: !snap.Complete(),
		draftTooLarge:  draft.TooLarge,
		resources:      len(snap.Resources),
	}
	return nil
}

// suggestInput turns the collection into the input of rules.Suggest, with
// the grouping of scan (inventory.CanonicalKey and its dictionary).
func suggestInput(snap *inventory.Snapshot, now time.Time) rules.SuggestInput {
	resources := make([]rules.TaggedResource, 0, len(snap.Resources))
	for _, r := range snap.Resources {
		resources = append(resources, rules.TaggedResource{ARN: r.ARN, Tags: r.Tags})
	}
	failed := make([]string, 0, len(snap.FailedRegions))
	for _, f := range snap.FailedRegions {
		failed = append(failed, f.Region)
	}
	return rules.SuggestInput{
		Resources:      resources,
		KeyGroup:       inventory.CanonicalKey,
		Preferred:      preferredSpelling(),
		ToolVersion:    toolVersion(),
		AccountID:      snap.AccountID,
		RegionsRead:    snap.DoneRegions(),
		RegionsEnabled: len(snap.EnabledRegions),
		FailedRegions:  failed,
		GeneratedAt:    now,
	}
}

// preferredSpelling returns the full word of the abbreviation dictionary for
// a group, or "" (BR6.1: on a tie the full word wins).
func preferredSpelling() func(group string) string {
	full := map[string]bool{}
	for _, word := range inventory.Abbreviations() {
		full[word] = true
	}
	return func(group string) string {
		if full[group] {
			return group
		}
		return ""
	}
}

// toolVersion is the module version the binary was built from (for example
// v0.1.0 from "go install"), or "devel" for a local build.
func toolVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "devel"
}

// checkOutputPath rejects an --output that already exists, or whose folder
// does not, before anything else happens.
func checkOutputPath(path string) error {
	if _, err := os.Lstat(path); err == nil {
		return outputExists(path)
	}
	dir := filepath.Dir(path)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Subject: path,
			Detail: fmt.Sprintf("the folder of %s does not exist", path),
			Hint:   "create the folder first, or pass another path to --output"}
	}
	return nil
}

func outputExists(path string) error {
	return &errkind.Error{Kind: errkind.InvalidInput, Reason: errkind.ReasonUsage, Subject: path,
		Detail: fmt.Sprintf("%s already exists; rules suggest never overwrites a file", path),
		Hint:   "delete the file first, or pass another file name to --output"}
}

func outputFailed(path string, err error) error {
	return &errkind.Error{Kind: errkind.StorageFailed, Subject: path, Detail: "the draft could not be written",
		Hint: "check that the folder is writable and has free space", Cause: err}
}

// writeDraftFile writes the draft to a temporary file in the same folder
// and then gives it its name, so an interrupted run never leaves a partial
// draft (security design §3). The name is given with a hard link, which
// fails when the file exists, so a file created meanwhile is not
// overwritten either; a file system without hard links gets a rename after
// one more check. The temporary file is always removed.
func writeDraftFile(path string, data []byte) error {
	return writeDraftFileWith(path, data, os.Link, os.Rename)
}

// writeDraftFileWith is writeDraftFile with the link and the rename passed
// in, so the tests can reach the rename and the failure paths.
func writeDraftFileWith(path string, data []byte, link, rename func(oldname, newname string) error) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tag-refactor-draft-*.tmp")
	if err != nil {
		return outputFailed(path, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return outputFailed(path, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return outputFailed(path, err)
	}
	if err := tmp.Close(); err != nil {
		return outputFailed(path, err)
	}
	err = link(tmpName, path)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fs.ErrExist):
		return outputExists(path)
	}
	if _, statErr := os.Lstat(path); statErr == nil {
		return outputExists(path)
	}
	if err := rename(tmpName, path); err != nil {
		return outputFailed(path, err)
	}
	return nil
}
