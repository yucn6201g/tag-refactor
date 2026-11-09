<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->
- 2026-11-08T04:00:00Z — ステージの手順にある別の「計画の承認(Approve Plan / Revise Plan)」は聞かなかった。要約の確認に、単位の数、種類、依存、最小の一本の中身をすべて載せて「Looks correct」をもらっており、同じことを二度確かめることになるため。

## Tradeoffs
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
- 2026-11-08T04:00:00Z — 最小の一本の検証コマンドは AWS から plan を作るので、最小の CDK アプリを検証用のアカウントに置く方法と片付け方を、U1 の設計で決める必要がある。AWS の変更は人の明示の許可を得てから行う。
