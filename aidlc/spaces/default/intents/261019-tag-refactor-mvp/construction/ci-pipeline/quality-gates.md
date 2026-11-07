# 品質の関門

## マージの前の関門

`ci-gate` が必須のチェックです(`main` のルールセット)。`ci-gate` は下のジョブがすべて成功したときだけ成功します。プルリクエストなしに `main` へは入れません。

| 関門 | 基準 | ジョブ | 根拠 |
|---|---|---|---|
| テスト | すべて通る。3つの OS × Go 2版 | `test` | チームの進め方、NFR9 |
| 並行の安全 | 競合の検出なし | `race` | NFR5.7 |
| カバレッジ | Go の文単位で 80% 以上 | `coverage` | チームの進め方、NFR10 |
| 整形 | `gofmt -l` の出力なし | `lint` | チームの進め方 |
| 静的解析 | `golangci-lint` の既定の linter で指摘なし | `lint` | チームの進め方 |
| テンプレート | `cfn-lint` が通る。`iac --write` で書き換えたものも通り、差分が4行 | `lint` | NFR3.21 |
| 脆弱性 | `govulncheck` の指摘なし | `vuln` | NFR8.7 |
| 依存 | `go mod verify`、`go mod tidy` の差分なし | `modcheck` | NFR10.13 |
| ビルド | `CGO_ENABLED=0` で3つの OS でビルドでき、`--help` が動く | `build` | NFR9 |

**下限を下げて通すことはしません**(チームの進め方)。

## Build and Test のコマンドとの対応

| Build and Test のコマンド | CI での関門 |
|---|---|
| `go build`(`CGO_ENABLED=0`) | `build`(`--help` まで) |
| `go test -mod=readonly ./...` | `test`(各作業単位の絞ったコマンドは、この中に含まれる) |
| `go test -race` | `race` |
| `go test -coverpkg=...`、`go tool cover` | `coverage`(80% 未満で失敗) |
| `gofmt -l`、`golangci-lint`、`go vet -tags integration` | `lint` |
| `go mod verify` | `modcheck` |
| `govulncheck` | `vuln` |
| `go vet scripts/u6check.go`、`bash -n scripts/verify.sh` | **関門なし**(`go vet ./...` の範囲に入るかを含め、確かめていない。手元のチェックポイントで動く) |
| `scripts/verify.sh`、`TestIntegration*` | **関門なし**(手元だけ。CI から AWS を呼ばない) |
| `-fuzz` の探索、実際の git の大きなテスト | **関門なし**(手元だけ) |

## マージのあとの関門

| 関門 | 基準 | 動くとき |
|---|---|---|
| `main` の CI | 同じジョブがすべて通る | `main` への push |
| 週1回の `govulncheck` | 指摘なし | 毎週月曜。失敗は GitHub の通知で分かる |

## 人の関門

| 関門 | 誰が | 根拠 |
|---|---|---|
| 作業単位のチェックポイント | 人が検証コマンドの結果と表示を確かめて承認 | AI-DLC の進め方 |
| squash マージ | 人 | チームの進め方 |
| 公開 | 人がタグ `vX.Y.Z` を付けて push(仕組みは別の作業) | チームの進め方 |

## この段階で埋めた穴

U1 のインフラの設計は「`ci-gate` だけを必須にする」と決めていましたが、ルールセットの必須のチェックは空でした。そのため、Dependabot の #11 と #12 は CI が赤のままマージされ、`main` の CI が U7 のプルリクエスト #14 まで赤のままになりました。2026-11-07 に `ci-gate` を必須にしました(人が許可)。Dependabot のプルリクエストも、この関門を通らないとマージできません。

## 必須にしたことを確かめた範囲

ルールセットの API の答えで、`ci-gate`(GitHub Actions)が一覧に入ったことと、#20 がマージできる状態のままであることを確かめました。**赤いプルリクエストが実際に止まることは試していません。**
