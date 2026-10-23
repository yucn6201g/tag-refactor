# 使う技術: U2 収集

U1 の技術の選択(`construction/u1-walking-skeleton/nfr-requirements/tech-stack-decisions.md`: Go、AWS の Go 用 SDK v2、cobra、goccy/go-yaml、`CGO_ENABLED=0`、GitHub Actions)は、そのまま使います。ここでは U2 で足すものだけを決めます。

出典: 機能設計 `construction/u2-scan/functional-design/`、`security-requirements.md`、`performance-requirements.md`、`nfr-requirements-questions.md` Q1・Q2。

| 分野 | 選んだもの | 理由 | 採らなかった案 |
|---|---|---|---|
| AWS Config | AWS の Go 用 SDK v2 の `service/configservice` | 公式で、U1 と同じ再試行と時間の上限の仕組みと、HTTP の層の差し込み口が使える | 自前の HTTP の呼び出し |
| Resource Explorer | AWS の Go 用 SDK v2 の `service/resourceexplorer2`。一覧には `ListResources` を使い、`MaxResults` は 1,000 未満の固定の値にする(値は NFR の設計で決める) | 公式。`Search` は1,000件までに限られる(BR3.1、NFR1.10)。`MaxResults` を 1,000 にすると `NextToken` が返らない | `Search`(件数の上限がある)。`MaxResults` を 1,000 にする(1,000件で打ち切られる) |
| 呼び出し回数の上限への対応 | AWS の SDK の標準の再試行を、最大の試行の回数10回、待ち時間の上限20秒に設定して使う(NFR5.4)。再試行の仕組みが持つ「再試行の予算」を使い切って早く諦めることがないよう、その設定も NFR の設計で確かめて決める | U1 の差し込み口でそのまま確かめられる。自前で再試行を書かない | 自前の再試行(SDK と二重になる)。SDK の既定(最大3回)のまま(呼び出し回数の上限が続くと足りない) |
| リージョンの同時の読み取り | Go の公式の補助ライブラリ `golang.org/x/sync` の `errgroup` で、同時に動く数を `SetLimit` で固定する。リージョンの失敗はエラーとして返さず、結果に集める(他のリージョンを取り消さない、BR4.1)。認証情報の誤りだけを、共有の取り消しで全体に伝える(BR4.3) | 同時に動く数を固定できる(Q2)。失敗の伝え方を2つに分けて、取り違えを防ぐ | `errgroup.WithContext` の既定の使い方(最初の失敗ですべてを取り消すため、BR4.1 に反する) |
| 端末かどうかの判定と書き換えの表示 | Go の公式の補助ライブラリ `golang.org/x/term` で標準エラーが端末かを判定する。Windows では、仮想端末の処理を有効にできたときだけ書き換える(NFR9.3) | 3つの OS で動く。判定や有効化に失敗したら追記に戻る(安全な側) | 環境変数だけで判定する(Windows で外れやすい) |
| 種類の対応表とタグを変更できない種類の一覧 | ツールのソースに表として持つ。「タグを変更できない」と分かっている種類だけを一覧に持ち、一覧にない種類はタグを変更できるものとして扱う(機能設計 BR3.3 を、この向きで実装する) | 表が古くなっても、必須タグの欠落を少なく数える向き(見逃し)ではなく、多めに数える向きにずれる。多めに数えた分は、利用者が直せない欠落として終了コード 2 の原因になりうるため、機能設計 BR3.2 でその旨を示す | 「タグを変更できる種類」を一覧に持つ(新しい種類が増えると、黙って欠落の件数から外れる) |
| Go のレース検出 | CI の Linux のテストのジョブだけ `CGO_ENABLED=1` にして `-race` 付きで実行する。ほかの OS のジョブと、配るバイナリは `CGO_ENABLED=0` のまま(NFR5.7) | `-race` は cgo と C のコンパイラが要る。Linux の CI の実行環境には C のコンパイラがある | 3つの OS すべてで `-race`(Windows と macOS で C のコンパイラの準備が要る) |

## 確かめた事実と、NFR の設計の前に確かめること

| 事柄 | 結果 | 出典 |
|---|---|---|
| `ListResources` に必要な IAM の操作 | `resource-explorer-2:Search`(`resource-explorer-2:ListResources` ではない) | AWS のドキュメント `resource-explorer/latest/apireference/API_ListResources.html` の最小の権限 |
| `ListResources` の `MaxResults` | 1〜1,000。1,000 にすると `NextToken` が返らない | 同上 |
| 既定のビューがないとき | `ListResources` は 401 で失敗する | 同上 |
| `ListResources` が `tag:none` の条件を受け付けるか | 受け付ける(`Filters` の `FilterString` に検索の構文を書ける) | 同上、Resource Explorer の検索の構文のドキュメント |
| Config の問い合わせで「利用者のタグがない」を条件に書けるか | 未確認。NFR の設計の前に AWS のドキュメントで確かめる。書けなければ、記録されたすべての件数を読み、ツールの側で利用者のタグがないものを選ぶ(NFR1.6 の式はこの場合の件数で数える) | AWS Config の高度な問い合わせのドキュメント(NFR の設計で記録する) |
| Config の記録の設定から「すべての種類を記録している」を判定できるか | 未確認。NFR の設計の前に、記録の設定の取得の応答の項目で確かめる | AWS Config の API のドキュメント(NFR の設計で記録する) |

## 表の確かめ方と更新の担当

- 種類の対応表とタグを変更できない種類の一覧は、テストで形を確かめる: ARN の形(`サービス:種類`)であること、重複がないこと、対応表の変換先がすべて ARN の形であること。
- 表の更新はツールの保守者が受け持ち、リリース(`vX.Y.Z` のタグ)の前に AWS のドキュメントと照らして見直す。見直したことをリリースの記録に書く。

## 依存を足すときの決まり

- 足す依存は、AWS の SDK のサービスのクライアント(`service/configservice`、`service/resourceexplorer2`)と、Go の公式の補助ライブラリ(`golang.org/x/sync`、`golang.org/x/term`)に限る。どれも `go.mod` の `go` 行(1つ前の版)以下で動く版を選び、`go.sum` で固定する。
- 足した依存も、U1 の CI(`govulncheck`、`go mod verify`、`go mod tidy` の差分の確認)と Dependabot の対象になる。
