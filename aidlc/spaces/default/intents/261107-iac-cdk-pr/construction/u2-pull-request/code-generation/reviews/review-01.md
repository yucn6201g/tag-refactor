## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-11-14T13:06:31Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-00 | Major | aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/code-generation/code-generation-plan.md > review findings table | The reviewer's findings table could not be read, so its rows are not listed here: aidlc/spaces/default/intents/261107-iac-cdk-pr/construction/u2-pull-request/code-generation/code-generation-plan.md: invalid finding ID "N1" | Address the reviewer's findings as written below this table. | Unresolved |

**The reviewer's findings, as written:**

> | ID | Severity | Location | Finding | Required action | Status |
> |---|---|---|---|---|---|
> | N1 | Minor | internal/redact/scan.go > shape.in | GitHub のトークンの形の探しなおしが二乗の時間になる。左の境目で数えなかった一致のたびに、次のバイトから同じ行をもう一度探す。一致が行の最後まで伸びるため、`aghp_a` を 10,000 回並べた 60,000 文字の1行で、手元の実測は 8.7 秒だった(別のプログラムに同じ正規表現と同じ探し方を写して測った)。`--pr-body-file` の上限(60,000 文字)の中で起きるので、書き換えの前の確かめが固まったように見える。自分のファイルだけが入力で、秘密の漏れにはならない | 次の一致の始まりを、数えなかった一致の先頭の次ではなく、次の接頭辞の出現から探す(例: 先に接頭辞だけで位置を集め、境目を確かめる)。または、1行の長さに上限を置く。長い1行を `Scan` に通して一定の時間内に終わる単体テストを足す | Open |
> | N2 | Minor | internal/display/iacprbody.go > iac.prNoURL と internal/cli/iac.go > publish | gh が URL を出さなかったときの案内 `gh pr list --head` のあとに、ブランチ名を引用符なしで示す。`--pr-branch` の名前には `$`、`;`、`&`、`(`、`'` などが使える(`git check-ref-format` は拒まない)ので、そのまま貼ると別の命令になりうる。続きの命令では値をシェルごとに引用する決まりなのに、この1行だけ例外になっている。名前は利用者が自分で付けたものなので重大度は低い | この1行も `ResumeCommands` と同じ引用の関数を通して、シェルごとに引用した命令にする。引用できないときは値を `QuoteToGraphic` で示す文にする | Open |
> | N3 | Minor | internal/display/iacprbody.go > resourceName | 手で直す一覧の「リソースの名前」を ARN の最後の `:` か `/` のあとにしたため、設計(entities.md の ManualLine.resourceName は論理 ID)と違い、名前として読めない値が出る。たとえば Lambda の `function:fn:live` は `live`、ELB の `loadbalancer/app/my-alb/50dc6c495c0c9188` は 16 桁の ID になる。code-summary.md はこれを逸脱として挙げているが、理由の「`internal/iac` を変えると触る範囲を超える」は、設計の要件を満たさないことの根拠として弱い | 設計の判断として残すなら、entities.md の ManualLine を実装に合わせて直し(物理名か ARN の末尾、ファイルと行で特定する前提)、`function:fn:alias` と ELB の例の期待値のテストを足す。論理 ID を出すなら `iac.ManualItem` に欄を足す | Open |
> | N4 | Minor | internal/cli/iac.go > checkPullRequest | `--pr-body-file ""` と `--pr-branch ""` を明示して渡しても、値が空のため、どちらも指定が無かったことになる(本文は既定の導入、ブランチは既定の名前)。`prGiven` には入るので `--pr` なしでは誤りになるが、`--pr` ありでは黙って無視される。`--pr-title ""` は誤りになるので、3つの扱いが揃っていない。BR1.1 は本文のファイルが読めることを求める | `Changed` で渡されたオプションの空の値は、`--pr-title` と同じく入力の誤りにする。空の値の受け入れテストを足す | Open |
> | N5 | Minor | internal/cli/iac.go > publish の本文が長すぎる道 | 手で直す一覧を省いても本文が 65,536 文字を超えるときの cli の道(Publish を呼ばない、終了コード 1、ブランチを作らない、書き換えたファイルは残る)に、cli を通すテストが無い。設計 security-design.md の8節の表(NFR1.8 の行)はこれを受け入れテストで確かめるとしている。display の単体テストが `ok` が false になることを確かめるだけで、`Publish` が呼ばれないことと終了コードは未検証。この道はコードが短く、読んだかぎり誤りは見つからなかった | 変更したファイルを約 90 個作る(または本文を 60,000 文字にしてファイル名を長くする)受け入れテストを足し、終了コード 1、gh と git の書き込みの呼び出しが無いこと、ブランチが作られないこと、書き換えたファイルが残ることを確かめる | Open |
> | N6 | Minor | internal/cli/github_pr_test.go > TestGitHubPullRequest options | 結合テストの `options` の枝名は秒までの UTC の日時に固定の `-options` を足すだけなので、同じ秒に動いた別の CI の実行(別のプルリクエストの実行など)と名前が重なると、BR2.1 のリモートの確かめで止まって失敗する。既定の枝名にも同じ性質があるが、`options` は名前を自分で決めているので避けられる | 枝名に実行の ID(`GITHUB_RUN_ID` など、無ければ乱数)を足す。古い枝の掃除は先頭の日時だけを読むので、そのまま使える | Open |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| go test (redact, pullrequest, display, cli; GOTOOLCHAIN=local, CGO_ENABLED=0, -count=1) | 4 パッケージとも ok(cli は 103 秒) | 手元(Windows)で全部通る。`pwsh` と実際の GitHub の結合テストは手元では動かせず、未検証 |
| 本文の形の探しの速さの実測(別のプログラム) | 60,000 文字の1行で 8.7 秒 | N1 の根拠 |
| 設計との突き合わせ(BR1.1 から BR5.1、NFR1.6 から NFR1.10、NFR6.11 から NFR6.15、NFR9.2) | 逸脱は N3 と N4 のみ。そのほかは設計のとおり | 下の要約を参照 |

### Summary

Critical と Major は見つからず、READY とする。確かめた点: (1) `redact.Scan` は設計の形・境目・「数えなかった一致の次のバイトから探しなおす」を実装どおりに持つ。(2) タイトルは `checkLocal` の最初に1回だけ伏せ字にしてから 256 文字を確かめ、`Plan.Title` をコミットのメッセージ、`gh --title`、続きの命令の3か所にそのまま使うので、トークンの形の値は届かない。本文は部品ごとに伏せ字を通し、利用者の本文は通さず、止める形は書き換えの前に止める。`--debug` の行は `Publisher.debug` で伏せ字を通り、表示は `say` と `Sanitize` を通る。(3) 続きの命令の引用は、sh は `'\''`、PowerShell は `'`、U+2018 から U+201B の二重化、制御文字と書式文字と不正な UTF-8 と `"` と「空白を含んで末尾が `\`」を `Manual` に回す。実際の sh と PowerShell 5.1 に貼るテストで引数を比べている。(4) 本文のファイルは `os.CreateTemp` で重ならない名前(100 文字以下)、0600、フォルダーは 0700 で、Publish の最初に作る。作れなければブランチを作らずに止まる。(5) 計画との違いの7件のうち、`DefaultShell` の引数、`PullRequestStopped` の引数、`describe` の Transient、本文のファイルの場所の表示、U1 のテストの直しは、設計の決まり(`TestNoTokenIsRead`、BR1.2、BR4.1、NFR1.8)から必要で、正当。リソースの名前の逸脱は N3 のとおり弱い。残る指摘はすべて Minor で、承認の判断を妨げない。
