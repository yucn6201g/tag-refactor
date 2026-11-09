<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->
- 2026-11-08T03:00:00Z — US-Q3 は任されたので、必ず自動にする形を Tags.of(x).add / remove とスタックの tags にし、L1 の tags は「できるだけ自動」に回した。L1 は使われる場面が限られ、両方の言語でプロパティの形が違って手間が大きいため。
- 2026-11-08T03:00:00Z — 要件のレビューの指摘のうち、書き換えたあとに synth し直してタグを確かめることを、CDK の反映の受け入れ条件(AC3.1.1、AC3.2.1)に入れた。テストでの確かめなので、利用者の実行時間には響かない。

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
