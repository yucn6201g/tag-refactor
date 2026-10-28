<!-- INVARIANT: examples are single-line HTML comments so a fresh template parses to total=0 (MEMORY_EMPTY). Do NOT un-comment or split across lines. t100 guards this. -->
> This file is kept up to date automatically while the stage runs. Add observations at the review step, not by editing here directly.

## Interpretations
- 2026-10-28T07:12:48Z [unit u5-impact] — 計画の「結果は code-summary.md に書く」は、作る人ではなく取りまとめが Step 5 で書くものと読んだ(作る人には aidlc/ の書き込みを計画のチェックだけに限ったため)。
<!-- example: 2026-05-29T10:14:32Z — chose REST over GraphQL; the consuming team only needs CRUD, revisit if subscriptions land -->

## Deviations
- 2026-10-28T07:12:48Z [unit u5-impact] — ワイルドカードの使い回しに 4,096 バイト・4,096 文字の守りを足し、インラインポリシーの一覧を読めないときの policyRef をユーザー・ロール・グループの ARN にした。作る人がシェルでファイルを書いた場面が3回あった。
- 2026-10-23T14:05:15Z [unit u2-scan] — scan が required のキーを受け取るため internal/rules に RequiredTags を足した; 設計では変えない部品だったが、契約 C8 の requiredTagRules に沿う追加として扱った。
<!-- example: 2026-05-29T10:14:32Z — skipped the optional caching layer the stage prose suggested; the dataset is small enough that it adds risk -->

## Tradeoffs
- 2026-10-28T07:12:48Z [unit u5-impact] — U2 の性能テストの呼び出し回数の上限に、U5 の空のアカウントの分を足した; 読む操作が増えた分で、下限を緩めたのではない。
- 2026-10-23T14:05:15Z [unit u2-scan] — レビューは READY だったが、2回の試行で見つかった中程度の不具合2件(不完全な scan の No issues found、集約の索引の失敗後のリージョンの帰属)をチェックポイントの前に直した; 誤った結果を利用者に見せないため。
<!-- example: 2026-05-29T10:14:32Z — picked TDD over BDD this run; the team is unit-first and the domain is well-understood -->

## Open questions
- 2026-10-28T07:12:48Z [unit u5-impact] — レビューの Minor 4件(verify.sh の測定ループの終了コード1、jsonUnread の構造体変換、? の理由の表示、unit-test-instructions.md のアカウント ID)は次の手直しで扱う。検証用アカウントのスタックの更新と作成は人の許可を待つ。
- 2026-10-23T14:05:15Z [unit u2-scan] — 既定のビューがないときの GetDefaultView の実機の形、Config の問い合わせの受理、-race、go1.27.3 での govulncheck は未確認; チェックポイントと CI で確かめる。
<!-- example: 2026-05-29T10:14:32Z — confirm the retention window with compliance before the next stage hardens the schema -->
