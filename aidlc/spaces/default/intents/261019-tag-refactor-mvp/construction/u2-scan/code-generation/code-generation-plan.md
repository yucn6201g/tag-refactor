# コードの生成の計画: U2 収集

## Summary

- Builds: 全リージョンを同時に8つまで読む収集、AWS Config と Resource Explorer による一度もタグが付いていないリソースの検出、略語の辞書と必須タグの欠落、進み具合の書き換え、再試行と時間の上限の変更、レース検出の CI と U2 の確認用のテンプレートと `scripts/verify.sh`
- Touches: `go.mod`、`internal/`(awsaccess、inventory、display、cli、errkind、testutil)、`docs/iam/readonly-policy.json`、`test/sandbox/`(u1-fixtures.yaml の R2、u2-fixtures.yaml、u2-rules.yaml)、`scripts/`(verify-u1.sh の `-run`、verify.sh)、`.github/workflows/ci.yml`、`README.md`
- Tests: 受け入れ条件のテスト約15件(先に書く)、パッケージごとの単体テスト5〜8件(約45件)、性能・決定性・取り消し・安全のテスト、`-race` の CI、検証用アカウントの結合テスト(手元だけ)。カバレッジ80%以上

## 前提と進め方

- 設計の出典: `construction/u2-scan/` の機能設計、非機能要件、非機能の設計、インフラの設計。契約 `inception/contract-design/contract-summary.md`(C3、C6、C7、C11、C12)。要件 `inception/requirements-analysis/requirements.md`、ストーリー US1.1、US1.2。
- **ブランチ:** `bolt/u2-scan`(`main` から作成済み)。ここまでの AI-DLC の記録はコミット済み(`453a166`)。マージは人がプルリクエストで squash マージする。
- **既存のコードに手を入れる:** U1 のファイルをその場で直す。同じ役割のファイルを別に作らない。
- **順番(Testing Contract の custom):** 受け入れ条件のテストを先に書いてコミットし、次に実装、実装の後に細かい単体テストを書き、最後にすべてのテストを実行する。
- **確かめながら決めること(結果は `code-summary.md` に書く):**
  - AWS Config の問い合わせで `SELECT arn, resourceType, resourceId, awsRegion, tags` が受け付けられるか(AWS のドキュメントと、チェックポイントでの実機の呼び出し)。受け付けられないときは、Config の経路を「失敗」として Resource Explorer に進む設計のまま動くことをテストで確かめる。
  - 呼び出しの途中で SDK が認証情報を取り直して失敗したときのエラーの型(`CredentialsMissing` に変える)。
  - `ListResources` の 401 の SDK のエラーの名前(`UnauthorizedException`)と、`GetDefaultView` が既定のビューのないときに返す形(空の `ViewArn` か、エラーか)。
  - `ListIndexes` の1ページの最大件数と、索引の種類の値(`LOCAL`、`AGGREGATOR`)。
  - `ListResources` 以外の6つの操作の IAM の名前が API と同じであること(Service Authorization Reference)。
  - Windows で仮想端末の処理を有効にする方法(`golang.org/x/sys/windows` のコンソールの設定。`golang.org/x/term` の依存として既に入るもの)。
  - `retry.NewStandard` で再試行の予算をなくす設定(`ratelimit.None`)と、待ち時間の上限20秒の設定。

## 手順

- [x] **Step 1: 依存の追加** — `go.mod` に `github.com/aws/aws-sdk-go-v2/service/configservice`、`github.com/aws/aws-sdk-go-v2/service/resourceexplorer2`、`golang.org/x/sync`、`golang.org/x/term` を足す。どれも `go.mod` の `go` 行(1つ前の版)で動く版を選び、`go mod tidy` で `go.sum` を固定する。(対応: tech-stack-decisions.md の依存の決まり)
- [x] **Step 2: テストの準備の拡張(`internal/testutil`)** — 偽の AWS で、`X-Amz-Target` も `Action` もない要求は URL のパスの最後の部分を操作の名前にする(Resource Explorer)。処理中の要求の数を数える口。Config(記録の設定と状態、問い合わせのページ)と Resource Explorer(索引、既定のビュー、ビューの条件、一覧のページ、`MaxResults` が1000のときは `NextToken` を返さない、401)の応答の雛形。リージョンごとの遅れを乱数の種で決める口。`IsReadOperation` に `config:SelectResourceConfig` を名前で足す。`unit-test-instructions.md` のコマンドが動くことを確かめる。(対応: NFR3.4、NFR1.10、NFR5.7、NFR5.8、security-design.md §1)
- [x] **Step 3: 受け入れ条件のテスト(先に書く)** — `internal/cli/acceptance_u2_test.go` に、コマンドの入口から偽の AWS に流す Given/When/Then のテストを書き、このステップだけで1つのコミットにする。対象: AC1.1.1(17リージョン、調べた数の表示)、AC1.1.2(Config の経路と Resource Explorer の経路の両方で、一度もタグが付いていないリソースが結果に入る)、AC1.1.3(どちらも使えないとき理由が出て、終了コードが1にならない)、AC1.1.4(端末でないときの進み具合の追記の行)、AC1.1.5(1つのリージョンの拒否で先頭に「Results are incomplete」とリージョンと理由、終了コード1)、AC1.1.6(`env`、`Env`、`environment` が1つのまとまり。略語の辞書だけのまとまりに印)、AC1.1.7(`Owner` を必須にしたルールファイルで欠落の件数が一致する。タグを変更できない種類は別に示す)、AC1.1.8(問題なしと件数。検出が partial のときの添え書き)、AC1.1.10(呼び出し回数の上限を3回返したあと最後まで終わる)、AC1.2.1(例のポリシーの操作だけで scan と plan が動き、書き込みの操作も API の `Search` も呼ばれない)、AC1.2.2(足りない操作の IAM の名前が出る。`resource-explorer-2:Search` を含む)、AC1.2.3 と AC1.2.5(収集の途中の認証情報の期限切れで標準出力が空、終了コード1、認証情報の文字列が出ない)。この時点では失敗する。(対応: US1.1、US1.2、NFR10.2)
- [x] **Step 4: エラーの種類(`internal/errkind`)** — 理由 `no-default-view` を足す(種類は `AccessDenied`)。(対応: C12、security-design.md §4)
- [x] **Step 5: AWS へのアクセスの変更(`internal/awsaccess`)** — 操作の表を `Operation{API, IAMAction, NeedsPermission}` に変え、U2 の7行を足す(`ListResources` の IAM は `resource-explorer-2:Search`、`GetView` を含む)。再試行を最大10回、待ち時間の上限20秒、再試行の予算なし(`ratelimit.None`)にし、1回の呼び出しの上限を120秒から5分にする(試行は30秒のまま)。U1 のテストを新しい値に合わせて直す。エラーの名前の表に `InternalServerException` など足りない一時的な失敗の名前と、各サービスの期限切れの名前を足す。呼び出しの途中の認証情報の取り直しの失敗を `CredentialsMissing` にする。(対応: NFR5.4、NFR5.5、NFR8.10、NFR8.11、security-design.md §1・§4、reliability-design.md §1)
- [x] **Step 6: Config と Resource Explorer の読み取り(`internal/awsaccess`)** — `ConfigReader`(記録の設定と状態、問い合わせを `Limit` 100 で最後まで読む。1件ずつ JSON として読み、`resourceType` がない、読めない、256 KiB を超える結果はその経路の失敗にする)。`ExplorerReader`(`ListIndexes` を `NextToken` がなくなるまで読む、`GetDefaultView`、`GetView`、`ListResources` を `MaxResults` 500 と `tag:none` で最後まで読む)。`ViewArn` が空なら `no-default-view`、ビューがあって 401 なら `AccessDenied`(IAM の名前 `resource-explorer-2:Search` を添える)。401 は共通の表に入れない。API の `Search` は呼ばない。(対応: BR3.1、NFR1.10、NFR5.6、performance-design.md §1・§2、reliability-design.md §3)
- [x] **Step 7: 収集の変更(`internal/inventory`)** — 全リージョンを `errgroup` の `SetLimit(8)` で読む。リージョンの失敗はエラーとして返さず結果に集める(`failedRegions`)。認証情報の誤りだけを共有の取り消しで全体に伝え、`Wait` のあとに返す。結果をリージョンの名前の順に合わせ、ARN の順に並べる。グローバルなリソースの重複を除く。CloudFormation の印(`iacManagedBy`)。進み具合の通知口に「待ち」「検出の段階」「検出のリージョンの状態」を足す。(対応: BR1.1、BR2.1〜BR2.4、BR4.1〜BR4.3、NFR1.9、NFR5.6〜NFR5.8、reliability-design.md §2・§4・§5)
- [x] **Step 8: 一度もタグが付いていないリソースの検出(`internal/inventory`)** — reliability-design.md §3 の分かれ道の表のとおり(全種類の記録の判定は `allSupported`、または除外が空の `EXCLUSION_BY_RESOURCE_TYPES`。`includeGlobalResourceTypes` は使わない)。集約の索引があればそのリージョンで1回読んで振り分け、索引のないリージョンは Config、Resource Explorer の順で試す。既定のビューに条件があれば `failed`。Config では利用者のタグ(`aws:` で始まらないキー)がないものを選ぶ。Resource Explorer の結果から IAM のリソースを除く。タグの取得が返した ARN を除き、Config と Resource Explorer が同じ ARN を返したら Config を残す。全体の状態(complete、partial、unavailable)を決める。(対応: BR3.1〜BR3.3、BR2.5、AC1.1.2、AC1.1.3)
- [x] **Step 9: 種類の対応表、タグを変更できない種類、略語の辞書、必須タグ(`internal/inventory`)** — `configToArnForm`(AWS Config の種類の形から ARN の形へ)と `notTaggableTypes` の表。略語の辞書(env、prod、dev、stg。キー全体の一致だけ)と「(abbreviation match)」の印。`--rules` の `required` のキーごとの欠落の件数(タグを変更できない種類は別に数える)、`aws:` の required のキーは無視して警告、下限の印。一度もタグが付いていないリソースの件数。(対応: BR2.5、BR3.3、BR5.1、BR5.2、BR5.5、AC1.1.6、AC1.1.7)
- [x] **Step 10: 表示(`internal/display`)** — 進み具合の書き換えと追記の2つの形(標準エラーだけで判定、`TERM=dumb` は追記、Windows は仮想端末の処理を有効にできたときだけ書き換え、0.2秒に1回まで、1つの書き手を鍵で守る)、端末の判定の差し込み口。scan の結果の新しい行(表記ゆれの印、`Required tags missing`、`Resources with no user tags`、`Untagged-resource discovery` と理由、下限の添え書き、タグを変更できない種類、`Checked:` の行に IAM のグローバルな種類を除くことの添え書き)。AccessDenied の理由は IAM の名前で出す。AWS からの文字列はすべて U1 の置き換えを通す。`--debug` の行に Config と Resource Explorer の呼び出し(API の名前)。(対応: BR5.4、BR5.7、BR6.1、NFR8.9、NFR9.3、NFR11.5、NFR11.6、observability-design.md)
- [x] **Step 11: コマンド(`internal/cli`)** — scan の `--rules` を受け付ける。scan と plan は既定で全リージョン(`--region` で絞れる)。1回の実行に1つの取り消しの仕組みを作って `inventory` と `impact` に渡す。scan の終了コード(BR5.3)、plan の不完全の扱い(BR5.6)。(対応: BR1.1、BR5.3、BR5.6、BR5.7、AC1.1.1、AC1.1.5)
- [x] **Step 12: 受け入れ条件のテストを通す** — Step 3 のテストと、U1 の受け入れ条件のテスト(`acceptance_test.go`)がすべて通ることを確かめる。U1 のテストで、U2 の変更(全リージョン、scan の出力の行の追加)により期待が変わるものは、U2 の仕様に合わせて直し、直した理由を `code-summary.md` に書く。
- [x] **Step 13: 細かい単体テスト(実装の後)** — パッケージごとに5〜8件。`awsaccess`(操作の表の照合、再試行の回数と待ち時間の上限、5分の上限、401 の2つの場合、Config の結果の検査、`MaxResults` 500)、`inventory`(分かれ道の表の5つの記録の設定、集約の索引の振り分け、ビューの条件、IAM の除外、重複の除き方と Config の優先、辞書、欠落の数え方、タグを変更できない種類、表の形の検査(ARN の形、重複なし))、`display`(書き換えの出力を差し込み口で固定、`TERM=dumb`、標準出力だけが端末、置き換え)、`cli`(終了コード、`--rules`)、`errkind`。(対応: NFR3.3、NFR5.4、NFR5.6、NFR8.9、NFR8.10、NFR11.5、tech-stack-decisions.md の表の確かめ方)
- [x] **Step 14: 性能・決定性・取り消しのテスト** — 17リージョン・1万件・一度もタグが付いていない1,000件・ポリシー100件の偽の AWS で、呼び出しの回数が performance-design.md §3 の式の上限以下(Config の経路、Resource Explorer の経路、両方、0件のリージョンを含む)、scan と plan の合計が30秒以内。乱数の種を10通り変えて標準出力が同じ。3つの段階(タグの取得、検出、plan の IAM の読み取り)で期限切れの各名前を返し、標準出力が空、終了コード1、戻った時点で処理中の要求が0、その後0.2秒新しい要求が来ない。`-race` 付きでの時間を手元(または CI の Linux)で測り、15分に収まらなければ、このテストだけ `-race` のときに件数を減らす(ビルドタグ `race` で切り替え。`-race` なしのテストは弱めない)。(対応: NFR1.6、NFR1.7、NFR5.7、NFR5.8、NFR10.3)
- [x] **Step 15: 例のポリシーと確認用のテンプレート** — `docs/iam/readonly-policy.json` に7つの IAM の操作を足す。読み取りだけの検査を「`Get`、`List`、`Describe` で始まるか、`config:SelectResourceConfig`、`resource-explorer-2:Search` のどちらか」に変え、誤りにする例のテストを足す。`test/sandbox/u1-fixtures.yaml` の R2 を例のポリシーと同じにする。`test/sandbox/u2-fixtures.yaml`(`AWS::ResourceExplorer2::Index` LOCAL、条件のない `View`、`DefaultViewAssociation`、パラメータ `CreateIndex` と `CreateDefaultView` と条件)。`test/sandbox/u2-rules.yaml`(U1 のルールに `required: [Owner]`)。手元で `cfn-lint` を通す。(対応: NFR3.3、NFR3.5、BR7.1、infrastructure-specification.md)
- [x] **Step 16: 検証用アカウントの結合テスト** — `internal/cli/integration_u2_test.go`(ビルドタグ `integration`、環境変数 `TAG_REFACTOR_IT_PROFILE` と、タグのないキューの ARN の環境変数 `TAG_REFACTOR_IT_UNTAGGED_ARN` があるときだけ動く。名前は `TestIntegrationU2...`)。infrastructure-specification.md の期待結果の表を確かめる: `ap-northeast-1` の scan が `complete` でキューを数える、全リージョンの scan は索引の実際の状態(下の Step 17 の事前確認の結果を環境変数で渡す)に従って `complete` か `unavailable`、ページの大きさ5と500で同じ ARN の集まり(テストだけの差し替え口)、Config の3つの操作を R2 から直接呼んで AccessDenied と `InvalidExpressionException` にならない、`--debug` の行から操作ごとの L95 を求めて記録する。(対応: NFR1.8、NFR1.10、NFR3.5、AC1.2.1)
- [x] **Step 17: 検証のスクリプト** — `scripts/verify-u1.sh` の `-run` を U1 の結合テストの名前 `^TestIntegrationSandboxScanAndPlan$` に絞る(ほかは変えない)。U1 の結合テストが U2 の scan の出力でも通ること(標準エラーに `Error:` がない、終了コード2)を確かめ、通らなければ U1 の結合テストの期待を U2 の仕様に合わせて直す。`scripts/verify.sh` を作る: (1) `verify-u1.sh` を呼ぶ、(2) 管理者のプロファイルで呼び出し元のアカウントを環境変数 `TAG_REFACTOR_SANDBOX_ACCOUNT_ID` と比べ、違えば止める、(3) U2 のスタックと `ap-northeast-1` の索引を確かめ、全リージョンの `list-indexes` で索引の種類を記録する(`ap-northeast-1` が集約の索引など前提と違えば、理由を出して止める)、(4) `ap-northeast-1` にタグのないキューを作り、終了の処理で必ず消す、(5) `resourcetype:sqs:queue tag:none` で、キューが現れるまで1分ごとに最大30分待つ。現れなければ「索引の中身がそろうまで数時間かかることがある。時間をおいて再実行する」と出して失敗にする、(6) U2 の結合テストを動かし、飛ばされたら失敗にする、(7) L95 と10分の見積もりを表示する、(8) Application Control で止められたら U1 と同じく再試行する。認証情報を表示しない(ARN と状態の値だけを読む)。(対応: cicd-pipeline.md の検証のスクリプト、インフラの設計のレビューの指摘)
- [x] **Step 18: CI** — `.github/workflows/ci.yml` に `race` ジョブ(ubuntu、Go の `stable`、`CGO_ENABLED=1`、`go test -race -mod=readonly ./...`、15分)を足し、`ci-gate` の `needs` に入れる。`cfn-lint` の対象を `test/sandbox/*-fixtures.yaml` にする。(対応: NFR5.7、cicd-pipeline.md)
- [x] **Step 19: 全体の確認(手元)** — `gofmt -l`、`go vet ./...`、`go vet -tags integration ./...`、`golangci-lint run`、`go tool govulncheck ./...`、`go test ./...`、`go test -race ./...`(手元で cgo が使えない場合は CI の `race` ジョブで確かめる)、カバレッジ80%以上(`-coverpkg`)。下限を下げて通すことはしない。
- [x] **Step 20: ドキュメント** — `README.md`(全リージョンの scan、`--rules`、一度もタグが付いていないリソースの検出と Config・Resource Explorer の前提、必要な権限の7つの追加、IAM のグローバルな種類の扱い、`scripts/verify.sh` の手順と環境変数、U2 のスタックのデプロイ)。

## ストーリーとの対応

| ストーリー | 手順 |
|---|---|
| US1.1(タグの状態を一覧で把握する) | Step 3、Step 6〜Step 11、Step 13、Step 14、Step 16 |
| US1.2(読み取りの権限だけで安全に使う) | Step 3、Step 4、Step 5、Step 10、Step 15、Step 16 |
| (インフラの設計) | Step 15、Step 17、Step 18 |

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
