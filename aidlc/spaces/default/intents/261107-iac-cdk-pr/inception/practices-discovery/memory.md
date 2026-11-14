<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->
- 2026-11-08T00:00:00Z — 再実行なので team.md の承認済みの内容を基準にし、面談では証拠から決められない9点だけを聞いた。最小の一本の中身は弾ごとに変わるので、team.md では進め方を一般の形にし、第1弾と第2弾の中身を例として書く形にした。

## Deviations
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
- 2026-11-08T00:00:00Z — 最小の一本の確かめは AWS から通す(Q3: B)ので、検証用アカウントの認証情報(SSO)に加えて、テスト専用の GitHub のリポジトリと gh のログインが要る。テスト専用のリポジトリとトークンは誰がいつ用意するかを、CI の段階までに人と決める。
