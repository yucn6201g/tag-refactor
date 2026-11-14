<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->
- 2026-11-07T16:30:00Z — 初めての調査なので全体を対象にした。深さは internal/iac、internal/cli、cmd、変更案の形式、CI に寄せ、ほかのパッケージは公開の API の単位で浅く読んだ(shallow.paths に記録)。今回の作業が iac に集中しているため。

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
- 2026-11-07T16:30:00Z — 今の iac の git の確かめは、設定のプログラム(フック、fsmonitor など)が動かないように読み取りの操作だけを使う。プルリクエストの作成でコミット、push、gh を動かすと、フックや署名が動く。どこまで許すかを要件で決める。
- 2026-11-07T16:30:00Z — TypeScript と Python を読む部品は Go の依存にない。実現性の段階で、利用者の端末の Node.js と Python を呼ぶと決めている(C-T2)ので、それに沿うか、cgo なしの Go の解析器を使うかを設計で決める。
