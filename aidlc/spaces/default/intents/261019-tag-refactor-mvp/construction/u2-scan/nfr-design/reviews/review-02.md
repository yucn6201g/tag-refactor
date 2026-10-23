## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-22
**Iteration:** 2

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | high | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u2-scan/nfr-design/reliability-design.md > §3 分かれ道の表「Config がすべての種類を記録している」の行 | 「Config だけを使う」の条件に `includeGlobalResourceTypes` が true であることを入れている。AWS のドキュメント(RecordingGroup)では、`allSupported` が true の既定の設定は IAM などのグローバルな種類を除くため、この値は既定で false である。また、2022年2月より後に Config に対応したリージョンでは、グローバルな種類は記録できず true にできない。そのため、ほとんどのリージョンが「一部の種類だけを記録」の行に当たり、Config と Resource Explorer の両方を使う扱いになる。Q3 で決めた「Config を先に使う」の道がほぼ使われず、Resource Explorer の索引がないアカウントでは、全種類を記録していても検出が常に `partial` になる。性能の見積もりと呼び出し回数の上限が想定する経路とも食い違う | 「全種類を記録している」の判定から `includeGlobalResourceTypes` を外す。グローバルな IAM の種類は検出の対象外と明記し、判定は「`allSupported` が true(または記録の戦略が `ALL_SUPPORTED_RESOURCE_TYPES`)かつ `recording` が true」とする。`EXCLUSION_BY_RESOURCE_TYPES` で除外が空の場合の扱いも決める。表と、これを確かめるテスト(既定の設定、後から対応したリージョンの設定)を直す | Resolved (reviewer) |
| R-02 | medium | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u2-scan/nfr-design/security-design.md > §1「GetView」の注記、§4 の 401 の扱い | (1) `GetView` を IAM の操作に足すが、承認済みの上流(NFR3.3 の表と BR7.1 の6つの操作)には含まれず、「要件の表も直す」とあるだけで、直す場所と担当がない。下流の Infrastructure Design(ロール R2)やコード生成が6つの操作だけを見るおそれがある。(2) ListResources の 401(`UnauthorizedException`)は、AWS のドキュメントでは「既定のビューがない」だけでなく「既定のビューを使う権限がない」場合にも返る。これを理由 `no-default-view` に固定すると、権限の不足を「ビューがない」と利用者に案内してしまう | (1) `GetView` を足すことを、BR7.1 と NFR3.3 への明示的な変更として、この設計の成果物に必須の引き継ぎ事項として書く(例のポリシー、R2、操作の表の3か所)。(2) 理由の文言を「既定のビューがない、または使う権限がない」に改め、`GetDefaultView` が空かどうかで「ビューがない」と切り分ける。切り分けを確かめるテストを足す | Resolved (reviewer) |
| R-03 | medium | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u2-scan/nfr-design/performance-design.md > §3 呼び出し回数の上限、§4 時間の見積もり | (1) `ListIndexes` を「1回」と数えているが、`MaxResults` は最大100で `NextToken` があり、ページ読みが要る。上限の式にも、ページ読みの仕様にも書かれていない。(2) 最も偏った場合の計算で Config を100回としているが、Config は一度もタグが付いていない1,000件を含む1.1万件を返すため110回で、合計は204回でなく214回である(Q&A の「約1分」とも食い違う) | `ListIndexes` を `NextToken` が空になるまで読む仕様にし、上限の式に ⌈索引の数 ÷ 100⌉ を入れる。見積もりの数字を直し、Q&A の概算との違い(最も偏った場合の見積もりであること)を書く | Resolved (reviewer) |
| R-04 | low | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u2-scan/nfr-design/reliability-design.md > §4 テスト(ゴルーチンの数が戻ること、待つ上限1秒) | 偽の AWS に HTTP で接続すると、Go の `net/http` は接続を保持し、読み書きのゴルーチンを残す(アイドルは既定で90秒)。「実行の前の数に戻る」を1秒以内で確かめるテストは、そのままだと不安定になる | テストの終了時に、偽の AWS と SDK の HTTP クライアントの保持中の接続を閉じる手順を決める。または、ゴルーチンの検査を、読み取りの処理が残っていないことを直接確かめる方法(完了の待ち合わせの確認)に変える | Resolved (reviewer) |

> R-01 Reviewer note: `reliability-design.md` §3 の「全種類を記録している」の判定から `includeGlobalResourceTypes` を外し、「判定に使わない」と明記した。`allSupported` が true の場合と、`EXCLUSION_BY_RESOURCE_TYPES` で除外が空の場合を Config だけの経路にしている。グローバルな IAM の種類は検出の対象外とし、結果の `Checked:` の行に示す。テストにも既定の設定と除外が空の場合が入っている

> R-02 Reviewer note: (1) `security-design.md` §5 に引き継ぎの表を足した。`security-requirements.md`、`rules.md`(BR7.1)、`functional-spec.md` に `GetView` が入っていることを確かめた。コード生成と Infrastructure Design の担当も表にあり、§1 のテストが直し忘れを見つける。(2) §4 と `reliability-design.md` §3 で、`GetDefaultView` の `ViewArn` が空の場合(理由 `no-default-view`)と、ビューはあるのに 401 の場合(権限の不足の可能性を示す文言)を分け、切り分けのテストも書いてある

> R-03 Reviewer note: `ListIndexes` を `NextToken` が空になるまで読む仕様にし、上限の式に max(1, ⌈索引の数 ÷ 100⌉) を入れた。最も偏った場合の見積もりは Config 110回、合計214回、約3分半に直り、Q&A の「約1分」との違い(均等に分かれた場合と最も偏った場合)も書いてある

> R-04 Reviewer note: ゴルーチンの数を比べるテストをやめ、「処理中の要求の数が0」と「戻ったあと0.2秒、新しい要求が来ない」の2つで直接確かめる方法に変えた。理由も書いてある

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| sensor-traceability | PASS(gaps、orphans、invalid はすべて 0) | 追跡の対応に抜けも不正もない |

### Summary

前回の4件はすべて直っており、新しい指摘はない。設計は、開発者が判断に迷わず実装に進める状態になっている。
