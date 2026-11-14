package display

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yucn6201g/tag-refactor/internal/iac"
	"github.com/yucn6201g/tag-refactor/internal/pullrequest"
	"github.com/yucn6201g/tag-refactor/internal/redact"
)

// The body of the pull request and what iac --pr shows around it (v0.2.0 U2):
// the body with the changed files and the list to fix by hand (BR3.1 to
// BR3.3, contract X2), the shapes found in the body of the user (NFR1.6,
// NFR1.7), the redaction of the title (NFR1.8), and what stays after a step
// that failed or was stopped, with the resume commands (BR4.3, NFR1.9,
// NFR1.10).

// iacPRBodyCatalog holds the messages of U2.
var iacPRBodyCatalog = map[string]message{
	"iac.prKindCDKPy":  {en: "CDK, Python", ja: "CDK、Python"},
	"iac.prManualHead": {en: "## To fix by hand (%d)", ja: "## 手で直すもの(%d)"},
	"iac.prManualMore": {en: "- and %d more (see the output of this command for all)", ja: "- ほか %d 件(全部はこのコマンドの表示を見てください)"},

	"iac.prNeedsPr":       {en: "%s needs --pr", ja: "%s は --pr と一緒に使ってください"},
	"iac.prEmptyValue":    {en: "%s needs a value that is not empty", ja: "%s に空の値は使えません"},
	"iac.prEmptyValueFix": {en: "nothing was changed: give %s a value, or leave it out for the default", ja: "何も変えていません。%s に値を指定するか、既定にするならオプションを外してください"},
	"iac.prNeedsPrFix":    {en: "nothing was changed: add --pr, or leave out the options of the pull request", ja: "何も変えていません。--pr を付けるか、プルリクエストのオプションを外してください"},

	"iac.prBodyUnreadable": {en: "the file of --pr-body-file %s could not be read", ja: "--pr-body-file のファイル %s を読めません"},
	"iac.prBodyNotRegular": {en: "the file of --pr-body-file %s is not a regular file", ja: "--pr-body-file の %s はふつうのファイルではありません"},
	"iac.prBodyTooLarge":   {en: "the file of --pr-body-file %s is larger than 1 MiB", ja: "--pr-body-file のファイル %s が 1 MiB を超えています"},
	"iac.prBodyNotUTF8":    {en: "the file of --pr-body-file %s is not UTF-8 text", ja: "--pr-body-file のファイル %s が UTF-8 の文字ではありません"},
	"iac.prBodyTooLong":    {en: "the file of --pr-body-file %s has more than 60,000 characters", ja: "--pr-body-file のファイル %s が 60,000 文字を超えています"},
	"iac.prBodySecret": {en: "the file of --pr-body-file holds values that must not be made public (the lines above)",
		ja: "--pr-body-file のファイルに、公開してはいけない値があります(上の行)"},
	"iac.prBodySecretFix": {en: "nothing was changed: take the values out of the file, or write them so that they do not take their shape (see iac in the README), and run iac --pr again",
		ja: "何も変えていません。ファイルから値を除くか、形を崩して書いてから(README の iac の節)、もう一度 iac --pr を実行してください"},
	"iac.prBodyShape": {en: "--pr-body-file: %s on %s", ja: "--pr-body-file: %s(%s)"},
	"iac.prBodyNotice": {en: "Note: the file of --pr-body-file holds %s on %s; it goes into the pull request as it is",
		ja: "注意: --pr-body-file のファイルに %s があります(%s)。そのままプルリクエストに入ります"},
	"iac.prShapeGitHubToken": {en: "GitHub token", ja: "GitHub のトークン"},
	"iac.prShapeAccessKeyID": {en: "AWS access key ID", ja: "AWS のアクセスキーの ID"},
	"iac.prShapeURLPassword": {en: "URL with a password", ja: "パスワードつきの URL"},
	"iac.prShapeARN":         {en: "an ARN", ja: "ARN"},
	"iac.prShapeAccountID":   {en: "a 12-digit number", ja: "12 桁の数字"},
	"iac.prLine":             {en: "line %s", ja: "%s 行目"},
	"iac.prLines":            {en: "lines %s", ja: "%s 行目"},
	"iac.prLinesMore":        {en: "lines %s and %d more", ja: "%s 行目ほか %d か所"},
	"iac.prTitleRedacted": {en: "Some values of --pr-title were replaced with <redacted> in the title and the commit message",
		ja: "--pr-title の一部の値を、タイトルとコミットのメッセージで <redacted> に置き換えました"},
	"iac.prBranchOnRemote": {en: "the branch %s is already on the remote %s", ja: "ブランチ %s はリモート %s にすでにあります"},
	"iac.prBranchOnRemoteFix": {en: "nothing was changed: give another name with --pr-branch and run iac --pr again",
		ja: "何も変えていません。--pr-branch で別の名前を付けて、もう一度 iac --pr を実行してください"},
	"iac.prNothingToReflect": {en: "Nothing to reflect, so no pull request was opened.", ja: "反映するものはありません。プルリクエストは作っていません。"},

	"iac.prBodyTooLongError": {en: "the body of the pull request is longer than 65,536 characters even without the list to fix by hand",
		ja: "手で直す一覧を省いても、プルリクエストの本文が 65,536 文字を超えます"},
	"iac.prBodyTooLongFix": {en: "the files stay written and no branch was made: commit them, push the branch and open the pull request with gh pr create",
		ja: "書き換えたファイルはそのままで、ブランチは作っていません。ファイルをコミットして push し、gh pr create でプルリクエストを作ってください"},
	"iac.prBodyFileFailed": {en: "the body file of the pull request could not be written: %s", ja: "プルリクエストの本文のファイルを書けませんでした: %s"},
	"iac.prBodyFileFix": {en: "check that the folder of git can be written; the lines below finish the pull request by hand",
		ja: "git のフォルダーに書けるかを確かめてください。下の行で、プルリクエストを手で仕上げられます"},
	"iac.prFilesKept": {en: "The written files stay as they are; no branch was made.", ja: "書き換えたファイルはそのまま残っています。ブランチは作っていません。"},
	"iac.prResumeSh": {en: "To finish by hand, run these lines in sh, one at a time from the top:",
		ja: "手で続けるには、次の行を sh で、上から順に1行ずつ実行してください:"},
	"iac.prResumePowerShell": {en: "To finish by hand, run these lines in PowerShell, one at a time from the top:",
		ja: "手で続けるには、次の行を PowerShell で、上から順に1行ずつ実行してください:"},
	"iac.prResumeManual": {en: "  %s by hand: a value cannot be written safely for this shell; type the command with these values:",
		ja: "  %sは手で行ってください。このシェルで安全に書けない値があります。次の値で命令を入力してください:"},
	"iac.prResumeNoBody": {en: "  %s by hand: run gh pr create without --body-file to write the body in an editor, with these values:",
		ja: "  %sは手で行ってください。gh pr create を --body-file なしで動かすと、エディターで本文を書けます。値は次のとおりです:"},
	"iac.prValueRoot":     {en: "folder of the repository", ja: "リポジトリのフォルダー"},
	"iac.prValueBranch":   {en: "branch", ja: "ブランチ"},
	"iac.prValueFile":     {en: "file", ja: "ファイル"},
	"iac.prValueTitle":    {en: "title", ja: "タイトル"},
	"iac.prValueRemote":   {en: "remote", ja: "リモート"},
	"iac.prValueRepo":     {en: "repository", ja: "リポジトリ"},
	"iac.prValueBase":     {en: "base", ja: "行き先"},
	"iac.prValueBodyFile": {en: "body file", ja: "本文のファイル"},
	"iac.prBodyDelete":    {en: "  Delete it when you no longer need it.", ja: "  使い終えたら消してかまいません。"},
	"iac.prNoURL": {en: "gh ended well without the URL of the pull request; check it with this line in %s:",
		ja: "gh は成功しましたが、プルリクエストの URL を読み取れませんでした。次の行を %s で実行して確かめてください:"},
	"iac.prNoURLManual": {en: "gh ended well without the URL of the pull request; check it with gh pr list --repo and --head, with these values (they cannot be written safely for this shell):",
		ja: "gh は成功しましたが、プルリクエストの URL を読み取れませんでした。gh pr list の --repo と --head に次の値を入れて確かめてください(このシェルで安全に書けない値があります):"},
	"iac.prShellSh":         {en: "sh", ja: "sh"},
	"iac.prShellPowerShell": {en: "PowerShell", ja: "PowerShell"},
}

func init() {
	for id, m := range iacPRBodyCatalog {
		catalog[id] = m
	}
}

// MaxPullRequestBody is the longest body of a pull request, in characters
// (code points) (BR3.3).
const MaxPullRequestBody = 65536

// PullRequestBody is the body of the pull request (BR3.1 to BR3.3, contract
// X2): the body of the user, or a few lines that say where it comes from, then
// the changed files with their kind of IaC and the list to fix by hand, with
// the files relative to the top of the working tree (prefix is the place of
// --dir in it). Each part tag-refactor makes is redacted on its own; the body
// of the user is put first as it is. When the body is too long, the list to
// fix by hand loses lines from its end; when it is still too long without
// them, ok is false.
func PullRequestBody(lang Lang, res *iac.Result, prefix, userBody string) (body string, ok bool) {
	var head strings.Builder
	if userBody != "" {
		head.WriteString(userBody)
		if !strings.HasSuffix(userBody, "\n") {
			head.WriteByte('\n')
		}
		head.WriteByte('\n')
	} else {
		head.WriteString(redact.String(T(lang, "iac.prIntro") + "\n" + T(lang, "iac.prReview") + "\n\n"))
	}
	head.WriteString(changedFiles(lang, res, prefix))
	if len(res.Manual) == 0 {
		return head.String(), utf8.RuneCountInString(head.String()) <= MaxPullRequestBody
	}
	manualHead := "\n" + redact.String(T(lang, "iac.prManualHead", len(res.Manual))) + "\n\n"
	lines := make([]string, len(res.Manual))
	for i := range res.Manual {
		lines[i] = redact.String(manualLine(lang, &res.Manual[i], prefix)) + "\n"
	}
	keep, fits := fitManual(lang, utf8.RuneCountInString(head.String())+utf8.RuneCountInString(manualHead), lines)
	var b strings.Builder
	b.WriteString(head.String())
	b.WriteString(manualHead)
	for _, l := range lines[:keep] {
		b.WriteString(l)
	}
	if keep < len(lines) {
		b.WriteString(moreLine(lang, len(lines)-keep))
	}
	return b.String(), fits
}

// fitManual returns how many lines of the list to fix by hand fit after fixed
// characters, with the line of the ones left out, and whether the body fits at
// all.
func fitManual(lang Lang, fixed int, lines []string) (keep int, fits bool) {
	total := fixed
	for _, l := range lines {
		total += utf8.RuneCountInString(l)
	}
	if total <= MaxPullRequestBody {
		return len(lines), true
	}
	for keep = len(lines) - 1; keep >= 0; keep-- {
		total -= utf8.RuneCountInString(lines[keep])
		if total+utf8.RuneCountInString(moreLine(lang, len(lines)-keep)) <= MaxPullRequestBody {
			return keep, true
		}
	}
	return 0, false
}

func moreLine(lang Lang, n int) string {
	return redact.String(T(lang, "iac.prManualMore", n)) + "\n"
}

// changedFiles is the section of the written files, redacted.
func changedFiles(lang Lang, res *iac.Result, prefix string) string {
	var b strings.Builder
	written := 0
	for i := range res.Files {
		if res.Files[i].Written {
			written++
		}
	}
	b.WriteString(T(lang, "iac.prChangedFiles", written) + "\n\n")
	for i := range res.Files {
		f := &res.Files[i]
		if !f.Written {
			continue
		}
		fmt.Fprintf(&b, "- `%s` (%s)\n", markdownFile(path.Join(prefix, f.File)), kindOf(lang, f))
	}
	return redact.String(b.String())
}

func kindOf(lang Lang, f *iac.FileResult) string {
	switch {
	case f.Tool == iac.ToolCDK && f.Language == "python":
		return T(lang, "iac.prKindCDKPy")
	case f.Tool == iac.ToolCDK:
		return T(lang, "iac.prKindCDKTS")
	case f.Tool == iac.ToolTerraform:
		return T(lang, "iac.prKindTF")
	}
	return T(lang, "iac.prKindCFN")
}

// markdownFile keeps a file name inside its backticks.
func markdownFile(name string) string {
	return oneLine(strings.ReplaceAll(name, "`", "'"))
}

// oneLine replaces the characters that do not show with spaces, so that a
// value cannot break a line of the body.
func oneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp) {
			return ' '
		}
		return r
	}, s)
}

// manualLine is one line of the list to fix by hand (entities.md ManualLine):
// the place, the name of the resource, the key and the fixed text of the
// reason; a failed CDK app adds the program.
func manualLine(lang Lang, m *iac.ManualItem, prefix string) string {
	at := ""
	if m.File != "" {
		at = path.Join(prefix, m.File)
		if m.Line > 0 {
			at += ":" + strconv.Itoa(m.Line)
		}
		at = "`" + markdownFile(at) + "` "
	}
	reason := T(lang, "reason."+string(m.Reason))
	if m.DetailKind == iac.DetailTool {
		reason += T(lang, "iac.detailAddress", oneLine(m.Detail))
	}
	return "- " + at + oneLine(resourceName(m)) + ", " + oneLine(m.Key) + ": " + reason
}

// resourceName is the name of the resource on a line of the list to fix by
// hand (entities.md ManualLine.resourceName, review N3): the logical ID for
// CloudFormation and CDK, the address for Terraform. When neither is known,
// it falls back to what follows the last colon or slash of the ARN.
func resourceName(m *iac.ManualItem) string {
	switch {
	case m.LogicalID != "":
		return m.LogicalID
	case m.Address != "":
		return m.Address
	}
	if i := strings.LastIndexAny(m.ARN, ":/"); i >= 0 {
		return m.ARN[i+1:]
	}
	return m.ARN
}

// shapeNames hold the names of the shapes of redact.Scan.
var shapeNames = map[string]string{
	redact.KindGitHubToken: "iac.prShapeGitHubToken",
	redact.KindAccessKeyID: "iac.prShapeAccessKeyID",
	redact.KindURLPassword: "iac.prShapeURLPassword",
	redact.KindARN:         "iac.prShapeARN",
	redact.KindAccountID:   "iac.prShapeAccountID",
}

// maxLinesShown is how many line numbers of a shape are shown.
const maxLinesShown = 10

// linesText writes the lines of a shape: the first ten and how many more.
func linesText(lang Lang, lines []int) string {
	shown := make([]string, 0, min(len(lines), maxLinesShown))
	for _, n := range lines[:min(len(lines), maxLinesShown)] {
		shown = append(shown, strconv.Itoa(n))
	}
	list := join(lang, shown)
	switch {
	case len(lines) > maxLinesShown:
		return T(lang, "iac.prLinesMore", list, len(lines)-maxLinesShown)
	case len(lines) > 1:
		return T(lang, "iac.prLines", list)
	}
	return T(lang, "iac.prLine", list)
}

// BodyShapes prints one line for each shape of the body that stops the run
// (NFR1.6), before the error: its name and its lines, never its value.
func (p *Printer) BodyShapes(findings []redact.Finding) {
	p.locked(func() {
		for _, f := range findings {
			p.say(styleImpact, "iac.prBodyShape", rawString(p.tr(shapeNames[f.Kind])), rawString(linesText(p.lang, f.Lines)))
		}
	})
}

// BodyNotices prints one line for each shape of the body that is told about
// (NFR1.7).
func (p *Printer) BodyNotices(findings []redact.Finding) {
	p.locked(func() {
		for _, f := range findings {
			p.say(styleChange, "iac.prBodyNotice", rawString(p.tr(shapeNames[f.Kind])), rawString(linesText(p.lang, f.Lines)))
		}
	})
}

// TitleRedacted says that the redaction changed --pr-title (NFR1.8).
func (p *Printer) TitleRedacted() {
	p.locked(func() { p.say(styleChange, "iac.prTitleRedacted") })
}

// NothingToReflect says that nothing was written and nothing is left to fix by
// hand (BR4.1).
func (p *Printer) NothingToReflect() {
	p.locked(func() { p.say(styleNone, "iac.prNothingToReflect") })
}

// PullRequestWithoutURL says that gh ended well but printed no URL, with the
// line of pullrequest.ListCommand that looks for the pull request (BR4.3,
// review N2). The command is printed as it is, like the resume commands; the
// values of a line by hand are quoted so that every character shows.
func (p *Printer) PullRequestWithoutURL(l pullrequest.ResumeLine, sh pullrequest.Shell) {
	p.locked(func() {
		if l.Command == "" {
			p.say(styleChange, "iac.prNoURLManual")
			p.manualValues(l.Manual)
			return
		}
		shell := "iac.prShellSh"
		if sh == pullrequest.ShellPowerShell {
			shell = "iac.prShellPowerShell"
		}
		p.say(styleChange, "iac.prNoURL", rawString(p.tr(shell)))
		p.sayText(styleNone, "  "+l.Command)
	})
}

// valueNames hold the names of the values of the resume lines.
var valueNames = map[string]string{
	"root": "iac.prValueRoot", "branch": "iac.prValueBranch", "file": "iac.prValueFile", "title": "iac.prValueTitle",
	"remote": "iac.prValueRemote", "repo": "iac.prValueRepo", "base": "iac.prValueBase", "bodyFile": "iac.prValueBodyFile",
}

// PullRequestStopped tells, after a step that failed or was stopped, what was
// done and where the working tree stays, the resume commands for sh, and where
// the body file is (BR4.3, security design §5). The commands are printed as
// they are, without Sanitize, which would double their backslashes: a value
// that does not show is never in a command (pullrequest.ResumeCommands). The
// values of a line done by hand are quoted so that every character shows.
func (p *Printer) PullRequestStopped(out pullrequest.PublishOutcome, lines []pullrequest.ResumeLine, sh pullrequest.Shell) {
	p.locked(func() {
		if len(out.CompletedSteps) == 0 {
			p.say(styleNone, "iac.prFilesKept")
		} else {
			done := make([]string, 0, len(out.CompletedSteps))
			for _, s := range out.CompletedSteps {
				done = append(done, string(p.stepWord(s)))
			}
			p.say(styleNone, "iac.prStay", out.Branch, rawString(strings.Join(done, p.tr("list.separator"))))
		}
		if len(lines) > 0 {
			header := "iac.prResumeSh"
			if sh == pullrequest.ShellPowerShell {
				header = "iac.prResumePowerShell"
			}
			p.say(styleNone, header)
			for _, l := range lines {
				p.resumeLine(out, l)
			}
		}
		if out.BodyFile != "" {
			p.say(styleNone, "iac.prBodyKept", shownPath(out.BodyFile))
			p.say(styleNone, "iac.prBodyDelete")
		}
	})
}

// shownPath gives a path as it is, so that its backslashes are not doubled,
// when every character of it shows; otherwise quoted so that they all show.
func shownPath(path string) fmt.Stringer {
	if !utf8.ValidString(path) {
		return rawString(strconv.QuoteToGraphic(path))
	}
	for _, r := range path {
		if unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp) {
			return rawString(strconv.QuoteToGraphic(path))
		}
	}
	return rawString(path)
}

func (p *Printer) resumeLine(out pullrequest.PublishOutcome, l pullrequest.ResumeLine) {
	if l.Command != "" {
		p.sayText(styleNone, "  "+l.Command)
		return
	}
	id := "iac.prResumeManual"
	if l.Step == pullrequest.StepCreate && out.BodyFile == "" {
		id = "iac.prResumeNoBody"
	}
	p.say(styleNone, id, p.stepWord(l.Step))
	p.manualValues(l.Manual)
}

// manualValues prints the values of a line by hand, each quoted so that every
// character shows.
func (p *Printer) manualValues(values []pullrequest.ManualValue) {
	for _, v := range values {
		p.sayText(styleNone, "    "+p.tr(valueNames[v.Name])+": "+strconv.QuoteToGraphic(v.Value))
	}
}
