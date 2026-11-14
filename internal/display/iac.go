package display

import (
	"strconv"
	"strings"

	"github.com/yucn6201g/tag-refactor/internal/iac"
)

// The output of iac (U8 BR6.1, NFR11.10). The difference, the counts and the
// lists go to standard output, the guidance and the warnings to standard error
// (contract C5). The lists are told apart by headings, and the lines of the
// difference by + and -, without colors too.

// iacCatalog holds the messages of iac, joined to the catalog at start.
var iacCatalog = map[string]message{
	"unit.change": {en: "%s changes", one: "%s change", ja: "%s件の変更"},
	"exit.iacDone": {en: "nothing to fix by hand",
		ja: "手で直すものはありません"},
	"exit.iacManual": {en: "some changes to fix by hand", ja: "手で直すものがあります"},
	"exit.iacFailed": {en: "the files were not all written", ja: "ファイルをすべては書き換えていません"},

	"iac.fileCan":        {en: "%s: %s can be reflected", ja: "%s: %sを反映できます"},
	"iac.fileDone":       {en: "%s: %s reflected", ja: "%s: %sを反映しました"},
	"iac.fileNotWritten": {en: "%s: %s not written", ja: "%s: %sを書き込んでいません"},
	"iac.already":        {en: "already reflected: %s", ja: "反映済みの変更: %s件"},
	"iac.assumedHead": {en: "Removals counted as reflected because the key is not in the source (%s; if a stack tag or default_tags adds the key, change it there):",
		ja: "ソースにないため反映済みとみなした削除(%s件。スタックのタグや default_tags で付けていれば、そちらを直してください):"},
	"iac.manualHead": {en: "To fix by hand (%s):", ja: "手で直すもの(%s件):"},
	"iac.noPlace":    {en: "(no place)", ja: "(場所なし)"},
	"iac.unmanaged": {en: "Not managed by IaC: %s (no CloudFormation tags and not in --tf-state)",
		ja: "IaC で管理されていないリソース: %s(CloudFormation のタグがなく、--tf-state にもありません)"},
	"iac.noWrite": {en: "No file was changed. Add --write to change the files.",
		ja: "ファイルは書き換えていません。書き換えるには --write を付けてください。"},
	"iac.review": {en: "Review the difference before you apply the IaC; iac runs neither terraform nor a stack update.",
		ja: "IaC を適用する前に差分をレビューしてください。iac は terraform もスタックの更新も実行しません。"},
	"iac.stackUpdate": {en: "A stack update then gives the same tags; the tags of a resource already applied do not change.",
		ja: "スタックを更新すると同じタグになります。apply 済みのリソースのタグは変わりません。"},
	"iac.tfStateHint": {en: "if Terraform manages some of these resources, pass the output of terraform show -json with --tf-state",
		ja: "Terraform で管理しているリソースがあれば、terraform show -json の出力を --tf-state で渡してください"},
	"iac.nothingReflected": {en: "Nothing was reflected in IaC", ja: "IaC に反映したものはありません"},
	"iac.interrupted": {en: "stopped by Ctrl+C: the files shown as reflected were written, the others were not",
		ja: "Ctrl+C で止めました。反映したと示したファイルは書き換え済みで、ほかは書き換えていません"},
	"iac.unreadableNote": {en: "unreadableFile looks like a fault of this tool: run again with --debug and report it",
		ja: "unreadableFile はこのツールの不具合の疑いがあります。--debug を付けて実行し、報告してください"},

	"iac.skipTooLarge":   {en: "%s left out: larger than 10 MB", ja: "%s個のファイルを外しました: 10 MB を超えています"},
	"iac.skipNotUTF8":    {en: "%s left out: not UTF-8", ja: "%s個のファイルを外しました: UTF-8 ではありません"},
	"iac.skipTooDeep":    {en: "%s left out: nested too deep or with too many parts in a template", ja: "%s個のファイルを外しました: 入れ子が深すぎるか、テンプレートの部品が多すぎます"},
	"iac.skipParseError": {en: "%s left out: could not be parsed", ja: "%s個のファイルを外しました: 解析できません"},
	"iac.skipUnreadable": {en: "%s left out: could not be read", ja: "%s個のファイルを外しました: 読めません"},
	"iac.skipFolders":    {en: "%s folders left out: deeper than 32 levels or not readable", ja: "%s個のフォルダーを外しました: 32 段より深いか、読めません"},

	"iac.detailAddress":    {en: " (%s)", ja: "(%s)"},
	"iac.detailCandidates": {en: " (candidates: %s)", ja: "(候補: %s)"},
	"iac.detailOtherType":  {en: " (another Type in: %s)", ja: "(種類が違うテンプレート: %s)"},
	"iac.detailSkipped":    {en: " (files left out: %s)", ja: "(外したファイル: %s)"},

	"reason.templateNotFound": {en: "no template in --dir has this logical ID and type (CDK and SAM generate their names; CDK is later work)",
		ja: "論理 ID と種類が合うテンプレートが --dir にありません(CDK や SAM の名前は生成されたものです。CDK は後の作業です)"},
	"reason.blockNotFound": {en: "no resource block with this address directly in --dir; pass the folder of the root module",
		ja: "このアドレスの resource ブロックが --dir の直下にありません。ルートモジュールのフォルダーを渡してください"},
	"reason.ambiguous":         {en: "more than one place could define it", ja: "定義の場所が2つ以上あり、1つに決められません"},
	"reason.inModule":          {en: "the resource is inside a Terraform module", ja: "Terraform のモジュールの中のリソースです"},
	"reason.multipleInstances": {en: "count or for_each makes the resource", ja: "count や for_each で作るリソースです"},
	"reason.tagsNotLiteral": {en: "the tags are an expression, a function, or YAML anchors",
		ja: "タグが式・関数、または YAML のアンカーで書かれています"},
	"reason.unsupportedTagShape": {en: "this way of writing tags takes only changes of keys and values",
		ja: "この書き方のタグには、キーと値の変更しか反映できません"},
	"reason.notLiteral": {en: "the value is an expression (!Ref, var.x, a block scalar and so on)",
		ja: "値が式(!Ref、var.x、ブロックスカラーなど)のため書き換えられません"},
	"reason.keyNotInSource": {en: "the key is not in the source; it may come from a stack tag or default_tags",
		ja: "キーがソースにありません。スタックのタグや default_tags で付いているかもしれません"},
	"reason.valueDiffers": {en: "the value in the source is neither the one before nor the one after the change; decide which is right",
		ja: "ソースの値が変更の前の値とも後の値とも違います。どちらが正しいかを決めてください"},
	"reason.keyAlreadyExists": {en: "the key is already in the source with another value; decide which is right",
		ja: "キーがソースに別の値ですでにあります。どちらが正しいかを決めてください"},
	"reason.noTagSite": {en: "the resource has no tags to copy the form of a new tag from",
		ja: "タグの書き場所がない、または空のため、足す形を決められません"},
	"reason.unreadableFile": {en: "the file did not pass the check of the edits, so it was left as it is",
		ja: "書き換えの確かめを通らなかったので、ファイルを書き換えていません"},

	"iac.needsPlan":    {en: "iac needs --plan with a plan saved by plan --out", ja: "iac には、plan --out で保存した変更案を --plan で渡してください"},
	"iac.needsPlanFix": {en: "for example: tag-refactor iac --plan plan.json --dir infra", ja: "例: tag-refactor iac --plan plan.json --dir infra"},
	"iac.needsDir":     {en: "iac needs --dir with the folder of the IaC sources", ja: "iac には、IaC のソースのフォルダーを --dir で渡してください"},
	"iac.notFolder":    {en: "--dir %s is not a folder", ja: "--dir %s はフォルダーではありません"},
	"iac.notFolderFix": {en: "pass an existing folder: the root of the templates, or the root module of Terraform",
		ja: "あるフォルダーを渡してください(テンプレートの上のフォルダーか、Terraform のルートモジュール)"},
	"iac.planTooLarge":    {en: "the plan %s is larger than 200 MB", ja: "変更案 %s が 200 MB を超えています"},
	"iac.planTooLargeFix": {en: "make smaller plans with --region", ja: "--region で分けて小さな変更案を作ってください"},
	"iac.tooManyFiles":    {en: "--dir holds more than %d candidate files", ja: "--dir の候補のファイルが %d 個を超えています"},
	"iac.tooManyFilesFix": {en: "pass a narrower folder", ja: "もっと狭いフォルダーを渡してください"},
	"iac.tooMuchData":     {en: "the candidate files of --dir are more than 500 MB in all", ja: "--dir の候補のファイルが合わせて 500 MB を超えています"},

	"iac.stateNotExist":    {en: "the file does not exist", ja: "ファイルがありません"},
	"iac.stateUnreadable":  {en: "the file could not be read", ja: "ファイルを読めません"},
	"iac.stateTooLarge":    {en: "the file is larger than 500 MB", ja: "ファイルが 500 MB を超えています"},
	"iac.stateTooLargeFix": {en: "pass the state of a smaller configuration", ja: "もっと小さな構成の state を渡してください"},
	"iac.stateUTF16":       {en: "the file is UTF-16", ja: "ファイルが UTF-16 です"},
	"iac.stateUTF16Fix": {en: "save it as UTF-8; in Windows PowerShell 5.1, pipe it to Out-File -Encoding utf8 instead of redirecting",
		ja: "UTF-8 で保存してください。Windows PowerShell 5.1 では、リダイレクトでなく Out-File -Encoding utf8 に渡してください"},
	"iac.stateNotOutput": {en: "this is not the output of terraform show -json", ja: "terraform show -json の出力ではありません"},
	"iac.statePlan":      {en: "this is the output for a plan file, not for the state", ja: "plan のファイルの出力で、state の出力ではありません"},
	"iac.stateFix": {en: "pass the output of terraform show -json (no arguments), run in the root module",
		ja: "ルートモジュールで実行した terraform show -json(引数なし)の出力を渡してください"},

	"iac.gitMissing":   {en: "--write needs git, and it was not found", ja: "--write には git が要りますが、見つかりません"},
	"iac.notWorkTree":  {en: "%s is not in a git working tree", ja: "%s は git の作業ツリーの中にありません"},
	"iac.gitFailed":    {en: "git %s failed; add --debug to see what it said", ja: "git %s が失敗しました。--debug を付けると内容が分かります"},
	"iac.noCommit":     {en: "the repository has no commit yet", ja: "リポジトリにまだコミットがありません"},
	"iac.notTracked":   {en: "%s is not tracked by git (new or ignored)", ja: "%s は git で追跡されていません(新しいか、ignore されています)"},
	"iac.conflict":     {en: "%s is in the middle of a merge conflict", ja: "%s は衝突の途中です"},
	"iac.notRegular":   {en: "%s is not a regular file in git", ja: "%s は git で通常のファイルではありません"},
	"iac.notCommitted": {en: "%s has changes that are not committed", ja: "%s にコミットしていない変更があります"},
	"iac.gitFix": {en: "nothing was written: commit the files first, or run without --write to see the difference only",
		ja: "何も書き換えていません。先にコミットするか、差分だけを見るには --write を外してください"},
	"iac.changedSinceRead":    {en: "%s changed after it was read", ja: "%s は読んだあとに変わりました"},
	"iac.changedSinceReadFix": {en: "nothing was written: run iac again", ja: "何も書き換えていません。もう一度 iac を実行してください"},
	"iac.readOnly":            {en: "%s is read-only", ja: "%s は読み取り専用です"},
	"iac.readOnlyFix": {en: "nothing was written: make it writable, or run without --write to see the difference only",
		ja: "何も書き換えていません。書き込めるようにするか、差分だけを見るには --write を外してください"},
	"iac.writeFailed": {en: "%s could not be written", ja: "%s を書き換えられませんでした"},
	"iac.writeFailedFix": {en: "the files shown as reflected were written; git can take them back",
		ja: "反映したと示したファイルは書き換え済みです。git で戻せます"},
}

func init() {
	for id, m := range iacCatalog {
		catalog[id] = m
	}
}

// The identifiers of the exit code line of iac.
const (
	ExitMeaningIacDone   = "exit.iacDone"
	ExitMeaningIacManual = "exit.iacManual"
	ExitMeaningIacFailed = "exit.iacFailed"
)

// sanitizeDiffLine makes a line of a difference safe for the terminal. A tab and
// a backslash stay: they are ordinary in the files, and the line is meant to
// show the file (NFR8.6).
func sanitizeDiffLine(line string) string {
	parts := strings.Split(line, "\t")
	for i, part := range parts {
		parts[i] = strings.ReplaceAll(Sanitize(part), `\\`, `\`)
	}
	return strings.Join(parts, "\t")
}

// writeDiff prints a difference with the color of each kind of line.
func (r *results) writeDiff(diff string) {
	for _, line := range strings.Split(diff, "\n") {
		st := styleNone
		switch {
		case strings.HasPrefix(line, "--- a/"), strings.HasPrefix(line, "+++ b/"), strings.HasPrefix(line, "@@ "):
		case strings.HasPrefix(line, "+"):
			st = styleAdd
		case strings.HasPrefix(line, "-"):
			st = styleRemove
		}
		r.text(st, sanitizeDiffLine(line))
	}
}

// IacResult prints the result of iac (BR6.1).
func (p *Printer) IacResult(res *iac.Result) {
	p.locked(func() {
		p.endProgress()
		p.iacWarnings(res)
		r := p.newResults()
		defer p.flush(r)
		for i := range res.Files {
			r.writeDiff(res.Files[i].Diff)
			r.blank()
		}
		for i := range res.Files {
			r.line(styleNone, iacFileMessage(res, &res.Files[i]), res.Files[i].File, rawString(r.count("unit.change", res.Files[i].Reflected)))
		}
		if res.AlreadyReflected > 0 {
			r.line(styleNone, "iac.already", FormatCount(res.AlreadyReflected))
		}
		r.iacLists(res)
		p.iacGuidance(res)
	})
}

func iacFileMessage(res *iac.Result, f *iac.FileResult) string {
	switch {
	case f.Written:
		return "iac.fileDone"
	case res.WriteRequested:
		return "iac.fileNotWritten"
	}
	return "iac.fileCan"
}

// iacLists prints the removals counted as reflected, the list to fix by hand and
// the resources in no IaC.
func (r *results) iacLists(res *iac.Result) {
	if n := len(res.AssumedRemovals); n > 0 {
		r.blank()
		r.line(styleChange, "iac.assumedHead", FormatCount(n))
		for i := range res.AssumedRemovals {
			m := &res.AssumedRemovals[i]
			r.text(styleNone, "  "+Sanitize(m.ARN)+"  "+Sanitize(m.Key)+"  "+Sanitize(m.File))
		}
	}
	if n := len(res.Manual); n > 0 {
		r.blank()
		r.line(styleChange, "iac.manualHead", FormatCount(n))
		for i := range res.Manual {
			r.text(styleNone, r.iacManualLine(&res.Manual[i]))
		}
	}
	if res.Unmanaged > 0 {
		r.blank()
		r.line(styleNone, "iac.unmanaged", rawString(r.count("unit.resource", res.Unmanaged)))
	}
}

// iacManualLine writes one item: the ARN, the key, the place and the reason.
func (r *results) iacManualLine(m *iac.ManualItem) string {
	at := r.tr("iac.noPlace")
	if m.File != "" {
		at = Sanitize(m.File)
		if m.Line > 0 {
			at += ":" + strconv.Itoa(m.Line)
		}
	}
	reason := r.tr("reason." + string(m.Reason))
	switch m.DetailKind {
	case iac.DetailAddress, iac.DetailAddresses:
		reason += r.tr("iac.detailAddress", Sanitize(m.Detail))
	case iac.DetailCandidates:
		reason += r.tr("iac.detailCandidates", Sanitize(m.Detail))
	case iac.DetailOtherType:
		reason += r.tr("iac.detailOtherType", Sanitize(m.Detail))
	case iac.DetailSkipped:
		reason += r.tr("iac.detailSkipped", Sanitize(m.Detail))
	case iac.DetailTool:
		reason += r.tr("iac.detailAddress", Sanitize(m.Detail))
	}
	return "  " + Sanitize(m.ARN) + "  " + Sanitize(m.Key) + "  " + at + "  " + reason
}

// iacWarnings prints the files and folders left out: how many on one line for
// each reason, and with --debug their names (BR3.1).
func (p *Printer) iacWarnings(res *iac.Result) {
	counts := map[iac.SkipReason]int{}
	for _, s := range res.SkippedFiles {
		counts[s.Reason]++
	}
	for _, row := range []struct {
		reason iac.SkipReason
		id     string
	}{
		{iac.SkipTooLarge, "iac.skipTooLarge"},
		{iac.SkipNotUTF8, "iac.skipNotUTF8"},
		{iac.SkipTooDeep, "iac.skipTooDeep"},
		{iac.SkipParseError, "iac.skipParseError"},
		{iac.SkipUnreadable, "iac.skipUnreadable"},
	} {
		if n := counts[row.reason]; n > 0 {
			p.say(styleChange, "stderr.warning", rawString(p.tr(row.id, p.iacFiles(n))))
		}
	}
	if n := len(res.SkippedFolders); n > 0 {
		p.say(styleChange, "stderr.warning", rawString(p.tr("iac.skipFolders", FormatCount(n))))
	}
	if !p.debug {
		return
	}
	for _, s := range res.SkippedFiles {
		p.sayText(styleNone, "debug: "+iacSkipLine(s))
	}
	for _, f := range res.SkippedFolders {
		p.sayText(styleNone, "debug: "+Sanitize(f))
	}
}

// iacFiles writes how many files, in the words of the message around it.
func (p *Printer) iacFiles(n int) string {
	if p.lang == Japanese {
		return FormatCount(n)
	}
	if n == 1 {
		return "1 file"
	}
	return FormatCount(n) + " files"
}

// iacSkipLine writes one file left out for --debug: nothing of its content, and
// nothing of the text of a parser (NFR8.27).
func iacSkipLine(s iac.SkippedFile) string {
	line := Sanitize(s.File) + ": " + string(s.Reason)
	if s.Reason == iac.SkipParseError {
		line += " " + Sanitize(s.Kind) + " line " + strconv.Itoa(s.Line)
		if s.Column > 0 {
			line += " column " + strconv.Itoa(s.Column)
		}
	}
	return line
}

// iacGuidance prints the messages after the result (BR6.1). The caller holds the
// lock.
func (p *Printer) iacGuidance(res *iac.Result) {
	if p.debug {
		for _, line := range res.Debug {
			p.sayText(styleNone, "debug: "+Sanitize(line))
		}
	}
	if !res.HasTFState && res.Unmanaged > 0 {
		p.say(styleNone, "iac.tfStateHint")
	}
	if res.Reflected == 0 && res.AlreadyReflected == 0 && res.Unmanaged > 0 {
		p.say(styleChange, "stderr.warning", rawString(p.tr("iac.nothingReflected")))
	}
	for i := range res.Manual {
		if res.Manual[i].Reason == iac.ReasonUnreadableFile {
			p.say(styleImpact, "iac.unreadableNote")
			break
		}
	}
	if res.Interrupted {
		p.say(styleImpact, "iac.interrupted")
	}
	if len(res.Files) > 0 && !res.WriteRequested {
		p.say(styleNone, "iac.noWrite")
	}
	written, cloudFormation, cdkSource := false, false, false
	for i := range res.Files {
		f := &res.Files[i]
		written = written || f.Written
		cloudFormation = cloudFormation || f.Written && f.Tool == iac.ToolCloudFormation
		cdkSource = cdkSource || f.Written && f.Tool == iac.ToolCDK
	}
	if written {
		p.say(styleNone, "iac.review")
	}
	if cloudFormation {
		p.say(styleNone, "iac.stackUpdate")
	}
	if cdkSource {
		p.say(styleNone, "iac.cdkWritten")
	}
}
