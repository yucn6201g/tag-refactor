# コード生成の計画: U2 プルリクエストの作成(残り)

## Summary

- Builds: `--pr-branch`、`--pr-title`、`--pr-body-file` と、その確かめ(本文の形の確かめを含む)、UTC の既定のブランチ名、同じ名前のリモートのブランチの確かめ、本文の部品ごとの伏せ字と手で直す一覧と上限、書き換えるものが無いときの終了コード、失敗と中断のときの続きの命令と本文のファイル
- Touches: 今の `internal/redact`、`internal/pullrequest`(新しいファイル `resume.go`)、`internal/display`、`internal/cli`、`internal/testutil`、README の `iac` の節、契約の「Construction での直し」の表
- Tests: 受け入れ条件のテスト 12 本と結合テストのサブテスト 1 本を先に、実装のあとで部品ごとに 5〜8 本の単体テスト(約 45 本)と、実際のシェルで続きの命令を動かすテスト

## 元にしたもの

- 機能の設計: `construction/u2-pull-request/functional-design/`(BR1.1〜BR5.1)
- 非機能の要件と設計: `construction/u2-pull-request/nfr-requirements/`(NFR1.6〜NFR1.10、NFR6.11〜NFR6.15、NFR9.2)、`nfr-design/`(security-design.md、logical-components.md)
- インフラの設計: `construction/u2-pull-request/infrastructure-design/cicd-pipeline.md`(CI の定義は変えない、テストでシェルを固定する)
- 契約: `inception/contract-design/contract-summary.md`(X1、X2、C1)
- 単位: `inception/units-generation/unit-of-work.md`(U2)
- ストーリー: `inception/user-stories/stories.md`(AC1.1.2〜AC1.1.10)
- U1 の非機能の設計と CI: `construction/u1-walking-skeleton/` の下(そのまま当てる)

## 前の段階から持ち越して、この計画で決めること

| 持ち越し | この計画での決め方 | 手順 |
|---|---|---|
| 機能の設計のレビューの残り: 本文のファイルを書けないとき | 非機能の設計の5節のとおり(branch の段階の失敗、ブランチを作らない、手順の文、終了コード 1) | 8、9 |
| 機能の設計のレビューの残り: 60,000 と 65,536 の数え方 | Unicode のコードポイント(`utf8.RuneCountInString`) | 7、9 |
| 機能の設計のレビューの残り: BR2.3 の gitCheck の時点 | 今の `iac` と同じく、書き換え案ができたあと、ファイルを書き出す前に行う。Check(BR2.1、BR2.2)のあと | 10 |
| 機能の設計の BR4.3 の PowerShell の引用 | 非機能の設計の4節で上書きする(`'` に加えて U+2018〜U+201B も重ねる、`"` と表示できない文字と空白を含んで末尾が `\` の値は手順の文) | 8 |
| 非機能の設計で足す `Plan` の2つの欄 | `Plan.TitleRedacted` と `Plan.BodyNotices` を足し、契約の「Construction での直し」の表に1行足す | 7、14 |

## Testing Contract

```json
{
  "version": 1,
  "methodology": "custom",
  "source": "team",
  "ordering": "まず受け入れ条件(Given/When/Then)を自動テストとして書き、次にそれを満たす実装を書き、実装のあとで細かい単体テストを書いて、すべてのテストを実行する。",
  "scope": "mvp",
  "test_strategy": "standard",
  "project_type": "brownfield",
  "applicable_notes": [
    {
      "layer": "org",
      "text": "We treat tests as a first-class deliverable in every Bolt. The specific\nmethodology (TDD, BDD, ATDD, or classic test-after) is affirmed at\npractices-discovery and recorded in `team.md` under this heading with explicit\n`Methodology` and `Ordering` fields; Code Generation resolves those fields\nindependently from coverage, tooling, and scope notes.\n\nWhen no posture has been affirmed, our default per scope is:\n- **Methodology**: test-after\n- **Ordering**: implement each applicable testable layer, then write and run\n  that layer's tests.\n- `mvp`, `enterprise`, `feature`, `infra`, `classic` add an 80% line-coverage\n  floor and CI execution before merge.\n- `bugfix`, `security-patch` add a targeted regression for the specific\n  bug/vulnerability and require the existing suite to remain green.\n- `express` uses the Minimal strategy: requirement-driven unit tests (one per\n  requirement, with a happy-path floor per component); existing tests remain\n  green.\n- `poc`, `refactor`, `workshop` add no extra new-test floor and require the\n  existing suite to remain green.\n\nThe active `Test Strategy` still applies in every scope and determines test\nvolume/types. Scope floors are additive; they never reduce or replace the\nselected strategy.\n\nBuild and Test verifies defined coverage floors and affirmed quality targets;\nthey may not be weakened to make a step pass.\n\nAffirm a stricter posture in `team.md` if the team commits to one."
    },
    {
      "layer": "team",
      "text": "私たちはテストを、すべての Bolt で作る成果物の一部として扱います。\n\n- **Methodology**: custom\n- **Methodology evidence**: 前の作業(`261019-tag-refactor-mvp`)の面談で、テストの進め方の質問の回答は C(受け入れ条件(Given/When/Then)を先にテストにし、細かい単体テストは実装のあとに書く)でした。テストを先に書く部分と、実装のあとに書く部分が混ざっているため、`tdd`、`bdd`、`atdd`、`test-after` のどれにも当てはまらず、`custom` とします。コミットの履歴にも、`iac` の受け入れ条件のテストを実装の前に足したコミット(`2296959`)があり、この進め方が守られています。\n- **Ordering**: まず受け入れ条件(Given/When/Then)を自動テストとして書き、次にそれを満たす実装を書き、実装のあとで細かい単体テストを書いて、すべてのテストを実行する。\n- カバレッジの下限は80%です。Go の標準の測り方(`go test -cover` の文(statement)単位)で測ります。コマンドの入口などの配線部分も、外のプログラム(`git`、`gh`、`cdk`、Node.js、Python)を呼ぶ部分も、計測の対象から外しません。外のプログラムの呼び出しは差し替えてテストします。\n- マージの前に、プルリクエストの CI でテストを実行します。\n- AWS を呼ぶ部分の単体テストでは、AWS の呼び出しを差し替え、AWS の認証情報なしで CI で実行します。実際のアカウントを使う結合テストは、手元だけで実行します。CI から実際の AWS アカウントは呼びません。`cdk synth` には AWS の認証情報を渡さず、テストには AWS を呼ばない CDK のアプリだけを使います。\n- GitHub については、CI でも実際の GitHub にプルリクエストを作る結合テストを実行します。テスト専用のリポジトリを1つ用意し、そこにだけ書ける fine-grained トークンを CI の secret に置きます。secret の無い実行(fork からのプルリクエスト、Dependabot のプルリクエストなど)では、このテストを飛ばします。テストで作ったブランチとプルリクエストは、テストのあとで閉じて消します。\n- CI のテストは、Linux、macOS、Windows の3つの OS と、Go の `stable` と `oldstable` の2つの版で、配るバイナリと同じ `CGO_ENABLED=0` で実行します。Linux では `-race` 付きでも実行します。\n- `cdk synth` を動かすテストも、CI の3つの OS すべてで実行します。そのために、CI に Node.js、Python、CDK の CLI を入れます。\n- `cdk synth` を動かすテストで Node.js、Python、CDK の CLI が見つからないとき、CI ではテストを失敗にします(飛ばして通すことはしません)。手元では、理由を出してテストを飛ばします。\n- テストのデータとして置く CDK のアプリ(TypeScript と Python)の依存(npm と pip のパッケージ、CDK の CLI)は、版を固定し、Dependabot で更新します。\n- テストの量と種類は、ワークフローのテスト戦略(Test Strategy)で決まります。上の下限は、それに加わる条件です。下限を下げて Build and Test を通すことはしません。"
    }
  ],
  "obligations": {
    "strategy": "standard",
    "strategy_volume": [
      "Five to eight tests per component.",
      "Unit tests plus integration tests for key boundaries.",
      "Add E2E, performance, or security tests when requirements demand them."
    ],
    "scope_floor": [
      "Meet an 80% line-coverage floor.",
      "Run the selected tests in CI before merge."
    ],
    "combination_rule": "Apply every selected-strategy obligation and every scope-floor obligation; neither replaces the other, and a targeted scope regression may add the narrowest necessary test type beyond the strategy default."
  },
  "plan_profile": {
    "methodology": "custom",
    "runner_step": "Verify the existing test runner/configuration and record the exact unit-scoped command.",
    "runner_ready_before_first_test": true,
    "testable_layers": [
      "Data model / database behavior",
      "Repository / data access",
      "Business logic",
      "API / endpoint",
      "Frontend behavior"
    ],
    "steps": [
      "Project structure and production configuration skeleton.",
      "Verify the existing test runner/configuration and record the exact unit-scoped command.",
      "Custom ordering - まず受け入れ条件(Given/When/Then)を自動テストとして書き、次にそれを満たす実装を書き、実装のあとで細かい単体テストを書いて、すべてのテストを実行する。",
      "Implementation and tests - preserve that exact ordering; do not convert it to layer-local TDD.",
      "Environment/build configuration.",
      "Documentation and traceability."
    ]
  },
  "input_sha256": "sha256:a0d6750d5eccdd3aff7d4d1895f455934eeacec8431076f4c18acc04ee0858cd",
  "contract_sha256": "sha256:36a5093ffce4ef8b71667a32cc2ced29ea3105b4fc692f6ae7906d40b17cc996"
}
```

## 手順

チームの進め方(custom)に従い、受け入れ条件のテストを先に書き(手順 4)、実装(手順 5〜10)、実装のあとで細かい単体テスト(手順 11)、すべてのテスト(手順 13)の順に進めます。

### 構成と準備

- [ ] **Step 1: 作業ブランチ。** B1 のプルリクエスト #24 を人が squash マージしたあとに行う。`git fetch origin` のあと、このフォルダーで `git switch -c bolt/v0.2-u2-pull-request origin/main` を実行する(このブランチの切り替えは、この計画の承認をもって人の許可とする)。`aidlc/` の下の未コミットの記録はそのまま持ち越す。切り替えのあと `git status` で、記録だけが変更として残ることを確かめる。README の `plan` の節、`internal/apply`、`scripts/`、`internal/iac/testdata` には触らない。(全体)
- [x] **Step 2: テストの道具の確かめ。** `GOTOOLCHAIN=local` で `go test ./internal/pullrequest/ ./internal/redact/ ./internal/cli/ ./internal/display/` が今の main で通ることを確かめ、`unit-test-instructions.md` の単位の命令を記録する。手元の Windows で `powershell.exe` と `pwsh` があるかを確かめる。(全体)
- [x] **Step 3: テストの偽物を広げる。** `internal/testutil` の偽の gh を、名前を選べる「引数を書き出す偽物」に広げる(`git` と `gh` の名前で置ける。今の `InstallFakeGH` の使い方は変えない)。Windows ではハードリンクでなく写しにする(U1 の直しのまま)。(NFR1.9 の確かめ)

### 受け入れ条件のテスト(先に書く)

- [x] **Step 4: 受け入れ条件と結合テストを先に書く。** `internal/cli/acceptance_v02u2_test.go` を書き、`internal/cli/github_pr_test.go` の `TestGitHubPullRequest` をサブテスト `default` と `options` に分ける。この時点では失敗することを確かめる。各テストの名前は `TestV02U2` で始める。U1 の受け入れテストと同じく、本物の git と、手元の裸のリポジトリの origin と、偽の gh を使う。
  1. AC1.1.2(US1.1): `--pr-branch`、`--pr-title`、`--pr-body-file` を付けると、ブランチ名、`--title`、`--body-file` の中身(利用者の本文が先頭、変更したファイルの節が続く)がその値になる。付けないと、ブランチ名は差し替えた時計の UTC と `Z` の形。
  2. AC1.1.2 の入力の誤り(BR1.1、NFR6.12): `--pr` なしの `--pr-title`、`@{-1}`、`-x`、`a..b` のブランチ名、改行を含むタイトル、1 MiB を超える本文、60,001 文字の本文、名前付きパイプ(Unix だけ)で、終了コード 1、ソースのファイルが変わらず、git の書き込みも gh も呼ばれない。
  3. NFR1.6: 本文に `ghp_` と20文字、`AKIA` と16文字、`https://u:p@h` があると、終了コード 1、どのファイルも変わらず、標準エラーと `--debug` に値が出ず、形の名前と行の番号が出る。
  4. NFR1.7: 本文に ARN があると、知らせの1行が出て、プルリクエストの本文に ARN がそのまま入る(終了コード 0)。
  5. NFR1.8: `--pr-title` の ARN が、偽の gh の `--title` とコミットのメッセージで `<redacted>` になり、そのことの1行が出る。
  6. AC1.1.3(US1.1): 今のブランチが `upstream` を追跡しているとき、`upstream` に push してそこに作る。追跡先が無ければ `origin`。
  7. AC1.1.4(US1.1、BR2.1): 作るブランチと同じ名前のブランチがリモートにあると、どのファイルも変わらず終了コード 1 で、`--pr-branch` の対処が出る。
  8. AC1.1.5(US1.1): pre-commit のフックがあり、ほかのファイルがステージされているとき、フックが動き、コミットには書き出したファイルだけが入り、ほかのステージは残る。
  9. AC1.1.6(US1.1): 手で直すものが1件ある変更案で `--lang ja --pr`。タイトルと本文が日本語で、本文に変更したファイル(種類つき)と手で直す一覧があり、終了コード 2。
  10. AC1.1.7、AC1.1.8(US1.1): 反映済みで手で直すものも無いと、ブランチも gh も無く「反映するものはありません」で 0。手で直すものだけなら一覧を示して 2。
  11. AC1.1.9(US1.1、NFR1.9、NFR1.10): 偽の gh の作成を失敗させると、済んだ段階、続きの命令(作成の1行、`--body-file` あり、本文の中身なし)、本文のファイルの場所が出て終了コード 1、ファイルが残る。commit で失敗させると、commit の段階が add と commit の2行で出る。テストは `MSYSTEM` を `t.Setenv` で選び、sh 用と PowerShell 用の両方の表示を確かめる。中断(コンテキストの取り消し)でも、場所と続きの命令が出てファイルが残る。
  12. AC1.1.10(US1.1): 応答しない偽の gh で、上限(テストでは短く差し替える)のあとに止まり、出力に GitHub のトークンの値が出ない。
  13. 結合テストのサブテスト `options`(NFR9.2): 別の clone で `--pr-branch tag-refactor/iac-<UTC>Z-options`、`--pr-title`、`--pr-body-file` を渡し、`gh pr view --json headRefName,title,body` で確かめる。`staleBranch` を UTC と、`Z` のあとの文字を無視する形に直す。

### 実装

- [x] **Step 5: `internal/redact` の `Scan`。** `Finding` と `Scan` を、非機能の設計の2節のとおりに作る(5つの形、境目の確かめ、数えなかった一致の次のバイトからの探しなおし、行の番号は重ならない昇順)。今の `String` と `URL` は変えない。(NFR1.6、NFR1.7、NFR6.11)
- [x] **Step 6: `internal/display` の本文。**
  - `PullRequestBody(lang, res, prefix, userBody) (string, bool)` にする。導入、変更したファイル(CDK の TypeScript、CloudFormation、Terraform の種類)、手で直す一覧(`entities.md` の ManualLine の書き方、理由はカタログの固定の文)を、部品ごとに `redact.String` に通してからつなぐ。利用者の本文はそのまま先頭に置く。最後の全体への伏せ字をやめる。
  - 65,536 文字(コードポイント)を超えたら、手で直す一覧の後ろから省き「ほか N 件」の行を置く。省いても超えれば false。
  - 新しい文(英語と日本語): `iac.prBodySecret`、`iac.prBodyNotice`、形の名前、`iac.prTitleRedacted`、`iac.prBodyTooLong`、`iac.prBodyFileFailed`、`iac.prFilesKept`、続きの命令の見出し(PowerShell 用、sh 用、上から順に1行ずつ)、手順の文、`iac.prBodyKept` の消し方、URL を読めなかったときの `gh pr list --head`。
  - (BR3.1〜BR3.3、NFR1.8、NFR10)
- [x] **Step 7: `internal/pullrequest` の Check。**
  - `checkLocal` の最初に、タイトルを `redact.String` に通してから `checkTitle`、`Plan.TitleRedacted`。
  - 本文の読み取り(ふつうのファイル、`LimitReader` で 1 MiB と1バイト、UTF-8、60,000 文字)と `Scan`。止める形があれば `iac.prBodySecret` の誤り、知らせは `Plan.BodyNotices`、本文は `Plan.UserBody`。
  - ブランチ名: `--pr-branch` があれば、先頭の `-` を断り、`check-ref-format --branch` の出力が名前と同じことを確かめる。
  - `DefaultBranch` を UTC と `Z` に。
  - `checkNetwork` に、作るブランチがリモートに無いこと(`git ls-remote --heads <リモート> refs/heads/<ブランチ>`)を足す。
  - (BR1.1、BR1.2、BR2.1、NFR1.6〜NFR1.8、NFR6.12〜NFR6.14)
- [x] **Step 8: `internal/pullrequest` の Publish と `resume.go`。**
  - Publish の始めに本文のファイルを `os.CreateTemp`(60 バイトで切った名前、0700 のフォルダー)で作る。作れなければ branch の失敗(started と failed を sink に送り、`BodyFile` は空)。
  - Publish の `redact.String` をやめ、`in.Title` と `in.Body` をそのまま使う。成功したら本文のファイルだけを消す。消せなければ `--debug` に場所。
  - `PublishOutcome.ResumeCommands` を消す。
  - `resume.go`: `Shell`、`DefaultShell`(Windows で `MSYSTEM` が空なら PowerShell)、`ResumeLine`、`ManualValue`、`ResumeCommands`、`quoteSh`、`quotePowerShell`。命令は1行に1つ、commit は add と commit の2行、どちらも `--literal-pathspecs`、ファイルは `out.CommittedFiles`、本文のファイルが無ければ作成を手順の文に。
  - (BR2.4、BR4.3、NFR1.9、NFR1.10、NFR6.15)
- [x] **Step 9: `internal/display` の失敗の表示。** `PullRequestStopped(out, lines)` を、非機能の設計の5節の表示のとおりに直す(済んだ段階が無ければ `iac.prFilesKept`、続きの命令、`Manual` の値は `strconv.QuoteToGraphic`、命令の行は `Sanitize` を通さない、本文のファイルの場所と消し方)。(BR4.3、NFR1.9、NFR1.10)
- [x] **Step 10: `internal/cli` の `iac`。**
  - `--pr-branch`、`--pr-title`、`--pr-body-file` を足し、`--pr` なしなら入力の誤り。`--pr-body-file` を動かしたフォルダーから絶対のパスに直す(FD2-Q3)。
  - Check のあと、書き換えの前に、知らせとタイトルの伏せ字の1行を出す。gitCheck は今のとおり Check のあと、書き出しの前。
  - 書き出したファイルが無いときの 0 と 2(BR4.1)。
  - `PullRequestBody` が false なら Publish を呼ばず、手で作る手順を示して 1。
  - Publish が失敗か中断で止まったら、非機能の設計の5節の順番で、誤りの文(中断でないとき)、`ResumeCommands`、`PullRequestStopped` を呼び、終了コードを返す。作成の後の終了コードは BR4.2(1 > 2 > 0)。
  - `iac --help` に3つのオプションを書く。
  - (X1、BR1.1、BR4.1〜BR4.3)

### 単体テスト(実装のあと)

- [x] **Step 11: 部品ごとの単体テスト(各 5〜8 本)。**
  - `redact.Scan`: 形ごとの境界(`ghp_` のあと19文字と20文字、`AKIA` の前後の英大文字)、数えなかった一致と重なる本物、同じ行の2回、12行で10個と「ほか 2 か所」、`ssh://git@host` は対象外、`\r\n`。
  - `pullrequest`: タイトルの伏せ字と 256 文字、本文の読み取り(ふつうのファイルでない、1 MiB、UTF-8、60,000 と 60,001 の日本語)、ブランチ名の確かめ、`DefaultBranch` の UTC(時間帯を UTC+9 にしても同じ)、本文のファイル(0600、成功で消える、失敗で残る、重ならない名前、200 文字のブランチ名で 100 文字以下、フォルダーが書けないとき)、Publish がタイトルと本文を変えない、同じ名前のリモートのブランチ。
  - `resume.go`: 段階ごとの行、`quoteSh` と `quotePowerShell` の値の一覧(`'`、`"`、`$x`、`$(echo x)`、`` `x ``、U+2018〜U+201B、空白、空白と末尾の `\`、改行、U+202E、`-x`、`*`、`[a]`、`:x`)、`Manual` になる値、本文のファイルが無いときの作成、`DefaultShell`(`MSYSTEM` を空と `MINGW64`)。
  - 実際のシェルのテスト: Linux と macOS は `sh -c`、Windows は `powershell.exe` と `pwsh` で命令を動かし、偽物が受け取った引数を比べる。Windows で見つからなければ、`CI` があれば失敗、無ければ飛ばす。
  - `display`: 部品ごとの伏せ字(利用者の本文は変わらない)、65,536 文字の境界と省略と false、`PullRequestStopped` の表示(手順の文、`QuoteToGraphic`、消し方)、新しい文のカタログのキーの抜け。
  - `cli`: `staleBranch`(`Z` と `-options`)、`--pr-body-file` の相対のパス。

### 文書

- [x] **Step 12: README と契約の記録。**
  - README の `iac` の節に、3つのオプション、本文の形で止まること(トークン、アクセスキーの ID、パスワードつきの URL)と知らせ(ARN、12 桁の数字)、形を崩して書く方法、止まったときの直し方、続きの命令(PowerShell 用と sh 用)、本文のファイルの場所と消し方、終了コードを書く。`plan` の節には触らない。
  - 契約の「Construction での直し」の表に、C1 の `Plan` に `TitleRedacted` と `BodyNotices` を足したことを1行足す。
  - (契約の決まり、NFR の残るリスク)

### 確かめと記録

- [x] **Step 13: すべてのテストと確かめ。**
  - `gofmt -l`、`go vet ./...`、`golangci-lint run`(手元にあれば)、`go test ./...`(CGO_ENABLED=0)、カバレッジ 80% 以上を確かめる。`govulncheck` も通す。
  - 構成の検証コマンド(`test/verify/v02-u1.sh`、人が承認したもの)で、端から端までの流れが壊れていないことを確かめる。
  - 失敗は直してから次に進む。下限は下げない。
- [x] **Step 14: 記録。** `code-summary.md`、`traceability.json`、`source-manifest.json`(この単位で作った、変えたすべてのファイル)を書く。

## ストーリーと手順の対応

| ストーリー | 受け入れ条件 | 手順 |
|---|---|---|
| US1.1 | AC1.1.2 | 4(1、2、13)、7、10 |
| US1.1 | AC1.1.3 | 4(6)、7 |
| US1.1 | AC1.1.4 | 4(7)、7、10 |
| US1.1 | AC1.1.5 | 4(8)、8 |
| US1.1 | AC1.1.6 | 4(9)、6、10 |
| US1.1 | AC1.1.7、AC1.1.8 | 4(10)、10 |
| US1.1 | AC1.1.9 | 4(11)、8、9、10 |
| US1.1 | AC1.1.10 | 4(12)、8 |
| 非機能 | NFR1.6〜NFR1.10、NFR9.2、NFR6.11〜NFR6.15 | 4(3、4、5、11、13)、5、7、8、9、11 |
