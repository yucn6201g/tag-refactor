<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
- 2026-10-24T06:30:19Z [unit u4-plan] — Q3 の「HTML として働かせない」は、データを必ずコードスパンに入れることで満たすと読んだ; コードスパンの中で文字の参照に変えると、コピーした値が元と変わるため。
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
- 2026-10-24T06:30:19Z [unit u4-plan] — Windows の標準出力でも仮想端末の処理を U4 で有効にし、終了時に戻すことにした; U2 は標準エラーの進み具合のときだけ有効にしていたため。
- 2026-10-24T00:10:00Z [unit u3-rules] — PowerShell 5.1 のリダイレクトが書く UTF-16 のルールファイルを読めるようにする要件(NFR9.5)を足した; 下書きを Windows でそのまま保存して使う主要な流れを壊さないため。
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
- 2026-10-24T06:30:19Z [unit u4-plan] — 読み込みの上限を 256 MiB とし、超える変更案は保存しない; 最悪の大きさ(500 MiB 以上)まで受けるとメモリが数 GB になるため。
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
- 2026-10-24T06:30:19Z [unit u4-plan] — コード生成の計画では、色の条件を機能設計 BR3.7 ではなく NFR11.7(TERM=dumb と Windows の仮想端末の有効化を含む)から取る。
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
