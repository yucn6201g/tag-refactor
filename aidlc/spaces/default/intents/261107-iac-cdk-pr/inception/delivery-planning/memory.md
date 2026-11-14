<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->
- 2026-11-08T08:50:00Z — Construction の検証コマンドは、人の依頼で決めるのを U1 の検証の前まで延ばした。コマンドの本体(AWS から plan を作って iac --pr を動かすもの)は U1 の Code Generation で作るため、今は実行できる形が無い。

## Tradeoffs
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
- 2026-11-08T08:50:00Z — 今の手元のブランチ bolt/u8-iac は squash マージ済みで、origin/main より古い。B1 の作業ブランチは、GitHub の最新の main から作る必要がある(ブランチの切り替えは、人に確かめてから)。
