package display

import (
	"fmt"
	"strings"
)

// The text of every line people read, in English and Japanese (BR3.9,
// NFR10.7), except --help, which stays English (Q7). Each entry is a
// format of the fmt package: both languages take the same values in the
// same order, which a test checks. Numbers, ARNs, policy names, keys,
// values, option names and JSON are never translated. one is the English
// form for a count of 1, when it differs.
type message struct {
	en, one, ja string
}

// catalog holds every message by its identifier.
var catalog = map[string]message{
	// Units of counts: "%s" is the count with its commas.
	"unit.resource":     {en: "%s resources", one: "%s resource", ja: "%s リソース"},
	"unit.region":       {en: "%s regions", one: "%s region", ja: "%s リージョン"},
	"unit.key":          {en: "%s keys", one: "%s key", ja: "%s キー"},
	"unit.type":         {en: "%s types", one: "%s type", ja: "%s 種類"},
	"unit.variantGroup": {en: "%s variant groups", one: "%s variant group", ja: "表記ゆれ %s グループ"},
	"unit.requiredTag":  {en: "%s required tags", one: "%s required tag", ja: "必須タグ %s キー"},
	// Counts of the summary line of plan; Japanese puts the word first.
	"unit.conflict":        {en: "%s conflicts", one: "%s conflict", ja: "衝突 %s"},
	"unit.removeCandidate": {en: "%s remove candidates", one: "%s remove candidate", ja: "消す候補 %s"},
	"unit.item":            {en: "%s", ja: "%s 件"},
	"word.resources":       {en: "resources", one: "resource", ja: "リソース"},
	"list.separator":       {en: ", ", ja: "、"},
	"list.semicolon":       {en: "; ", ja: "、"},

	// Messages on standard error.
	"stderr.warning":      {en: "warning: %s", ja: "警告: %s"},
	"stderr.fileWarning":  {en: "warning: %s line %d: %s", ja: "警告: %s %d 行目: %s"},
	"stderr.account":      {en: "Account %s · region %s", ja: "アカウント %s · リージョン %s"},
	"stderr.error":        {en: "Error: %s", ja: "エラー: %s"},
	"stderr.fix":          {en: "  Fix: %s", ja: "  直し方: %s"},
	"stderr.done":         {en: "Done in %s · %s", ja: "完了 %s · %s"},
	"stderr.exitCode":     {en: "Exit code: %d (%s)", ja: "終了コード: %d(%s)"},
	"stderr.interrupted":  {en: "Interrupted. Nothing was saved.", ja: "中断しました。何も保存していません。"},
	"stderr.planHeader":   {en: "Account %s · %s · plan created %s (local time)", ja: "アカウント %s · %s · 変更案の作成 %s(現地時刻)"},
	"stderr.planSaved":    {en: "Saved plan to %s (%s, account %s).", ja: "変更案を %s に保存しました(%s、アカウント %s)。"},
	"stderr.savePlan":     {en: "Save this plan: tag-refactor plan --rules %s --out plan.json", ja: "変更案を保存する: tag-refactor plan --rules %s --out plan.json"},
	"stderr.draftWritten": {en: "Draft written to %s (%s)", ja: "下書きを %s に書きました(%s)"},
	"stderr.draftTooLarge": {en: "the draft is larger than 64 KiB and cannot be read as it is; remove keys you do not need",
		ja: "下書きが 64 KiB を超えているため、このままでは読めません。要らないキーを消してください"},
	"stderr.checked":         {en: "Checked: %s", ja: "確認した範囲: %s"},
	"stderr.checkedScan":     {en: "Checked: %s; %s", ja: "確認した範囲: %s。%s"},
	"stderr.checkedPolicies": {en: "Checked: %s, and customer managed IAM policies attached to a user, role or group", ja: "確認した範囲: %s、ユーザー・ロール・グループに付けたカスタマー管理の IAM ポリシー"},
	"stderr.checkedNoPolicy": {en: "Checked: %s; no IAM policies were read: nothing to change", ja: "確認した範囲: %s。変更がないため、IAM ポリシーは読んでいません"},

	// Exit code meanings.
	"exit.error":              {en: "error", ja: "エラー"},
	"exit.interrupted":        {en: "interrupted", ja: "中断"},
	"exit.resultsIncomplete":  {en: "results incomplete", ja: "結果が不完全"},
	"exit.impactIncomplete":   {en: "impact analysis incomplete", ja: "影響分析が不完全"},
	"exit.draftTooLarge":      {en: "draft larger than 64 KiB", ja: "下書きが 64 KiB を超えています"},
	"exit.draftReady":         {en: "draft ready", ja: "下書きができました"},
	"exit.issuesFound":        {en: "issues found", ja: "問題あり"},
	"exit.reviewNeeded":       {en: "review needed", ja: "要確認"},
	"exit.noIssues":           {en: "no issues", ja: "問題なし"},
	"exit.noChangesNeeded":    {en: "no changes needed", ja: "変更は不要"},
	"scope.tagged":            {en: "tagged resources in %d of %d regions (tagging API)", ja: "タグ付きのリソース(%d / %d リージョン、タグ付けの API)"},
	"scope.notLookedFor":      {en: "never-tagged resources were not looked for", ja: "一度もタグが付いていないリソースは探していません"},
	"scope.config":            {en: "AWS Config in %s (without the global IAM types)", ja: "AWS Config(%s。グローバルな IAM の種類を除く)"},
	"scope.explorer":          {en: "Resource Explorer in %s", ja: "Resource Explorer(%s)"},
	"scope.aggregator":        {en: "the Resource Explorer aggregator index for %s", ja: "Resource Explorer の集約インデックス(%s)"},
	"scope.notChecked":        {en: "never-tagged resources not checked", ja: "一度もタグが付いていないリソースは調べていません"},
	"scope.via":               {en: "never-tagged resources via %s", ja: "一度もタグが付いていないリソースは %s で調べました"},
	"scope.notCheckedIn":      {en: "%s; not checked in %s", ja: "%s。%s では調べていません"},
	"progress.waiting":        {en: "scan: waiting: %s", ja: "scan: 待機中: %s"},
	"progress.started":        {en: "scan: %s started", ja: "scan: %s 開始"},
	"progress.done":           {en: "scan: %s done (%s, elapsed %s)", ja: "scan: %s 完了(%s、経過 %s)"},
	"progress.failed":         {en: "scan: %s failed (%s, elapsed %s)", ja: "scan: %s 失敗(%s、経過 %s)"},
	"progress.discovering":    {en: "scan: discovering never-tagged resources", ja: "scan: 一度もタグが付いていないリソースを探しています"},
	"progress.discoveryDone":  {en: "scan: %s never-tagged resources: %s", ja: "scan: %s 一度もタグが付いていないリソース: %s"},
	"progress.tableHeader":    {en: "Account %s · %s", ja: "アカウント %s · %s"},
	"progress.tableFooter":    {en: "  %d/%d regions · elapsed %s", ja: "  %d/%d リージョン · 経過 %s"},
	"progress.tableDiscovery": {en: "  discovering never-tagged resources: %d/%d regions · elapsed %s", ja: "  一度もタグが付いていないリソースを探しています: %d/%d リージョン · 経過 %s"},
	"state.waiting":           {en: "[waiting]", ja: "[待機]"},
	"state.running":           {en: "[running]", ja: "[実行中]"},
	"state.done":              {en: "[done]", ja: "[完了]"},
	"state.failed":            {en: "[failed]", ja: "[失敗]"},

	// Results shared by scan and plan.
	"result.incomplete":      {en: "! Results are incomplete", ja: "! 結果が不完全です"},
	"result.failedRegion":    {en: "    %s: %s (%s)", ja: "    %s: %s(%s)"},
	"status.complete":        {en: "complete", ja: "完了"},
	"status.partial":         {en: "partial", ja: "一部"},
	"status.unavailable":     {en: "unavailable", ja: "使えません"},
	"status.failed":          {en: "failed", ja: "失敗"},
	"discovery.notCheckedIn": {en: "%s (not checked in %s)", ja: "%s(%s では調べていません)"},
	"discovery.withReasons":  {en: "%s (%s)", ja: "%s(%s)"},

	// scan in the table format (screen 2).
	"scan.noIssuesDiscovery": {en: "No issues found (untagged-resource discovery: %s)", ja: "問題は見つかりませんでした(一度もタグが付いていないリソースの検出: %s)"},
	"scan.noIssues":          {en: "No issues found", ja: "問題は見つかりませんでした"},
	"scan.variants":          {en: "Tag key variants (candidates; confirm them in your rule file)", ja: "同じ意味で書き方が違うタグキー(候補です。ルールファイルで確定してください)"},
	"scan.abbreviation":      {en: "  (abbreviation match)", ja: "  (略語の辞書による一致)"},
	"scan.requiredNoRules":   {en: "Required tags missing: not checked (no rule file)", ja: "必須タグの欠落: 調べていません(ルールファイルがありません)"},
	"scan.requiredNone":      {en: "Required tags missing (from %s): none", ja: "必須タグの欠落(%s から): なし"},
	"scan.required":          {en: "Required tags missing (from %s)", ja: "必須タグの欠落(%s から)"},
	"scan.notTaggableMore":   {en: "  (%s more on resource types that cannot be tagged)", ja: "  (ほかにタグを変更できない種類のリソースが %s)"},
	"scan.noUserTags":        {en: "Resources with no user tags: %s", ja: "利用者のタグがないリソース: %s"},
	"scan.discovery":         {en: "Untagged-resource discovery: %s", ja: "一度もタグが付いていないリソースの検出: %s"},
	"scan.lowerBound":        {en: "  Required tag counts are a lower bound.", ja: "  必須タグの件数は下限です。"},
	"scan.unsupported":       {en: "Resource types not supported for tag changes: %s, %s", ja: "タグを変更できないリソースの種類: %s、%s"},
	"scan.summary":           {en: "Summary: %s scanned in %d of %d regions · %s", ja: "要約: %sを調べました(%d / %d リージョン)· %s"},
	"scan.summaryMissing":    {en: "%s · %s missing", ja: "%s · %sの欠落"},

	// plan in the table and diff formats (screens 4 and 5).
	"plan.impactIncomplete": {en: "! Impact analysis is incomplete", ja: "! 影響分析が不完全です"},
	"plan.unreadList":       {en: "    IAM managed policies: %s", ja: "    IAM 管理ポリシー: %s"},
	"plan.unreadPolicy":     {en: "    IAM managed policy %s: %s", ja: "    IAM 管理ポリシー %s: %s"},
	"plan.noChanges":        {en: "No changes needed", ja: "変更は不要です"},
	"plan.summaryNothing":   {en: "Summary: %s scanned · 0 to change", ja: "要約: %sを調べました · 変更 0"},
	"plan.summaryNothingRC": {en: "Summary: %s scanned · 0 to change · %s", ja: "要約: %sを調べました · 変更 0 · %s"},
	"plan.changes":          {en: "Changes: %s", ja: "変更: %s"},
	"kind.renameKey":        {en: "rename key", ja: "キー名の変更"},
	"kind.changeValue":      {en: "value", ja: "値の変更"},
	"kind.addRequired":      {en: "add", ja: "追加"},
	"kind.removeKey":        {en: "remove", ja: "削除"},
	"plan.fromRules":        {en: "(from rules)", ja: "(ルールの値)"},
	"plan.countedOnce":      {en: "  (A resource with several changes is counted once in the total.)", ja: "  (複数の変更があるリソースも、総数では1件と数えます。)"},
	"plan.conflicts":        {en: "Conflicts (not changed; values differ): %s", ja: "衝突(値が違うため変更しません): %s"},
	"plan.unset":            {en: "Required tag value not set (not changed): %s", ja: "必須タグの値が未指定(変更しません): %s"},
	"plan.unsetReason":      {en: "  (no default or matching condition)", ja: "(default も当てはまる条件もありません)"},
	"plan.candidates":       {en: "Remove candidates (keys not in your rules; not changed): %s", ja: "消す候補(ルールにないキー。変更しません): %s"},
	"plan.candidate":        {en: "%s (%s)", ja: "%s(%s)"},
	"plan.iac":              {en: "Managed by IaC (changes may be reverted by the next deploy): %s", ja: "IaC で管理されています(次のデプロイで元に戻るおそれがあります): %s"},
	"plan.impact":           {en: "Impact (references that use the tags being changed)", ja: "影響(変更するタグを使っている参照)"},
	"plan.policy":           {en: "  %s IAM policy  %s", ja: "  %s IAM ポリシー  %s"},
	"plan.statementValue":   {en: "      Statement \"%s\"  Condition %s %s = \"%s\"", ja: "      ステートメント \"%s\"  条件 %s %s = \"%s\""},
	"plan.statement":        {en: "      Statement \"%s\"  Condition %s %s", ja: "      ステートメント \"%s\"  条件 %s %s"},
	// A plan read from a file has no condition operator (contract C2).
	"plan.statementValueNoOp": {en: "      Statement \"%s\"  Condition %s = \"%s\"", ja: "      ステートメント \"%s\"  条件 %s = \"%s\""},
	"plan.statementNoOp":      {en: "      Statement \"%s\"  Condition %s", ja: "      ステートメント \"%s\"  条件 %s"},
	"plan.cannotDecide":       {en: "      cannot decide (policy variable)", ja: "      判定できません(ポリシー変数)"},
	"plan.affectedBy":         {en: "      affected by: %s", ja: "      原因: %s"},
	"cause.renameKey":         {en: "rename key %s -> %s", ja: "キー名の変更 %s -> %s"},
	"cause.changeValue":       {en: "value %s: %s -> %s", ja: "値の変更 %s: %s -> %s"},
	"cause.addRequired":       {en: "add %s = %s", ja: "追加 %s = %s"},
	"cause.removeKey":         {en: "remove %s", ja: "削除 %s"},
	"plan.summary": {en: "Summary: %s to change · %s · %s affected · %s undecided · %s value not set · %s · %s IaC-managed",
		ja: "要約: 変更 %s · %s · 影響あり %s · 判定できない %s · 値が未指定 %s · %s · IaC %s"},
	"diff.conflict":   {en: "! conflict: %s", ja: "! 衝突: %s"},
	"diff.unset":      {en: "! required tag value not set: %s", ja: "! 必須タグの値が未指定: %s"},
	"diff.iac":        {en: "[IaC: CloudFormation stack \"%s\"]", ja: "[IaC: CloudFormation スタック \"%s\"]"},
	"diff.iacOther":   {en: "[IaC: %s]", ja: "[IaC: %s]"},
	"diff.changes":    {en: "Changes: %s (rename key %s · value %s · add %s · remove %s)", ja: "変更: %s(キー名の変更 %s · 値の変更 %s · 追加 %s · 削除 %s)"},
	"md.target":       {en: "## Target", ja: "## 対象"},
	"md.targetLine":   {en: "Account %s · %s · %s", ja: "アカウント %s · %s · %s"},
	"md.scanTarget":   {en: "Account %s · %s of %s regions · %s", ja: "アカウント %s · %s / %s リージョン · %s"},
	"md.incomplete":   {en: "## ! Results are incomplete", ja: "## ! 結果が不完全です"},
	"md.regionHead":   {en: "| Region | Reason |", ja: "| リージョン | 理由 |"},
	"md.impactInc":    {en: "## ! Impact analysis is incomplete", ja: "## ! 影響分析が不完全です"},
	"md.unreadHead":   {en: "| Policy | Reason |", ja: "| ポリシー | 理由 |"},
	"md.allPolicies":  {en: "(the list of IAM managed policies)", ja: "(IAM 管理ポリシーの一覧)"},
	"md.changes":      {en: "## Changes (%s)", ja: "## 変更(%s)"},
	"md.changesHead":  {en: "| Kind | Change | Resources |", ja: "| 種類 | 内容 | リソース |"},
	"md.byResource":   {en: "<details><summary>Changes by resource (%s)</summary>", ja: "<details><summary>リソースごとの変更(%s 件)</summary>"},
	"md.resourceHead": {en: "| ARN | Changes | IaC | Impact |", ja: "| ARN | 変更 | IaC | 影響 |"},
	"md.conflicts":    {en: "## Conflicts (%s)", ja: "## 衝突(%s)"},
	"md.conflictHead": {en: "| ARN | Tags |", ja: "| ARN | タグ |"},
	"md.unset":        {en: "## Required tag value not set (%s)", ja: "## 必須タグの値が未指定(%s)"},
	"md.unsetNote":    {en: "No default or matching condition; not changed.", ja: "default も当てはまる条件もないため、変更しません。"},
	"md.keyHead":      {en: "| Key | Resources |", ja: "| キー | リソース |"},
	"md.unsetDetails": {en: "<details><summary>Resources with a required tag value not set (%s)</summary>", ja: "<details><summary>値が未指定のリソース(%s 件)</summary>"},
	"md.keyARNHead":   {en: "| Key | ARN |", ja: "| キー | ARN |"},
	"md.candidates":   {en: "## Remove candidates (%s)", ja: "## 消す候補(%s)"},
	"md.candNote":     {en: "Keys not in your rules; not changed.", ja: "ルールにないキーです。変更しません。"},
	"md.iac":          {en: "## Managed by IaC (%s)", ja: "## IaC で管理されています(%s)"},
	"md.iacNote":      {en: "Changes may be reverted by the next deploy.", ja: "次のデプロイで元に戻るおそれがあります。"},
	"md.impact":       {en: "## Impact", ja: "## 影響"},
	"md.impactHead":   {en: "| | Policy | Statement | Condition | Caused by |", ja: "| | ポリシー | ステートメント | 条件 | 原因 |"},
	"md.undecided":    {en: "cannot decide (policy variable)", ja: "判定できません(ポリシー変数)"},
	"md.summary":      {en: "## Summary", ja: "## 要約"},
	"md.variants":     {en: "## Tag key variants", ja: "## 同じ意味で書き方が違うタグキー"},
	"md.variantsNote": {en: "Candidates; confirm them in your rule file.", ja: "候補です。ルールファイルで確定してください。"},
	"md.variantHead":  {en: "| Group | Spelling | Resources |", ja: "| グループ | 書き方 | リソース |"},
	"md.abbreviation": {en: "%s (abbreviation match)", ja: "%s(略語の辞書による一致)"},
	"md.required":     {en: "## Required tags missing", ja: "## 必須タグの欠落"},
	"md.requiredFrom": {en: "From %s.", ja: "%s から調べました。"},
	"md.noRules":      {en: "Not checked (no rule file).", ja: "調べていません(ルールファイルがありません)。"},
	"md.none":         {en: "None.", ja: "なし。"},
	"md.notTaggable":  {en: "| Key | Resources | Resources that cannot be tagged |", ja: "| キー | リソース | タグを変更できないリソース |"},
	"md.neverTagged":  {en: "## Never-tagged resources", ja: "## 一度もタグが付いていないリソース"},
	"md.unsupported":  {en: "## Resource types not supported for tag changes", ja: "## タグを変更できないリソースの種類"},
	"md.typeHead":     {en: "| Type | Resources |", ja: "| 種類 | リソース |"},

	// The two-line errors of the command line (cli). Values are tool-made
	// or sanitized when printed.
	"err.help":              {en: `run "tag-refactor --help" to see the commands and options`, ja: `"tag-refactor --help" でコマンドとオプションを確かめてください`},
	"err.permissions":       {en: "docs/iam/readonly-policy.json lists the permissions tag-refactor needs", ja: "tag-refactor に要る権限は docs/iam/readonly-policy.json にあります"},
	"err.supported":         {en: "this version has scan, plan and rules suggest, plus --debug and --lang", ja: "この版で使えるのは scan、plan、rules suggest と、--debug、--lang です"},
	"err.notAvailable":      {en: "%s is not available in this version", ja: "%s はこの版では使えません"},
	"err.inRegion":          {en: " (region %s)", ja: "(リージョン %s)"},
	"err.accessDenied":      {en: "access denied for %s%s", ja: "%s の権限がありません%s"},
	"err.accessDeniedFix":   {en: "allow %s for the identity you use; %s", ja: "使っている ID に %s を許可してください。%s"},
	"err.timeout":           {en: "AWS did not answer %s in time%s", ja: "AWS が %s に時間内に答えませんでした%s"},
	"err.timeoutFix":        {en: "try again later; add --debug to see the time of each call", ja: "時間をおいてやり直してください。--debug を付けると、呼び出しごとの時間が分かります"},
	"err.transient":         {en: "AWS kept failing on %s after retries%s", ja: "AWS の %s が、再試行のあとも失敗しました%s"},
	"err.transientFix":      {en: "try again later; add --debug to see each call and its retries", ja: "時間をおいてやり直してください。--debug を付けると、呼び出しと再試行が分かります"},
	"err.profileNotFound":   {en: "the AWS profile %q was not found", ja: "AWS のプロファイル %q が見つかりません"},
	"err.profileFix":        {en: `check the profile name in your AWS config file, or create it with "aws configure --profile <name>"`, ja: `AWS の設定ファイルのプロファイル名を確かめるか、"aws configure --profile <名前>" で作ってください`},
	"err.regionNotSet":      {en: "no AWS region is set", ja: "AWS のリージョンが設定されていません"},
	"err.regionNotSetFix":   {en: "pass --region (for example --region us-east-1) or set AWS_REGION", ja: "--region を付けるか(例 --region us-east-1)、AWS_REGION を設定してください"},
	"err.regionInvalid":     {en: "%q is not an AWS region name", ja: "%q は AWS のリージョン名ではありません"},
	"err.regionInvalidFix":  {en: "pass a region name such as us-east-1 with --region", ja: "--region に us-east-1 のようなリージョン名を渡してください"},
	"err.regionNA":          {en: "the region %s is not available for this account", ja: "リージョン %s はこのアカウントでは使えません"},
	"err.regionNAFix":       {en: "pass an enabled region with --region or set AWS_REGION", ja: "--region に有効なリージョンを渡すか、AWS_REGION を設定してください"},
	"err.fileLine":          {en: "%s line %d: %s", ja: "%s %d 行目: %s"},
	"err.file":              {en: "%s: %s", ja: "%s: %s"},
	"err.checkRules":        {en: "check the rule file", ja: "ルールファイルを確かめてください"},
	"err.ssoExpired":        {en: "the AWS SSO session has expired or is not signed in", ja: "AWS SSO のセッションが切れているか、サインインしていません"},
	"err.ssoExpiredFix":     {en: "run %s and try again", ja: "%s を実行してから、もう一度実行してください"},
	"err.credExpired":       {en: "the AWS credentials have expired (%s)", ja: "AWS の認証情報の期限が切れています(%s)"},
	"err.credExpiredFix":    {en: "sign in again, for example with %s, or refresh the temporary credentials", ja: "%s などでサインインし直すか、一時的な認証情報を更新してください"},
	"err.credRejected":      {en: "AWS rejected the credentials (%s)", ja: "AWS が認証情報を受け付けませんでした(%s)"},
	"err.credRejectedFix":   {en: "check the access key of the profile or the environment, or sign in again", ja: "プロファイルか環境変数のアクセスキーを確かめるか、サインインし直してください"},
	"err.credTimeout":       {en: "AWS credentials could not be obtained in time", ja: "AWS の認証情報を時間内に取得できませんでした"},
	"err.credTimeoutFix":    {en: "check the profile settings and the network, or pass --profile", ja: "プロファイルの設定とネットワークを確かめるか、--profile を付けてください"},
	"err.credMissing":       {en: "no AWS credentials were found", ja: "AWS の認証情報が見つかりません"},
	"err.credMissingFix":    {en: `run "aws configure" or "aws sso login", set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY, or pass --profile`, ja: `"aws configure" か "aws sso login" を実行するか、AWS_ACCESS_KEY_ID と AWS_SECRET_ACCESS_KEY を設定するか、--profile を付けてください`},
	"err.internal":          {en: "an unexpected error occurred in %s%s", ja: "%s で想定外のエラーが起きました%s"},
	"err.internalDetail":    {en: "an unexpected error occurred in %s%s: %s", ja: "%s で想定外のエラーが起きました%s: %s"},
	"err.internalFix":       {en: "run the command again with --debug to see the failing call", ja: "--debug を付けてもう一度実行すると、失敗した呼び出しが分かります"},
	"err.outExists":         {en: "%s already exists; tag-refactor never overwrites a file", ja: "%s は既にあります。tag-refactor はファイルを上書きしません"},
	"err.outExistsFix":      {en: "delete the file first, or pass another file name to %s", ja: "先にファイルを消すか、%s に別のファイル名を渡してください"},
	"err.outFolder":         {en: "the folder of %s does not exist", ja: "%s のフォルダがありません"},
	"err.outFolderFix":      {en: "create the folder first, or pass another path to %s", ja: "先にフォルダを作るか、%s に別のパスを渡してください"},
	"err.outFailed":         {en: "%s could not be written", ja: "%s に書き込めませんでした"},
	"err.outFailedFix":      {en: "check that the folder is writable and has free space", ja: "フォルダに書き込めることと、空きがあることを確かめてください"},
	"err.outTooLarge":       {en: "the plan is larger than 256 MiB, so %s was not saved", ja: "変更案が 256 MiB を超えるため、%s は保存しませんでした"},
	"err.outTooLargeFix":    {en: "create smaller plans with --region, for example one region at a time", ja: "--region で分けて、たとえば1つのリージョンずつ作ってください"},
	"err.badFormat":         {en: "--format %q is not one of %s", ja: "--format %q は %s のどれでもありません"},
	"err.badFormatFix":      {en: "pass one of %s to --format", ja: "--format には %s のどれかを渡してください"},
	"err.badLang":           {en: "--lang %q is not en or ja", ja: "--lang %q は en でも ja でもありません"},
	"err.badLangFix":        {en: "pass en (English) or ja (Japanese) to --lang", ja: "--lang には en(英語)か ja(日本語)を渡してください"},
	"err.planNeedsRules":    {en: "plan needs a rule file", ja: "plan にはルールファイルが要ります"},
	"err.planNeedsRulesFix": {en: "pass it with --rules, for example: tag-refactor plan --rules tagrules.yaml", ja: "--rules で渡してください。例: tag-refactor plan --rules tagrules.yaml"},
}

// T returns the message id in the language lang, with args put in. An
// unknown id returns the id itself, which the catalog test prevents.
func T(lang Lang, id string, args ...any) string {
	m, ok := catalog[id]
	if !ok {
		return id
	}
	format := m.en
	if lang == Japanese {
		format = m.ja
	}
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// Count returns n with its unit in lang, such as "1,234 resources" or "1
// resource" (BR3.6).
func Count(lang Lang, unit string, n int) string {
	m, ok := catalog[unit]
	if !ok {
		return FormatCount(n)
	}
	format := m.en
	switch {
	case lang == Japanese:
		format = m.ja
	case n == 1 && m.one != "":
		format = m.one
	}
	return fmt.Sprintf(format, FormatCount(n))
}

// word returns the word of a unit for a count n without the number.
func word(lang Lang, id string, n int) string {
	m := catalog[id]
	if lang == Japanese {
		return m.ja
	}
	if n == 1 && m.one != "" {
		return m.one
	}
	return m.en
}

// join joins parts with the list separator of lang.
func join(lang Lang, parts []string) string {
	return strings.Join(parts, T(lang, "list.separator"))
}
