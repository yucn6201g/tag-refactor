package pullrequest

import (
	"runtime"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The resume commands of a pull request that stopped (v0.2.0 U2, security
// design §4, BR4.3, NFR1.9, NFR6.15): the commands that do, by hand, the steps
// from the one that failed up to the creation. Each line holds one command,
// for the shell the user is shown, with every value quoted for it. A value
// that cannot be quoted safely for that shell is not put in a command: the
// line then names its values, for the user to type. The body is never in a
// command; gh reads it from the body file.

// Shell is the shell the resume commands are written for.
type Shell int

// The shells of the resume commands.
const (
	ShellSh Shell = iota
	ShellPowerShell
)

// DefaultShell is PowerShell on Windows, unless MSYSTEM is set (Git Bash and
// MSYS2 set it), and sh elsewhere. getenv reads the environment; this package
// reads no variable by its name itself (TestNoTokenIsRead).
func DefaultShell(getenv func(string) string) Shell {
	if runtime.GOOS == "windows" && getenv("MSYSTEM") == "" {
		return ShellPowerShell
	}
	return ShellSh
}

// ResumeLine is one line of the resume commands. When Command is empty, the
// line is done by hand with the values of Manual.
type ResumeLine struct {
	// Step is branch, commit, push or create; commit has two lines, add and
	// commit.
	Step string
	// Command is one command, to paste into the shell as it is.
	Command string
	// Manual are the values of the line when it has no command.
	Manual []ManualValue
}

// ManualValue is a value of a line done by hand. Name says what it is: root,
// branch, file, title, remote, repo, base or bodyFile.
type ManualValue struct{ Name, Value string }

// part is a word of a command: a fixed word of the tool, or a value with its
// name.
type part struct {
	word        string
	name, value string
}

func fixed(w string) part       { return part{word: w} }
func named(name, v string) part { return part{name: name, value: v} }
func (t part) isValue() bool    { return t.name != "" }
func fileParts(fs []string) []part {
	out := make([]part, 0, len(fs))
	for _, f := range fs {
		out = append(out, named("file", f))
	}
	return out
}

// stepOrder is the order of the steps of Publish.
var stepOrder = []string{StepBranch, StepCommit, StepPush, StepCreate}

// ResumeCommands returns the lines that do the steps from out.FailedStep up to
// the creation, for sh. A step that did not fail gives nothing. title is the
// title of the pull request and the commit message, as Publish used it.
func ResumeCommands(plan Plan, title string, out PublishOutcome, sh Shell) []ResumeLine {
	from := slices.Index(stepOrder, out.FailedStep)
	if from < 0 {
		return nil
	}
	root := plan.RepoRoot
	branchRef := "refs/heads/" + plan.Branch
	var lines []ResumeLine
	for _, step := range stepOrder[from:] {
		switch step {
		case StepBranch:
			lines = append(lines, line(sh, step, fixed("git"), fixed("-C"), named("root", root), fixed("switch"), fixed("-c"),
				named("branch", plan.Branch)))
		case StepCommit:
			add := append([]part{fixed("git"), fixed("-C"), named("root", root), fixed("--literal-pathspecs"), fixed("add"), fixed("--")},
				fileParts(out.CommittedFiles)...)
			commit := append([]part{fixed("git"), fixed("-C"), named("root", root), fixed("--literal-pathspecs"), fixed("commit"),
				fixed("--only"), fixed("-m"), named("title", title), fixed("--")}, fileParts(out.CommittedFiles)...)
			lines = append(lines, line(sh, step, add...), line(sh, step, commit...))
		case StepPush:
			lines = append(lines, line(sh, step, fixed("git"), fixed("-C"), named("root", root), fixed("push"),
				named("remote", plan.Remote), named("branch", branchRef+":"+branchRef)))
		case StepCreate:
			create := []part{fixed("gh"), fixed("pr"), fixed("create"), fixed("--repo"), named("repo", plan.RepoPath),
				fixed("--base"), named("base", plan.Base), fixed("--head"), named("branch", plan.Branch),
				fixed("--title"), named("title", title)}
			if out.BodyFile == "" {
				// Without a body file, gh asks for the body in an editor: the
				// line is done by hand (the display says how).
				lines = append(lines, manualLine(step, create))
				continue
			}
			lines = append(lines, line(sh, step, append(create, fixed("--body-file"), named("bodyFile", out.BodyFile))...))
		}
	}
	return lines
}

// StepList is the step of the line of ListCommand; Publish has no such step.
const StepList = "list"

// ListCommand is the line that looks for the pull request of the branch, for
// when gh ended well without printing its URL (BR4.3, review N2): the values
// are quoted for sh like those of ResumeCommands, and a value that cannot be
// quoted makes the line one done by hand.
func ListCommand(plan Plan, sh Shell) ResumeLine {
	return line(sh, StepList, fixed("gh"), fixed("pr"), fixed("list"), fixed("--repo"), named("repo", plan.RepoPath),
		fixed("--head"), named("branch", plan.Branch))
}

// line quotes the values of the parts for sh and joins them; when a value
// cannot be quoted, the line is done by hand.
func line(sh Shell, step string, parts ...part) ResumeLine {
	quote := quoteSh
	if sh == ShellPowerShell {
		quote = quotePowerShell
	}
	words := make([]string, 0, len(parts))
	for _, t := range parts {
		if !t.isValue() {
			words = append(words, t.word)
			continue
		}
		q, ok := quote(t.value)
		if !ok {
			return manualLine(step, parts)
		}
		words = append(words, q)
	}
	return ResumeLine{Step: step, Command: strings.Join(words, " ")}
}

func manualLine(step string, parts []part) ResumeLine {
	l := ResumeLine{Step: step}
	for _, t := range parts {
		if t.isValue() {
			l.Manual = append(l.Manual, ManualValue{Name: t.name, Value: t.value})
		}
	}
	return l
}

// unprintable reports a value with a character that a terminal does not show
// as itself: the same classes display.Sanitize replaces (Cc, Cf, Zl, Zp), and
// bytes that are not UTF-8.
func unprintable(v string) bool {
	if !utf8.ValidString(v) {
		return true
	}
	for _, r := range v {
		if unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp) {
			return true
		}
	}
	return false
}

// quoteSh quotes v for sh with single quotes; a ' inside ends the quotes, is
// written as \' and opens them again. A value with a character that does not
// show is not quoted.
func quoteSh(v string) (string, bool) {
	if unprintable(v) {
		return "", false
	}
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'", true
}

// psQuotes are the characters PowerShell reads as a single quote: ' and the
// curly ones U+2018 to U+201B. Each is written twice inside a quoted string.
var psQuotes = strings.NewReplacer("'", "''", "‘", "‘‘", "’", "’’",
	"‚", "‚‚", "‛", "‛‛")

// quotePowerShell quotes v for PowerShell with single quotes. Windows
// PowerShell 5.1 passes a " and a \ at the end of a value with spaces to a
// program in a way that changes them, so such values are not quoted, nor is a
// value with a character that does not show.
func quotePowerShell(v string) (string, bool) {
	if unprintable(v) || strings.Contains(v, `"`) || strings.HasSuffix(v, `\`) && strings.ContainsFunc(v, unicode.IsSpace) {
		return "", false
	}
	return "'" + psQuotes.Replace(v) + "'", true
}
