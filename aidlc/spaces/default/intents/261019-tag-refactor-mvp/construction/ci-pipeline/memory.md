<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
- 2026-11-07T05:59:36Z — CI の道具や進め方はすでに決まっているので質問せず、見つかった穴(必須のチェックが空)だけを質問にした。
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
- 2026-11-07T05:59:36Z — CI のファイルは変えず、GitHub のルールセットだけを変えた。段階の定義の「CI の設定を作る」は、既存のものの記録に読み替えた。
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
- 2026-11-07T05:59:36Z — ci-gate を必須にすると、Dependabot を含め CI が赤いプルリクエストはマージできなくなる。一人の開発でも止まるが、main を赤くした実例があるので、人の判断で必須にした。
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
- 2026-11-07T05:59:36Z — 赤いプルリクエストが実際に止まることは試していない。go vet scripts/u6check.go と bash -n は CI の関門に入れていない。
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
