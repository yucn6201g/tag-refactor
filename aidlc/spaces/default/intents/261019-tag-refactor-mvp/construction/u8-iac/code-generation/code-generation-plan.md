# コードの生成の計画: U8 IaC への反映

## Summary

- Builds: `iac`(変更案を CloudFormation の YAML・JSON と Terraform の `.tf` に、変える範囲だけ差し替えて反映する。既定は差分だけ、`--write` は git の確かめのあと)、種類の表 `internal/restype`、`fileout` の置き換えとやり直しの部品、`verify.sh` の U8 の確かめと CI の2つの変更
- Touches: `internal/iac/`(新しい)、`internal/restype/`(新しい)、`internal/fileout/`、`internal/journal/`、`internal/cli/`、`internal/display/`、`internal/testutil/`、`internal/rules/rules_test.go`、`test/sandbox/u8-rules.yaml`、`scripts/verify.sh`、`scripts/u6check.go`、`.github/workflows/ci.yml`、`README.md`、`go.mod`・`go.sum`(hcl/v2 と yaml/v3)
- Tests: 受け入れ条件のテスト(AC7.1.1、AC7.1.2 と、差分だけ・`--write`・終了コード・ヘルプ。先に書く)、部品ごとの単体テスト(iac の各読み取り、Edit、引用符、差分、git、上限と深さ、書き出し、restype、fileout、display、cli で各5〜8件以上)、大きなテスト(1万リソース・2,000ファイル)、U1〜U7 の回帰。カバレッジ80%以上

## 前提と進め方

- **設計の出典:** `construction/u8-iac/` の機能設計(BR1.1〜BR6.3、`functional-spec.md` の流れ W1・反映の例・状態 S1)、NFR 要件(NFR3.16〜NFR12.1)、NFR の設計(`security-design.md` §0〜§8、`logical-components.md` の種類の表の行・読み取りの口・並行とメモリ・差分・テストの設計)、インフラの設計(`cicd-pipeline.md`)。契約 C2(変更案)と C5(`iac` のオプションと終了コード)。ストーリー US7.1(AC7.1.1、AC7.1.2)。US7.2(CDK)と US7.3(プルリクエスト)は作らない(機能設計 Q1)。
- **ブランチ:** `main`(U7 のプルリクエスト #14 のマージ後の `82bb6f3`)から `bolt/u8-iac` を作って始める。AI-DLC の記録のコミットは、人が頼んだときに行う。U8 のマージも人がプルリクエストで squash マージする。
- **既存のコードに手を入れる:** `internal/fileout` に部品を足し、`internal/journal` の名前の変更のやり直しをそれに置き換える(振る舞いと U6・U7 のテストの期待は変えない)。`internal/awsaccess` は変えない。新しいパッケージは `internal/iac` と `internal/restype` の2つ。
- **新しい依存:** `github.com/hashicorp/hcl/v2`(MPL-2.0)と `go.yaml.in/yaml/v3`(MIT と Apache-2.0)。足す前と後の `go.mod` の `go` の行、間接の依存の一覧、実行ファイルの大きさを `code-summary.md` に記録する(NFR9.12、NFR10.13)。`go` の行が上がるなら、足す前に人に相談する。
- **順番(Testing Contract の custom):** 受け入れ条件のテストを先に書いてコミットし、次に実装、実装の後に細かい単体テストを書き、最後にすべてのテストを実行する。
- **AWS への変更は、この計画では行わない:** U8 は検証用アカウントのスタックを変えない。`verify.sh` の U8 の確かめ(読み取りだけ)は、チェックポイントで人が許可した検証コマンドとして動かす。
- **NFR 設計のレビュー3回目の Minor の扱い:** JSON で末尾の要素を続けて消すと、要素ごとの削除の範囲が重なる。Step 8 で「続けて消す要素は、最後に残る要素の終わりから最後に消す要素の終わりまでを1つの範囲(1つの Edit)にまとめる」規則を実装し、テストを足す。
- **確かめながら決めること(結果は `code-summary.md` に書く):** yaml/v3 と hclsyntax の解析の速さとメモリ(NFR5.25 の見込み。30 秒を超えるなら人に相談)、`internal/restype` の resourceType の書き方を scan の書き方に合わせた結果、検証が読む英語の文言(keyNotInSource の理由、反映済みの数)。

## 手順

- [x] **Step 1: テストの準備と依存** — `unit-test-instructions.md` のコマンドが動くことを確かめる。`go get github.com/hashicorp/hcl/v2 go.yaml.in/yaml/v3` で依存を足し、`go` の行・間接の依存・実行ファイルの大きさの前後を記録する。10 MB の YAML(yaml/v3)と 10 MB の `.tf`(hclsyntax)の解析の時間とメモリを `testing.B` で測って記録し、NFR5.25 の見込みを出す(`logical-components.md` の速さの測り方)。CI と verify で使うテストのデータの変更案 `internal/iac/testdata/sandbox-plan.json`(確認用のスタックの論理 ID とスタック名のタグ、`project` の名前の変更と値の変更)を作る。(対応: Testing Contract の runner_step、NFR5.25、NFR9.12、NFR10.13)
- [x] **Step 2: 受け入れ条件のテスト(先に書く。`internal/cli/acceptance_u8_test.go`)** — 一時のフォルダーと一時の git のリポジトリで `iac` を動かし、AC7.1.2(CloudFormation の YAML と JSON のテンプレートが、変更後のタグを表すように書き換わる。`--write` なしでは差分だけで、ファイルは変わらない)、AC7.1.1(`--tf-state` を渡すと Terraform の `.tf` が書き換わる)、終了コード(すべて反映で 0、手で直すものがあれば 2、未コミットの変更や入力の誤りで 1)、2回目は反映済みで差分なし、AWS を呼ばない(偽の AWS への要求0件、認証情報なし)、`iac --help`(BR6.3)を確かめる。この時点では失敗することを確かめ、これだけで1つのコミットにする。(対応: US7.1、AC7.1.1、AC7.1.2、BR5.1〜BR6.3、NFR3.16)
- [x] **Step 3: 書き出しの部品(`internal/fileout`、`internal/journal`)** — `RetryRename`(待ちの列、`retryable`、名前の変更と待ちの関数を引数に。`security-design.md` §7)と `Replace`(一時ファイル、一時ファイルの確かめ、権限の合わせ、名前の変更)を `fileout` に足し、Windows だけで 32・33・5 を見る `retryable` を作る。`journal` の名前の変更のやり直しを `RetryRename` に置き換え、パッケージの変数とすべてやり直す `retryable` を渡す。U4・U6・U7 のテストが変わらず通ることを確かめ、これだけで1つのコミットにする。(対応: NFR6.5、NFR9.9、NFR9.11)
- [x] **Step 4: 種類の表(`internal/restype`)** — `logical-components.md` の 18 行の表と、CloudFormation の Type から resourceType を引く関数。resourceType の書き方を `internal/inventory` の対応と scan の書き方(S3 のバケットは U6 のルールファイルの `s3` の形)に合わせ、合わせた結果を記録する。SDK に依存しない。(対応: BR3.2、NFR10.14)
- [x] **Step 5: 入力と対象(`internal/iac`)** — `Request`、`Result`、`Writer.Run` の骨組み。変更案の読み込み(U4 の `planner`)と版の確かめ(BR1.2)、`--tf-state` の `Decoder.Token` での流し読み(`address`・`mode`・`type`・`values.arn` だけ、plan の出力と空の state の扱い、500 MB の上限。BR1.3、§2)、対象の決め方(変更のあるリソース、`aws:cloudformation` のタグ、ARN の完全一致、モジュールと添字。BR2.1〜BR2.4)。(対応: BR1.1〜BR2.4、NFR3.16、NFR3.19 (g)、NFR8.27)
- [x] **Step 6: ファイルの集め方と上限(`internal/iac`)** — `--dir` の実際の場所、`WalkDir` と `Lstat` の種類のビット(リンクとジャンクションを見ない)、除くフォルダー、深さ 32、候補の数 20,000、1ファイル 10 MB と合計 500 MB、UTF-8 と BOM、`Resources` の絞り込み、形式ごとの入れ子の深さの走査(YAML の括弧・インデント・行の始まりの `- ` と `? `、JSON の括弧、HCL の括弧・単項の演算子・`?` の数)、外したものの警告。(対応: BR3.1、NFR3.18、NFR3.19、`security-design.md` §2)
- [x] **Step 7: 形式ごとの読み取り(`internal/iac`)** — 同じ形の口(解析と索引、`TagSite`、確かめ用の取り出し)で、CloudFormation の YAML(yaml/v3 の `Node`、行と桁からバイトの位置、スカラーの終わりの走査と読み直しの確かめ、アンカー・別名・マージのキー・タグ・ブロックスカラー・複数行の見分け、要素の範囲とコメント)、JSON(`Decoder` と `InputOffset`、要素と区切り)、Terraform(`hclsyntax`、`resource` ブロック、`tags` のオブジェクト、literal の判定、1行か)。解析の誤りはファイル名・行・桁・種類だけ。メモリの予算(同時に解析する合計 32 MB)と `debug.SetMemoryLimit`。場所の決め方(BR3.2 の論理 ID と種類、BR3.3 のアドレス)。(対応: BR3.2〜BR4.2、NFR3.17、NFR3.19、NFR8.27、`security-design.md` §3)
- [x] **Step 8: 変更の当て方と Edit(`internal/iac`)** — BR4.3 の pre・post・反映済み(リソース全体と変更ごと、`mergesIntoExisting`、ソースにないため反映済みとみなした削除)、要素ごとの Edit(キーと値の差し替え、要素の削除、挿入、同じ位置の削除と挿入の結合、続けて消す要素の1つの範囲へのまとめ、すべて消す場合、1行の書き方と追加項目の扱い)、引用符とエスケープ(`security-design.md` §5 の表)、当てる順と重なりの確かめ、書き換えたあとの解析の確かめ (a)〜(d)。(対応: BR4.3〜BR4.6、NFR3.17、`security-design.md` §4・§5)
- [x] **Step 9: 差分、git、書き出し、流れ全体(`internal/iac`)** — Edit からの差分(行の範囲、0行の側、先頭と末尾の同じ行を文脈に、まとまりの結合、終わりに改行がない行)、git の確かめ(`GIT_` の除去、`core.fsmonitor=false`、`rev-parse`・`ls-files -s`・`ls-tree HEAD`、blob のハッシュと CRLF、パスの照合、引数の分け方、時間の上限と `WaitDelay`)、書き出し(直前の確かめ、読み取り専用、`fileout.Replace`、取り消し)、並行の作業者(CPU の数、上限 8)とファイル名の順、結果と終了コード(BR6.1、BR6.2)。(対応: BR5.1〜BR6.2、NFR3.20、NFR5.26、NFR6.5、NFR6.6、NFR9.11、`security-design.md` §6・§7、`logical-components.md`)
- [x] **Step 10: 表示とコマンドの入口(`internal/display`、`internal/cli`)** — `iac` の結果(差分、ファイルごとの件数、反映済みの数、反映済みとみなした削除、手で直す一覧と理由の案内、管理されていない数、`--tf-state` の案内と反映0件の警告、`--write` の案内)の英語と日本語の文言(U4 の文言の一覧のテストを通す)。`iac` のサブコマンド(U1 の「まだ使えない」を置き換える。`--plan`・`--dir`・`--tf-state`・`--write`、AwsAccess を受け取らない組み立ての口)とヘルプ(BR6.3)。U1 の「まだ使えない」のテストと `err.supported` の文言を直す(まだ使えないコマンドがなくなる場合の扱いを含む)。検証が読む英語の文言を決めて記録する。(対応: BR1.1、BR6.1〜BR6.3、NFR3.16、NFR11.10)
- [x] **Step 11: 受け入れ条件のテストを通す** — Step 2 のテストと、U1〜U7 の受け入れ条件のテストがすべて通ることを確かめる。期待が変わるものは理由を `code-summary.md` に書く。
- [x] **Step 12: 細かい単体テスト(実装の後)** — `logical-components.md` のテストの設計の表のすべての行(AWS を呼ばない、範囲の外が変わらない(改行・BOM・コメント・日本語・Tags の前のコメントとブロックスカラー)、リンクとジャンクション、上限と深さ(YAML の爆弾、10 万重、`- ` と `!` の連続、`?` の数、正しいファイルが外れない)、git(fsmonitor とフィルターの印、`GIT_DIR`、特殊な名前、未追跡・ignore・変更あり・`git add` 済み・衝突・コミットなし、`core.autocrlf`、約 1,000 ファイル、時間切れ、git がない)、書き出し(途中の停止、一時ファイルの確かめ、権限、直前の確かめ、読み取り専用、2つ目の失敗、取り消し、`retryable` ごと)、出さないもの、機能設計の規則と引用符の表、差分、種類の表と実物の `terraform show -json` の例(採った元と版を書く)、表示の期待の出力(英語・日本語・`NO_COLOR`)、3つの OS で同じ差分と結果)。(対応: 機能設計の各規則、NFR3.16〜NFR11.10)
- [x] **Step 13: 大きなテスト** — 測る条件(1万リソース、変更約3万、テンプレート1,000個と `.tf` 1,000個で約 100 MB、候補でない `.json` 1,000個)で、60 秒以内(`coverage` では3倍)、ヒープの最大 2 GB 以下、ファイルを開いた回数が式以下。`race` のビルドでは件数を10分の1にする。`TAG_REFACTOR_LARGE_REAL_GIT=1` のときは実際の git で動かし、手元で1回測って記録する。(対応: NFR3.19、NFR5.25、NFR5.26)
- [x] **Step 14: CI、検証のスクリプト、ルールファイル** — `ci.yml` の `test` のジョブに `CGO_ENABLED: "0"`、`lint` のジョブに写しの `iac --write`(終了コード 0 と 2 を受け付け、4行の差分を確かめてから)と cfn-lint を足す。`test/sandbox/u8-rules.yaml` と `TestSandboxU8RuleFilesAreValid`。`scripts/u6check.go` に `includes` を足す。`scripts/verify.sh` に `cicd-pipeline.md` の手順 1〜6 と人の確認の表示を足す(一時のリポジトリの作り方、`cygpath`、`validate-template`、cfn-lint があれば、すべて通ったあとにだけ人の確認用のフォルダー)。`bash -n` と `go vet scripts/u6check.go` で確かめる。(対応: NFR3.21、NFR9.12、`cicd-pipeline.md`)
- [x] **Step 15: 全体の確認とドキュメント** — `gofmt -l`、`go vet ./...`、`go vet -tags integration ./...`、`golangci-lint run`(`integration` のタグありとなし)、`go test ./...`(Go 1.27 と 1.26)、カバレッジ80%以上(`-coverpkg`。`internal/iac` 単体の値も記録)、`govulncheck ./...`。下限を下げて通すことはしない。`README.md` に `iac` の節(使い方、`--dir` と `--tf-state`、差分と `--write`、git の確かめ、手で直す主な場合と理由、反映済みとみなした削除、限界: CDK とプルリクエストは後の作業、`default_tags` とスタックのタグ、差分にファイルの中身が出ること、利用者の git を信用すること、合計 500 MB のメモリ)と、ライセンスの節(hcl/v2 が MPL-2.0 で、ソースを変えずにライブラリとして使うこと、ソースの入手先。NFR12.1)を足す。依存の記録をまとめる。
- [x] **Step 16: プルリクエストの準備** — `bolt/u8-iac` のコミットを見直す(push とプルリクエストは人の許可を得てから)。

## ストーリーとの対応

| ストーリー | 手順 |
|---|---|
| US7.1(タグの変更を IaC に反映する。AC7.1.1、AC7.1.2) | Step 2、Step 5〜Step 13 |
| US7.2(CDK)、US7.3(プルリクエスト) | 作らない(機能設計 Q1。README に後の作業として書く。Step 15) |
| (NFR の設計: 書き出しの部品、種類の表、位置と Edit、git、上限と深さ、並行とメモリ) | Step 3〜Step 9、Step 12、Step 13 |
| (インフラの設計: CI、ルールファイル、確かめの補助、検証のスクリプト) | Step 14 |

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
