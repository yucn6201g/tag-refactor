<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
- 2026-10-23T14:05:15Z [unit u2-scan] — scan が required のキーを受け取るため internal/rules に RequiredTags を足した; 設計では変えない部品だったが、契約 C8 の requiredTagRules に沿う追加として扱った。
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
- 2026-10-23T14:05:15Z [unit u2-scan] — レビューは READY だったが、2回の試行で見つかった中程度の不具合2件(不完全な scan の No issues found、集約の索引の失敗後のリージョンの帰属)をチェックポイントの前に直した; 誤った結果を利用者に見せないため。
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
- 2026-10-23T14:05:15Z [unit u2-scan] — 既定のビューがないときの GetDefaultView の実機の形、Config の問い合わせの受理、-race、go1.27.3 での govulncheck は未確認; チェックポイントと CI で確かめる。
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
