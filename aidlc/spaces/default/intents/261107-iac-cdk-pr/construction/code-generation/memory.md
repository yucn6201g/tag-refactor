<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->
- 2026-11-16T06:45:49Z [unit u3-cdk-synth] — Python の最小の版を 3.10 にした(py-minimal に固定した aws-cdk-lib 2.276.0 の Requires-Python。設計の見込みは 3.9)。windowsQuotedApp は手元の Windows で引用符つきの --app が cdk.cmd に届かなかったので false にし、README に回避策を書いた。止め方のテストは上限 8 秒と testutil の偽物(PythonHang)に変えた(並べて動かすと 2 秒では孫のプロセスを起こす前に落ちる)。rules.md の BR2.3 の profile の文を実際の動き(lookup などで失敗)に直した。

## Tradeoffs
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
- 2026-11-16T06:45:49Z [unit u3-cdk-synth] — 手元の go1.27.0 の標準ライブラリに govulncheck の指摘が残る(呼ばれているモジュールの脆弱性は 0 件)。公開の Go の下限(1.27.3)を上げるかは U3 の外で決める。-race と 3つの OS の py-minimal の synth は PR の CI で確かめる。
