<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->
- 2026-11-07T15:45:00Z — Q1 の「できるだけ自動」は合否を判断できないので、Q7 で必ず自動にする書き方を選んでもらい、それを MVP の境界にした。残りは Could(IB-11)とし、できないものは手で直す一覧に出す。
- 2026-11-07T15:45:00Z — 範囲に入らないものは、意図の Q8 が「特になし」だったため、実現性の制約(C-T4、C-T5)から出たものだけを書いた。CDK の Java などは除外とも対象とも書かず、対象の言語を TypeScript と Python と書くだけにした。

## Deviations
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->
- 2026-11-07T15:45:00Z — 期限の質問は、実現性の段階で「期限なし、2つそろったら v0.2.0」と決まっていたので聞かなかった。

## Tradeoffs
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
