package display

import (
	"time"
	"unicode/utf8"

	"github.com/yucn6201g/tag-refactor/internal/cdk"
	"github.com/yucn6201g/tag-refactor/internal/pullrequest"
	"github.com/yucn6201g/tag-refactor/internal/redact"
)

// The lines of the CDK apps and of the pull request of iac (v0.2.0 U1,
// observability design §1, contract C6), and the title and the body of the
// pull request (contract X2). The states go to standard error one line at a
// time; the URL of the pull request is the one line of standard output that
// --pr adds. U1 has the shortest form of each text; U6 completes the reasons.

var (
	_ cdk.ProgressSink         = (*Printer)(nil)
	_ pullrequest.ProgressSink = (*Printer)(nil)
)

// iacPRCatalog holds the messages of the CDK apps and the pull request.
var iacPRCatalog = map[string]message{
	"exit.iacPRFailed": {en: "the pull request was not opened", ja: "プルリクエストを作れませんでした"},

	"iac.cdkWaiting": {en: "CDK apps to synthesize: %s (waiting)", ja: "synth する CDK のアプリ: %s(待ち)"},
	"iac.cdkStarted": {en: "%s: synthesizing (%s)", ja: "%s: synth しています(%s)"},
	"iac.cdkDone":    {en: "%s: done (%s)", ja: "%s: 済み(%s)"},
	"iac.cdkFailed":  {en: "%s: failed (%s): %s", ja: "%s: 失敗(%s): %s"},
	"iac.cdkSkipped": {en: "%s: skipped: %s", ja: "%s: 外しました: %s"},
	"iac.seconds":    {en: "%ds", ja: "%d 秒"},
	"iac.cdkNeedsAWSFix": {en: "  Fix: run cdk synth once where AWS credentials are set, so that cdk.context.json keeps the values it looked up, then run iac again",
		ja: "  直し方: AWS の認証情報のある端末で一度 cdk synth を実行して cdk.context.json に値を保存してから、もう一度 iac を実行してください"},
	"iac.cdkWhyToolMissing":    {en: "a program is missing", ja: "プログラムが見つかりません"},
	"iac.cdkWhyNeedsAWS":       {en: "AWS credentials needed", ja: "AWS の認証情報が要る"},
	"iac.cdkWhyTimeout":        {en: "it did not end in time", ja: "時間内に終わりませんでした"},
	"iac.cdkWhySynthError":     {en: "cdk synth failed; add --debug to see why", ja: "cdk synth が失敗しました。--debug を付けると理由が分かります"},
	"iac.cdkWhyHelper":         {en: "its sources could not be read; add --debug to see why", ja: "ソースを読めませんでした。--debug を付けると理由が分かります"},
	"iac.cdkWhyV1":             {en: "CDK v1", ja: "CDK v1 です"},
	"iac.cdkWhyUnknownVersion": {en: "the version of CDK is not known", ja: "CDK の版が分かりません"},
	"iac.cdkWhyNotTypeScript":  {en: "neither TypeScript nor Python", ja: "TypeScript でも Python でもありません"},
	"iac.cdkWritten": {en: "A deploy of the CDK app then gives the same tags; see it first with cdk diff.",
		ja: "CDK のアプリをデプロイすると同じタグになります。先に cdk diff で確かめてください。"},

	"reason.cdkAppFailed":           {en: "the CDK app could not be synthesized or read", ja: "CDK のアプリを synth できないか、読めませんでした"},
	"reason.cdkUnsupportedLanguage": {en: "the CDK app is neither TypeScript nor Python", ja: "CDK のアプリが TypeScript でも Python でもありません"},
	"reason.cdkV1":                  {en: "the CDK app is CDK v1 or of a version not known", ja: "CDK のアプリが CDK v1 か、版が分かりません"},
	"reason.cdkNoSource":            {en: "the place that makes the resource was not found in the source", ja: "リソースを作る箇所がソースに見つかりません"},
	"reason.cdkScopeTag":            {en: "the tag is added to a scope with other resources that keep the key", ja: "このタグは、同じキーを持つほかのリソースも含む範囲に付けています"},
	"reason.cdkSharedValue":         {en: "the same string tags other resources that do not change the same way", ja: "同じ文字列が、同じように変わらないほかのリソースにも使われています"},
	"reason.cdkAmbiguous":           {en: "more than one place of the source could be it", ja: "ソースの候補が2つ以上あり、1つに決められません"},
	"reason.cdkExpression":          {en: "the key or the value is an expression", ja: "キーか値が式です"},
	"reason.cdkUnsupportedShape":    {en: "this way of writing the tag, or this kind of change, is not handled yet", ja: "このタグの書き方か、この種類の変更には、まだ対応していません"},

	"iac.prPlan":       {en: "Pull request: %s (%s), base %s, branch %s", ja: "プルリクエスト: %s(%s)、行き先 %s、ブランチ %s"},
	"iac.prChecked":    {en: "Pull request: checks passed", ja: "プルリクエスト: 確かめが済みました"},
	"iac.prStarted":    {en: "Pull request: %s started", ja: "プルリクエスト: %sを始めました"},
	"iac.prDone":       {en: "Pull request: %s done", ja: "プルリクエスト: %sが済みました"},
	"iac.prFailed":     {en: "Pull request: %s failed", ja: "プルリクエスト: %sが失敗しました"},
	"iac.prStepBranch": {en: "branch", ja: "ブランチの作成"},
	"iac.prStepCommit": {en: "commit", ja: "コミット"},
	"iac.prStepPush":   {en: "push", ja: "push"},
	"iac.prStepCreate": {en: "create", ja: "プルリクエストの作成"},
	"iac.prNothing": {en: "No file was written, so no pull request was opened.",
		ja: "書き換えたファイルが無いため、プルリクエストは作っていません。"},
	"iac.prNotOpened": {en: "The pull request was not opened: the files were not all written.",
		ja: "ファイルをすべては書き換えていないため、プルリクエストは作っていません。"},
	"iac.prStay": {en: "The working tree stays on the branch %s; the steps done are not undone (done: %s).",
		ja: "作業ツリーはブランチ %s のままです。済んだ段階は元に戻していません(済んだ段階: %s)。"},
	"iac.prBodyKept": {en: "The body of the pull request is in %s", ja: "プルリクエストの本文は %s にあります"},
	"iac.prNone":     {en: "none", ja: "なし"},

	"iac.prTitle": {en: "Reflect tag changes from tag-refactor (%s)", ja: "tag-refactor によるタグの変更の反映(%s)"},
	"iac.prIntro": {en: "This pull request was opened by `tag-refactor iac --pr`: it reflects the tag changes of a saved plan in the infrastructure code.",
		ja: "このプルリクエストは `tag-refactor iac --pr` が作りました。保存した変更案のタグの変更を、インフラのコードに反映したものです。"},
	"iac.prReview":       {en: "Review the difference before you merge and deploy it.", ja: "マージしてデプロイする前に、差分をレビューしてください。"},
	"iac.prChangedFiles": {en: "## Changed files (%d)", ja: "## 変更したファイル(%d)"},
	"iac.prKindCDKTS":    {en: "CDK, TypeScript", ja: "CDK、TypeScript"},
	"iac.prKindCFN":      {en: "CloudFormation", ja: "CloudFormation"},
	"iac.prKindTF":       {en: "Terraform", ja: "Terraform"},

	"iac.prNoGit":             {en: "--pr needs git, and it was not found", ja: "--pr には git が要りますが、見つかりません"},
	"iac.prNotRepo":           {en: "%s is not in a git working tree", ja: "%s は git の作業ツリーの中にありません"},
	"iac.prDetached":          {en: "HEAD is not on a branch, so there is no branch for the pull request to go to", ja: "HEAD がブランチを指していないため、プルリクエストの行き先がありません"},
	"iac.prInProgress":        {en: "a %s is not finished in the repository", ja: "リポジトリで %s が終わっていません"},
	"iac.prNoRemote":          {en: "the remote %s is not set", ja: "リモート %s がありません"},
	"iac.prNotGitHub":         {en: "the remote %s (%s) is not on github.com", ja: "リモート %s(%s)は github.com ではありません"},
	"iac.prBadBranch":         {en: "%s is not a valid branch name", ja: "%s はブランチの名前に使えません"},
	"iac.prBranchExists":      {en: "the branch %s exists already", ja: "ブランチ %s はすでにあります"},
	"iac.prBadTitle":          {en: "the title of the pull request must be one line of 1 to 256 characters", ja: "プルリクエストのタイトルは、1〜256 文字の1行にしてください"},
	"iac.prNoGh":              {en: "--pr needs gh (GitHub CLI), and it was not found", ja: "--pr には gh(GitHub CLI)が要りますが、見つかりません"},
	"iac.prNotLoggedIn":       {en: "gh is not logged in to %s", ja: "gh が %s にログインしていません"},
	"iac.prRemoteUnreachable": {en: "the remote %s could not be read; add --debug to see what git said", ja: "リモート %s を読めませんでした。--debug を付けると git の出力が分かります"},
	"iac.prBaseNotOnRemote":   {en: "the branch %s is not on the remote %s", ja: "ブランチ %s がリモート %s にありません"},
	"iac.prCheckFix":          {en: "nothing was changed: fix this and run iac --pr again", ja: "何も変えていません。これを直してから、もう一度 iac --pr を実行してください"},
	"iac.prStepFailed":        {en: "the %s step of the pull request failed", ja: "プルリクエストの %s の段階が失敗しました"},
	"iac.prStepTimeout":       {en: "the %s step of the pull request did not end in time", ja: "プルリクエストの %s の段階が時間内に終わりませんでした"},
	"iac.prStepFix":           {en: "add --debug to see what git or gh said; finish the remaining steps with git and gh", ja: "--debug を付けると git か gh の出力が分かります。残りの段階は git と gh で行ってください"},
}

func init() {
	for id, m := range iacPRCatalog {
		catalog[id] = m
	}
}

// ExitMeaningIacPRFailed is the meaning of exit code 1 when a step of the pull
// request failed.
const ExitMeaningIacPRFailed = "exit.iacPRFailed"

// cdkWhy holds the text of each reason of an app.
var cdkWhy = map[string]string{
	string(cdk.ReasonToolMissing):         "iac.cdkWhyToolMissing",
	string(cdk.ReasonNeedsAWS):            "iac.cdkWhyNeedsAWS",
	string(cdk.ReasonTimeout):             "iac.cdkWhyTimeout",
	string(cdk.ReasonSynthError):          "iac.cdkWhySynthError",
	"helper":                              "iac.cdkWhyHelper",
	string(cdk.ReasonCDKV1):               "iac.cdkWhyV1",
	string(cdk.ReasonUnknownVersion):      "iac.cdkWhyUnknownVersion",
	string(cdk.ReasonUnsupportedLanguage): "iac.cdkWhyNotTypeScript",
	string(cdk.ReasonToolTooOld):          "iac.cdkWhyToolTooOld",
}

func (p *Printer) why(reason string) rawString {
	if id, ok := cdkWhy[reason]; ok {
		return rawString(p.tr(id))
	}
	return rawString(Sanitize(reason))
}

// seconds writes an elapsed time in whole seconds.
func (p *Printer) seconds(d time.Duration) rawString {
	return rawString(p.tr("iac.seconds", int(d.Round(time.Second)/time.Second)))
}

func languageName(language string) string {
	switch language {
	case string(cdk.TypeScript):
		return "TypeScript"
	case string(cdk.Python):
		return "Python"
	}
	return language
}

// stepWord is the word of a step of the pull request.
func (p *Printer) stepWord(step string) rawString {
	switch step {
	case pullrequest.StepBranch:
		return rawString(p.tr("iac.prStepBranch"))
	case pullrequest.StepCommit:
		return rawString(p.tr("iac.prStepCommit"))
	case pullrequest.StepPush:
		return rawString(p.tr("iac.prStepPush"))
	case pullrequest.StepCreate:
		return rawString(p.tr("iac.prStepCreate"))
	}
	return rawString(Sanitize(step))
}

// Step implements pullrequest.ProgressSink.
func (p *Printer) Step(step, status, _ string) {
	p.locked(func() {
		switch {
		case step == pullrequest.StepChecked:
			p.say(styleNone, "iac.prChecked")
		case status == pullrequest.StatusStarted:
			p.say(styleNone, "iac.prStarted", p.stepWord(step))
		case status == pullrequest.StatusDone:
			p.say(styleNone, "iac.prDone", p.stepWord(step))
		default:
			p.say(styleRemove, "iac.prFailed", p.stepWord(step))
		}
	})
}

// PullRequestPlan shows where the pull request goes, before anything is
// written (security design §7): the remote without user information, the
// branch it goes to and the new branch. Nothing is asked.
func (p *Printer) PullRequestPlan(plan pullrequest.Plan) {
	p.locked(func() {
		p.say(styleNone, "iac.prPlan", plan.Remote, plan.Host+"/"+plan.RepoPath, plan.Base, plan.Branch)
	})
}

// PullRequestOpened prints the URL as the one line of standard output that
// --pr adds.
func (p *Printer) PullRequestOpened(url string) {
	p.locked(func() {
		r := p.newResults()
		defer p.flush(r)
		r.text(styleNone, Sanitize(url))
	})
}

// PullRequestNotOpened says why no pull request was made: nothing written, or
// a write that failed.
func (p *Printer) PullRequestNotOpened(writeFailed bool) {
	id := "iac.prNothing"
	if writeFailed {
		id = "iac.prNotOpened"
	}
	p.locked(func() { p.say(styleChange, id) })
}

// DebugLine prints one line for --debug, already redacted by its maker.
func (p *Printer) DebugLine(line string) {
	p.locked(func() {
		if p.debug {
			p.sayText(styleNone, "debug: "+Sanitize(line))
		}
	})
}

// maxTitle is the longest title of a pull request, in characters (X1).
const maxTitle = 256

// PullRequestTitle is the default title (contract X2), with the name of the
// plan file redacted (BR4.3). A long name keeps its start and ends with an
// ellipsis, so that the default title always fits in maxTitle characters
// (review 1): --pr is never stopped by a title the user did not write.
func PullRequestTitle(lang Lang, planFile string) string {
	name := redact.String(planFile)
	for {
		title := T(lang, "iac.prTitle", name)
		over := utf8.RuneCountInString(title) - maxTitle
		if over <= 0 {
			return title
		}
		runes := []rune(name)
		keep := max(len(runes)-over-1, 0)
		// Redacting again keeps the cut from showing part of a hidden shape.
		name = redact.String(string(runes[:keep])) + "…"
	}
}
