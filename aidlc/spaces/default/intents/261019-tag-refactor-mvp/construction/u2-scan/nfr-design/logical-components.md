# 論理的な部品: U2 収集

U1 のパッケージの構成(`construction/u1-walking-skeleton/nfr-design/logical-components.md`)を保ち、U2 で足すものと変えるものを示します。新しいパッケージは作りません。障害の範囲は U1 と同じく「1回のコマンドの実行」です。

出典: `nfr-design-questions.md` Q1〜Q4、`security-design.md`、`performance-design.md`、`reliability-design.md`、`observability-design.md`、契約 C3・C6・C7・C11・C12、機能設計 `functional-spec.md`。

## パッケージごとの変更

| パッケージ | U2 で足すもの、変えるもの | 契約 |
|---|---|---|
| `internal/awsaccess` | 操作の表を API と IAM の2つの名前に分ける。Config の読み取り(記録の設定と状態、問い合わせのページ)と Resource Explorer の読み取り(索引、既定のビュー、ビューの条件、一覧のページ)のインターフェースと実装。再試行(最大10回、待ち時間の上限20秒、再試行の予算なし)と時間の上限(1回の呼び出し5分)。Resource Explorer の 401 を「既定のビューがない」に変える | C6、C12 |
| `internal/inventory` | 全リージョンを同時に8つまで読む収集。リージョンの失敗を結果に集める。一度もタグが付いていないリソースの検出(分かれ道、集約の索引、結果の振り分け)。ARN での重複の除去と Config を残す優先順位。種類の対応表とタグを変更できない種類の一覧。略語の辞書。必須タグの数え上げ | C3、C7 |
| `internal/display` | 進み具合の書き換えと追記の2つの形、端末の判定の差し込み口、Windows の仮想端末の処理。scan の結果の新しい行(検出の状態、`Checked:`、タグを変更できない種類) | C11 |
| `internal/cli` | scan の `--rules`、全リージョンの読み取りへの切り替え、取り消しの仕組みの作成と、認証情報の誤りでの終了 | C5、C12 |
| `internal/errkind` | 理由 `no-default-view` を足す(種類は `AccessDenied`) | C12 |
| `internal/testutil` | 偽の AWS で、URL のパスから操作の名前を取り出す。Config と Resource Explorer の応答の雛形。`IsReadOperation` に `config:SelectResourceConfig` を足す | — |
| `docs/iam/readonly-policy.json` | §1 の表(`security-design.md`)の IAM の操作を足す | — |

`internal/planner`、`internal/impact`、`internal/rules` は変えない(plan は `inventory` の新しい収集を使うだけ)。

### 依存の向き

U1 の依存の向きのまま。`inventory` は `awsaccess` の読み取りのインターフェース(TagReader に加えて ConfigReader、ExplorerReader)と、Go の補助ライブラリ `golang.org/x/sync/errgroup` を使う。`display` は `golang.org/x/term` を使う。新しい循環はない。

## 障害の範囲と隔て方(U1 の表に足す)

| 範囲 | 起きうる障害 | 広がりを止める仕組み |
|---|---|---|
| 1つのリージョンのタグの取得 | 拒否、再試行を使い切った、時間切れ | そのリージョンだけ `failedRegions` に入れ、他のリージョンを続ける(`reliability-design.md` §2) |
| 1つのリージョンの検出 | Config と Resource Explorer の拒否、既定のビューがない、想定外の応答 | そのリージョンの検出の状態だけを変える。タグの取得の結果と終了コードは変えない(§3) |
| 認証情報 | 期限切れ、拒否 | 共有の取り消しで全体を止め、標準出力に何も出さない(§4) |
| 呼び出し回数の上限 | 上限の応答が続く | `awsaccess` の中の再試行だけで扱う。他の部品は再試行しない |
| 進み具合の表示 | 端末の判定の誤り | 判定できなければ追記にする(`observability-design.md` §1) |

## 共有するもの

- AWS の SDK の設定と再試行の仕組み(`awsaccess` が1回だけ作り、タグの取得、Config、Resource Explorer、IAM のクライアントで共有する)。
- 取り消しの仕組み(`cli` が作り、`inventory` と `impact` に渡す)。
- 進み具合の通知口と `--debug` の記録先(`display` の1つの書き手。鍵で守る)。

## 確認用の環境(Q4)

- 確認用の CloudFormation のテンプレート(`test/sandbox/`)に、Resource Explorer の索引と既定のビュー、読み取り用ロール R2 への新しい操作、一度もタグが付いていないリソースを足す。中身は Infrastructure Design で決める。
