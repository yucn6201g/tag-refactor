<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
- 2026-10-22T12:30:55Z [unit u2-scan] — AWS Config の全種類記録の判定に includeGlobalResourceTypes を使わない; 既定の設定でも false で、2022年2月以降のリージョンでは true にできないため。グローバルな IAM の種類は Config の経路の検出の対象外とし、Checked の行で示す。
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
- 2026-10-24T00:29:16Z [unit u3-rules] — rules suggest に --output を足した; PowerShell のリダイレクトはコンソールのコードページで子の出力を読むため、日本語の値が保存で文字化けするから。
- 2026-10-22T12:30:55Z [unit u2-scan] — Resource Explorer の GetView を操作に足した; 既定のビューの絞り込みの条件を確かめるため。承認済みの NFR 要件と機能設計の操作の表を同時に直した。
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
- 2026-10-24T00:29:16Z [unit u3-rules] — 文字コードの見分けと大きさの判定を Parse の先頭にまとめた; ファジングの対象と一致させ、すべての経路に届かせるため。
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->

