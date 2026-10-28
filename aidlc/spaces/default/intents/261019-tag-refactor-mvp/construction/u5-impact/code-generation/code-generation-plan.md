# コードの生成の計画: U5 影響分析

## Summary

- Builds: 影響分析の完成(IAM のインラインポリシーと権限の上限、S3・KMS・SQS・SNS・Secrets Manager のリソースポリシーの読み取り、`ResourceTag`・`RequestTag`・`PrincipalTag`・`TagKeys` とポリシー変数の照合、4種類の変更の判定、同時に読む作り)、保存と出力への `conditionOperator`・`policyRef`・`region` の追加、読み取り用の IAM ポリシーの例と R2、U5 の確認用のスタックと `verify.sh` の U5 の確認
- Touches: `internal/impact/`、`internal/awsaccess/`、`internal/planner/`、`internal/display/`、`internal/cli/`、`internal/testutil/`、`docs/iam/readonly-policy.json`、`test/sandbox/`(`u1-fixtures.yaml`、新しい `u5-fixtures.yaml`、`u5-rules.yaml`)、`scripts/verify.sh`、`README.md`、`go.mod`
- Tests: 受け入れ条件のテスト6件(先に書く)、検出率100%の期待する判定の表(252行と32行)、パッケージごとの単体テスト5〜8件(約50件)、大きな偽の AWS のテスト(回数、10秒、同時の数、同じ結果)、ファジング(種の入力は CI)、検証用アカウントでの結合テスト1件(手元だけ)。カバレッジ80%以上

## 前提と進め方

- 設計の出典: `construction/u5-impact/` の機能設計(BR1.1〜BR5.4)、非機能要件(NFR1.16〜NFR1.19、NFR2.1、NFR2.2、NFR3.9、NFR5.10、NFR5.11、NFR8.20〜NFR8.22、NFR10.9)、非機能の設計(`security-design.md`、`logical-components.md`)、インフラの設計(`cicd-pipeline.md`)。契約 `inception/contract-design/contract-summary.md`(C2、C3、C9。U5 で足した項目と C9 の改訂を含む)。ストーリー US3.2(AC3.2.1〜AC3.2.6)。
- **ブランチ:** `bolt/u5-impact`(`main` の `2fe56af` から作成済み)。AI-DLC の記録のコミットは、人が頼んだときに行う。マージは人がプルリクエストで squash マージする。
- **既存のコードに手を入れる:** U1 の `internal/impact`(`impact.go`、`policy.go`)、U1・U2 の `internal/awsaccess`、U2 の `cli` の `stoppingPolicyReader` をその場で直す。同じ役割のファイルを別に作らない。新しいパッケージは作らない。
- **順番(Testing Contract の custom):** 受け入れ条件のテストを先に書いてコミットし、次に実装、実装の後に細かい単体テストを書き、最後にすべてのテストを実行する。
- **設計のレビューで残った Minor の扱い(この計画で決める):**
  - IAM の `NoSuchEntity` も「一覧のあとで消えた」に入れ、NotFound の追加は操作ごとの誤りのコードの表(`opErrorCodes`)に入れて、U1〜U4 の分類を変えない(NFR の設計のレビュー)。
  - `aws:TagKeys` に `Null` の演算子が付いた条件は undecided、文字列でない演算子の否定のもの(`ArnNotLike` など)は「その他」として undecided(NFR の設計 `logical-components.md`)。
  - `ValidatePolicy` は、RU5 のインラインポリシーをアイデンティティポリシー、ほかの5つをリソースポリシー(リソースの種類つき)として当てる。測る値の「上限に当たった回数」は「再試行が起きた行の数」と呼ぶ(インフラの設計のレビュー)。
- **AWS への変更は、この計画では行わない:** U1 のスタックの更新と U5 のスタックの作成は、コードを書いたあと、チェックポイントの前に、変更セットを人に示して許可を得てから行う(`cicd-pipeline.md` の手順)。
- **確かめながら決めること(結果は `code-summary.md` に書く):**
  - SDK のモジュールの版を、今の `go.mod` の AWS の SDK と同じ時期のものにできること。
  - S3 の `ListBuckets` の答えに `BucketRegion` が入ること(SDK の型で確かめる)。
  - KMS、SQS、SNS、Secrets Manager の誤りのコードの名前(SDK の型と AWS のドキュメントで確かめる)。

## 手順

- [x] **Step 1: テストの準備** — `unit-test-instructions.md` のコマンドが動くことを確かめる。HTTP の層の偽の AWS(`internal/testutil`)に、IAM のユーザー・ロール・グループとインラインポリシー、管理ポリシーの付いている数と権限の上限の数、S3(バケットの一覧、場所、ポリシー、ポリシーなし)、KMS(キーの一覧、情報、キーポリシー)、SQS(キューの一覧、属性)、SNS(トピックの一覧、属性)、Secrets Manager(シークレットの一覧、リソースポリシー)の応答を足す。一覧は `logical-components.md` の API ごとの件数で分けて返し、操作ごとの回数、同時に受けている要求の数、応答の遅れ(乱数)、誤りの応答を決められるようにする。(対応: Testing Contract の runner_step)
- [x] **Step 2: 受け入れ条件のテスト(先に書く。`internal/cli/acceptance_u5_test.go`)** — 偽の AWS を使った `plan` で、AC3.2.1(ロールのインラインポリシーの `aws:ResourceTag/env` が `env` → `Environment` で `!`、ポリシー `<ロールの ARN>#<名前>`・ステートメント・条件が出る)、AC3.2.2(S3 のバケットポリシーの値 `Prod` が `Prod` → `prod` で `!`)、AC3.2.3(タグの削除で `!`)、AC3.2.4(ポリシー変数の条件が `?`、ワイルドカードが当てはまる条件が `!`。影響なしとは出ない)、AC3.2.5(ロールのインラインポリシーの一覧が拒否されると、読めなかった種類と理由、「影響分析は不完全」、終了コード1、ほかの指摘は出る)、AC3.2.6(7種類のポリシーと参照の種類の組み合わせの代表がすべて検出される。全部の組み合わせは Step 11 の表)。この時点では失敗することを確かめ、これだけで1つのコミットにする。(対応: US3.2)
- [x] **Step 3: 読み取りの口(`internal/awsaccess`)** — `go.mod` に `service/s3`、`service/kms`、`service/sqs`、`service/sns`、`service/secretsmanager` を足す。`security-design.md` §1 の口(管理ポリシーの一覧は付いているものだけの指定を外し、付いている数と権限の上限の数を返す。インラインポリシー、バケットの一覧と場所とポリシー、キー、キュー、トピック、シークレット)。リージョンごとの接続。操作の表(API と IAM の操作の対応。`ListBuckets` は `s3:ListAllMyBuckets`)。誤りのコードの表(§1 の誤りの表。NotFound は操作ごとの表に入れ、IAM の `NoSuchEntity` も入れる。`NoSuchBucketPolicy` は誤りにしない)。API ごとの1ページの件数の定数。すべて U1 の `invoke`(時間の上限、再試行、`--debug` の行)を通す。(対応: BR1.1〜BR1.3、BR5.3、NFR3.9、NFR8.22)
- [x] **Step 4: 文書の読み取り(`internal/impact`)** — 1つの読み取りの関数(大きさ 1 MiB の上限と固定の理由、IAM だけ URL エンコードを戻す、Statement が1つでも配列でも、Sid か `#<番号>`)。タグへの参照の取り出し(BR2.1 の条件キー、BR2.3 の演算子の種類、BR2.4 の条件キーの中・条件の値・`Resource`・`NotResource` の中のポリシー変数。既定値つきの変数)。`FuzzParsePolicy`(種の入力は `security-design.md` §2 の4。1回1秒以内、`race` のビルドタグで10倍)。(対応: BR1.5〜BR1.7、BR2.1〜BR2.4、NFR8.20、NFR8.21)
- [x] **Step 5: 結び付けと判定(`internal/impact`)** — 変更の索引(変更のキー、リソースの ARN、IAM のユーザーとロールの変更。読むだけ)。結び付け(BR3.1〜BR3.3)。判定(BR4.1〜BR4.7。`TagKeys` と `Null` は undecided、文字列でない演算子の否定は undecided)。ワイルドカードの正規表現の使い回しの入れ物(索引とは別、ロックで守る、1,024個まで。U1 の `changeIndex.patterns` を移す)。(対応: BR3.1〜BR3.3、BR4.1〜BR4.7、AC3.2.1〜AC3.2.4)
- [x] **Step 6: 読み込みの流れと結果(`internal/impact`)** — `New(reader, regions)`。`logical-components.md` の「同時に読む作り」の段(IAM の配り手と4つの仕事、S3 の配り手と場所の4つの仕事、リージョンの8つの配り手と、リージョンの中の4つの仕事。仕事は配り手だけが足す)。失敗を UnreadPolicy に集める(一覧はリージョンつき、1つのポリシーは policyRef つき。消えたリソースはポリシーなし)。取り消されたら途中の結果を返さない。終わってから指摘をまとめ(BR5.1。判定ごとに1件、`Resource` の値のつなぎ方)、並べる(BR5.2 の7つの鍵、unreadPolicies の順)。(対応: BR1.1〜BR1.5、BR5.1、BR5.2、BR5.4、NFR1.18、NFR5.10、NFR5.11)
- [x] **Step 7: 保存と出力(`internal/planner`、`internal/display`)** — `UnreadPolicy` に `Region`。保存の JSON(C2)と JSON の出力(C3)に `impactFindings[].conditionOperator`、`unreadPolicies[].policyRef`・`region` を書き、`LoadPlan` でも読む(省略できる項目。今の保存した変更案も読めること)。一覧と Markdown の「読めなかったもの」にリージョンと policyRef を出す。英語と日本語の文言を足す(U4 の文言の一覧のテストを通す)。(対応: 契約 C2・C3 の U5 で足した項目、NFR8.22)
- [x] **Step 8: コマンドの入口(`internal/cli`)** — 変更案のリージョンを `impact.New` に渡す。`stoppingPolicyReader` を足した口のすべてに広げる(最初の1つの認証情報の誤りを覚えて実行を取り消す)。(対応: NFR5.10、security-design.md §4)
- [x] **Step 9: 例のポリシーと確認用のテンプレート** — `docs/iam/readonly-policy.json` に操作と `Resource` の範囲を足す(`security-design.md` §1)。`internal/awsaccess/docs_test.go` の例の検査に、`Resource` が `"*"` かその操作のサービスの ARN の形であることを足す。`test/sandbox/u1-fixtures.yaml` の R2 を同じ内容にする。`test/sandbox/u5-fixtures.yaml`(`cicd-pipeline.md` の10個のリソース、`${!aws:PrincipalTag/team}`、出力の6つの ARN)と `test/sandbox/u5-rules.yaml` を作る。`cfn-lint` を手元で通す。(対応: NFR3.9、NFR2.2)
- [x] **Step 10: 受け入れ条件のテストを通す** — Step 2 のテストと、U1〜U4 の受け入れ条件のテストがすべて通ることを確かめる。U1 の影響分析のテスト(`internal/impact/impact_test.go`)は `New(reader, regions)` と広げた口に合わせて直し、U1 で決めた振る舞い(付いている管理ポリシー、大文字・小文字だけの変更、変数、ワイルドカード、読めなかったとき)の期待は変えない。期待が変わるものは理由を `code-summary.md` に書く。
- [x] **Step 11: 細かい単体テスト(実装の後)** — 検出率100%の期待する判定の表(ポリシーの種類9 × 参照7 × 変更4 = 252行と、演算子8 × 変更4 = 32行。行の数がこの数と同じことも確かめる。NFR2.1)。`internal/impact`: 判定の各規則、結び付け、指摘のまとめ方、並べ方、使い回しの入れ物の上限、文書の大きさの境目(1 MiB + 1 バイトと、ちょうど 1 MiB)、URL エンコードを戻すのは IAM だけ。`internal/awsaccess`: 口ごとの読み取り(ページ、ポリシーなし、消えたリソース、誤りのコード)、管理ポリシーの付いている数と権限の上限。`internal/planner`: 足した項目の保存と読み込み。`internal/display`: 読めなかったものの表示、文言。`internal/cli`: 認証情報の誤りで止まる(足した口のそれぞれ、同時に2つ、案内と終了コード1)。(対応: BR1.x〜BR5.x、NFR2.1、NFR5.10、NFR8.20、NFR8.22)
- [x] **Step 12: 大きな偽の AWS のテスト** — 測る条件の分布(`logical-components.md`)で、呼び出しの回数が式の上限以下(置いた件数から式で計算する。NFR1.16)、遅れなしで `Analyze` が10秒以内(NFR1.17)、同時の数が IAM 4・リージョンの中 4・リージョン 8 を超えない、枠をすべて1にしても終わる(NFR1.18)、乱数の種を変えた10回で標準出力と保存した変更案が同じ(ワイルドカードを多く含める。NFR5.11)、取り消したあと goroutine が残らない(NFR5.10)。`race` のビルドタグのときは件数を10分の1にし、上限を10倍にする。U5 の大きなテストは CI の各 OS で合わせて2分以内。(対応: NFR1.16〜NFR1.18、NFR5.10、NFR5.11、NFR10.9)
- [x] **Step 13: 検証のスクリプト** — `internal/cli/integration_u5_test.go`(`-tags integration`。`TestIntegrationU5ImpactSandbox`、サブテストなし、`--profile` に `TAG_REFACTOR_IT_PROFILE`、`--lang en` と `LC_ALL=C`、`cicd-pipeline.md` の結合テストの手順1〜7。RU5 の変更が無いときはログに出して `Resource` の確かめだけを飛ばす)。`scripts/verify.sh`: U4 の確認のあと、U2 のキューの前に、U5 のスタックの状態と出力の確かめ、R2 の `iam:GetRolePolicy` の確かめ、`run_go_test ... -run '^TestIntegrationU5'`、`--- PASS: TestIntegrationU5ImpactSandbox (` がちょうど1つ、`--debug` 付きの plan を5回動かして操作ごとの標本の数と L95、種類ごとの最大、再試行が起きた行の数を出す。先頭のコメントの手順の説明を直す。(対応: cicd-pipeline.md の検証のスクリプトと測る値の記録、NFR1.19、NFR2.2)
- [x] **Step 14: 全体の確認(手元)** — `gofmt -l`、`go vet ./...`、`go vet -tags integration ./...`、`golangci-lint run`、`go test ./...`、カバレッジ80%以上(`-coverpkg`)、`govulncheck ./...`。手元でファジングを5分動かし、結果を `code-summary.md` に書く(Application Control でふさがれたら再実行)。下限を下げて通すことはしない。
- [x] **Step 15: ドキュメント** — `README.md` の影響分析の節(読むもの: 付いている管理ポリシーと権限の上限、インラインポリシー、5種類のリソースポリシー。読まないもの: ロールの信頼ポリシー、AWS が管理するポリシー、どこにも付いていない管理ポリシー、AWS が管理する KMS のキー、SCP と RCP など。`!` と `?` の意味、ワイルドカードは機械的に判定すること、リソースポリシーはそのリソース自身の変更だけに結び付くこと、`PrincipalTag` は IAM のユーザーとロールの変更だけに結び付くこと)、読み取り用の IAM ポリシーの例の説明、U5 の確認用のスタックの置き方と消し方。

## ストーリーとの対応

| ストーリー | 手順 |
|---|---|
| US3.2(変更で壊れる参照元を事前に知る) | Step 2〜Step 8、Step 10〜Step 12 |
| (非機能の設計: 読み取りだけ、文書の安全、失敗の扱い、同時に読む作り) | Step 3、Step 4、Step 6、Step 8、Step 11、Step 12 |
| (インフラの設計: 例のポリシー、確認用のスタック、検証のスクリプト) | Step 9、Step 13 |

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
