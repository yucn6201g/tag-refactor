<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->
- 2026-11-08T04:30:00Z — 前の段階のレビューで残った指摘のうち、契約で決められるもの(失敗したアプリのリソースの扱い、synth を動かす条件、スコープのタグの判断に要る synth のタグ、書く前の読み直し、進み具合の渡し方、実行後に手元で出ているブランチ、オプションの誤用、本文のリソース名)を、C1〜C7 と X1、X2 の決まりに入れた。
- 2026-11-08T05:10:00Z — 1回目のレビューの指摘 R-01〜R-09 を、人の依頼(すべて直す)に従ってすべて直した。C6 は、既存の `inventory`、`apply` と同じく使う側のパッケージが通知口を定義する流儀に改めた。
- 2026-11-08T08:20:00Z — 2回目のレビューは書式の誤りでやり直し、やり直したレビューは最初の試みの Major 2件(必須タグの追加の契約、アカウントとリージョンの取り方)を出さなかった。人にその2件を示し、依頼に従って両方の試みの指摘をすべて直した。

## Deviations
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->
- 2026-11-08T05:10:00Z — git と gh の時間の上限は、v0.2.0 では固定の値にし、オプションを増やさなかった。コミットと push だけ、利用者のフックと署名を含むので 10 分と長くした。

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
