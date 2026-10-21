# 使う技術: U1 最小の一本

出典: 要件 CON1、NFR9、`nfr-requirements-questions.md` Q1〜Q4、チームの進め方(§Testing Posture、§Code Style、§Deployment)。

| 分野 | 選んだもの | 理由 | 採らなかった案 |
|---|---|---|---|
| 言語 | Go | 要件 CON1。1つの実行ファイルで3つの OS に配れる(NFR9) | — |
| Go の版 | Go の公式のサポート範囲(その時点の最新版と1つ前の版)。`go.mod` の最小は1つ前の版。実際の番号はコードの生成のときに確かめる | 利用者の手元の Go が少し古くても `go install` できる。公式にサポートされない版は対象にしない(Q1) | 最新版だけ(利用者の手元の Go を上げる必要が出やすい) |
| AWS へのアクセス | AWS の公式の Go 用 SDK(v2)。タグの読み取り、IAM、STS のクライアント | 公式で、標準の認証情報の読み方(環境変数、プロファイル、SSO)と、既定の再試行と時間の上限の設定を持つ | 自前の HTTP の呼び出し(署名や再試行を自分で作ることになる) |
| コマンドの組み立て | cobra | ヘルプ、サブコマンドの入れ子(`rules suggest`)、共通のオプション(`--debug`、`--profile`)を任せられる。Go の CLI で広く使われている(Q2) | 標準の flag だけ(ヘルプとサブコマンドを自分で作る) |
| YAML の読み込み | goccy/go-yaml | 誤りの位置(行と列)を返せ、知らないキーを誤りにする読み方ができるため、BR3.1 の「行番号、キー、直し方」を作りやすい。大きすぎる入力は、ライブラリに渡す前にツールが大きさで拒む(NFR8.4)。保守の状況と、別名の展開の上限の設定は、コードの生成のときに確かめる | go.yaml.in/yaml/v3(YAML の団体が保守を引き継いだ版。誤りの位置の情報が goccy/go-yaml より少ない)。gopkg.in/yaml.v3(元の版は保守が止まっている)。標準ライブラリには YAML がない |
| ビルド | `CGO_ENABLED=0` | C のライブラリに頼らず、3つの OS で1つの実行ファイルにする(NFR9.2) | C のライブラリを使う形(環境によって動かない) |
| テスト | Go の標準の testing。AWS は、部品のインターフェース(契約 C6)を差し替えて CI で動かす | チームの進め方: CI では AWS を呼ばない | 実際の AWS を CI から呼ぶ(チームの進め方で禁止) |
| 整形と lint | `gofmt`、`golangci-lint`(既定の linter だけ) | チームの進め方 §Code Style | goimports、gofumpt |
| 脆弱性の確認 | `govulncheck`(CI)、Dependabot | チームの進め方 §Code Style | — |
| CI | GitHub Actions。Linux、macOS、Windows と、Go の2つの版の組み合わせで、テスト、カバレッジ、lint、govulncheck を実行する。Actions はコミットの SHA で固定し、govulncheck を週1回の定期実行でも動かす(NFR8.7) | チームの進め方 §Testing Posture。リポジトリが GitHub にある | — |
| 改行コード | `.gitattributes` で Go のファイルを LF に固定 | チームの進め方 §Code Style、Windows の CI | — |

## U1 で置くもの(リポジトリ)

- 読み取り用の IAM ポリシーの例(U1 が呼ぶ API: タグの読み取り、リージョンの確認、IAM のポリシーの一覧と中身の読み取り、付いている先の確認、STS の呼び出し元の確認)。正確な操作の名前は、コードの生成で、実際に呼ぶ API と照らして確かめる(Q4、NFR3.2)。
- CI の設定(上の表のとおり)。
- 配布の仕組み(GitHub Releases、チェックサム)は U1 に入れない(別の作業。`external-dependency-map.md` の E6)。
