# CI の構成

CI は GitHub Actions です。設定はリポジトリの `.github/` にあり、この段階では CI のファイルを変えていません(U8 のプルリクエスト #20 の時点のものを使います)。この段階で変えたのは、GitHub の `main` のルールセットだけです。

## ワークフロー

| ファイル | 動くとき | 中身 |
|---|---|---|
| `.github/workflows/ci.yml` | プルリクエストと `main` への push | 下のジョブ |
| `.github/workflows/scheduled.yml` | 毎週月曜 03:17 UTC と手動 | `main` の `govulncheck`。マージのあとに公表された脆弱性を見つける |
| `.github/dependabot.yml` | 週1回 | Go の依存と GitHub Actions の更新のプルリクエスト |

## `ci.yml` のジョブ

| ジョブ | 環境 | 動かすもの |
|---|---|---|
| `test` | ubuntu、macos、windows × Go stable、oldstable | `CGO_ENABLED=0 go test -mod=readonly ./...` |
| `race` | ubuntu、Go stable | `CGO_ENABLED=1 go test -race -mod=readonly ./...` |
| `coverage` | ubuntu、Go stable | `-coverpkg` で `cmd` と `internal` を測り、80% 未満で失敗 |
| `lint` | ubuntu、Go stable | `gofmt -l`、`golangci-lint` v2.15.0、`go vet -tags integration`、`cfn-lint` 1.57.2、`iac --write` で書き換えたテンプレートの `cfn-lint` |
| `vuln` | ubuntu、Go stable | `go tool govulncheck ./...` |
| `modcheck` | ubuntu、Go stable | `go mod verify`、`go mod tidy` の差分なし |
| `build` | ubuntu、macos、windows、Go stable | `CGO_ENABLED=0 go build -trimpath`、空のフォルダーで `--help` |
| `ci-gate` | ubuntu | 上のすべてが成功したときだけ成功。**必須のチェックはこれだけ** |
| `artifacts` | ubuntu | `main` への push で `ci-gate` のあとに動く。5つの組み合わせをクロスビルドし、7日間残す。公開しない |

## 共通の設定

| 項目 | 内容 |
|---|---|
| 権限 | `contents: read` |
| Actions の固定 | すべて SHA で固定。Dependabot が更新する |
| Go のツールチェーン | `GOTOOLCHAIN=local`。マトリクスの版だけを使う |
| 時間の上限 | 各ジョブ 15 分 |
| 取り消し | プルリクエストの古い実行を取り消す |
| AWS | 認証情報を持たず、呼ばない。実際のアカウントの確かめは手元の `scripts/verify.sh` だけ |
| 秘密の値 | CI に秘密の値はない |

## `main` のルールセット(GitHub の設定、ID 25636149)

| 規則 | 内容 |
|---|---|
| 削除の禁止 | 有効 |
| 強制 push の禁止 | 有効 |
| プルリクエストの必須 | 有効。承認の人数は 0(一人で進めるため) |
| 必須のステータスチェック | **`ci-gate`(GitHub Actions)。2026-11-07 にこの段階で足した。それまでは一覧が空だった** |
| 例外(bypass) | なし |

リポジトリの設定で、マージの方法は squash だけ、マージのあとにブランチを消します。

### 戻し方

ルールセット `main` の `required_status_checks` の一覧を空に戻します。GitHub の Settings の Rules、または API で変えられます。

## 成果物

| 種類 | 置き場所 | 期間 |
|---|---|---|
| 確認用のバイナリ | GitHub Actions の成果物 | 7日間 |
| 公開のバイナリ、チェックサム | **まだない**(別の作業) | — |

ECR、CodeArtifact、S3 は使いません。S3 は、利用者が `apply --record-to` で選ぶ記録の置き場所としてだけ使います。

## CI で動かしていないもの

| もの | 動く場所 | 理由 |
|---|---|---|
| 検証用アカウントの確かめ(`scripts/verify.sh`、`TestIntegration*`) | 手元だけ | CI から実際の AWS を呼ばない(チームの決まり)。チェックポイントで人が許可して動かす |
| `TAG_REFACTOR_LARGE_REAL_GIT=1` の大きなテスト | 手元だけ | 実際の git で時間がかかる |
| ファズの探索(`-fuzz`) | 手元だけ | 時間がかかる。`go test` は登録済みの種だけを動かす |
| `windows/arm64` のクロスビルド | 手元で確かめた | `artifacts` は5つの組み合わせだけ。配る組み合わせは配布の作業で決める |
