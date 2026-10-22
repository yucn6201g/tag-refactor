## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-22T12:47:02Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | medium | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u2-scan/infrastructure-design/cicd-pipeline.md > 検証のスクリプト(Q3)の手順1と手順5 | 手順1は `scripts/verify-u1.sh` を「中身は変えない」で呼ぶとしている。しかし現在の `verify-u1.sh` は `-tags integration -run 'TestIntegration' ./internal/cli/...` を実行する。U2 の結合テストが同じパッケージで `TestIntegration` から始まる名前になると、手順1の中で、キューの ARN の環境変数がないまま U2 のテストも走り、飛ばされるか失敗する。また U1 の結合テスト(`internal/cli/integration_test.go`)は、U2 で scan の出力と終了条件が変わった後も、そのまま通ることが前提になっている。この前提は書かれていない | U2 の結合テストの名前と置き場所を決め、`verify-u1.sh` の `-run` を U1 のテスト名に限る(例 `^TestIntegrationSandboxScanAndPlan$`)。その場合は「中身は変えない」を「`-run` だけ直す」に改める。あわせて、U1 の結合テストが U2 の scan の出力でも通るか(標準エラーに `Error:` が出ないこと、終了コード 2)を、期待結果の表か手順に書く | New |
| R-02 | medium | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u2-scan/infrastructure-design/infrastructure-specification.md > 期待結果(全リージョンの scan)、確認用のリソースの変更の「既にある索引とビュー」 | 全リージョンの scan で「他のリージョンは索引も Config もないので `unavailable`」と断定している。しかし事前の確認は `list-indexes --regions ap-northeast-1` と `get-default-view` だけで、他のリージョンの索引や Config の有無は調べない。検証用アカウントに、他のリージョンの索引、集約(`AGGREGATOR`)の索引、Config の記録が既にあると、この期待結果は成り立たない。`ap-northeast-1` に既にある索引が集約の種類のとき(`CreateIndex=false` の経路)も、「LOCAL」を前提にした期待結果と食い違う | デプロイの手順4で、全リージョンの `list-indexes` を確かめ、索引の種類(`LOCAL` か `AGGREGATOR`)も記録する。期待結果は「`ap-northeast-1` は `complete`、他は `complete` か `unavailable`(アカウントの実際の状態に従う)」と、状態を見て判定する形にするか、前提の状態が違うときは検証を中止する条件を足す | New |
| R-03 | low | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u2-scan/infrastructure-design/infrastructure-specification.md > タグのないキュー(待ち方) | 待ち方は `tag:none` の結果にキューの ARN が現れるまで待つとしている。AWS のドキュメントでは Resource Explorer の検索結果は先頭の1,000件に限られる。検証用アカウントの一度もタグが付いていないリソースが多いと、キューが1,000件の外に出て、30分待っても見つからない。待つ側の検索の条件が書かれていない | 待つときの検索を `tag:none` だけにせず、種類で絞る(例 `resourcetype:sqs:queue`)と書く。ページを最後まで読む方法でも構わないが、どちらかを明記する | New |
| R-04 | low | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u2-scan/infrastructure-design/cicd-pipeline.md > `ci.yml` の変更(`race` ジョブ) | `race` ジョブは `go test -race ./...` で全テストを動かし、時間の上限は15分としている。`-race` は通常の実行より数倍遅い。2分以内に収めると決めた大きな偽の AWS のテスト(NFR10.3)と乱数の遅れの10通りの実行も、そのまま対象に入る。15分に収まる根拠が書かれていない | `-race` 付きでの実測を、大きな偽の AWS のテストを入れる条件に加える。収まらないときは、そのテストだけ `-race` のときに件数を減らす方法(ビルドタグまたは `testing.Short`)を決めておく | New |
| R-05 | low | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u2-scan/infrastructure-design/cicd-pipeline.md > 検証のスクリプトの手順2と3 | 手順2は「検証用アカウントであること」を確かめるとしているが、何と比べるかが書かれていない。手順3のキューを作るリージョンも書かれていない(期待結果は `ap-northeast-1` の Resource Explorer を前提にしている) | 比べる値(期待するアカウント ID を持つ環境変数、または U1/U2 のスタックの存在など)を決める。キューは `ap-northeast-1` に作ると明記する | New |
| R-06 | low | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u2-scan/infrastructure-design/infrastructure-specification.md > IX の中身がそろうまでの時間 | 索引が使えるまで「最大1時間ほど」と書き、チェックポイントの1時間以上前にデプロイするとしている。AWS のドキュメントの別の箇所は、完全な索引の作成に「数時間」かかりうると書いている | 「1時間以上」を、検証の手順の下限と読めるように直す。あわせて、検証が失敗したときは、時間をおいて再実行することを、手順6に書く | New |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| aidlc engine sensor-traceability | pass: true(gaps、orphans、invalid はすべて空) | 上流の NFR の ID はすべて対応づけられている。N/A の項目の理由も、設計の内容と合っている |

### Summary

U1 の CI、確認用のスタック、検証のスクリプトの構成を壊さずに、必要な差分(`race` ジョブ、R2 の更新、U2 のスタック、タグのないキュー、`verify.sh`)に絞れており、実装者が推測せずに作れる粒度です。残る懸念は、`verify-u1.sh` と U2 の結合テストの選択の衝突、検証用アカウントの既存の状態への前提、検索結果の1,000件の上限で、いずれもコードの生成の前に文面で直せます。
