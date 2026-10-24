## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-24T15:04:56Z
**Iteration:** 1

### Prior findings

| Severity | Location | Finding | Status |
|---|---|---|---|
| なし | なし | なし | なし |

### New findings

| Severity | Location | Finding | Required action |
|---|---|---|---|
| medium | `internal/cli/suggest.go` と `internal/display/draft.go`(`rules suggest` の下書き)。出典: `functional-design/rules.md` BR3.9、`functional-design/functional-spec.md` 87 行目、`functional-design/functional-design-questions.md` Q7(答えは A)。`code-summary.md` の「ずれ」1 | 機能設計の BR3.9 と、人が選んだ Q7(A)は「`rules suggest` の下書きのコメントも日本語にする」です。実装は英語のままです。理由は「NFR 設計で `rules` は変えないとした」「64 KiB の判定に効く」「U3 のテストと `verify.sh` が英語を確かめている」でした。しかし `nfr-design/logical-components.md` の「`internal/rules` は変えない」は、下書きの言語を決めたものではありません。承認済みの計画(`code-generation-plan.md`)にも、この例外はありません。つまり、機能設計と NFR 設計のあいだの食い違いを、実装側が人に聞かずに片方へ寄せています。`README.md` 155 行目には書いてあるので隠れてはいません | 承認の画面で、人が次のどちらかを明示して決める。(a) BR3.9 のとおり、`--lang ja` のときは下書きのコメントを日本語にする(言語を `rules.Suggest` か `display` に渡す。64 KiB の判定は日本語の大きさで測り直し、U3 のテストと `verify.sh` の期待を直す)。(b) BR3.9、`functional-spec.md` の言語の節、Q7 の答えを「下書きのコメントは英語のまま」に直して記録する。決めたら `README.md` と食い違わないことを確かめる |
| low | `internal/planner/saved.go` の `planError`、`decodeError`、`readPlan` 系の各メッセージ。`internal/cli/messages.go` の `describeFileError`。`code-summary.md` の「ずれ」2 | `LoadPlan` の誤りの詳細は、`planner` が英語の文字列を直接作ります(`planner` は `display` の文言の一覧を使えません)。ルールファイルの誤り、AWS へのアクセスの詳細、`unreadPolicies[].reason`(日本語の golden にも `access denied to iam:GetPolicyVersion` が英語のまま出る)も同じです。BR3.9 は「エラー」「案内」を日本語にするとしていて、日本語の表示では 2 行の枠だけが日本語になり、詳細は英語が混ざります。`LoadPlan` の誤りは U4 で作った文言なので、U6 と U8 が使う前に形を決めておけます | BR3.9 に「他のパッケージが作る詳細の文は英語のまま」という例外を書いて記録する。`LoadPlan` の誤りだけは、`Detail` と `Hint` を文言の識別子と引数で返す形にして、`describeFileError` が日本語にできるようにする(U6 が使い始める前に直す) |
| low | `internal/display/results.go` の `planFindings`(`plan.statement`、`plan.statementValue`)、`internal/display/planout.go` の `mdFindings`。`internal/planner/saved.go` の `readFinding` | 保存した変更案には `conditionOperator`(と `unreadPolicies[].policyRef`)がなく、`LoadPlan` で読んだ変更案の `ConditionOperator` は空になります。契約 C2 に無いので書かないのは正しいです。しかし、読み込んだ変更案を U6 の確認画面などが同じ表示の部品で出すと、一覧は `Condition  aws:ResourceTag/env`(空白が 2 つ)、Markdown は空のコードスパン(`mdSpan("")` は空白 2 つ)になります。今の U4 のテストは、保存して読み戻したあとの表示を確かめていません | 演算子が空のときは、その語と区切りの空白を出さない。読み込んだ変更案を一覧、差分、Markdown で出すテストを 1 つ足す(U6 に渡す前に) |
| low | `internal/planner/planner.go` の `Plan.ExitCode`、`internal/cli/cli.go` の `outcome.exitCode` | 終了コードの規則(BR2.6)が 2 か所にあります。製品のコードは `outcome.exitCode`(JSON の `exitCode` もここから)だけを使い、`Plan.ExitCode` はテストと golden しか呼びません。今は同じ結果ですが、片方だけ直すとテストが通ったまま JSON の `exitCode` と実際の終了コードがずれます | `cli` が `Plan.ExitCode` を使うようにする。できなければ、収集の不完全・影響分析の不完全・要確認・消す候補だけ、の全組み合わせで両方が一致するテストを足す |
| low | `internal/display/messages.go` の `plan.summary`(英語は `N conflicts`、`N affected` など)、`internal/display/testdata/plan/*.golden`、`counts_test.go` の正規表現 | 要約の行は件数が 1 でも `1 conflicts` と出ます(`Changes: 1 resource` は単数形に直っています)。形は golden と NFR10.6 の正規表現に固定されています | 単数形を持たない言い回しにする(例: `conflicts: 1`)か、単数形の分岐を足し、golden と正規表現を合わせて直す |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| `go vet ./...` | 出力なし(指摘なし) | 通常のビルドで静的な問題はない。`-tags integration` と `-tags race` は自分では動かしていない(要約の記録を信じる) |
| `go test ./internal/planner/... ./internal/display/... ./internal/fileout/... ./internal/cli/...` | 4 パッケージとも ok | U4 の受け入れ条件 15 件、golden、ファジングの種(通常のテストとして)、割り込みのテストを含めて通る |
| `go test -cover ./cmd/... ./internal/...`(Windows) | planner 95.5%、display 94.9%、fileout 86.1%、cli 98.9%、cmd 71.4%(Unix 用の `signals_unix_test.go` は Windows では動かない) | 下限 80% は CI の `coverage` ジョブの合算(`-coverpkg`)で測る。個々のパッケージの数字は要約と大きくずれない |
| `bash -n scripts/verify.sh` | SYNTAX_OK | 構文だけの確認。検証用アカウントでの実行はしていない(チェックポイントで行う)。`TAG_REFACTOR_SANDBOX_ACCOUNT_ID` は `verify.sh` が必須として確かめ、環境変数として `go test` に渡る |
| `aidlc engine sensor-traceability`(`code-generation/traceability.json`) | `pass: true`、`gaps` `orphans` `invalid_*` はすべて空 | 追跡の表は満たしている |
| 生の見えない文字の検索(U4 の差分と追跡対象のファイル。DEL、C1、書式文字、U+2028/2029、BOM、双方向の制御、タグ文字、C0 の制御) | U4 で触れたファイルに 0 件。ほかに見つかった 4 件は U4 と無関係のフレームワークのファイル | `safejson.go` のコメントや期待値に生の文字は残っていない。期待値は文字の番号から組み立てている |
| 人の目での確認(`-race`、`-tags integration`、`scripts/verify.sh`、Windows PowerShell の表示) | 未実行 | `-race` は CI の `race` ジョブ、検証用アカウントでの確認と見た目はチェックポイントで確かめる。この判定に含まれない |

### Summary

機能設計の BR1.1〜BR4.4 に対して、コード、golden、テストを読んで反証を試みましたが、致命的な不具合は見つかりませんでした。次の点は設計どおりで、テストが実際の振る舞いを確かめています。値の別名を当ててから比べること、`mergesIntoExisting` が true のとき何も書かないこと(`applyChange` と `AfterTags` が共通)、変更の順と `changeId`、必須タグの値を正しい形にすること、`aws:` の大文字・小文字を区別しない除外、変更が 1 つ以上あるときだけ影響分析をすること、終了コード、標準出力に結果だけを出すこと。`SafeJSONWriter` は `Write` の切れ目と代理対を正しく扱い、Markdown はデータをコードスパンにだけ入れ、`fileout` は終わり方によらず一時ファイルを消し、`LoadPlan` は 256 MiB と BR4.4 の確かめを満たし、外側の `context` で Ctrl+C を見分けます。13 件のずれのうち、人の判断が要るのは 1 件だけです。`rules suggest` の下書きのコメントが英語のままで、Q7 と BR3.9 に反しています(medium)。残りの low は、U6 が読み込んだ変更案を表示する前に直しておくと手戻りが減るものです。

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| - | - | - | No findings | No action required | Resolved |
