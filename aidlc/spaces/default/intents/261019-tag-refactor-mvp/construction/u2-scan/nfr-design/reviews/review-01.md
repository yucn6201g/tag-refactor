## Review

**Verdict:** NOT-READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-22T12:27:41Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | high | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u2-scan/nfr-design/reliability-design.md > §3 分かれ道の表「Config がすべての種類を記録している」の行 | 「Config だけを使う」の条件に `includeGlobalResourceTypes` が true であることを入れている。AWS のドキュメント(RecordingGroup)では、`allSupported` が true の既定の設定は IAM などのグローバルな種類を除くため、この値は既定で false である。また、2022年2月より後に Config に対応したリージョンでは、グローバルな種類は記録できず true にできない。そのため、ほとんどのリージョンが「一部の種類だけを記録」の行に当たり、Config と Resource Explorer の両方を使う扱いになる。Q3 で決めた「Config を先に使う」の道がほぼ使われず、Resource Explorer の索引がないアカウントでは、全種類を記録していても検出が常に `partial` になる。性能の見積もりと呼び出し回数の上限が想定する経路とも食い違う | 「全種類を記録している」の判定から `includeGlobalResourceTypes` を外す。グローバルな IAM の種類は検出の対象外と明記し、判定は「`allSupported` が true(または記録の戦略が `ALL_SUPPORTED_RESOURCE_TYPES`)かつ `recording` が true」とする。`EXCLUSION_BY_RESOURCE_TYPES` で除外が空の場合の扱いも決める。表と、これを確かめるテスト(既定の設定、後から対応したリージョンの設定)を直す | New |
| R-02 | medium | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u2-scan/nfr-design/security-design.md > §1「GetView」の注記、§4 の 401 の扱い | (1) `GetView` を IAM の操作に足すが、承認済みの上流(NFR3.3 の表と BR7.1 の6つの操作)には含まれず、「要件の表も直す」とあるだけで、直す場所と担当がない。下流の Infrastructure Design(ロール R2)やコード生成が6つの操作だけを見るおそれがある。(2) ListResources の 401(`UnauthorizedException`)は、AWS のドキュメントでは「既定のビューがない」だけでなく「既定のビューを使う権限がない」場合にも返る。これを理由 `no-default-view` に固定すると、権限の不足を「ビューがない」と利用者に案内してしまう | (1) `GetView` を足すことを、BR7.1 と NFR3.3 への明示的な変更として、この設計の成果物に必須の引き継ぎ事項として書く(例のポリシー、R2、操作の表の3か所)。(2) 理由の文言を「既定のビューがない、または使う権限がない」に改め、`GetDefaultView` が空かどうかで「ビューがない」と切り分ける。切り分けを確かめるテストを足す | New |
| R-03 | medium | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u2-scan/nfr-design/performance-design.md > §3 呼び出し回数の上限、§4 時間の見積もり | (1) `ListIndexes` を「1回」と数えているが、`MaxResults` は最大100で `NextToken` があり、ページ読みが要る。上限の式にも、ページ読みの仕様にも書かれていない。(2) 最も偏った場合の計算で Config を100回としているが、Config は一度もタグが付いていない1,000件を含む1.1万件を返すため110回で、合計は204回でなく214回である(Q&A の「約1分」とも食い違う) | `ListIndexes` を `NextToken` が空になるまで読む仕様にし、上限の式に ⌈索引の数 ÷ 100⌉ を入れる。見積もりの数字を直し、Q&A の概算との違い(最も偏った場合の見積もりであること)を書く | New |
| R-04 | low | aidlc/spaces/default/intents/261019-tag-refactor-mvp/construction/u2-scan/nfr-design/reliability-design.md > §4 テスト(ゴルーチンの数が戻ること、待つ上限1秒) | 偽の AWS に HTTP で接続すると、Go の `net/http` は接続を保持し、読み書きのゴルーチンを残す(アイドルは既定で90秒)。「実行の前の数に戻る」を1秒以内で確かめるテストは、そのままだと不安定になる | テストの終了時に、偽の AWS と SDK の HTTP クライアントの保持中の接続を閉じる手順を決める。または、ゴルーチンの検査を、読み取りの処理が残っていないことを直接確かめる方法(完了の待ち合わせの確認)に変える | New |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| sensor-traceability | PASS(gaps、orphans、invalid なし) | 構造上の問題はない。上の指摘は内容に関するもの |

### Summary

U1 の設計を引き継ぐ構成と、再試行・取り消し・出力の守りは筋が通っている(`ratelimit.None` は使っている SDK v1.47.3 に存在する)。ただし、Config だけを使う分かれ道の判定が AWS の既定の設定と合わず、Q3 の方針が実際にはほとんど働かない。この点の修正が要る。
