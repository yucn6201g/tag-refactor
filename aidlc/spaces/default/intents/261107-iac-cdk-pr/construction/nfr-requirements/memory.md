<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->
- 2026-11-08T11:00:00Z — U1 でアプリが2つ以上見つかったときは、フォルダーの名前の順で最初の1つだけを synth し、残りは外して理由を示すことにした(NFR3.8)。単位の定義の「cdk.json を1つ」から決めたもので、複数のアプリの扱いの全体は U3 で足す。

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
- 2026-11-08T11:20:00Z — U1 の非機能の要件のレビュー(READY)で残った Major 2件(git と gh の標準エラーや外の出力を経由した ARN とアカウント ID の漏れ、CI のトークンの権限と受け取る仕事の範囲と npm ci に渡さないこと)と Minor 5件は、U1 の NFR Design で手当てを決める。
