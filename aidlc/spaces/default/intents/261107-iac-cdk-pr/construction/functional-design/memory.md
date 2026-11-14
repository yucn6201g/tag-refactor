<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
- 2026-11-09T08:30:00Z — U2 の機能の設計の2回目のレビュー(READY)で残った Minor 3件は、U2 の Code Generation の計画で決める。(1) 本文のファイルを Publish の始めに書けないときは、ブランチを作らずに止め、終了コード 1、続きの命令は本文を --body を使わずに手で作る案内にする。(2) 60,000 文字と 65,536 文字は Unicode のコードポイントで数える。(3) BR2.3 の gitCheck は、書き換え案ができたあと、ファイルを書き出す前に行う(今の iac と同じ)。W1-3 の説明をそれに合わせる。entities.md の line_format の YAML の崩れは直した。
- 2026-11-08T10:40:00Z — U1 の機能の設計の2回目のレビューで残った Minor 4件(キーの箇所が0か2以上のときの場所のファイルと理由を出す段階、BR3.4 と契約の recheck_failed の理由の食い違い、テストの push の認証の渡し方と Windows での node_modules のつなぎ方、BR4.1 の行き先のブランチと merge の途中の確かめ)は、U1 の Code Generation の計画で決める。
