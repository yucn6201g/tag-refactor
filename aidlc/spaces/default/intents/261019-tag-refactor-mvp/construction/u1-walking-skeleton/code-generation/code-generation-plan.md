# コードの生成の計画: U1 最小の一本

## Summary

- Builds: 1つのリージョンを読む `scan` と、別名の名前の変更と IAM の管理ポリシーの影響を示す `plan` の最小版。CI(3つの OS)と、検証用アカウントの確認用のテンプレートも作る
- Touches: `go.mod`、`cmd/tag-refactor/`、`internal/`(cli、display、awsaccess、inventory、rules、planner、impact、errkind、testutil)、`docs/iam/`、`test/sandbox/`、`.github/`、`.gitattributes`、`README.md`
- Tests: 受け入れ条件のテスト13件(先に書く)、パッケージごとの単体テスト5〜8件(約50件)、性能・安全・出力の守りのテスト、検証用アカウントのテスト(手元だけ)。カバレッジ80%以上

## 前提と進め方

- 設計の出典: `construction/u1-walking-skeleton/` の機能設計、非機能要件、非機能の設計、インフラの設計。契約 `inception/contract-design/contract-summary.md`。
- **人の準備(E1):** コードの生成を始める前に、人が手元に Go(最新の安定版)、`golangci-lint`、`gh` を入れる。`govulncheck` は `go.mod` の `tool` で管理する。`cfn-lint` は CI で入れる(手元では任意)。
- **ブランチ:** Bolt のブランチ `bolt/u1-walking-skeleton` を `main` から作り、その上で作業する。マージはプルリクエストで、人が行う(チームの進め方)。
- **順番(Testing Contract の custom):** 受け入れ条件のテストを先に書いてコミットし、次に実装、実装の後に細かい単体テストを書き、最後にすべてのテストを実行する。
- **モジュール名:** `github.com/yucn6201g/tag-refactor`。
- **確かめながら決めること:** Go の安定版と1つ前の版の番号、`tag:GetResources` の1ページの最大件数、SDK の試行ごとの時間の上限のかけ方、再試行の回数の取り出し方、SSO の期限切れのエラーの見分け方、goccy/go-yaml の構文木の扱い。確かめた結果は `code-summary.md` に書く。

## 手順

- [x] **Step 1: 記録のコミット(人の依頼)** — `main` から `bolt/u1-walking-skeleton` を作り、ここまでの AI-DLC の記録(`aidlc/` の変更)を1つのコミット(`docs: add AI-DLC records through U1 infrastructure design`)にする。(対応: 依頼)
- [x] **Step 2: プロジェクトの骨組み** — `go.mod`(`go` 行は1つ前の版、`tool golang.org/x/vuln/cmd/govulncheck`)、`.gitattributes`(`* text=auto eol=lf`)、`.gitignore`、`cmd/tag-refactor/main.go`(入口とパニックの受け止め)、`internal/` の各パッケージの空の骨組み。(対応: NFR9.1、NFR9.2、NFR8.7)
- [x] **Step 3: テストの準備** — `internal/testutil/`: HTTP の層の偽の AWS(決まった応答、受け取った操作の記録、ページ分け、遅延、エラー)、環境の隔離の手助け(一時的な AWS の設定ファイル、プロファイルの環境変数の上書き、EC2 のメタデータの無効化)、時計の差し込み。`unit-test-instructions.md` のコマンドが動くことを確かめる。(対応: NFR の設計 テストの差し込み口)
- [x] **Step 4: 受け入れ条件のテスト(先に書く)** — `internal/cli/acceptance_test.go` に、コマンドの入口から流す Given/When/Then のテストを書き、このステップだけで1つのコミットにする。対象: AC1.1.8、AC1.2.1、AC1.2.2、AC1.2.3、AC1.2.4、AC1.2.5、AC2.1.2、AC3.1.8、AC3.2.1、AC3.2.4、AC3.2.5、AC6.1.3、AC6.1.4。この時点では失敗する(実装がない)。(対応: US1.2、US2.1、US3.1、US3.2、US6.1、NFR10.2)
- [x] **Step 5: エラーの種類(`internal/errkind`)** — 契約 C12 の種類(`Internal` を含む)と、案内の文言の手がかり。(対応: C12、BR6.3)
- [x] **Step 6: AWS へのアクセス(`internal/awsaccess`)** — SDK の設定(プロファイル、リージョン、試行ごと30秒・1回の呼び出しで120秒の上限、標準の再試行、SDK のログを出さない)、認証情報の解決(30秒の上限)、AccountResolver、リージョンの確認、TagReader(続きの印が空になるまで読む)、PolicyReader(付いている利用者の管理ポリシー、既定の版)、エラーの種類への変換、`--debug` の記録先のインターフェース、呼ぶ操作の一覧の表。書き込みの口は作らない。(対応: US1.2、BR1.1〜BR1.6、BR1.8、BR5.1、NFR3.1、NFR5.1、NFR5.2、NFR8.1、NFR8.3、NFR8.8、NFR1.3)
- [x] **Step 7: 収集(`internal/inventory`)** — タグの付いたリソースの収集、ProgressSink、キーの正規化とまとまり、並び順。(対応: US1.1 の最小版、BR2.1〜BR2.4、NFR1.4)
- [x] **Step 8: ルールファイル(`internal/rules`)** — 通常のファイルだけ、1 MiB の上限(読んだバイト数で判定)、改行の正規化、YAML の構文木でアンカー・別名・統合キー・重複キーを拒む、知らないキーと型の誤り(行、キー、直し方)、`version`、別名の検証、`aws:` の警告。(対応: US2.1 の最小版、BR3.1〜BR3.3、NFR8.4)
- [x] **Step 9: 変更案(`internal/planner`)** — 名前の変更、まとめ(`mergesIntoExisting`)、ぶつかり、changeId と並び順、変更0件、影響分析のインターフェースと型(TagChangeInput、ImpactResult)。(対応: US3.1 の最小版、BR4.1〜BR4.4、C9)
- [x] **Step 10: 影響分析(`internal/impact`)** — 条件キーの取り出し(`<前置>:ResourceTag/<キー>`)、大文字・小文字の扱い、ポリシー変数は `?`、ワイルドカードは線形時間の照合、読めないポリシーは不完全、影響の単位と並び順。(対応: US3.2 の最小版、BR5.1〜BR5.6、NFR1.5)
- [x] **Step 11: 表示(`internal/display`)** — 一覧の形(記号、英語)、標準出力と標準エラーの分け方、制御文字の置き換え(Cc、Cf、Zl、Zp、不正な UTF-8、バックスラッシュ)、進み具合と `--debug` の行、エラーの2行。(対応: US6.1 の最小版、BR6.1、BR6.2、NFR8.6、NFR11.1〜NFR11.4)
- [x] **Step 12: コマンド(`internal/cli`)** — cobra(`SilenceErrors`、`SilenceUsage`)、`scan` と `plan`、オプション(BR1.7、`--debug`)、組み立て(読み取りのインターフェースだけ)、終了コード(BR6.3)、時間と件数の行(NFR1.1)。(対応: US1.1、US1.2、US3.1、US3.2、US6.1、BR1.7、BR6.3)
- [x] **Step 13: 受け入れ条件のテストを通す** — Step 4 のテストがすべて通ることを確かめる。
- [x] **Step 14: 細かい単体テスト(実装の後)** — パッケージごとに5〜8件。BR ごとのテスト、安全のテスト(偽のキーが出ない、`--debug` にタグの値が出ない、SDK のログが無効)、ページ分け(空のページ + 続きの印)、時間の上限と再試行、性能のテスト(1万件、ポリシー500件 × 変更1,000件、病的なワイルドカード)、出力の守りのテスト(`display` と `cmd` の外から標準出力・標準エラーに書かない)。(対応: NFR1.3〜NFR1.5、NFR3.1、NFR5.1〜NFR5.3、NFR8.1、NFR8.2、NFR8.6、NFR8.8、NFR10.1)
- [x] **Step 15: 読み取り用の IAM ポリシーの例と確認用のテンプレート** — `docs/iam/readonly-policy.json`、`test/sandbox/u1-fixtures.yaml`(QA、QB、QC、B1、P1〜P3、R1、R2。名前は自動)、`test/sandbox/u1-rules.yaml`。テスト: 例の操作が読み取りだけで `*` を含まない、awsaccess の操作の表と一致する、テンプレートの R2 と一致する。(対応: NFR3.2、インフラの設計)
- [x] **Step 16: 検証用アカウントのテスト** — ビルドタグ `integration`、環境変数 `TAG_REFACTOR_IT_PROFILE` のときだけ動く `internal/cli/integration_test.go`(期待結果の表を確かめる)。(対応: NFR1.2、Q4)
- [x] **Step 17: CI** — `.github/workflows/ci.yml`(test、coverage、lint(gofmt、golangci-lint、`go vet -tags integration`、cfn-lint)、vuln、modcheck、build、ci-gate、artifacts)、`.github/workflows/scheduled.yml`、`.github/dependabot.yml`。Actions は SHA で固定、`contents: read`、`GOTOOLCHAIN=local`、時間の上限、`concurrency`。(対応: NFR8.5、NFR8.7、NFR9.1、NFR9.2、NFR10.1)
- [x] **Step 18: 全体の確認(手元)** — `gofmt -l`、`go vet ./...`、`go vet -tags integration ./...`、`golangci-lint run`、`go tool govulncheck ./...`、`go test ./...`、カバレッジ80%以上(`-coverpkg=./...`)。下限を下げて通すことはしない。
- [x] **Step 19: ドキュメント** — `README.md`(U1 でできること、使い方、検証用アカウントの手順へのリンク、必要な権限)。

## ストーリーとの対応

| ストーリー | 手順 |
|---|---|
| US1.1(最小版) | Step 6、Step 7、Step 12 |
| US1.2 | Step 4、Step 6、Step 12、Step 14、Step 15 |
| US2.1(最小版) | Step 8、Step 9 |
| US3.1(最小版) | Step 9、Step 11、Step 12 |
| US3.2(最小版) | Step 10、Step 12 |
| US6.1(最小版) | Step 11、Step 12 |

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
