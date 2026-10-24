<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
- 2026-10-24T00:04:11Z [unit u3-rules] — match.tags は別名と値の別名を置き換え、消すキーを除いたあとのタグと比べる; 利用者が正しい名前で条件を書けるようにし、U4 が置き換えの順を決めなくて済むようにするため。
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
- 2026-10-24T00:04:11Z [unit u3-rules] — match.resourceType は scan と同じ形(service と service:type)にした; 収集の種類の決め方が s3 や sqs のようにコロンのない値を返すため。
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
