# 論理的な部品: U1 最小の一本

非機能の設計が、どの部品のどこで効くかを示します。tag-refactor は利用者の端末で動く1つの実行ファイルで、サーバーやネットワーク越しのサービスはありません。障害の範囲は「1回のコマンドの実行」です。

出典: `nfr-design-questions.md` Q1、ドメイン設計 `components.md`、契約 C6〜C12、機能設計 `functional-spec.md`。

## パッケージの構成(Q1)

| パッケージ | 部品 | 受け持つこと(U1) | 契約 |
|---|---|---|---|
| `cmd/tag-refactor` | 入口 | 実行ファイルの入口。`internal/cli` を呼び、終了コードで終える | — |
| `internal/cli` | Cli | サブコマンドとオプション(cobra)、部品の組み立て、エラーの種類から終了コードと2行の案内への変換、`--debug` の切り替え | C5、C12 |
| `internal/display` | Display | 一覧の形の出力、制御文字の置き換え、標準出力と標準エラーの分け方、進み具合の通知口の実装 | C11 |
| `internal/awsaccess` | AwsAccess | AWS の SDK の設定(プロファイル、リージョン、時間の上限、標準の再試行)、読み取りの口(TagReader、PolicyReader、AccountResolver)、エラーの種類への変換、`--debug` の記録 | C6 |
| `internal/inventory` | Inventory | タグの付いたリソースの収集と、キーのまとまり(表記ゆれ) | C7 |
| `internal/rules` | RuleEngine | ルールファイルの読み込みと検証 | C1、C8 |
| `internal/planner` | Planner | 名前の変更とぶつかり、changeId、影響分析のインターフェースの定義(TagChangeInput、ImpactResult) | C2、C9 |
| `internal/impact` | ImpactAnalyzer | IAM の管理ポリシーの読み取りと、条件キーの照合 | C9 |
| `internal/errkind` | (共通) | エラーの種類(InvalidInput、CredentialsMissing、Internal など)の定義。すべてのパッケージが使う | C12 |

### 依存の向き(「A → B」は A が B を使う)

| パッケージ | 使うパッケージ | 理由 |
|---|---|---|
| `cmd/tag-refactor` | `cli` | 入口 |
| `cli` | `display`、`awsaccess`、`inventory`、`rules`、`planner`、`impact`、`errkind` | 部品を組み立てる。`impact` の実装を作って `planner` に渡す |
| `impact` | `planner`(型だけ)、`awsaccess`(読み取りのインターフェース)、`errkind` | `planner` が定義した影響分析のインターフェースと型(TagChangeInput、ImpactResult)を実装する |
| `planner` | `inventory`(Resource の型)、`rules`(RuleSet の型)、`errkind` | 変更案を作る。`impact` は使わない(インターフェースを自分で定義する) |
| `inventory` | `awsaccess`(読み取りのインターフェース)、`errkind` | 進み具合の通知口(ProgressSink)のインターフェースは `inventory` が定義し、`display` が実装し、`cli` がつなぐ |
| `awsaccess` | `errkind` | `--debug` の記録先のインターフェースは `awsaccess` が定義し、`display` が実装し、`cli` がつなぐ |
| `rules` | `errkind` | — |
| `display` | `inventory`、`awsaccess`(通知口と記録先のインターフェースを実装するため)、`planner`(結果の型を表示するため) | 表示 |

循環はない。`impact` → `planner` の向きは、ドメインの依存(U5 → U4)と一致する。

## 障害の範囲と隔て方

| 範囲 | 起きうる障害 | 広がりを止める仕組み |
|---|---|---|
| AWS への呼び出し | 応答しない、一時的な失敗、拒否 | `awsaccess` の中だけで時間の上限と再試行を行い、種類つきのエラーで返す。他の部品は再試行しない |
| ポリシーの読み取り | 一部だけ拒否 | `impact` が読めない分を記録して続け、結果を不完全にする。変更案は出す |
| ルールファイル | 誤り、巨大、壊れた YAML | `rules` が大きさを先に確かめてから読み、誤りで止める。AWS は呼ばない |
| 表示 | AWS の文字列に含まれる制御文字 | `display` が出力の直前に置き換える |
| 書き込み | (U1 にはない) | 書き込みの口を作らない。`cli` が scan と plan に渡すのは読み取りのインターフェースだけ |

## 共有するもの

- AWS の SDK の設定(`awsaccess` が1回だけ作り、各クライアントで共有する)。
- 進み具合の通知口と `--debug` の記録先(どちらも標準エラー。`cli` が作って渡す)。
