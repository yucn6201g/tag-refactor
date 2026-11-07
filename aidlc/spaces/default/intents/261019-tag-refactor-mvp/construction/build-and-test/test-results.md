# テストの結果

実行日: 2026-11-07。ブランチ `bolt/u8-iac`、コミット `da2dad9`。手元は Windows、Go 1.27.0。

## 全体

| 項目 | 結果 |
|---|---|
| ビルドと確かめ(14 コマンド) | すべて通過 |
| テストのコマンド(36 件) | すべて通過 |
| ファズ(3 件、各5分) | 通過、失敗の入力なし |
| `govulncheck`(Go 1.27.0) | **失敗**(標準ライブラリの脆弱性9件。下を参照) |
| `govulncheck`(Go 1.27.3) | 通過(`No vulnerabilities found`) |
| 飛ばしたテスト | 手元の結合テストのコマンドで7件(環境変数がないため。`verify.sh` の中で動く) |
| カバレッジ | 96.1%(statements) |

`go test` のパッケージ別の失敗はありません。`go test` は件数を一覧にしないので、合計の件数はここに書きません。

## ビルド

`CGO_ENABLED=0`、`-trimpath`。

| 組み合わせ | 大きさ(バイト) |
|---|---|
| linux/amd64 | 26,411,586 |
| linux/arm64 | 24,457,938 |
| darwin/amd64 | 26,867,872 |
| darwin/arm64 | 25,071,778 |
| windows/amd64 | 26,829,824 |
| windows/arm64 | 24,589,824 |

`go vet`(タグありとなし)、`gofmt -l`(出力なし)、`golangci-lint`(タグありとなし、0 issues)、`go mod verify`、`bash -n scripts/verify.sh`、`go vet scripts/u6check.go` が通りました。

## 動かさなかったコマンドと、その理由

| コマンド | 理由 | 代わりの根拠 |
|---|---|---|
| `go test ... -run 'TestGolden' ./internal/display/... -update` | 期待の出力のファイルを書き換えるコマンドで、確かめのコマンドではない | `TestGolden*` は `go test ./...` の中で動く |
| `go test -race ...`(4 件) | 手元に C コンパイラーがない | プルリクエスト #20 の `race` ジョブが通過 |
| `TAG_REFACTOR_IT_UNTAGGED_ARN=<...> ... TestIntegrationU2` | 値が検証の中で作られる | `scripts/verify.sh` の中で動く |

## 大きなテスト

| テスト | 結果 |
|---|---|
| `TestLargeIacU8`(1万リソース、3万変更、候補3,000ファイル) | 14.6 秒、ヒープの最大 244 MB、開いた回数 9,000 |
| 同じテスト、実際の git(`TAG_REFACTOR_LARGE_REAL_GIT=1`) | 16.0 秒、ヒープの最大 255 MB、開いた回数 9,000 |
| `TestLargeApplyU6`、`TestLargeRollbackU7`、`TestLargeAnalysisU5`、`TestPerformanceU2`・`U3`・`U4` | 上限以内で通過 |

## プルリクエストの CI(#20、`da2dad9`)

`test`(ubuntu、macos、windows × stable、oldstable)、`race`、`coverage`、`lint`(書き換えたテンプレートの cfn-lint を含む)、`vuln`、`modcheck`、`build`(3つの OS)、`ci-gate` が通過。`artifacts` は `main` 以外で飛ばします。

## 検証用アカウント

`scripts/verify.sh` を2026-11-07に3回動かし、3回目で `verify: OK`(終了コード0、03:38〜03:54 UTC)。詳しくは `construction/u8-iac/code-generation/code-summary.md`。そのあとに `TestIntegrationSandboxScanAndPlan` が通りました。

## 失敗の扱い

### `govulncheck`(手元の Go 1.27.0)

- **内容:** 標準ライブラリの脆弱性9件。外部のモジュールの指摘はありません。
- **原因:** 手元の Go の版です。コードの問題ではありません。
- **対応:** 段階の中の1回目の直しで、`GOTOOLCHAIN=go1.27.3` で動かし、指摘なしを確かめました(手元の Go の設定は変えていません)。CI の `vuln` ジョブも通っています。

### `NFR12`(Unverified)と、未カバーの ID

- **内容:** 配布(GitHub Releases、`go install`、チェックサム)を確かめられません。`AC6.2.1`〜`AC6.2.3`、`AC7.2.1`、`AC7.3.1`、`FR8.3`、`FR8.4` が未カバーです。
- **原因:** 生成したコードの誤りではありません。人が範囲の外と決めたものです(配布: units-generation の Q3、Q4。CDK とプルリクエストの作成: U8 の機能設計 Q1)。
- **示した直しの案:** 公開の仕組み(リリースの CI、チェックサム)を作る。作業は小さな Bolt 1つ、費用はほぼなし、リスクは低い(公開は人がタグを付けたときだけ)。CDK とプルリクエストの作成は、この案に含めていない。
- **人の判断(2026-11-07):** `Accept failure`。これまでの決定のとおり配布は別の作業のままとし、未達をここに記録して、この段階の確認に進む。Code Generation には戻していない(0/3)。
- **残る影響:** 配布の仕組みができるまで `v0.1.0` は出せない。この段階の準備の判定の「配布できる」は「いいえ」のまま。
