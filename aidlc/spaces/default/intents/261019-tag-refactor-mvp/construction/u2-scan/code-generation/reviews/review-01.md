## Review

**Verdict:** READY
**Reviewer:** aidlc-architecture-reviewer-agent
**Date:** 2026-10-23T13:52:17Z
**Iteration:** 1

### Findings

| ID | Severity | Location | Finding | Required action | Status |
|---|---|---|---|---|---|
| R-01 | medium | internal/display/results.go > ScanResult(「No issues found」の分岐) | scan の表示が、タグの取得で読めなかったリージョンがある場合でも「問題がなければ」の分岐に入る。`IssuesFound()` は表記ゆれと必須タグの欠落しか見ないため、読めなかったリージョンがあり問題が0件のとき、標準出力に「! Results are incomplete」の直後に「No issues found」が出て、終了コードは1になる。BR5.4 は「終了コードが 0 のとき」に出すと定めており、利用者に「問題なし」と誤読させる。この組み合わせを確かめるテストもない(`acceptance_u2_test.go` の「No issues found」のテストは、読めなかったリージョンのない場合だけ)。 | `snap.Complete()` が偽のとき「No issues found」を出さない(または「Results are incomplete」の扱いに一本化する)。読めなかったリージョンがあり問題が0件の scan のテストを足し、標準出力と終了コード 1 を確かめる。plan の「No changes needed」も、読めなかったリージョンがあるときの表示を同じ方針で揃える。 | New |
| R-02 | low | internal/inventory/discover.go > overallStatus と searchRegion | 全リージョンで「Config は一部の種類だけ記録、Resource Explorer は使えない」(各リージョンが `failed`)のとき、全体の状態は `unavailable` ではなく `partial` になる。BR3.2 は「すべて探せなかった → unavailable」と定める。Config で一部は探せているので `partial` は妥当とも読めるが、規則の文言と実装が一致しているか明記されていない。 | 意図どおりなら BR3.2 の記述か実装コメントに「`failed` は `partial` に数える」と書く。意図が違うなら `overallStatus` を直し、全リージョンが `failed` のケースのテストを足す。 | New |
| R-03 | low | internal/inventory/types.go > notTaggableTypes(`iam:group`)と isIAMType | `iam:group` を「タグを変更できない種類」に載せているが、`isIAMType` が IAM の型を検出の結果から除くので、この項目は検出の経路では一度も使われない(死んだ項目)。「タグを変更できない種類」の一覧を見直す人が、IAM が数えられていると誤解しうる。 | 項目を消すか、「IAM は検出から除くので載せていない」と注記する。 | New |
| R-04 | low | internal/display/terminal_windows.go > enableVT | Windows のコンソールに `ENABLE_VIRTUAL_TERMINAL_PROCESSING` を足した後、終了時に元のモードへ戻さない。コンソールは親のシェルと共有されるため、ツール終了後も設定が残る。害は小さいが、利用者の環境を黙って変える。 | 設定前のモードを覚え、プログラムの終了時(正常終了とエラー終了の両方)に戻す。戻さない判断なら、README か NFR9.3 の記述に残すと書く。 | New |

### Validation Tool Results

| Tool | Result | Interpretation |
|---|---|---|
| aidlc engine sensor-traceability | pass: true(gaps、orphans、invalid_entries はいずれも空) | 追跡の表は上流の ID と整合している。 |
| go vet ./... | 出力なし(指摘なし) | 静的な誤りはない。 |
| go test ./internal/... | 全パッケージ ok(一部は cached) | 手元の偽の AWS では成功している。実際の AWS、`-race`、`govulncheck` の確認はチェックポイントと CI に残っている(code-summary.md に明記あり)。 |

### Summary

実際の AWS を呼ばずに読める範囲では、U2 の収集(全リージョン、Config から Resource Explorer への切り替え、集約の索引、ページ送りの打ち切りの回避、認証情報の誤りで全体を止める処理)と IAM ポリシーの例は設計と整合していた。ただし、読めなかったリージョンがあるのに「No issues found」を出す表示が BR5.4 とずれており、直すべきである(medium、blocking ではない)。ほかは low で、READY とする。
