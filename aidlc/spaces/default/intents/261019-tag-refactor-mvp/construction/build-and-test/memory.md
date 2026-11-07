<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
- 2026-11-07T05:32:13Z — FR は対応表に直接は出ないので、stories.md の表のストーリーと要件の対応を通し、そのストーリーの受け入れ条件がすべて OK のときに OK と読んだ。NFR12.1 はライセンスの記載なので、NFR12 の根拠にしなかった。
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
- 2026-11-07T05:32:13Z — -update の付いたコマンドは期待のファイルを書き換えるので動かさなかった。-race は手元で動かせず CI の結果を使った。govulncheck は Go 1.27.3 で確かめ直した。U5 の対応表の存在しないファイル名を直した。
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
- 2026-11-07T05:32:13Z — 細かい NFR 144 件の Met は、対応表の対象があり、その確かめが通ったことを根拠にし、1行ずつの読み直しはしていない。その確かめは各作業単位のレビューに任せた。
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
- 2026-11-07T05:32:13Z — NFR12 と US6.2(配布)、US7.2、US7.3 は範囲の外で、この段階の成功の条件を満たせない。受け入れるかは人が決める。
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
