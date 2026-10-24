<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
- 2026-10-24T06:15:31Z [unit u4-plan] — 必須タグは付いていないときだけ付ける(既にある値は変えない)と読んだ; 値の書き方の統一は値の別名で行う役割分担にした(Q2)。
- 2026-10-24T00:04:11Z [unit u3-rules] — match.tags は別名と値の別名を置き換え、消すキーを除いたあとのタグと比べる; 利用者が正しい名前で条件を書けるようにし、U4 が置き換えの順を決めなくて済むようにするため。
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
- 2026-10-24T06:15:31Z [unit u4-plan] — 契約 C2 の renameKey の意味を mergesIntoExisting で分け、C2・C3 に項目を足した; 値の別名を入れると既にある正しいキー名の値を上書きしてしまうため。contract-summary.md と story map にも書いた。
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
- 2026-10-24T06:15:31Z [unit u4-plan] — 消す候補のリソース数は一覧と Markdown だけに出し、JSON はキー名だけのままにした; 契約 C2・C3 を変えずに済むため。
- 2026-10-24T00:04:11Z [unit u3-rules] — match.resourceType は scan と同じ形(service と service:type)にした; 収集の種類の決め方が s3 や sqs のようにコロンのない値を返すため。
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
- 2026-10-24T06:15:31Z [unit u4-plan] — U6 が同じキーへの書き込みをまとめて最終の値だけを書くかは U6 の設計で決める; 途中の値が rollback の判定に当たりうるため。
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
