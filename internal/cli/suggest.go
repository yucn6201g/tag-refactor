package cli

import (
	"context"
	"io"
	"runtime/debug"
	"time"

	"github.com/spf13/cobra"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
	"github.com/yucn6201g/tag-refactor/internal/fileout"
	"github.com/yucn6201g/tag-refactor/internal/inventory"
	"github.com/yucn6201g/tag-refactor/internal/rules"
)

// suggestCommandName is the command of U3 workflow W2.
const suggestCommandName = "rules suggest"

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
			return ensureKind(a.runSuggest(cmd.Context(), profile, output, regions))
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "AWS profile to use (default: the standard AWS settings)")
	cmd.Flags().StringArrayVar(&regions, "region", nil, regionFlagHelp)
	cmd.Flags().StringVar(&output, "output", "",
		"write the draft to this file (UTF-8) instead of standard output; an existing file is never overwritten")
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
		if err := fileout.Prepare(output, draftPrefix); err != nil {
			return withOption(err, "--output")
		}
	}
	_, snap, err := collect(ctx, a, profile, regions, tagReadersOf)
	if err != nil {
		return err
	}
	draft := rules.Suggest(suggestInput(snap, a.deps.Now()))
	if output != "" {
		if err := a.saveFile(ctx, output, draftPrefix, draftFileLimit, func(w io.Writer) error {
			_, err := w.Write(draft.Text)
			return err
		}); err != nil {
			return withOption(err, "--output")
		}
		a.p.DraftWritten(output, len(draft.Keys))
	} else {
		a.p.WriteDraft(draft.Text)
	}
	a.p.DraftIncomplete(snap.FailedRegions)
	if draft.TooLarge {
		// U3 BR6.7: over 64 KiB even without value candidates.
		a.p.WarnText("stderr.draftTooLarge")
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

// The temporary file prefix and the size limit of the --output draft. A
// draft over 64 KiB is still written (U3 BR6.7), so the limit only guards
// against a draft no rule file reader would take.
const (
	draftPrefix    = ".tag-refactor-draft-"
	draftFileLimit = 64 << 20
)

// withOption names the option of an error about a file the command writes,
// so its message can tell which option to change.
func withOption(err error, option string) error {
	if e, ok := errkind.As(err); ok {
		switch e.Reason {
		case errkind.ReasonOutputExists, errkind.ReasonOutputFolderMissing, errkind.ReasonOutputFailed, errkind.ReasonOutputTooLarge:
			if e.Key == "" {
				e.Key = option
			}
		}
	}
	return err
}

// saveFile writes a file through internal/fileout: a temporary file in the
// folder of path, written by write and then given its name, so an
// interrupted run never leaves a partial file and an existing file is never
// overwritten (security design §3). The temporary file is removed however
// the command ends. A run interrupted while it writes stops at the next
// write.
func (a *app) saveFile(ctx context.Context, path, prefix string, limit int64, write func(io.Writer) error) error {
	pending, err := fileout.Create(path, prefix, limit)
	if err != nil {
		return err
	}
	defer pending.Discard()
	if a.deps.savingHook != nil {
		a.deps.savingHook()
	}
	if err := write(contextWriter{ctx: ctx, w: pending.Writer()}); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return pending.Commit()
}

// contextWriter stops writing once ctx is cancelled (Ctrl+C).
type contextWriter struct {
	ctx context.Context
	w   io.Writer
}

func (c contextWriter) Write(b []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.w.Write(b)
}
