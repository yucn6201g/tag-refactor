<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->
- 2026-11-08T02:45:00Z — 要件の ID はこの intent で FR1〜FR3 と振り直し、前の intent の FR8.3、FR8.4、AC7.2.1、AC7.3.1 との対応を表にした。前の ID のまま足すと、前の intent の記録と混ざるため。
- 2026-11-08T02:45:00Z — 想定外の条件の洗い出しから、コミットには書き換えたファイルだけを入れること(FR1.8)と、ほかのリソースまで変わる書き換えはしないこと(FR3.5)を足し、仮定 A3、A4 として理由を書いた。人に聞かずに既定にしたので、承認のときに直せるようにした。

## Deviations
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
