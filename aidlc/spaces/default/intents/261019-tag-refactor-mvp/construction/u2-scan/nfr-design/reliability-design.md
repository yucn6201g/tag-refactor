# 信頼性の設計: U2 収集

U1 の信頼性の設計(`construction/u1-walking-skeleton/nfr-design/reliability-design.md`)は、ここで変えない限りそのまま使います。

出典: `nfr-requirements/performance-requirements.md`(NFR5.4〜NFR5.8)、`nfr-requirements/security-requirements.md`(NFR8.11)、`nfr-design-questions.md` Q1〜Q3、機能設計 `functional-spec.md`(BR3.1、BR3.2、BR4.1〜BR4.3)、AWS のドキュメント(`API_RecordingGroup.html`、`API_ListResources.html`)。

## 1. 再試行と時間の上限(NFR5.4、NFR5.5)

`internal/awsaccess` の SDK の設定を、次のように変える。すべての AWS の呼び出しに共通。

| 設定 | U1 | U2 |
|---|---|---|
| 再試行の仕組み | SDK の標準(`retry.NewStandard`)、既定の回数 | SDK の標準。最大の試行の回数10、待ち時間の上限20秒(揺らぎは SDK の標準のまま) |
| 再試行の予算(SDK の中の、使い切ると再試行をやめる仕組み) | 既定 | 使わない(`ratelimit.None`)。呼び出し回数の上限が続いたときに、10回より前に諦めないようにするため |
| 1回の試行の上限 | 30秒 | 30秒(U1 のまま) |
| 再試行を含めた1回の呼び出しの上限 | 120秒(U1 の `reliability-design.md`) | 5分 |

- 10回 × 30秒は5分に等しいが、待ち時間を足すと5分を超えうる。その場合は5分の上限が先に効き、`Transient`(時間切れ)になる。
- 変えるのは `session.go` の既定の値と `newRetryer`。U1 のテスト(時間の上限と再試行の回数を確かめるもの)を、新しい値に合わせて直す。テストでは、待ち時間と時間の上限を差し込み口で短くする(U1 のまま)。
- テスト(NFR5.4): 上限の応答を3回返してから成功する偽の AWS で完了すること。返し続ける偽の AWS で、試行がちょうど10回で、タグの取得なら `failedRegions`、検出なら `partial` になること。差し込み口で記録した待ち時間が、回数とともに増え、20秒(揺らぎの範囲)を超えないこと。

## 2. リージョンの失敗を他に広げない(NFR5.6)

- リージョンを読む処理は、`errgroup` の関数としてエラーを返さない。結果(読めた、失敗と理由)をリージョンごとの入れ物に書く。こうすると、1つのリージョンの失敗が `errgroup` の取り消しで他のリージョンを止めることがない。
- 認証情報の誤りだけは、共有の取り消しで全体を止める(§4)。
- 検出の失敗は、タグの取得の結果に影響しない。検出の状態は `untaggedDiscoveryByRegion` に入り、終了コードを変えない(BR3.2、BR5.3)。

## 3. 一度もタグが付いていないリソースの検出の分かれ道(BR3.1 の実装)

リージョンごとの分かれ道を、次の表で決める。

| 状況 | 判定のもと | 扱い |
|---|---|---|
| Config がすべての種類を記録している | `DescribeConfigurationRecorders` の記録の設定が次のどちらか。(a) `recordingGroup.allSupported` が true(記録の戦略は `ALL_SUPPORTED_RESOURCE_TYPES` か未設定)。(b) 記録の戦略が `EXCLUSION_BY_RESOURCE_TYPES` で、除外の一覧が空。かつ `DescribeConfigurationRecorderStatus` の `recording` が true。`includeGlobalResourceTypes` は判定に使わない | Config だけを使う |
| Config が一部の種類だけを記録している | 記録はしているが、上の条件に当たらない(種類を選んでいる `INCLUSION_BY_RESOURCE_TYPES`、除外が1つ以上ある) | Config と Resource Explorer の両方を使い、ARN で重複を除く。Config を残す |
| Config が記録していない | 記録の設定がない、または `recording` が false | Resource Explorer を使う |
| Config の操作が拒否された、再試行を使い切った、問い合わせの文が受け付けられない、想定外の応答 | エラーの種類 | Resource Explorer を使う。Config の理由も記録する |
| Resource Explorer の索引がない | `ListIndexes` にそのリージョンがない | Resource Explorer は使えない |
| 既定のビューがない | `GetDefaultView` の応答の `ViewArn` が空 | Resource Explorer は使えない。理由は「既定のビューがない」(理由 `no-default-view`)。`ListResources` は呼ばない |
| 既定のビューはあるが、一覧が 401 を返した | `GetDefaultView` は ARN を返したが、`ListResources` が 401(`UnauthorizedException`)。AWS のドキュメントでは、ビューを使う権限がない場合にも 401 が返る | Resource Explorer は使えない。理由は「既定のビューを使えない(権限がないか、ビューが使えない)」とし、IAM の操作の名前 `resource-explorer-2:Search` を添える(種類 `AccessDenied`) |
| 既定のビューに絞り込みの条件がある | `GetView` の `Filters.FilterString` が空でない | 読んだ結果は使うが、そのリージョンの状態は `failed` とし、理由に「既定のビューに条件がある」を出す(全体は `partial`) |
| Resource Explorer の操作が拒否された、再試行を使い切った | エラーの種類 | Resource Explorer は使えない |
| どちらも使えない | — | `unavailable`。両方の理由を記録する |

- グローバルな IAM の種類(ユーザー、グループ、ロール、管理ポリシー): AWS Config の `allSupported` の既定の設定はこれらを記録せず、2022年2月より後に Config に対応したリージョンでは記録できない(AWS のドキュメント `API_RecordingGroup.html`)。そのため Config の判定ではこれらを数えず、Config の経路では探さない。Resource Explorer の経路でも、IAM のリソースは結果から除く。Resource Explorer は IAM のリソースのタグを索引しないため(AWS のドキュメント `using-search-query-syntax.html`)、タグの付いた IAM のロールも `tag:none` に当たってしまうから。一度もタグが付いていない IAM のリソースを見落としうることは、結果の `Checked:` の行の Config の経路の説明に「IAM のグローバルな種類を除く」と添えて示す。
- テスト: 記録の設定の既定(`allSupported` true、`includeGlobalResourceTypes` false)、除外が空の `EXCLUSION_BY_RESOURCE_TYPES`、除外のある `EXCLUSION_BY_RESOURCE_TYPES`、`INCLUSION_BY_RESOURCE_TYPES`、`recording` が false の5つを偽の AWS で返し、表のとおりの経路になることを確かめる。既定のビューがない場合と、ビューはあるが一覧が 401 を返す場合で、理由の文言が分かれることも確かめる。
- 集約の索引: `ListIndexes` で種類が集約(`AGGREGATOR`)の索引があれば、そのリージョンで `GetDefaultView`、`GetView`、`ListResources` を呼び、結果をリソースのリージョンごとに振り分ける。`ListIndexes` に索引がないリージョンは、集約の索引でも探せないので、そのリージョンだけ上の表の Config の行から試す。
- グローバルなリソース(ARN にリージョンがないもの)は、どのリージョンの結果にも現れうる。ARN での重複の除去は、すべてのリージョンの結果を合わせてから1回行う(BR2.2 と同じ)。

## 4. 認証情報の誤りで全体を止める(NFR5.8、NFR8.11)

- scan と plan は、1回の実行に1つの取り消しの仕組み(`context` の取り消し)を作る。どのリージョンのどの段階でも、`CredentialsMissing` の誤りを受けたら、その取り消しを呼ぶ。
- 取り消しを受けた読み取りは、次の呼び出しを始めずに戻る。`errgroup.Wait` がすべての読み取りの終わりを待ってから、`cli` に `CredentialsMissing` を返す。`cli` は U1 の W3 のとおり2行の案内を標準エラーに出し、標準出力には何も書かない。結果の表は、すべての読み取りが成功したあとにしか作らないので、途中の結果が標準出力に出る経路はない。
- テスト: タグの取得、検出、plan の IAM の読み取りのそれぞれで、期限切れの各エラーの名前(`security-design.md` §4)を返す偽の AWS を使い、標準出力が空で終了コードが 1 になることを確かめる。さらに、読み取りの処理が残っていないことを、次の2つで直接確かめる。(1) 偽の AWS が数える「処理中の要求の数」が、コマンドが戻った時点で0である。(2) コマンドが戻ったあと0.2秒待っても、偽の AWS に新しい要求が来ない。ゴルーチンの数そのものは比べない(Go の HTTP の部品が接続を保持するためのゴルーチンが残り、テストが不安定になるため)。

## 5. 結果を実行のたびに同じにする(NFR5.7)

- リージョンの結果は、終わった順ではなく、リージョンの名前の順に合わせる。
- 合わせたあとのリソースは ARN の順に並べる。表記ゆれのまとまりと必須タグの欠落は、U1 と同じく決まった鍵(件数の多い順、同じなら名前の順)で並べる。
- Config と Resource Explorer が同じ ARN を返したときは Config の結果を残す。タグの取得が返した ARN は、検出の結果から除く(BR3.1)。
- テスト: リージョンごとの応答の遅れを乱数で入れ替える偽の AWS で、乱数の種を10通り変えて実行し、標準出力がすべて同じであることを確かめる。
- レース検出: CI の Linux のテストのジョブだけ `CGO_ENABLED=1` にして `go test -race ./...` を実行する。ほかの OS のジョブと、配るバイナリは `CGO_ENABLED=0` のまま。

## 6. チェックポイントでの確認(U1 の手順に足す)

- U1 の検証のスクリプト(`scripts/verify-u1.sh`)を U2 用に広げ、結合テストで次を確かめる。
  - 例のポリシーだけを付けたロール R2 で、scan と plan が全リージョンで終わる。
  - Resource Explorer の経路で、`tag:none` のリソースを、ページの大きさを小さくして(テストだけの設定)複数のページで最後まで読める(NFR1.10)。
  - Config の3つの操作が、記録のない状態で AccessDenied にも問い合わせの文の誤りにもならない(`security-design.md` §2)。
  - `--debug` の行から、操作ごとの L95 を求めて記録する(`performance-design.md` §4)。
- 1,000件を超える結果の実機での確認は、検証用アカウントの件数が足りないため行わない。そのことをチェックポイントの記録に書く。
