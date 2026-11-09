# 技術の構成

## 言語と実行環境

| 項目 | 内容 |
|---|---|
| 言語 | Go(`go.mod` の `go 1.26.0`)。CI は `stable` と `oldstable` の2つの版でテストする(`GOTOOLCHAIN=local` で別の版を取りに行かない) |
| ビルド | Go modules。配るバイナリは `CGO_ENABLED=0`、`-trimpath` の1つの静的なバイナリ。`linux/amd64`、`linux/arm64`、`darwin/amd64`、`darwin/arm64`、`windows/amd64` |
| 動く場所 | 利用者の端末(Linux、macOS、Windows)。サーバーには配置しない |
| 補助の言語 | bash(`scripts/verify.sh`、`scripts/verify-u1.sh`。手元の検証だけ)、YAML(CloudFormation の検証用スタック、ルールファイル、GitHub Actions) |
| 外部のプログラム | `git`(`iac --write` の確かめだけ、利用者のもの)。手元の検証では AWS CLI、`cfn-lint`(任意) |

## ライブラリ(直接の依存)

| モジュール | 版 | 使う場所と目的 |
|---|---|---|
| `github.com/spf13/cobra` | v1.10.2 | `internal/cli`。コマンドとオプション |
| `github.com/aws/aws-sdk-go-v2` | v1.47.4 | `internal/awsaccess`。AWS の SDK の本体(再試行、レート制限、HTTP) |
| `github.com/aws/aws-sdk-go-v2/config` | v1.33.11 | 共有の設定、プロファイル、SSO |
| `github.com/aws/aws-sdk-go-v2/credentials` | v1.20.11 | 認証情報(SSO の判定) |
| `github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi` | v1.41.5 | タグの読み書き(`GetResources`、`TagResources`、`UntagResources`) |
| `github.com/aws/aws-sdk-go-v2/service/sts` | v1.51.5 | 呼び出し元の確認 |
| `github.com/aws/aws-sdk-go-v2/service/ec2` | v1.339.0 | 有効なリージョンの一覧 |
| `github.com/aws/aws-sdk-go-v2/service/iam` | v1.64.4 | IAM のポリシーの一括の読み取り |
| `github.com/aws/aws-sdk-go-v2/service/s3` | v1.114.5 | バケットポリシー、記録と印のオブジェクト |
| `github.com/aws/aws-sdk-go-v2/service/kms` | v1.62.0 | キーポリシー |
| `github.com/aws/aws-sdk-go-v2/service/sqs` | v1.52.6 | キューポリシー |
| `github.com/aws/aws-sdk-go-v2/service/sns` | v1.47.5 | トピックポリシー |
| `github.com/aws/aws-sdk-go-v2/service/secretsmanager` | v1.50.4 | シークレットのリソースポリシー(値は読まない) |
| `github.com/aws/aws-sdk-go-v2/service/configservice` | v1.74.5 | AWS Config(一度もタグのないリソース) |
| `github.com/aws/aws-sdk-go-v2/service/resourceexplorer2` | v1.33.5 | Resource Explorer(一度もタグのないリソース) |
| `github.com/aws/smithy-go` | v1.28.6 | SDK のミドルウェア(試行の数え上げ、ログの抑止) |
| `github.com/goccy/go-yaml` | v1.19.2 | `internal/rules`。ルールファイルの YAML(構文木の検査) |
| `go.yaml.in/yaml/v3` | v3.0.6 | `internal/iac`。CloudFormation の YAML(ノードの木だけ。別名は展開しない) |
| `github.com/hashicorp/hcl/v2` | v2.25.0 | `internal/iac`。Terraform の `.tf`(`hclsyntax`。`hclwrite` は使わない)。MPL-2.0 |
| `golang.org/x/sync` | v0.24.0 | `errgroup`(収集、影響分析)、`semaphore`(iac の解析のメモリの予算) |
| `golang.org/x/sys` | v0.49.0 | Windows のコンソール(仮想端末、コードページ、表示言語) |
| `golang.org/x/term` | v0.47.0 | 端末の判定、確認の入力 |

ツールとして `golang.org/x/vuln`(v1.9.0)の `govulncheck` を `go.mod` の `tool golang.org/x/vuln/cmd/govulncheck` で固定しています(`go tool govulncheck ./...`)。間接の依存には `zclconf/go-cty` v1.19.0(hcl/v2)、`golang.org/x/text` v0.39.0(脆弱性 GO-2026-5970 の修正の版に上げた記録がコミットの履歴にある)などがあります。

テストは Go の標準の `testing` だけで、外部のアサーションやモックのライブラリはありません。AWS は `internal/testutil` の HTTP の層の偽物で置き換えます。

## 標準ライブラリの主な使い方

- `encoding/json`: 変更案、実行の記録、iac の CloudFormation JSON(トークンと `InputOffset` で範囲を取る)、`terraform show -json` の1リソースずつの読み取り。
- `crypto/sha256`、`crypto/sha1`: iac の2回の読み取りの比較と、git のブロブのハッシュ(SHA-1 は git のオブジェクトの名前のためで、安全のためではない)。
- `os/exec`: `git` の実行(シェルを通さない、`WaitDelay` と時間の上限)。
- `os/signal`: Ctrl+C と SIGTERM。
- `runtime/debug`: iac のソフトなメモリの上限(`SetMemoryLimit`)。

## 開発と CI の道具

| 道具 | 版 | 使い方 |
|---|---|---|
| `gofmt` | Go に付属 | 整形(`goimports`、`gofumpt` は使わない) |
| `golangci-lint` | v2.15.0(`ci.yml` の環境変数) | 既定の linter だけ。設定ファイルなし |
| `go vet -tags integration` | Go に付属 | 実際のアカウントの結合テストの型の確認(動かさない) |
| `govulncheck` | v1.9.0(`go.mod` の `tool`) | PR ごとと毎週 |
| `cfn-lint` | 1.57.2(`ci.yml` の環境変数) | 検証用のテンプレートと、iac が書き換えた写し |
| GitHub Actions | `actions/checkout` v7.0.1、`actions/setup-go` v7.0.0、`actions/setup-python` v7.1.0(Python 3.13)、`golangci/golangci-lint-action` v9.3.0、`actions/upload-artifact` v7.0.2 | コミットのハッシュで固定 |
| Dependabot | — | gomod と github-actions を毎週。接頭辞 `chore(deps)` と `ci` |

## 今回の意図に関わる技術の空白

- TypeScript と Python のソースを読む部品は、今の依存にありません。CDK の反映には、それぞれのパーサー(または字句の解析)を選ぶ必要があります。選ぶ部品は、`CGO_ENABLED=0` の静的なバイナリ、3つの OS、`govulncheck`、ライセンスの表示(README の「ライセンス」の節)の条件を満たす必要があります。
- GitHub への接続(プルリクエストの作成)の部品もありません。意図は利用者の `gh` と `git` を使うとしているため、新しい Go の依存は要らない見込みですが、外部のプロセスの呼び出しは今は `internal/iac/git.go` の読み取りだけです。
