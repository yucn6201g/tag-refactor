# コードの生成の計画: U4 変更案

## Summary

- Builds: 変更案の4つの種類(キー名の変更、値の変更、必須タグの追加、削除)と衝突・値が未指定・消す候補・集計、4つの出力形式(一覧、差分、JSON、Markdown)と scan の JSON・Markdown、英語と日本語、色と `NO_COLOR`、`--out` の保存と保存した変更案の読み込み、Ctrl+C の後片付け、`verify.sh` への U4 の実機の確認
- Touches: `internal/planner/`、`internal/display/`、`internal/fileout/`(新規)、`internal/cli/`、`cmd/tag-refactor/`、`scripts/verify.sh`、`README.md`
- Tests: 受け入れ条件のテスト約15件(先に書く)、パッケージごとの単体テスト5〜8件(約60件)、期待のファイル(4形式 × 2言語)、件数の一致(200通り)、ファジング(種の入力は CI)、性能、検証用アカウントでの結合テスト1件(手元だけ)。カバレッジ80%以上

## 前提と進め方

- 設計の出典: `construction/u4-plan/` の機能設計(BR1.1〜BR4.4)、非機能要件(NFR1.13〜NFR1.15、NFR3.8、NFR8.15〜NFR8.19、NFR9.6、NFR9.7、NFR10.6〜NFR10.8、NFR11.7)、非機能の設計(`security-design.md`、`logical-components.md`)、インフラの設計(`cicd-pipeline.md`)。契約 `inception/contract-design/contract-summary.md`(C2、C3、C5、C9。U4 で足した項目を含む)。ストーリー US3.1(AC3.1.1〜AC3.1.8)、US6.1(AC6.1.1〜AC6.1.5。AC6.1.1 の apply と rollback は U6・U7)、U3 から引き継いだ AC2.1.3・AC2.1.4。
- **ブランチ:** `bolt/u4-plan`(`main` の `3aa4d95` から作成済み)。AI-DLC の記録のコミットは、人が頼んだときに行う。マージは人がプルリクエストで squash マージする。
- **既存のコードに手を入れる:** U1〜U3 のファイルをその場で直す。同じ役割のファイルを別に作らない。新しいパッケージは `internal/fileout` だけ。
- **順番(Testing Contract の custom):** 受け入れ条件のテストを先に書いてコミットし、次に実装、実装の後に細かい単体テストを書き、最後にすべてのテストを実行する。
- **色の条件:** 機能設計 BR3.7 ではなく、NFR11.7(`TERM=dumb` と Windows の仮想端末の有効化を含む)から取る。
- **Markdown のエスケープ:** NFR8.16 のとおり、データはコードスパンに入れ、コードスパンの中の `<`、`>`、`&` は文字の参照に変えない。
- **確かめながら決めること(結果は `code-summary.md` に書く):**
  - `json.Marshal` の出力に、DEL、C1 の制御文字、書式の文字がそのまま出ること(後処理が要る理由)。
  - Windows の `GetUserPreferredUILanguages(MUI_LANGUAGE_NAME)` が CI の Windows のランナーで返す値。
  - 東アジアの幅の表を作るスクリプトの URL が Unicode 17.0.0 のファイルを返すこと。

## 手順

- [x] **Step 1: テストの準備** — `unit-test-instructions.md` のコマンドが動くことを確かめる。U4 の受け入れ条件のテストのための手助け(ルールファイルと `--out` の一時フォルダ、4つの形式の出力から件数を読み取る関数、`--lang` と環境変数の固定)を `internal/cli` のテストに用意する。偽の AWS(`internal/testutil`)は U2 のまま使う。(対応: Testing Contract の runner_step)
- [x] **Step 2: 受け入れ条件のテスト(先に書く。`internal/cli/acceptance_u4_test.go`)** — AC3.1.1(4種類の変更の件数)、AC3.1.2(重なる変更は総数で1件)、AC3.1.3(値が未指定と、残る他の変更)、AC3.1.4(消す候補は削除に入らない)、AC3.1.5(4つの形式で件数が一致)、AC3.1.6(`--out` のファイルの中身)、AC3.1.7(IaC の警告)、AC3.1.8(標準出力に案内が混ざらない)、AC6.1.1(scan、rules suggest、plan のヘルプ)、AC6.1.2(`LANG=ja_JP.UTF-8` で日本語、`fr_FR` で英語)、AC6.1.3(`NO_COLOR` で色なし、記号で分かる)、AC6.1.4(scan の JSON の `exitCode` と終了コード)、AC6.1.5(scan の JSON が正しい)、AC2.1.3(必須タグの条件ごとの値が default より優先)、AC2.1.4(消すキーだけが削除)。この時点では失敗することを確かめ、これだけで1つのコミットにする。(対応: US3.1、US6.1、AC2.1.3、AC2.1.4)
- [x] **Step 3: `internal/fileout` を作る** — U3 の `cli.writeDraftFileWith` を移し、`Prepare`(書けるかの試し。作ってすぐ消す)、`Create`(書く直前に一時ファイル、Unix は 0600)、`Writer`(大きさを数え、超えたら何も書かずに InvalidInput)、`Commit`(Sync、閉じる、ハードリンク、使えなければ確かめてから名前を変える)、`Discard`(閉じてから消す)。名前を付ける関数の差し替えの口を残す。`rules suggest --output` をこれに切り替える。(対応: NFR8.17、NFR8.18、security-design.md §3)
- [x] **Step 4: 変更の作り方(`internal/planner`)** — 4つの種類の `Change`、値の別名を当ててから比べるキー名の変更と衝突(BR1.1、BR1.2。`mergesIntoExisting` が true なら書かずに消すだけ)、値の変更(BR1.3)、必須タグの追加と値が未指定(BR1.4。値に正しい値の照会を当てる。衝突しているキーは付いているものとする)、削除(BR1.5)、`aws:` の除外(BR1.6。大文字・小文字を区別しない `IsAWSReservedKey`)、タグを変更できない種類を飛ばす(BR1.7)、順と `changeId`(BR1.8)、変更後のタグ(BR1.9。U6 も使う公開の関数)、消す候補(BR2.3)、影響分析を呼ぶ条件(BR2.7。変更が1つ以上)。U3 の「values / required / remove are read but not applied」の1行を消す。(対応: BR1.1〜BR1.9、BR2.3、BR2.7、AC3.1.1〜AC3.1.4、AC2.1.3、AC2.1.4)
- [x] **Step 5: 集計と終了コード(`internal/planner`)** — `PlanSummary`、`ChangeLine`(種類ごとのまとめる単位と並び。BR2.1)、値が未指定(BR2.2)、IaC(BR2.4)、影響の印(BR2.5)、終了コード(BR2.6。消す候補だけなら0)。(対応: BR2.1〜BR2.6、AC3.1.2、AC3.1.7)
- [x] **Step 6: 保存と読み込み(`internal/planner`)** — 契約 C2 の型(`mergesIntoExisting`、`keyConflicts`、`fromValue` を含む)、`SafeJSONWriter`(DEL、C1、Cf を小文字の16進でエスケープ、基本多言語面の外はサロゲートペア、`Write` の切れ目を持ち越す)、`WritePlan`(配列の項目を1つずつ `Marshal`)、`LoadPlan`(通常のファイル、256 MiB、UTF-8 の BOM、UTF-16 の BOM は案内つきの誤り、BR4.4 の確かめ、印の求め直し)、変更案の ID(BR4.3)。(対応: BR4.2〜BR4.4、NFR8.15、NFR8.18、NFR8.19、NFR1.15)
- [x] **Step 7: 言語と色と幅(`internal/display`)** — 文言の一覧(識別子ごとに英語と日本語)、言語の決め方(`--lang`、`LC_ALL`、`LC_MESSAGES`、`LANG`、Windows の `GetUserPreferredUILanguages`、読めなければ英語)、色の判定(NFR11.7 の4条件をストリームごとに。Windows の仮想端末の処理を標準出力にも広げ、`Close` で戻す)、端末での幅(置き換えのあと、色の前の文字列で測る)と東アジアの幅の表(`gen_width.go` は `//go:build ignore`。表と `widthTableUnicodeVersion` を書き出す)、3桁区切り(BR3.6)。(対応: BR3.6〜BR3.9、NFR9.7、NFR11.7)
- [x] **Step 8: 4つの形式(`internal/display`)** — 一覧(BR3.2)、差分(BR3.3。変更前と変更後)、JSON(BR3.4。契約 C3。`SafeJSONWriter` を通す。`exitCode`、`keyConflicts`、summary の足した項目)、Markdown(BR3.5。`mdCode` の型でデータをコードスパンにだけ入れる)。scan の JSON と Markdown。出力先の表のとおり、結果だけを標準出力に出す(BR3.10)。U1〜U3 の英語の文言を文言の一覧に移し、日本語を足す(エラーの2行の案内を含む。`--help` は英語のまま)。(対応: BR3.1〜BR3.5、BR3.10、NFR8.15、NFR8.16、AC3.1.5、AC3.1.8、AC6.1.5)
- [x] **Step 9: コマンドの入口(`internal/cli`、`cmd/tag-refactor`)** — `plan` の `--format`、`--out`、`scan` の `--format`、すべてのコマンドの `--lang`。U1 の「この版では使えない」の扱いを消す。使い方の確かめの順(security-design.md §1。`--out` は `fileout.Prepare`)。`plan --out` のつなぎ(`fileout.Create` → `SafeJSONWriter` → `WritePlan` → `Commit`、`Saved plan` の行は標準エラー)。`cmd/tag-refactor` で `signal.NotifyContext`(Ctrl+C、Unix は SIGTERM も。1回目で `stop()`)。外側の `context` の `Err()` で「Interrupted. Nothing was saved.」と終了コード1。(対応: BR4.1、NFR3.8、NFR8.17、Q1)
- [x] **Step 10: 受け入れ条件のテストを通す** — Step 2 のテストと、U1〜U3 の受け入れ条件のテストがすべて通ることを確かめる。U1〜U3 のテストで、U4 の変更(文言の一覧への移し替え、使えないオプションの扱い、衝突の比べ方、3桁区切り)により期待が変わるものは直し、理由を `code-summary.md` に書く。
- [x] **Step 11: 細かい単体テスト(実装の後)** — `internal/planner`: 変更の作り方の各規則(値の別名の4つの形、必須タグの既存の値、衝突しているキー、`AWS:Foo`、タグを変更できない種類)、変更後のタグ、集計の行、終了コード、`WritePlan` と `LoadPlan`(各誤り、BOM、上限、印の求め直し)、`SafeJSONWriter`(1バイトずつ書いても同じ、サロゲートペア)。`internal/display`: 期待のファイル(4形式 × 2言語、`-update` で作り直せる)、Markdown のエスケープ、色の判定の組み合わせ、言語の決め方、幅(半角カナ、全角記号、漢字、結合文字、U+FFFD、曖昧な幅、ASCII)、表の版と `unicode.Version` の比べ、文言の一覧(NFR10.7)。`internal/fileout`: 試し、上限、上書きしない、途中で失敗したときに何も残らない、0600(Unix)。`internal/cli`: Ctrl+C(リージョンの読み取り中、影響分析中、保存中。`plan --out` と `rules suggest --output`)と、内側だけの取り消しは Interrupted にならないこと、操作の集まり(NFR3.8)、認証情報が出ないこと(NFR8.19)。(対応: BR1.x〜BR4.x、NFR3.8、NFR8.15〜NFR8.19、NFR9.6、NFR10.7、NFR11.7)
- [x] **Step 12: 件数の一致、ファジング、性能のテスト** — 件数の一致(乱数の種を固定した200通りの変更案。NFR10.6)。`FuzzLoadPlan`(パニックしない、1回1秒以内(`race` のビルドタグで10倍)、読めたら保存し直して同じ。種は `internal/planner/testdata/fuzz/FuzzLoadPlan/`)。性能: NFR1.13(作成2秒)、NFR1.14(各形式と保存3秒。置き換えの要る値を半数)、NFR1.15(読み込み3秒、メモリの増えはファイルの10倍以内)。`race` のときは件数を減らす。(対応: NFR1.13〜NFR1.15、NFR8.18、NFR10.6)
- [x] **Step 13: 検証のスクリプト** — `internal/cli/integration_u4_test.go`(`-tags integration`。`TestIntegrationU4PlanFormatsAndSavedPlan`、サブテストなし、`--lang en` と `LC_ALL=C`、JSON と `--out` を同じ1回で、ほかの3形式を別に、ずれたら新しい保存先で1回だけやり直す、`LoadPlan`、件数の一致、`env`・`Env` からの `renameKey` と `Owner` の `addRequired`)。`scripts/verify.sh`: U3 の確認のあと、U2 のキューの前に `run_go_test ... -run '^TestIntegrationU4'` を足し、`--- SKIP` を失敗にし、`--- PASS: TestIntegrationU4PlanFormatsAndSavedPlan (` がちょうど1つであることを確かめる。先頭のコメントの手順の説明を直す。(対応: cicd-pipeline.md の検証のスクリプト)
- [x] **Step 14: 全体の確認(手元)** — `gofmt -l`、`go vet ./...`、`go vet -tags integration ./...`、`golangci-lint run`、`go test ./...`、カバレッジ80%以上(`-coverpkg`)。手元でファジングを5分動かし、結果を `code-summary.md` に書く(Application Control でふさがれたら再実行)。下限を下げて通すことはしない。
- [x] **Step 15: ドキュメント** — `README.md`(`plan` の4つの形式と例、`--out` と保存したファイル、`--lang` と言語の決め方、色と `NO_COLOR`、Windows PowerShell 5.1 のリダイレクトは UTF-16 になるので変更案は `--out` で保存すること、Markdown を貼るときの注意、終了コード、幅の表の作り直し)。U3 の README の `[` と `{` のエスケープの記述の直し(作業中の変更として既にあるものをこの Step のコミットに含める)。

## ストーリーとの対応

| ストーリー | 手順 |
|---|---|
| US3.1(変更案を確かめる) | Step 2、Step 4〜Step 6、Step 8〜Step 12 |
| US6.1(自分の環境に合った表示で使う) | Step 2、Step 7、Step 8、Step 11 |
| (U3 から引き継いだ AC2.1.3、AC2.1.4) | Step 2、Step 4、Step 11 |
| (非機能の設計: 保存、Ctrl+C、出力の守り) | Step 3、Step 6、Step 9、Step 11、Step 12 |
| (インフラの設計) | Step 13 |

## Testing Contract

```json
{
  "version": 1,
  "methodology": "custom",
  "source": "team",
  "ordering": "まず受け入れ条件(Given/When/Then)を自動テストとして書き、次にそれを満たす実装を書き、実装のあとで細かい単体テストを書いて、すべてのテストを実行する。",
  "scope": "mvp",
  "test_strategy": "standard",
  "project_type": "greenfield",
  "applicable_notes": [
    {
      "layer": "org",
      "text": "We treat tests as a first-class deliverable in every Bolt. The specific\nmethodology (TDD, BDD, ATDD, or classic test-after) is affirmed at\npractices-discovery and recorded in `team.md` under this heading with explicit\n`Methodology` and `Ordering` fields; Code Generation resolves those fields\nindependently from coverage, tooling, and scope notes.\n\nWhen no posture has been affirmed, our default per scope is:\n- **Methodology**: test-after\n- **Ordering**: implement each applicable testable layer, then write and run\n  that layer's tests.\n- `mvp`, `enterprise`, `feature`, `infra`, `classic` add an 80% line-coverage\n  floor and CI execution before merge.\n- `bugfix`, `security-patch` add a targeted regression for the specific\n  bug/vulnerability and require the existing suite to remain green.\n- `express` uses the Minimal strategy: requirement-driven unit tests (one per\n  requirement, with a happy-path floor per component); existing tests remain\n  green.\n- `poc`, `refactor`, `workshop` add no extra new-test floor and require the\n  existing suite to remain green.\n\nThe active `Test Strategy` still applies in every scope and determines test\nvolume/types. Scope floors are additive; they never reduce or replace the\nselected strategy.\n\nBuild and Test verifies defined coverage floors and affirmed quality targets;\nthey may not be weakened to make a step pass.\n\nAffirm a stricter posture in `team.md` if the team commits to one."
    },
    {
      "layer": "team",
      "text": "私たちはテストを、すべての Bolt で作る成果物の一部として扱います。\n\n- **Methodology**: custom\n- **Methodology evidence**: Q6 の回答は C(受け入れ条件(Given/When/Then)を先にテストにし、細かい単体テストは実装のあとに書く)です。テストを先に書く部分と、実装のあとに書く部分が混ざっているため、`tdd`、`bdd`、`atdd`、`test-after` のどれにも当てはまらず、`custom` とします。\n- **Ordering**: まず受け入れ条件(Given/When/Then)を自動テストとして書き、次にそれを満たす実装を書き、実装のあとで細かい単体テストを書いて、すべてのテストを実行する。\n- カバレッジの下限は80%です。Go の標準の測り方(`go test -cover` の文(statement)単位)で測ります。コマンドの入口などの配線部分も、計測の対象から外しません。\n- マージの前に、プルリクエストの CI でテストを実行します。\n- AWS を呼ぶ部分の単体テストでは、AWS の呼び出しを差し替え、AWS の認証情報なしで CI で実行します。実際のアカウントを使う結合テストは、手元だけで実行します。CI から実際の AWS アカウントは呼びません。\n- CI のテストは、Linux、macOS、Windows の3つの OS で実行します。\n- テストの量と種類は、ワークフローのテスト戦略(Test Strategy)で決まります。上の下限は、それに加わる条件です。下限を下げて Build and Test を通すことはしません。"
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
    "runner_step": "Bootstrap the minimal test runner/configuration and record the exact unit-scoped command.",
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
      "Bootstrap the minimal test runner/configuration and record the exact unit-scoped command.",
      "Custom ordering - まず受け入れ条件(Given/When/Then)を自動テストとして書き、次にそれを満たす実装を書き、実装のあとで細かい単体テストを書いて、すべてのテストを実行する。",
      "Implementation and tests - preserve that exact ordering; do not convert it to layer-local TDD.",
      "Environment/build configuration.",
      "Documentation and traceability."
    ]
  },
  "input_sha256": "sha256:718a6e7f6adfab859e079f3d60a6687e52d7f72eeadae5429a96c212c2e81b35",
  "contract_sha256": "sha256:db7ed43eac0b272a1d28a3d6455a9d1ea63b45af7f36d138842ba8e9b43fa3fc"
}
```
