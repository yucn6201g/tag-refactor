# コードの要約: U1 最小の一本

承認された計画(`code-generation-plan.md`)の19手順を、すべて終えました。コードは作業ブランチ `bolt/u1-walking-skeleton` にあります(まだ push していません)。

## 作ったもの

- `tag-refactor scan`: 1つのリージョンのタグの付いたリソースを読み、大文字・小文字と区切り文字だけで表記ゆれをまとめて表にします。
- `tag-refactor plan --rules <file>`: ルールファイルの別名から名前の変更、ぶつかりを作ります。さらに、付いている利用者の IAM の管理ポリシーの `<前置>:ResourceTag/<キー>` の条件と照らして、`!` と `?` を示します。
- 共通: `--profile`、`--region`、`--debug`。終了コードは 0、1、2(BR6.3)。結果は標準出力に、それ以外は標準エラーに出します。
- CI(GitHub Actions)、読み取り用の IAM ポリシーの例、検証用アカウントの確認用のテンプレート、README。

## 作った・変えたファイル

| 区分 | ファイル |
|---|---|
| ルートと設定 | `.gitattributes`、`.gitignore`(変更)、`go.mod`、`go.sum`、`README.md` |
| CI | `.github/workflows/ci.yml`、`.github/workflows/scheduled.yml`、`.github/dependabot.yml` |
| 入口 | `cmd/tag-refactor/main.go`、`main_test.go` |
| `internal/awsaccess` | `awsaccess.go`、`session.go`、`invoke.go`、`read.go`、`awsaccess_test.go`、`docs_test.go` |
| `internal/cli` | `cli.go`、`commands.go`、`messages.go`、`acceptance_test.go`、`cli_test.go`、`messages_test.go`、`integration_test.go` |
| `internal/display` | `display.go`、`results.go`、`display_test.go`、`outputguard_test.go` |
| `internal/errkind` | `errkind.go`、`errkind_test.go` |
| `internal/impact` | `impact.go`、`policy.go`、`impact_test.go` |
| `internal/inventory` | `inventory.go`、`inventory_test.go` |
| `internal/planner` | `planner.go`、`planner_test.go` |
| `internal/rules` | `rules.go`、`schema.go`、`rules_test.go` |
| `internal/testutil` | `fakeaws.go`、`responses.go`、`env.go`、`clock.go`、`testutil_test.go` |
| 文書と確認用 | `docs/iam/readonly-policy.json`、`test/sandbox/u1-fixtures.yaml`、`test/sandbox/u1-rules.yaml` |

## コミット(テストの順番)

| コミット | 内容 |
|---|---|
| 017ea47 | AI-DLC の記録(Step 1) |
| 5e4fee0 | 骨組みとテストの手助け(Step 2〜3) |
| 2c97588 | 受け入れ条件のテスト13件(Step 4。実装より前) |
| 3dc58e7 | scan と plan の実装(Step 5〜12) |
| 094f34e、0534878 | 整形と、表示の書き込みエラーの修正 |
| ab4bf2d | 細かい単体テスト、安全・性能・出力の守りのテスト(Step 14。実装の後) |
| 46d5b74 | IAM の例と確認用のテンプレート(Step 15) |
| 958fff8 | 検証用アカウントのテスト(Step 16) |
| c50ab5f | CI、週1回の govulncheck、Dependabot(Step 17) |
| a089588 | README(Step 19) |

## テストと確認の結果

オーケストレーターが開発担当の報告とは別に、手元で再実行して確かめました。

| 確認 | 結果 |
|---|---|
| `go test -mod=readonly`(10パッケージ) | すべて ok |
| カバレッジ(`-coverpkg=./cmd/tag-refactor/...,./internal/...`、文単位) | 合計 97.1%(下限80%) |
| `gofmt -l` | 差分なし |
| `go vet -tags integration ./...` | ok |
| `golangci-lint run`(2.15.0、既定の linter) | 0 issues(開発担当の実行) |
| `go mod verify`、`go mod tidy` の差分 | ok、差分なし(開発担当の実行) |
| `govulncheck` | Go 1.27.0 では標準ライブラリの8件(1.27.2 で修正済み)が出た。Go 1.27.3 で再実行し「No vulnerabilities found.」 |
| `CGO_ENABLED=0` のビルドと、空のディレクトリからの `--help` | 終了コード 0(開発担当の実行) |
| 性能 | 1万件の scan と plan が約0.08秒、ポリシー500件 × 変更1,000件が約0.51秒(上限10秒) |

テストの数: 受け入れ条件13件、各パッケージの単体テスト(Standard の5〜8件の目安)、安全のテスト(偽のキーが出ない、`--debug` にタグの値が出ない、SDK のログが無効)、性能のテスト、出力の守りのテスト。

## 確かめながら決めたこと

| 項目 | 結果 |
|---|---|
| Go の版 | `stable` = 1.27.3、`oldstable` = 1.26.10。`go.mod` は `go 1.26.0`。依存の `go` 行はすべて 1.26.0 以下 |
| `tag:GetResources` の1ページ | 最大100(`ResourcesPerPage=100`)。`iam:ListPolicies` は `MaxItems=1000` |
| 試行ごとの30秒 | HTTP クライアントの `Timeout`。これなら SDK の再試行の対象になる。1回の呼び出し全体の120秒は呼び出しごとの文脈でかける |
| 再試行の回数 | SDK の再試行の後ろに試行を数える部品を差し込み、「試行回数 − 1」を出す。数えられないときは `retries=n/a` |
| SSO の期限切れ | `ssocreds.InvalidTokenError` で見分ける。`sso_session` の形では型がないため、プロファイルが SSO を使うかを設定から判定する |
| YAML の構文木 | goccy/go-yaml の `parser.ParseBytes` で読み、ツールがアンカー、別名、統合キー、タグ、重複キー、深さ32を超える入れ子を拒む |

## 計画・設計からの違い

| 違い | 理由 |
|---|---|
| ぶつかりの表示は「別名、正しいキー」の順 | 機能仕様の出力例(`env="dev"  Environment="prod"`)に合わせた |
| 影響の行に条件の演算子を足した(例 `StringEquals aws:ResourceTag/env = "dev"`) | どの条件かを見分けやすくするため(BR5.6 の影響の単位に演算子が入っている) |
| 変更が0件でぶつかりだけのときも、影響分析をしない | 変更がなければ影響はありえない。「No changes needed」は変更もぶつかりもないときだけ(BR4.4 のとおり) |
| 契約 C9 の `Analyze` は `PolicyReader` を引数に取らず、実装が持つ | `planner` から `awsaccess` への依存を作らないため(`logical-components.md` の依存の表) |
| 契約 C6 の PolicyReader を、一覧と1件の読み取りの2つに分けた | 1件だけ読めなくても残りを続けるため(BR5.5) |
| 時間の上限のテストは、上限を数十ミリ秒にして確かめた。偽の時計は時間の表示に使う | 実時間を長く待たずに、SDK の本物の動きで確かめるため |
| R1・R2 の信頼の設定に、リージョンの段がない SSO のロールの ARN の形も加えた | Identity Center が us-east-1 にある場合の形に合わせるため |
| CI の cfn-lint の対象は `test/sandbox/u1-fixtures.yaml` だけ | `u1-rules.yaml` はテンプレートではないため |

これらの違いは、契約と設計の意図(読み取りだけ、見逃さない側、依存の向き)を変えていません。契約 C6・C9 の形の細部の変更は、後の作業単位(U2、U4、U5)が引き継ぎます。

## コードのレビュー(`reviews/review-01.md`、READY)を受けた修正

| 指摘 | 修正 | コミット | 確かめるテスト |
|---|---|---|---|
| R-01(中): 深い入れ子のルールファイルで、YAML のライブラリが2乗の時間とメモリを使い、20万段で out of memory | ライブラリに渡す前に1回走査し、括弧とインデントの入れ子が32段を超えたら InvalidInput(`internal/rules/nesting.go`)。上限を 1 MiB から 64 KiB に下げた | 575ab42 | `TestDeepNestingIsRejectedBeforeTheParserRuns`(20万段で2秒以内に InvalidInput) |
| R-02(中): 有効でないオプトインのリージョンが「認証情報の誤り」に見える | 指定したリージョンの STS が認証情報を拒否したときだけ、us-east-1 で有効なリージョンを確かめ直す(先に us-east-1 を呼ぶと、SCP で us-east-1 を拒否する組織で正常な実行まで失敗するため) | 3430684 | `TestResolveRechecksARejectionByStsAgainstTheEnabledRegions` ほか |
| R-03(低): 契約 C6・C9 が実装と違う | 契約の要約(`contract-summary.md`)を実際の形に改訂した(記録側) | — | — |
| R-04(低): 調べた範囲が出力にない | 標準エラーに `Checked:` の行を出す。README に範囲を書いた | c3da490 | `TestScanAndPlanStateWhatTheyChecked` |
| R-05(低): 追跡の表の対応先のずれ | BR4.4 を `commands.go`、BR5.6 を `impact.go` に直した(記録側) | — | 追跡の検査 |
| R-06(低): 例のポリシーの `iam:GetPolicyVersion` が `*` | 別の文にし、`arn:aws:iam::<account-id>:policy/*` に絞った。確認用のテンプレートの R2 も合わせた | ec5e465 | `TestReadOnlyPolicyExampleLimitsPolicyVersionsToTheAccount`、`TestSandboxReadOnlyRoleMatchesThePolicyExample` |
| R-07(低): `keys` がないファイルが黙って通る | 警告 `no key rules` を出す | e0e9274 | `TestMissingOrEmptyKeysGiveAWarning` |
| R-08(低): 確かめていない点 | 統合テストに、P1 の `! IAM policy` の行の検査を足した。Action の SHA、Go 1.26、P2 の受理は、最初の push の CI と U1 のチェックポイントで確かめる | e9f9fe9 | (手元では実行しない) |
| R-09(低): 機能仕様の出力の例が古い | 例に条件の演算子を入れた(記録側) | — | — |

修正の後に、オーケストレーターが手元で再実行しました。テストは10パッケージすべて ok、カバレッジの合計は 97.1%、gofmt は差分なし、`go vet -tags integration` は ok でした。golangci-lint は 0 issues、govulncheck(Go 1.27.3)は「No vulnerabilities found」で、この2つは開発担当が実行しています。

**人の判断が要ること:** R-01 の修正で、開発担当は指示より1つ広げました。入れ子に数えない括弧(引用符の中、コメント、テキストの中)が合わせて1024個を超えるファイルも InvalidInput にしています。走査の見誤りで深い入れ子を見逃さないための安全策です。タグのキーと値には括弧を使えないため、普通のルールファイルには影響しません。

## 確かめられなかったこと

- **cfn-lint を手元で動かしていない:** 代わりに aws-iac の MCP の cfn-lint で検査し、0 errors / 0 warnings でした。
- **CI の実際の実行:** push していないため未確認です。
- **oldstable(1.26.10)での実行:** 手元にその版がないため未確認です(CI のマトリクスで確かめる)。
- **検証用アカウントのテスト:** 実行していません(コンパイルと vet だけ)。U1 のチェックポイントで、人が確認用のスタックをデプロイしてから動かします。
- **SSO の文脈と、認証情報の解決の30秒の上限:** この経路を起こすテストは書けていません。
- **手元の Go:** 手元に入れた Go は 1.27.0(winget の版)です。標準ライブラリの修正が入った 1.27.2 以上に上げることを勧めます。
