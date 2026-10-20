# 観測のしやすさの設計: U1 最小の一本

出典: `nfr-requirements/observability-requirements.md`(NFR11.1〜NFR11.4)、`nfr-design-questions.md` Q3、機能設計 BR6.1〜BR6.3。

CLI なので、メトリクスの収集、分散トレース、警報、ダッシュボード、SLI と SLO はありません。観測のしやすさは、標準エラーの表示の設計です。

## 出力先の分け方(NFR11.2)

| 出すもの | 出力先 | 例 |
|---|---|---|
| 結果の表と Summary の行 | 標準出力 | `Summary: 208 to change · 2 conflicts · 1 affected · 1 undecided` |
| アカウントとリージョン | 標準エラー | `Account 123456789012 · region us-east-1` |
| 進み具合 | 標準エラー | `scan: us-east-1 started`、`scan: us-east-1 done (412 resources)` |
| 警告 | 標準エラー | `warning: tagrules.yaml line 8: "required" is not used in this version` |
| エラー(2行) | 標準エラー | `Error: AWS credentials expired` / `  Fix: run "aws sso login --profile dev"` |
| かかった時間と件数 | 標準エラー | `Done in 00:12 · 412 resources` |
| 終了コード | 標準エラー | `Exit code: 2 (review needed)` |
| `--debug` | 標準エラー | `debug: tag:GetResources us-east-1 132ms retries=0` |

`display` が、標準出力と標準エラーの2つの書き込み先を持ち、他の部品は直接書かない。進み具合は C11 の通知口を通す。

## 記号と色(NFR11.1)

- 記号: `~`(名前の変更)、`!`(影響あり)、`?`(判定できない)。U1 は色を使わない。

## `--debug`(NFR11.3、Q3)

- `awsaccess` が、AWS の呼び出しのたびに「操作の名前(`サービス:操作`)、リージョン、かかった時間(ミリ秒)、再試行の回数」を1行にして、`--debug` のときだけ標準エラーに出す。
- 1行の形: `debug: <サービス>:<操作> <リージョン> <ミリ秒>ms retries=<回数>`。失敗した呼び出しには、末尾に ` error=<エラーの種類>` を付ける。`Internal` のときは、エラー文ではなくエラーの型の名前だけを付ける。再試行の回数を SDK から取れない場合は `retries=n/a` とする。
- この行も、`display` の置き換え(`security-design.md` §4)を通して出す。
- 引数、応答の中身、認証情報は渡さない(NFR8.2)。

## エラーの2行(NFR11.4)

- 1行目は `Error: <何が起きたか>`、2行目は `  Fix: <どう直すか>`。エラーの種類ごとの文言は `cli` が持つ。
